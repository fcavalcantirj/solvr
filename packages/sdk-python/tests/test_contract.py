"""Contract tests: the SDK held to contract/openapi-examples.json.

The fixture holds the recorded examples of the served OpenAPI document, each held to the running
API by the backend. Each example is served from a local server and the SDK is held to it: the
request it sends (method, path, query, headers, credential, body), what it surfaces (the answer,
or the API's error code), and its types (every example request field is a value of the method's
parameter type, and every value the SDK surfaces is a value of its declared type).
"""

import enum
import json
import re
from contextlib import contextmanager
from dataclasses import asdict, dataclass, fields, is_dataclass
from http.server import BaseHTTPRequestHandler
from pathlib import Path
from typing import Any, Callable, Dict, Iterator, List, Literal, Optional, Tuple, Union
from typing import get_args, get_origin, get_type_hints
from urllib.parse import parse_qsl, quote, urlsplit

import pytest

from solvr import RoomStreamEvent, Solvr, SolvrError

from .local_server import read_body, send_body, serve

FIXTURE = Path(__file__).resolve().parents[3] / "contract" / "openapi-examples.json"

AGENT_KEY = "solvr_contract_agent_key"
ROOM_TOKEN = "solvr_rt_contract_room_token"
DEAD_TOKEN = "solvr_rt_contract_not_live"


def load_contract() -> List[Dict[str, Any]]:
    operations = json.loads(FIXTURE.read_text(encoding="utf-8")).get("operations") or []
    if not operations:
        raise AssertionError("the client contract lists no operation")
    return operations


OPERATIONS = {op["operation_id"]: op for op in load_contract()}
# getReply answers the ETag the updateReply example sends back as If-Match (the read before the edit).
ETAG = OPERATIONS.get("updateReply", {}).get("headers", {}).get("If-Match", "")


def method_name(operation_id: str) -> str:
    """The SDK method of an operation: its operationId in snake_case."""
    return re.sub(r"(?<!^)(?=[A-Z])", "_", operation_id).lower()


class Call:
    """One example as the SDK caller sees it."""

    def __init__(self, op: Dict[str, Any], req: Dict[str, Any]):
        self.op = op
        self.req = req

    def path(self, name: str) -> str:
        if name not in self.req["path_params"]:
            raise AssertionError(f"{self.op['operation_id']}: the example has no path parameter {name}")
        return str(self.req["path_params"][name])

    def query(self, name: str) -> Any:
        """A query parameter (a str), or None."""
        return self.req["query"].get(name)

    def query_int(self, name: str) -> Optional[int]:
        value = self.query(name)
        return None if value is None else int(value)

    def header(self, name: str) -> Any:
        """A request header (a str), or None."""
        return self.req["headers"].get(name)

    def body(self) -> Dict[str, Any]:
        """The example's request body, passed to the method as keyword arguments."""
        body = self.req["request_body"]
        if body is None:
            return {}
        assert isinstance(body, dict), f"{self.op['operation_id']}: the request body is not an object"
        return body


def first_stream_event(client: Solvr, x: Call) -> Optional[RoomStreamEvent]:
    with client.stream_room(
        x.path("slug"),
        last_event_id=x.header("Last-Event-ID"),
        ticket=x.query("ticket"),
        type=x.query("type"),
        issue=x.query("issue"),
    ) as stream:
        return stream.next()


# Each caller calls an operation through the SDK with the example's inputs and returns what the
# SDK surfaces. The key is the operationId; the method is method_name(operationId), and a request
# body reaches it as keyword arguments named after the body's fields.
CALLERS: Dict[str, Callable[[Solvr, Call], Any]] = {
    "createPost": lambda c, x: c.create_post(**x.body()),
    "getPost": lambda c, x: c.get_post(x.path("id")),
    "search": lambda c, x: c.search(
        x.query("q") or "", limit=x.query_int("per_page"), page=x.query_int("page"), sort=x.query("sort")
    ),
    "createReply": lambda c, x: c.create_reply(x.path("id"), **x.body()),
    "listReplies": lambda c, x: c.list_replies(x.path("id"), cursor=x.query("cursor"), limit=x.query_int("limit")),
    "getReply": lambda c, x: c.get_reply(x.path("id")),
    "updateReply": lambda c, x: c.update_reply(x.path("id"), x.header("If-Match") or "", **x.body()),
    "createRoom": lambda c, x: c.create_room(**x.body()),
    "handshakeRoom": lambda c, x: c.handshake_room(x.path("slug"), **x.body()),
    "addRoomMember": lambda c, x: c.add_room_member(x.path("slug"), **x.body()),
    "listRoomMembers": lambda c, x: c.list_room_members(x.path("slug")),
    "listRoomEntries": lambda c, x: c.list_room_entries(
        x.path("slug"),
        cursor=x.query("cursor"),
        limit=x.query_int("limit"),
        kind=x.query("kind"),
        issue=x.query("issue"),
    ),
    "createRoomEntry": lambda c, x: c.create_room_entry(x.path("slug"), **x.body()),
    "createRoomStreamTicket": lambda c, x: c.create_room_stream_ticket(x.path("slug")),
    "streamRoom": first_stream_event,
}


@dataclass
class Recorded:
    method: str
    path: str
    query: List[Tuple[str, str]]
    headers: Dict[str, str]
    body: bytes


@contextmanager
def contract_server(op: Dict[str, Any], req: Dict[str, Any]) -> Iterator[Tuple[str, List[Recorded]]]:
    """Serves one example's answer and records what the SDK sent."""
    sent: List[Recorded] = []

    def handle(h: BaseHTTPRequestHandler) -> None:
        url = urlsplit(h.path)
        sent.append(Recorded(
            method=h.command,
            path=url.path,
            query=parse_qsl(url.query, keep_blank_values=True),
            headers={name.lower(): value for name, value in h.headers.items()},
            body=read_body(h),
        ))
        headers = {"ETag": ETAG} if op["operation_id"] == "getReply" and ETAG else {}
        if req["status"] < 400 and op["response_media_type"] == "text/event-stream":
            send_body(h, req["status"], "text/event-stream", req["response_body"].encode("utf-8"), headers)
            return
        send_body(h, req["status"], "application/json", json.dumps(req["response_body"]).encode("utf-8"), headers)

    with serve(handle) as url:
        yield url, sent


def client_for(op: Dict[str, Any], credential: str, base_url: str) -> Tuple[Solvr, Optional[str]]:
    """The SDK client presenting the example's credential, and the Authorization it must send."""
    if credential == "invalid":
        if op["credential"] != "room_token":
            raise AssertionError(f"{op['operation_id']}: an invalid {op['credential']} has no SDK case yet")
        return Solvr(api_key=AGENT_KEY, base_url=base_url, retries=1).with_room_token(DEAD_TOKEN), f"Bearer {DEAD_TOKEN}"
    if credential == "none":
        return Solvr(api_key=None, base_url=base_url, retries=1), None
    if credential == "agent_api_key":
        return Solvr(api_key=AGENT_KEY, base_url=base_url, retries=1), f"Bearer {AGENT_KEY}"
    if credential == "room_token":
        return Solvr(api_key=AGENT_KEY, base_url=base_url, retries=1).with_room_token(ROOM_TOKEN), f"Bearer {ROOM_TOKEN}"
    raise AssertionError(f"{op['operation_id']}: unknown credential {credential}")


def expected_path(op: Dict[str, Any], params: Dict[str, str]) -> str:
    path = str(op["path"])
    for name, value in params.items():
        path = path.replace("{" + name + "}", quote(value, safe=""))
    assert "{" not in path, f"{op['operation_id']}: path {path} keeps a variable the example gives no value"
    return path


def same_value(want: Any, got: Any) -> bool:
    if isinstance(want, bool) or isinstance(got, bool):
        return type(want) is type(got) and want == got
    if isinstance(want, (int, float)) and isinstance(got, (int, float)):
        return want == got
    return type(want) is type(got) and want == got


def json_diff(want: Any, got: Any, exact: bool, at: str = "$") -> str:
    """Compares two JSON values: a field the example has must be in got with the same value (null and
    absent agree); exact also fails on a field only got has."""
    if isinstance(want, dict):
        if not isinstance(got, dict):
            return f"{at}: want an object, got {got!r}"
        for key in sorted(want):
            if want[key] is None and got.get(key) is None:
                continue
            if key not in got or got[key] is None:
                return f"{at}.{key}: the example has it, the SDK lost it"
            diff = json_diff(want[key], got[key], exact, f"{at}.{key}")
            if diff:
                return diff
        if exact:
            for key in got:
                if key not in want and got[key] is not None:
                    return f"{at}.{key}: the SDK sent it, the example does not"
        return ""
    if isinstance(want, list):
        if not isinstance(got, list) or len(got) != len(want):
            return f"{at}: want {len(want)} items, got {got!r}"
        for i, (w, g) in enumerate(zip(want, got)):
            diff = json_diff(w, g, exact, f"{at}[{i}]")
            if diff:
                return diff
        return ""
    return "" if same_value(want, got) else f"{at}: want {want!r}, got {got!r}"


def type_error(hint: Any, value: Any, at: str = "$") -> str:
    """Why value is not a value of the type hint, or "" when it is."""
    if hint is Any:
        return ""
    origin = get_origin(hint)
    if origin is Union:
        args = get_args(hint)
        if value is None:
            return "" if type(None) in args else f"{at}: None, but {hint} is not Optional"
        errors = [type_error(arg, value, at) for arg in args if arg is not type(None)]
        return "" if "" in errors else errors[0]
    if value is None:
        return f"{at}: None, but {hint} is not Optional"
    if origin is Literal:
        return "" if value in get_args(hint) else f"{at}: {value!r} is not one of {get_args(hint)}"
    if origin is list:
        if not isinstance(value, list):
            return f"{at}: {value!r} is not a list"
        (item,) = get_args(hint)
        return next((e for e in (type_error(item, v, f"{at}[{i}]") for i, v in enumerate(value)) if e), "")
    if origin is dict:
        if not isinstance(value, dict):
            return f"{at}: {value!r} is not a dict"
        item = get_args(hint)[1]
        return next((e for e in (type_error(item, v, f"{at}.{k}") for k, v in value.items()) if e), "")
    if isinstance(hint, type) and is_dataclass(hint):
        if not isinstance(value, hint):
            return f"{at}: {value!r} is not a {hint.__name__}"
        hints = get_type_hints(hint)
        for f in fields(hint):
            error = type_error(hints[f.name], getattr(value, f.name), f"{at}.{f.name}")
            if error:
                return error
        return ""
    if hint is bool:
        return "" if isinstance(value, bool) else f"{at}: {value!r} is not a bool"
    if hint is int:
        return "" if isinstance(value, int) and not isinstance(value, bool) else f"{at}: {value!r} is not an int"
    if hint is float:
        ok = isinstance(value, (int, float)) and not isinstance(value, bool)
        return "" if ok else f"{at}: {value!r} is not a float"
    if isinstance(hint, type) and issubclass(hint, enum.Enum):
        return "" if value in {m.value for m in hint} or isinstance(value, hint) else f"{at}: {value!r} not in {hint}"
    if isinstance(hint, type):
        return "" if isinstance(value, hint) else f"{at}: {value!r} is not a {hint.__name__}"
    return f"{at}: no check for the type {hint}"


def surfaced(result: Any) -> Dict[str, Any]:
    """What the SDK surfaced, in the answer's shape: a method answers the answer's data, a page or
    a write the API answers with meta answers both, and a list answered as data alone (RoomMemberList)
    answers that wrapper."""
    value = asdict(result)
    if "data" in value and ("meta" in value or set(value) == {"data"}):
        return value
    return {"data": value}


def check_request_types(op: Dict[str, Any], req: Dict[str, Any]) -> None:
    """Every request body field is a parameter of the method, and its value a value of that type."""
    name = method_name(op["operation_id"])
    hints = get_type_hints(getattr(Solvr, name))
    for key, value in (req["request_body"] or {}).items():
        assert key in hints, f"{op['operation_id']}: {key} is not a parameter of Solvr.{name}"
        assert type_error(hints[key], value, f"$.{key}") == "", f"{op['operation_id']}: request field {key}"


def check_request(op: Dict[str, Any], req: Dict[str, Any], sent: List[Recorded], auth: Optional[str]) -> None:
    """Holds what the SDK sent to the example."""
    op_id = op["operation_id"]
    assert len(sent) == 1, f"{op_id}: requests sent {sent}"
    got = sent[0]
    assert got.method == op["method"], f"{op_id}: method"
    assert got.path == expected_path(op, req["path_params"]), f"{op_id}: path"
    assert got.headers.get("authorization") == auth, f"{op_id}: Authorization"
    names = [name for name, _ in got.query]
    assert len(set(names)) == len(names), f"{op_id}: a query parameter sent twice: {names}"
    assert dict(got.query) == req["query"], f"{op_id}: query"
    for name, value in req["headers"].items():
        assert got.headers.get(name.lower()) == value, f"{op_id}: header {name}"
    for name in ("If-Match", "Last-Event-ID"):
        if name not in req["headers"]:
            assert name.lower() not in got.headers, f"{op_id}: sent {name} the example does not"
    if req["request_body"] is None:
        assert got.body == b"", f"{op_id}: sent a body the example does not: {got.body!r}"
        return
    assert got.body != b"", f"{op_id}: the SDK sent no body"
    diff = json_diff(req["request_body"], json.loads(got.body), exact=True)
    assert diff == "", f"{op_id}: request body {got.body!r}; example {req['request_body']}"


def first_frame(text: str) -> Dict[str, str]:
    """The id, event and data of an event-stream example's first event."""
    frame: Dict[str, str] = {}
    for line in text.split("\n"):
        if line == "" and "data" in frame:
            break
        name, _, value = line.partition(": ")
        if name in ("id", "event", "data"):
            frame[name] = value
    return frame


def check_result(op: Dict[str, Any], result: Any) -> None:
    """Holds what the SDK surfaced to the example's answer, and to its own types."""
    op_id = op["operation_id"]
    assert type_error(type(result), result) == "", f"{op_id}: {type_error(type(result), result)}"
    if op["response_media_type"] == "text/event-stream":
        want = first_frame(op["response_body"])
        assert isinstance(result, RoomStreamEvent), f"{op_id}: the SDK read no event: {result!r}"
        assert (result.id, result.event) == (want["id"], want["event"]), f"{op_id}: event id and name"
        frame = json.loads(want["data"])
        assert json_diff(frame, asdict(result.frame), False) == "", f"{op_id}: the SDK frame"
        assert result.message is not None, f"{op_id}: the SDK decoded no message"
        assert json_diff(frame["payload"], asdict(result.message), False) == "", f"{op_id}: the SDK message"
        return
    diff = json_diff(op["response_body"], surfaced(result), False)
    assert diff == "", f"{op_id}: the SDK result {surfaced(result)}"
    if op_id == "getReply":
        assert result.etag == ETAG, "getReply: the ETag the edit sends back"


def test_every_operation_is_a_client_method_named_after_its_operation_id() -> None:
    assert sorted(CALLERS) == sorted(OPERATIONS), "every contract operation has a caller, and no caller is extra"
    for operation_id in OPERATIONS:
        name = method_name(operation_id)
        assert callable(getattr(Solvr, name, None)), f"{operation_id}: Solvr.{name} is missing"


@pytest.mark.parametrize("operation_id", sorted(OPERATIONS))
def test_each_example_is_what_the_sdk_sends_and_surfaces(operation_id: str) -> None:
    op = OPERATIONS[operation_id]
    check_request_types(op, op)
    with contract_server(op, op) as (url, sent):
        client, auth = client_for(op, op["credential"], url)
        result = CALLERS[operation_id](client, Call(op, op))
    check_request(op, op, sent, auth)
    check_result(op, result)


ERROR_CASES = [
    (operation_id, index)
    for operation_id, op in sorted(OPERATIONS.items())
    for index in range(len(op.get("errors") or []))
]


def test_the_contract_has_error_examples() -> None:
    assert ERROR_CASES, "the contract records no error example: the error test below would hold nothing"


@pytest.mark.parametrize("operation_id,index", ERROR_CASES)
def test_each_error_example_surfaces_the_api_error_code(operation_id: str, index: int) -> None:
    op = OPERATIONS[operation_id]
    req = op["errors"][index]
    check_request_types(op, req)
    with contract_server(op, req) as (url, sent):
        client, auth = client_for(op, req["credential"], url)
        with pytest.raises(SolvrError) as raised:
            CALLERS[operation_id](client, Call(op, req))
    check_request(op, req, sent, auth)
    want = req["response_body"]["error"]
    error = raised.value
    assert error.status == req["status"]
    assert error.code == want["code"]
    assert error.message == want["message"]
    assert error.request_id == want.get("request_id")
    assert error.details == want.get("details")


def test_json_diff_catches_a_lost_field_a_value_and_an_extra_request_field() -> None:
    want = {"a": 1, "b": {"c": [1, 2]}, "n": None}
    assert json_diff(want, {"a": 1, "b": {"c": [1, 2]}, "extra": True}, False) == ""
    assert json_diff(want, {"b": {"c": [1, 2]}}, False) == "$.a: the example has it, the SDK lost it"
    assert json_diff(want, {"a": 2, "b": {"c": [1, 2]}}, False) == "$.a: want 1, got 2"
    assert json_diff(want, {"a": 1, "b": {"c": [1]}}, False) == "$.b.c: want 2 items, got [1]"
    assert json_diff(want, {"a": True, "b": {"c": [1, 2]}}, False) == "$.a: want 1, got True"
    assert json_diff(want, {"a": 1, "b": {"c": [1, 2]}, "x": 0}, True) == "$.x: the SDK sent it, the example does not"
    assert json_diff(want, {"a": 1, "b": {"c": [1, 2]}, "x": None}, True) == ""


@dataclass
class Probe:
    name: str
    count: int
    kind: Literal["a", "b"]
    tags: List[str]
    note: Optional[str] = None


def test_type_error_catches_a_wrong_type_a_forbidden_none_and_a_literal() -> None:
    assert type_error(Probe, Probe("n", 1, "a", ["t"])) == ""
    assert type_error(Probe, Probe("n", "1", "a", ["t"])) == "$.count: '1' is not an int"  # type: ignore[arg-type]
    assert type_error(Probe, Probe(None, 1, "a", ["t"])) == "$.name: None, but <class 'str'> is not Optional"  # type: ignore[arg-type]
    assert type_error(Probe, Probe("n", 1, "c", ["t"])) == "$.kind: 'c' is not one of ('a', 'b')"  # type: ignore[arg-type]
    assert type_error(Probe, Probe("n", 1, "a", [2])) == "$.tags[0]: 2 is not a str"  # type: ignore[list-item]
    assert type_error(Probe, Probe("n", True, "a", ["t"])) == "$.count: True is not an int"
