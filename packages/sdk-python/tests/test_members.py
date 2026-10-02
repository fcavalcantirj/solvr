"""A room's participants are a collection the owner reads (list_room_members) and adds to
(add_room_member): a third and any later agent is admitted to the SAME room, then joins it with
its own handshake_room."""

import inspect
import json
from contextlib import contextmanager
from dataclasses import MISSING, fields
from http.server import BaseHTTPRequestHandler
from typing import Any, Callable, Dict, Iterator, List, Literal, Optional, Tuple, get_type_hints

import pytest

import solvr
from solvr import Solvr, SolvrError

from .local_server import read_body, send_body, serve

ROOM = "7f8c2a7e-3d1b-4c55-9a51-0b6f1c2d3e4f"


def member(agent_id: str, role: str, created_at: str) -> Dict[str, str]:
    return {"room_id": ROOM, "agent_id": agent_id, "role": role, "added_by": "planner", "created_at": created_at}


@contextmanager
def answering(status: int, answer: Dict[str, Any]) -> Iterator[Tuple[str, List[Dict[str, Any]]]]:
    """Serves one JSON answer to every request; yields the base URL and what was sent."""
    sent: List[Dict[str, Any]] = []

    def handle(h: BaseHTTPRequestHandler) -> None:
        sent.append({"method": h.command, "path": h.path, "auth": h.headers.get("Authorization"), "body": read_body(h)})
        send_body(h, status, "application/json", json.dumps(answer).encode("utf-8"))

    with serve(handle) as url:
        yield url, sent


class TestListRoomMembers:
    def test_reads_the_participants_with_the_agent_key_owners_first_every_field(self) -> None:
        participants = [
            member("planner", "owner", "2026-10-02T11:00:00Z"),
            member("executor", "member", "2026-10-02T11:30:00Z"),
            member("reviewer", "member", "2026-10-02T12:00:00Z"),
        ]
        with answering(200, {"data": participants}) as (url, sent):
            result = Solvr(api_key="solvr_planner_key", base_url=url).list_room_members("handoff room/1")

        assert len(sent) == 1
        assert sent[0]["method"] == "GET"
        assert sent[0]["path"] == "/v1/rooms/handoff%20room%2F1/members"
        assert sent[0]["auth"] == "Bearer solvr_planner_key"
        assert sent[0]["body"] == b""
        assert isinstance(result, solvr.RoomMemberList)
        assert all(isinstance(m, solvr.RoomMember) for m in result.data)
        assert [vars(m) for m in result.data] == participants


class TestAddRoomMember:
    ADMITTED = {"data": member("reviewer", "member", "2026-10-02T12:00:00Z")}

    def test_admits_an_agent_without_a_role_the_body_is_the_agent_id_alone(self) -> None:
        with answering(201, self.ADMITTED) as (url, sent):
            result = Solvr(api_key="solvr_planner_key", base_url=url).add_room_member("handoff room/1", agent_id="reviewer")

        assert len(sent) == 1
        assert sent[0]["method"] == "POST"
        assert sent[0]["path"] == "/v1/rooms/handoff%20room%2F1/members"
        assert sent[0]["auth"] == "Bearer solvr_planner_key"
        assert json.loads(sent[0]["body"]) == {"agent_id": "reviewer"}
        assert isinstance(result, solvr.RoomMember)
        assert vars(result) == self.ADMITTED["data"]

    def test_sends_the_role_it_is_given(self) -> None:
        with answering(201, self.ADMITTED) as (url, sent):
            Solvr(api_key="solvr_planner_key", base_url=url).add_room_member("demo", agent_id="reviewer", role="owner")

        assert json.loads(sent[0]["body"]) == {"agent_id": "reviewer", "role": "owner"}


ERROR_CASES: List[Tuple[str, int, str, Callable[[Solvr], Any]]] = [
    ("a participant that is not an owner lists", 403, "FORBIDDEN", lambda c: c.list_room_members("demo")),
    ("an unknown agent is admitted", 400, "INVALID_AGENT", lambda c: c.add_room_member("demo", agent_id="ghost")),
    ("the last owner is demoted", 409, "LAST_OWNER",
     lambda c: c.add_room_member("demo", agent_id="planner", role="member")),
]


@pytest.mark.parametrize("name,status,code,call", ERROR_CASES, ids=[c[0] for c in ERROR_CASES])
def test_a_member_error_is_a_solvr_error_sent_once(
    name: str, status: int, code: str, call: Callable[[Solvr], Any]
) -> None:
    answer = {"error": {"code": code, "message": f"{code} message", "request_id": "req-7"}}
    with answering(status, answer) as (url, sent):
        with pytest.raises(SolvrError) as raised:
            call(Solvr(api_key="solvr_agent", base_url=url))

    error = raised.value
    assert (error.status, error.code, error.message, error.request_id) == (status, code, f"{code} message", "req-7")
    assert len(sent) == 1


# The member types carry the fields of the schemas the API publishes (backend
# openapi_member_paths.go RoomMember, RoomMemberList and AddRoomMemberRequest, pinned there to the
# stored model).
class TestMemberTypes:
    def test_room_member_has_every_field_of_the_published_room_member_each_required(self) -> None:
        declared = fields(solvr.RoomMember)
        assert sorted(f.name for f in declared) == ["added_by", "agent_id", "created_at", "role", "room_id"]
        assert [f.name for f in declared if f.default is not MISSING or f.default_factory is not MISSING] == []
        hints = get_type_hints(solvr.RoomMember)
        assert {name: hints[name] for name in ("room_id", "agent_id", "added_by", "created_at")} == {
            "room_id": str, "agent_id": str, "added_by": str, "created_at": str}
        assert hints["role"] == solvr.RoomRole

    def test_a_role_is_owner_or_member(self) -> None:
        assert solvr.RoomRole == Literal["owner", "member"]

    def test_the_list_is_the_published_envelope(self) -> None:
        assert [f.name for f in fields(solvr.RoomMemberList)] == ["data"]
        assert get_type_hints(solvr.RoomMemberList)["data"] == List[solvr.RoomMember]

    def test_add_room_member_takes_the_published_request_agent_id_and_an_optional_role(self) -> None:
        params = inspect.signature(Solvr.add_room_member).parameters
        assert list(params) == ["self", "slug", "agent_id", "role"]
        assert params["agent_id"].default is inspect.Parameter.empty
        assert params["role"].default is None
        hints = get_type_hints(Solvr.add_room_member)
        assert (hints["agent_id"], hints["role"], hints["return"]) == (str, Optional[solvr.RoomRole], solvr.RoomMember)
        assert get_type_hints(Solvr.list_room_members)["return"] == solvr.RoomMemberList
