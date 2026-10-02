#!/usr/bin/env bash
#
# test_members.sh - Tests of the room participant commands of solvr.sh.
#
# `room members <slug>` (GET /v1/rooms/{slug}/members) and `room add-member <slug> <agent_id>
# [--role <role>]` (POST same) are how a room owner sees and grows the set of agents in a room:
# a third, fourth or later agent is admitted by its id and then joins with its own API key.
# Both present the agent API key, never a room token; --role goes to the API unvalidated (the
# API decides); the participants are shown in the order the API answered; every failure shows
# the API's code, message and request id (--json: the API's error answer on stderr).
#
# A local HTTP server bound to 127.0.0.1 answers canned responses and records each request,
# so no network beyond 127.0.0.1 (and never production) is touched. Needs python3 and jq.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOLVR_SH="${SCRIPT_DIR}/solvr.sh"

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

KEY="solvr_members_owner_key"
SAVED_TOKEN="solvr_rt_saved-for-this-room"
ENV_TOKEN="solvr_rt_from-the-environment"
SLUG="a b/c"
ESCAPED="/v1/rooms/a%20b%2Fc/members"

cat > "$WORK/server.py" <<'PY'
# Answers the response the test selected in <work>/scenario and appends the request to
# <work>/requests.jsonl.
import json, os, sys
from http.server import ThreadingHTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlsplit, parse_qsl

work = sys.argv[1]

def row(agent, role, by, at):
    return {"room_id": "r-1", "agent_id": agent, "role": role, "added_by": by, "created_at": at}

def error(code, message, request_id):
    return {"error": {"code": code, "message": message, "request_id": request_id}}

ANSWERS = {
    # The API's order, which is not the agents' alphabetical order.
    "members": (200, {"data": [row("agent_planner", "owner", "system", "2026-10-02T10:00:00Z"),
                               row("zeta_executor", "member", "agent_planner", "2026-10-02T10:01:00Z"),
                               row("alpha_reviewer", "member", "agent_planner", "2026-10-02T10:02:00Z")]}),
    "added": (201, {"data": row("zeta_executor", "member", "agent_planner", "2026-10-02T10:01:00Z")}),
    "added_owner": (201, {"data": row("zeta_executor", "owner", "agent_planner", "2026-10-02T10:01:00Z")}),
    "forbidden": (403, error("FORBIDDEN", "only the room owner or admin can manage members", "req-members-403")),
    "bad_role": (400, error("VALIDATION_ERROR", "role must be 'member' or 'owner'", "req-role-400")),
    "invalid_agent": (400, error("INVALID_AGENT", "agent_id does not reference an existing agent", "req-agent-400")),
    "last_owner": (409, error("LAST_OWNER", "a room must keep at least one owner; add another owner first or delete the room", "req-owner-409")),
}

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
                  "content_type": self.headers.get("Content-Type"),
                  "idempotency_key": self.headers.get("Idempotency-Key"), "body": body}
        with open(work + "/requests.jsonl", "a") as f:
            f.write(json.dumps(record) + "\n")
        status, answer = ANSWERS[open(work + "/scenario").read().strip()]
        raw = json.dumps(answer).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    do_GET = do_POST = do_PATCH = do_PUT = do_DELETE = answer

server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
server.daemon_threads = True
with open(work + "/port.tmp", "w") as f:
    f.write(str(server.server_address[1]))
os.rename(work + "/port.tmp", work + "/port")
server.serve_forever()
PY

python3 "$WORK/server.py" "$WORK" &
SERVER_PID=$!
for _ in $(seq 1 50); do [ -f "$WORK/port" ] && break; sleep 0.1; done
[ -f "$WORK/port" ] || { echo "the test server did not start"; exit 1; }
BASE="http://127.0.0.1:$(cat "$WORK/port")/v1"

# run_solvr KEY|NOKEY SCENARIO ARGS... - runs solvr.sh against the server (15 s watchdog) with an
# empty request log; sets $code and leaves stdout/stderr in $WORK/out and $WORK/err. The config
# dir holds a saved room token for the room and SOLVR_ROOM_TOKEN is set, so a command that
# presented a room token instead of the API key would be seen.
run_solvr() {
    local with_key="$1" scenario="$2"; shift 2
    echo "$scenario" > "$WORK/scenario"
    : > "$WORK/requests.jsonl"
    local config="$WORK/config"
    rm -rf "$config"; mkdir -p "$config"
    jq -n --arg slug "$SLUG" --arg token "$SAVED_TOKEN" \
        '{($slug): {token: $token, created_at: "2026-10-02T00:00:00Z"}}' > "$config/rooms.json"
    code=0
    (
        unset SOLVR_API_KEY
        [ "$with_key" = KEY ] && export SOLVR_API_KEY="$KEY"
        SOLVR_ROOM_TOKEN="$ENV_TOKEN" SOLVR_API_URL="$BASE" SOLVR_CONFIG_DIR="$config" exec "$SOLVR_SH" "$@"
    ) > "$WORK/out" 2> "$WORK/err" &
    local pid=$!
    ( sleep 15; kill "$pid" 2>/dev/null ) >/dev/null 2>&1 &
    local dog=$!
    wait "$pid" || code=$?
    kill "$dog" 2>/dev/null || true
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

# check_request METHOD BODY - one line per way the single recorded request differs from METHOD on
# the escaped members path with the API key, no query, no Idempotency-Key, and BODY ("-" for none,
# else the exact JSON text sent, with Content-Type application/json).
check_request() {
    python3 - "$WORK/requests.jsonl" "$1" "$2" "$ESCAPED" "Bearer $KEY" <<'PY'
import json, sys
log, method, body, path, auth = sys.argv[1:6]
lines = [l for l in open(log).read().splitlines() if l]
if len(lines) != 1:
    print("sent %d requests, want exactly 1: %s" % (len(lines), lines))
    sys.exit()
sent = json.loads(lines[0])
want = {"method": method, "path": path, "query": {}, "authorization": auth, "idempotency_key": None}
for field, value in want.items():
    if sent[field] != value:
        print("%s %r, want %r" % (field, sent[field], value))
if body == "-":
    if sent["body"]:
        print("sent a body %r, want none" % sent["body"])
else:
    if sent["body"] is None or json.loads(sent["body"]) != json.loads(body) or list(json.loads(sent["body"])) != list(json.loads(body)):
        print("body %r, want exactly %s" % (sent["body"], body))
    if sent["content_type"] != "application/json":
        print("Content-Type %r, want application/json" % sent["content_type"])
PY
}

# check_output EXACT - the human output is exactly EXACT and nothing went to stderr.
check_output() {
    [ "$code" -eq 0 ] || echo "exit $code, want 0 (stderr: $(cat "$WORK/err"))"
    [ ! -s "$WORK/err" ] || echo "stderr $(cat "$WORK/err"), want nothing"
    [ "$(cat "$WORK/out")" = "$1" ] || printf 'output:\n%s\nwant:\n%s\n' "$(cat "$WORK/out")" "$1"
}

# check_answer SCENARIO - with --json stdout is the API's answer for SCENARIO.
check_answer() {
    [ "$code" -eq 0 ] || echo "exit $code, want 0 (stderr: $(cat "$WORK/err"))"
    python3 - "$WORK/server.py" "$1" "$WORK/out" <<'PY'
import json, re, sys
src, scenario, out = sys.argv[1:4]
namespace = {}
exec(re.search(r"(def row.*?\n}\n)", open(src).read(), re.S).group(1), namespace)
want = namespace["ANSWERS"][scenario][1]
try:
    if json.loads(open(out).read()) != want:
        print("printed %s, want the answer %s" % (open(out).read(), json.dumps(want)))
except ValueError:
    print("printed %r, not the API's answer" % open(out).read())
PY
}

# check_error CODE MESSAGE REQUEST_ID MODE - exit 1, nothing on stdout; human: stderr shows
# "CODE: MESSAGE" and the request id; json: stderr is the API's error answer.
check_error() {
    [ "$code" -eq 1 ] || echo "exit $code, want 1"
    [ ! -s "$WORK/out" ] || echo "stdout $(cat "$WORK/out"), want nothing on an error"
    if [ "$4" = json ]; then
        local want
        want=$(jq -cn --arg c "$1" --arg m "$2" --arg r "$3" '{error: {code: $c, message: $m, request_id: $r}}')
        [ "$(jq -c . "$WORK/err" 2>/dev/null)" = "$want" ] || echo "stderr $(cat "$WORK/err"), want the API's error answer $want"
    else
        grep -qF "$1: $2" "$WORK/err" || echo "stderr $(cat "$WORK/err") does not show '$1: $2'"
        grep -qF "request id: $3" "$WORK/err" || echo "stderr $(cat "$WORK/err") does not show 'request id: $3'"
    fi
}

no_request() {
    [ ! -s "$WORK/requests.jsonl" ] || echo "sent $(cat "$WORK/requests.jsonl"), want no request"
}

echo "=== room members ==="
run_solvr KEY members room members "$SLUG"
record "room members: one GET to the escaped members path with the API key, no query, no body" "$(check_request GET -)"
record "room members: the participants in the API's order, each with role, who added it and since when" "$(check_output "3 participants of a b/c:
  agent_planner owner (added by system, since 2026-10-02T10:00:00Z)
  zeta_executor member (added by agent_planner, since 2026-10-02T10:01:00Z)
  alpha_reviewer member (added by agent_planner, since 2026-10-02T10:02:00Z)")"
run_solvr KEY members room members "$SLUG" --json
record "room members --json: the API's answer" "$(check_request GET -; check_answer members)"

echo ""
echo "=== room add-member ==="
run_solvr KEY added room add-member "$SLUG" zeta_executor
record "room add-member: one POST with the API key and the agent id alone (no role, no Idempotency-Key)" \
    "$(check_request POST '{"agent_id":"zeta_executor"}')"
record "room add-member: names the agent, the room, its role and who added it, and how it joins" "$(check_output "zeta_executor is in a b/c as member (added by agent_planner)
It joins with its own API key: solvr.sh room join a b/c")"
run_solvr KEY added_owner room add-member "$SLUG" zeta_executor --role owner
record "room add-member --role owner: the role goes in the body" \
    "$(check_request POST '{"agent_id":"zeta_executor","role":"owner"}'; [ "$code" -eq 0 ] || echo "exit $code")"
run_solvr KEY added room add-member "$SLUG" zeta_executor --json
record "room add-member --json: the API's answer" "$(check_request POST '{"agent_id":"zeta_executor"}'; check_answer added)"
run_solvr KEY bad_role room add-member "$SLUG" zeta_executor --role admin
record "room add-member --role admin is sent as given and the API's refusal reported (the API decides)" \
    "$(check_request POST '{"agent_id":"zeta_executor","role":"admin"}'; check_error VALIDATION_ERROR "role must be 'member' or 'owner'" req-role-400 human)"

echo ""
echo "=== the API's refusals: code, message and request id ==="
for mode in human json; do
    flag=(); [ "$mode" = json ] && flag=(--json)
    run_solvr KEY forbidden room members "$SLUG" ${flag[@]+"${flag[@]}"}
    record "room members, not the owner (${mode}): 403 FORBIDDEN" \
        "$(check_request GET -; check_error FORBIDDEN "only the room owner or admin can manage members" req-members-403 "$mode")"
    run_solvr KEY invalid_agent room add-member "$SLUG" agent_nobody ${flag[@]+"${flag[@]}"}
    record "room add-member, unknown agent (${mode}): 400 INVALID_AGENT" \
        "$(check_request POST '{"agent_id":"agent_nobody"}'; check_error INVALID_AGENT "agent_id does not reference an existing agent" req-agent-400 "$mode")"
    run_solvr KEY last_owner room add-member "$SLUG" agent_planner --role member ${flag[@]+"${flag[@]}"}
    record "room add-member, demoting the only owner (${mode}): 409 LAST_OWNER" \
        "$(check_request POST '{"agent_id":"agent_planner","role":"member"}'; check_error LAST_OWNER "a room must keep at least one owner; add another owner first or delete the room" req-owner-409 "$mode")"
done

echo ""
echo "=== credentials and arguments ==="
run_solvr NOKEY members room members "$SLUG"
record "room members without an API key fails before any request (a room token is never used)" \
    "$(no_request; [ "$code" -eq 1 ] || echo "exit $code"; grep -q "No API key" "$WORK/err" || echo "stderr $(cat "$WORK/err")")"
run_solvr NOKEY added room add-member "$SLUG" zeta_executor
record "room add-member without an API key fails before any request (a room token is never used)" \
    "$(no_request; [ "$code" -eq 1 ] || echo "exit $code"; grep -q "No API key" "$WORK/err" || echo "stderr $(cat "$WORK/err")")"
run_solvr KEY members room members
record "room members without a slug fails before any request" \
    "$(no_request; [ "$code" -eq 1 ] || echo "exit $code"; grep -q "room members requires a slug" "$WORK/err" || echo "stderr $(cat "$WORK/err")")"
run_solvr KEY added room add-member "$SLUG"
record "room add-member without an agent id fails before any request" \
    "$(no_request; [ "$code" -eq 1 ] || echo "exit $code"; grep -q "room add-member requires a slug and an agent id" "$WORK/err" || echo "stderr $(cat "$WORK/err")")"
run_solvr KEY added room add-member "$SLUG" ""
record "room add-member with an empty agent id fails before any request" \
    "$(no_request; [ "$code" -eq 1 ] || echo "exit $code"; grep -q "room add-member requires a slug and an agent id" "$WORK/err" || echo "stderr $(cat "$WORK/err")")"
run_solvr KEY added room add-member "$SLUG" --role owner
record "room add-member with a flag where the agent id goes fails before any request" \
    "$(no_request; [ "$code" -eq 1 ] || echo "exit $code"; grep -q "room add-member requires a slug and an agent id" "$WORK/err" || echo "stderr $(cat "$WORK/err")")"
run_solvr KEY added room add-member
record "room add-member without arguments fails before any request" \
    "$(no_request; [ "$code" -eq 1 ] || echo "exit $code")"

run_solvr NOKEY members help
record "help lists room members and room add-member" \
    "$(grep -q "room members <slug>" "$WORK/out" || echo "help lacks 'room members <slug>'"; grep -q "room add-member <slug> <agent_id> \[--role owner|member\]" "$WORK/out" || echo "help lacks 'room add-member <slug> <agent_id> [--role owner|member]'")"

echo ""
echo "Passed: ${PASSED}  Failed: ${FAILED}"
[ "$FAILED" -eq 0 ]
