#!/usr/bin/env bash
#
# test_contract.sh - Contract test of solvr.sh against contract/openapi-examples.json.
#
# The examples are the recorded requests and answers of the 15 operations every Solvr client
# shares (the SDKs, both CLIs, the MCP servers and this skill). A local HTTP server bound to
# 127.0.0.1 answers each example as the API did and records the request solvr.sh sent. Every
# operation must have a command that sends exactly the example's request (method, escaped
# path, query, credential, If-Match / Last-Event-ID, body) and shows what the API answered;
# every recorded error must come back as the API's code, message and request id.
#
# No network beyond 127.0.0.1, never production. Needs python3 and jq.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOLVR_SH="${SCRIPT_DIR}/solvr.sh"
CONTRACT="${SCRIPT_DIR}/../../contract/openapi-examples.json"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

PASSED=0
FAILED=0

WORK=$(mktemp -d)
SERVER_PID=""
cleanup() {
    if [ -n "$SERVER_PID" ]; then
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
    rm -rf "$WORK"
}
trap cleanup EXIT

KEY="solvr_contract_agent_key"

cat > "$WORK/server.py" <<'PY'
# Answers each request with the example the test selected in <work>/scenario ("<operationId> ok"
# or "<operationId> <error index>") and appends the request to <work>/requests.jsonl.
import json, sys, time
from http.server import ThreadingHTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlsplit, parse_qsl

contract, work = json.load(open(sys.argv[1])), sys.argv[2]
ops = {o["operation_id"]: o for o in contract["operations"]}
etag = ops["updateReply"]["headers"]["If-Match"]

class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def answer(self):
        length = int(self.headers.get("Content-Length") or 0)
        body = self.rfile.read(length).decode() if length else None
        url = urlsplit(self.path)
        record = {"method": self.command, "path": url.path,
                  "query": dict(parse_qsl(url.query, keep_blank_values=True)),
                  "authorization": self.headers.get("Authorization"),
                  "if_match": self.headers.get("If-Match"),
                  "last_event_id": self.headers.get("Last-Event-ID"),
                  "accept": self.headers.get("Accept"), "body": body}
        with open(work + "/requests.jsonl", "a") as f:
            f.write(json.dumps(record) + "\n")
        op_id, case = open(work + "/scenario").read().split()
        op = ops[op_id]
        if op_id == "streamRoom" and case == "rotated":
            # An explicit rotation replaced the token the stream was opened with.
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            self.wfile.write(b'event: credential_rotated\ndata: {"code":"CREDENTIAL_ROTATED","message":"this room token was replaced by a rotation"}\n\n')
            self.wfile.flush()
            time.sleep(20)
            return
        example = op if case == "ok" else op["errors"][int(case)]
        if op_id == "streamRoom" and case == "ok":
            # The example's frames, then the stream stays open and silent like a quiet room.
            self.send_response(example["status"])
            self.send_header("Content-Type", "text/event-stream")
            self.end_headers()
            self.wfile.write(example["response_body"].encode())
            self.wfile.flush()
            time.sleep(20)
            return
        raw = json.dumps(example["response_body"]).encode()
        self.send_response(example["status"])
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        if case == "ok" and op_id in ("getReply", "updateReply"):
            self.send_header("ETag", etag)
        self.end_headers()
        self.wfile.write(raw)

    do_GET = do_POST = do_PATCH = do_PUT = do_DELETE = answer

server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
server.daemon_threads = True
with open(work + "/port.tmp", "w") as f:
    f.write(str(server.server_address[1]))
import os
os.rename(work + "/port.tmp", work + "/port")
server.serve_forever()
PY

cat > "$WORK/check.py" <<'PY'
# check.py CONTRACT OP CASE WORK AUTH MODE CODE [EXPECT...]: prints one line per problem with
# the request solvr.sh sent (held to the example) and what it printed (held to the answer).
import json, sys
from urllib.parse import quote

contract, op_id, case, work, auth, mode, code = sys.argv[1:8]
expect = sys.argv[8:]
op = {o["operation_id"]: o for o in json.load(open(contract))["operations"]}[op_id]
example = op if case == "ok" else op["errors"][int(case)]
etag = {o["operation_id"]: o for o in json.load(open(contract))["operations"]}["updateReply"]["headers"]["If-Match"]
problems = []

lines = [l for l in open(work + "/requests.jsonl").read().splitlines() if l]
if len(lines) != 1:
    problems.append("sent %d requests, want exactly 1: %s" % (len(lines), lines))
else:
    sent = json.loads(lines[0])
    path = op["path"]
    for name, value in example["path_params"].items():
        path = path.replace("{%s}" % name, quote(value, safe=""))
    if sent["method"] != op["method"]:
        problems.append("method %s, want %s" % (sent["method"], op["method"]))
    if sent["path"] != path:
        problems.append("path %s, want %s" % (sent["path"], path))
    if sent["query"] != example["query"]:
        problems.append("query %s, want %s" % (sent["query"], example["query"]))
    want_auth = None if auth == "-" else "Bearer " + auth
    if sent["authorization"] != want_auth:
        problems.append("Authorization %r, want %r" % (sent["authorization"], want_auth))
    for header, field in (("If-Match", "if_match"), ("Last-Event-ID", "last_event_id")):
        if sent[field] != example["headers"].get(header):
            problems.append("%s %r, want %r" % (header, sent[field], example["headers"].get(header)))
    if example["request_body"] is None:
        if sent["body"]:
            problems.append("sent a body %r, want none" % sent["body"])
    else:
        try:
            body = json.loads(sent["body"] or "")
        except ValueError:
            body = sent["body"]
        if body != example["request_body"]:
            problems.append("body %s, want %s" % (json.dumps(body), json.dumps(example["request_body"])))
    if op_id == "streamRoom" and sent["accept"] != "text/event-stream":
        problems.append("Accept %r, want text/event-stream" % sent["accept"])

out = open(work + "/out").read()
err = open(work + "/err").read()

def frames(text):
    found, event, data, ident = [], "message", [], ""
    for line in text.split("\n"):
        if line == "":
            if data:
                found.append({"id": ident, "event": event, "frame": json.loads("\n".join(data))})
            event, data = "message", []
        elif line.startswith("id:"):
            ident = line[3:].strip()
        elif line.startswith("event:"):
            event = line[6:].strip()
        elif line.startswith("data:"):
            data.append(line[5:].lstrip(" "))
    return found

if case != "ok":
    error = example["response_body"]["error"]
    if code != "1":
        problems.append("exit %s, want 1" % code)
    if out.strip():
        problems.append("stdout %r, want nothing on an error" % out)
    if mode == "json":
        try:
            if json.loads(err) != example["response_body"]:
                problems.append("stderr %r, want the API's error answer %s" % (err, json.dumps(example["response_body"])))
        except ValueError:
            problems.append("stderr %r is not the API's error answer" % err)
    else:
        for text in ("%s: %s" % (error["code"], error["message"]), "request id: " + error["request_id"]):
            if text not in err:
                problems.append("stderr %r does not show %r" % (err, text))
else:
    if code != "0":
        problems.append("exit %s, want 0 (stderr %r)" % (code, err))
    if err.strip():
        problems.append("stderr %r, want nothing" % err)
    if mode == "json":
        if op_id == "streamRoom":
            try:
                printed = [json.loads(l) for l in out.splitlines() if l]
            except ValueError:
                printed = out
            if printed != frames(example["response_body"]):
                problems.append("printed %r, want one {id, event, frame} line per event %s" % (out, json.dumps(frames(example["response_body"]))))
        else:
            want = json.loads(json.dumps(example["response_body"]))
            if op_id in ("getReply", "updateReply"):
                want["data"]["etag"] = etag
            try:
                if json.loads(out) != want:
                    problems.append("printed %s, want the answer %s" % (out, json.dumps(want)))
            except ValueError:
                problems.append("printed %r, not the API's answer" % out)
    else:
        for text in expect:
            if text not in out:
                problems.append("output does not show %r:\n%s" % (text, out))

print("\n".join(problems))
PY

python3 "$WORK/server.py" "$CONTRACT" "$WORK" &
SERVER_PID=$!
for _ in $(seq 1 50); do [ -f "$WORK/port" ] && break; sleep 0.1; done
[ -f "$WORK/port" ] || { echo "the contract server did not start"; exit 1; }
BASE="http://127.0.0.1:$(cat "$WORK/port")/v1"

# ex OP JQ - a value from the operation's example.
ex() { jq -r --arg op "$1" ".operations[] | select(.operation_id == \$op) | $2" "$CONTRACT"; }

ROOM_TOKEN=$(ex handshakeRoom .response_body.data.room_token)
INVALID_TOKEN="solvr_rt_not-a-live-token"

# run_solvr KEY|NOKEY ARGS... - runs solvr.sh against the server (15 s watchdog); sets
# $code and $elapsed and leaves stdout/stderr in $WORK/out and $WORK/err. The config dir holds
# this agent's room token for the example room, as `room join` saves it.
run_solvr() {
    local with_key="$1"; shift
    local config="$WORK/config"
    if [ "${KEEP_CONFIG:-}" = 1 ]; then
        mkdir -p "$config"
    else
        rm -rf "$config"; mkdir -p "$config"
        [ "${NO_ROOM_TOKEN:-}" = 1 ] || jq -n --arg slug "$(ex handshakeRoom .path_params.slug)" --arg token "$ROOM_TOKEN" \
        '{($slug): {token: $token, created_at: "2026-10-01T00:00:00Z"}}' > "$config/rooms.json"
    fi
    local started; started=$(date +%s)
    code=0
    (
        unset SOLVR_API_KEY SOLVR_ROOM_TOKEN
        [ "$with_key" = KEY ] && export SOLVR_API_KEY="$KEY"
        SOLVR_API_URL="$BASE" SOLVR_CONFIG_DIR="$config" exec "$SOLVR_SH" "$@"
    ) > "$WORK/out" 2> "$WORK/err" &
    local pid=$!
    ( sleep 15; kill "$pid" 2>/dev/null ) >/dev/null 2>&1 &
    local dog=$!
    wait "$pid" || code=$?
    kill "$dog" 2>/dev/null || true
    elapsed=$(( $(date +%s) - started ))
}

record() {
    local name="$1" problems="$2"
    if [ -z "$problems" ]; then
        echo -e "${GREEN}PASS${NC} ${name}"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}FAIL${NC} ${name}"
        echo "$problems" | sed 's/^/    /'
        FAILED=$((FAILED + 1))
    fi
}

# contract OP CASE KEY|NOKEY AUTH EXPECT -- ARGS...: runs the command twice (--json and human)
# against the example and holds both runs to it. AUTH is the bearer the request must carry
# ("-" for none); EXPECT is a "|"-separated list of texts the human output must show.
contract() {
    local op="$1" case="$2" with_key="$3" auth="$4" expect="$5"
    shift 6
    local mode problems
    for mode in json human; do
        echo "$op $case" > "$WORK/scenario"
        : > "$WORK/requests.jsonl"
        if [ "$mode" = json ]; then run_solvr "$with_key" "$@" --json; else run_solvr "$with_key" "$@"; fi
        # shellcheck disable=SC2086
        problems=$(IFS='|'; python3 "$WORK/check.py" "$CONTRACT" "$op" "$case" "$WORK" "$auth" "$mode" "$code" $expect)
        record "${op} ${case} (${mode}): solvr.sh $*" "$problems"
    done
}

echo "=== solvr.sh against contract/openapi-examples.json ==="

POST_ID=$(ex getPost .path_params.id)
REPLY_ID=$(ex getReply .path_params.id)
SLUG=$(ex createRoom .request_body.slug)

contract createPost ok KEY "$KEY" "$(ex createPost .response_body.data.id)|$(ex createPost .request_body.title)" -- \
    post "$(ex createPost .request_body.title)" "$(ex createPost .request_body.description)" \
    --tags "$(ex createPost '.request_body.tags | join(",")')"
contract createReply ok KEY "$KEY" "$(ex createReply .response_body.data.id)|$POST_ID" -- \
    reply "$(ex createReply .path_params.id)" "$(ex createReply .request_body.body)"
contract getPost ok NOKEY - "$POST_ID|$(ex getPost .response_body.data.title)" -- get "$POST_ID"
contract listReplies ok NOKEY - "$REPLY_ID|$(ex listReplies '.response_body.data[0].body')" -- \
    replies "$POST_ID" --limit "$(ex listReplies .query.limit)"
contract getReply ok NOKEY - "$REPLY_ID|$(ex getReply .response_body.data.body)|ETag: $(ex updateReply '.headers["If-Match"]')" -- \
    get-reply "$REPLY_ID"
contract updateReply ok KEY "$KEY" "$REPLY_ID|$(ex updateReply .response_body.data.body)" -- \
    update-reply "$REPLY_ID" --if-match "$(ex updateReply '.headers["If-Match"]')" --body "$(ex updateReply .request_body.body)"
contract search ok NOKEY - "$POST_ID|$(ex search '.response_body.data[0].title')" -- \
    search "$(ex search .query.q)" --limit "$(ex search .query.per_page)" --sort "$(ex search .query.sort)"
contract createRoom ok KEY "$KEY" "$SLUG|solvr.sh room join $SLUG" -- \
    room create "$(ex createRoom .request_body.display_name)" --slug "$SLUG" \
    --description "$(ex createRoom .request_body.description)" --tags "$(ex createRoom '.request_body.tags | join(",")')"
contract handshakeRoom ok KEY "$KEY" "$(ex handshakeRoom .response_body.data.agent_id)|$SLUG" -- room join "$SLUG"
# member ROW_JQ - a participant of the listRoomMembers example as `room members` shows it.
member() { ex listRoomMembers "$1 | \"  \\(.agent_id) \\(.role) (added by \\(.added_by), since \\(.created_at))\""; }
contract addRoomMember ok KEY "$KEY" "$(ex addRoomMember '.response_body.data | "\(.agent_id) is in '"$SLUG"' as \(.role) (added by \(.added_by))"')|solvr.sh room join $SLUG" -- \
    room add-member "$SLUG" "$(ex addRoomMember .request_body.agent_id)"
contract listRoomMembers ok KEY "$KEY" "$(ex listRoomMembers '.response_body.data | length') participants of $SLUG:|$(member '.response_body.data[0]')|$(member '.response_body.data[1]')" -- \
    room members "$SLUG"
contract createRoomEntry ok KEY "$ROOM_TOKEN" "$(ex createRoomEntry .response_body.data.id)|$SLUG" -- \
    room send "$SLUG" "$(ex createRoomEntry .request_body.body)" --client-entry-id "$(ex createRoomEntry .request_body.client_entry_id)"
contract listRoomEntries ok KEY "$ROOM_TOKEN" "$(ex listRoomEntries '.response_body.data[0].id')|$(ex listRoomEntries '.response_body.data[0].body')" -- \
    room read "$SLUG" --limit "$(ex listRoomEntries .query.limit)"
contract createRoomStreamTicket ok KEY "$ROOM_TOKEN" "$(ex createRoomStreamTicket .response_body.data.ticket)|$(ex createRoomStreamTicket .response_body.data.expires_at)" -- \
    room ticket "$SLUG"
contract streamRoom ok KEY "$ROOM_TOKEN" "1043|Parser built and its tests pass; ready for review." -- \
    room watch "$SLUG" --last-event-id "$(ex streamRoom '.headers["Last-Event-ID"]')" --max 1
record "room watch --max 1 returns while the server keeps the stream open (${elapsed}s)" \
    "$([ "$elapsed" -lt 10 ] || echo "took ${elapsed}s")"

echo ""
echo "=== recorded errors: the API's code, message and request id ==="
contract createRoomEntry 0 KEY "$INVALID_TOKEN" "" -- \
    room send "$SLUG" "$(ex createRoomEntry '.errors[0].request_body.body')" --token "$INVALID_TOKEN"
contract getPost 0 NOKEY - "" -- get "$(ex getPost '.errors[0].path_params.id')"
contract search 0 NOKEY - "" -- search ""
contract streamRoom 0 KEY - "" -- room watch "$SLUG" --ticket "$(ex streamRoom '.errors[0].query.ticket')"
contract updateReply 0 KEY "$KEY" "" -- \
    update-reply "$REPLY_ID" --if-match "$(ex updateReply '.errors[0].headers["If-Match"]')" --body "$(ex updateReply '.errors[0].request_body.body')"
contract addRoomMember 0 KEY "$KEY" "" -- room add-member "$SLUG" "$(ex addRoomMember '.errors[0].request_body.agent_id')"

echo ""
echo "=== room credentials ==="
# scenario OP CASE - the example the server answers next, with an empty request log.
scenario() { echo "$1 $2" > "$WORK/scenario"; : > "$WORK/requests.jsonl"; }
sent() { jq -r "$1" "$WORK/requests.jsonl"; }

scenario handshakeRoom ok
NO_ROOM_TOKEN=1 run_solvr KEY room join "$SLUG"
scenario createRoomEntry ok
KEEP_CONFIG=1 run_solvr KEY room send "$SLUG" "after the join"
record "room send presents the token room join saved, not the API key" \
    "$([ "$code" -eq 0 ] && [ "$(sent .authorization)" = "Bearer $ROOM_TOKEN" ] || echo "exit $code, Authorization $(sent .authorization)")"

for sub in read send ticket watch; do
    scenario listRoomEntries ok
    NO_ROOM_TOKEN=1 run_solvr KEY room "$sub" "$SLUG" "body"
    record "room $sub without a room token fails before any request and points to room join" \
        "$([ "$code" -eq 1 ] && [ ! -s "$WORK/requests.jsonl" ] && grep -q "room join $SLUG" "$WORK/err" || echo "exit $code, requests $(cat "$WORK/requests.jsonl"), stderr $(cat "$WORK/err")")"
done

scenario streamRoom rotated
run_solvr KEY room watch "$SLUG"
record "room watch ends with exit 1 and the code when the stream ends on a rotation (${elapsed}s)" \
    "$([ "$code" -eq 1 ] && grep -q "CREDENTIAL_ROTATED: this room token was replaced" "$WORK/err" && [ "$elapsed" -lt 10 ] || echo "exit $code, stderr $(cat "$WORK/err")")"
run_solvr KEY room watch "$SLUG" --json
record "room watch --json prints the end as an error answer" \
    "$([ "$code" -eq 1 ] && [ "$(jq -r .error.code "$WORK/err" 2>/dev/null)" = CREDENTIAL_ROTATED ] && [ ! -s "$WORK/out" ] || echo "exit $code, stdout $(cat "$WORK/out"), stderr $(cat "$WORK/err")")"

echo ""
echo "=== every operation has a command ==="
covered="createPost createReply getPost listReplies getReply updateReply search createRoom handshakeRoom addRoomMember listRoomMembers createRoomEntry listRoomEntries createRoomStreamTicket streamRoom"
missing=""
for op in $(jq -r '.operations[].operation_id' "$CONTRACT"); do
    case " $covered " in *" $op "*) ;; *) missing="$missing $op" ;; esac
done
record "every operation in the contract is exercised" "$([ -z "$missing" ] || echo "no command for:$missing")"

echo ""
echo "Passed: ${PASSED}  Failed: ${FAILED}"
[ "$FAILED" -eq 0 ]
