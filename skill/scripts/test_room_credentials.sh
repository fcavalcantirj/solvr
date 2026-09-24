#!/usr/bin/env bash
#
# test_room_credentials.sh - Offline tests for the room credential flow of solvr.sh.
#
# Room commands must authenticate with the agent's OWN per-agent room token
# (solvr_rt_...), obtained by handshake with the agent API key. The shared room
# token (solvr_rm_...) is never stored, printed, or sent by the default flows.
#
# A stub `curl` on PATH records every request and answers with canned responses,
# so no network (and never production) is touched.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOLVR_SH="${SCRIPT_DIR}/solvr.sh"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

PASSED=0
FAILED=0

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/bin"
cat > "$WORK/bin/curl" <<'STUB'
#!/usr/bin/env bash
# Stub curl: logs "METHOD URL | AUTH | DATA" and prints body + "\n" + status code.
method="GET"; url=""; auth=""; data=""
while [ $# -gt 0 ]; do
    case "$1" in
        -X) method="$2"; shift 2 ;;
        -H) case "$2" in Authorization:*) auth="${2#Authorization: }" ;; esac; shift 2 ;;
        -d) data="$2"; shift 2 ;;
        -w) shift 2 ;;
        -s) shift ;;
        *) url="$1"; shift ;;
    esac
done
echo "${method} ${url} | ${auth} | ${data}" >> "$STUB_LOG"
case "${method} ${url}" in
    "POST "*/v1/rooms)
        printf '%s\n%s' '{"data":{"slug":"stub-room","id":"r1","display_name":"Stub"},"token":"solvr_rm_SHARED"}' 201 ;;
    "POST "*/v1/rooms/*/handshake)
        printf '%s\n%s' '{"data":{"agent_id":"agent_stub","room_slug":"stub-room","room_token":"solvr_rt_AGENT","a2a_base":"/r/stub-room"}}' 201 ;;
    "POST "*/r/*/join)
        printf '%s\n%s' '{"data":{"agent_name":"stub-agent","ttl_seconds":600}}' 200 ;;
    "POST "*/r/*/message)
        printf '%s\n%s' '{"data":{"id":1,"sequence_num":1}}' 201 ;;
    *)
        printf '%s\n%s' '{"error":{"message":"stub: unexpected request"}}' 500 ;;
esac
STUB
chmod +x "$WORK/bin/curl"

# fresh_env resets the config dir and request log for one test case.
fresh_env() {
    export SOLVR_CONFIG_DIR="$WORK/config-$1"
    export STUB_LOG="$WORK/log-$1"
    rm -rf "$SOLVR_CONFIG_DIR" "$STUB_LOG"
    mkdir -p "$SOLVR_CONFIG_DIR"
    : > "$STUB_LOG"
}

run_solvr() {
    PATH="$WORK/bin:$PATH" SOLVR_API_KEY="solvr_agentkey" SOLVR_API_URL="http://stub.local/v1" \
        SOLVR_ROOM_TOKEN="" "$SOLVR_SH" "$@"
}

stored_token() {
    jq -r --arg s "$1" '.[$s].token // empty' "$SOLVR_CONFIG_DIR/rooms.json" 2>/dev/null || true
}

check() {
    local name="$1" cond="$2" detail="${3:-}"
    echo -n "Testing: ${name}... "
    if [ "$cond" = true ]; then
        echo -e "${GREEN}PASS${NC}"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}FAIL${NC}"
        [ -n "$detail" ] && echo "  $detail"
        FAILED=$((FAILED + 1))
    fi
}

is() { if eval "$1"; then echo true; else echo false; fi; }

echo "========================================="
echo "Solvr room credential tests (stub curl)"
echo "========================================="

# 1. room-create stores the per-agent token from handshake, never the shared token.
fresh_env create
out=$(run_solvr room-create "Stub Room" 2>&1) || true
check "room-create handshakes for a per-agent token" \
    "$(is 'grep -q "POST http://stub.local/v1/rooms/stub-room/handshake" "$STUB_LOG"')" "log: $(cat "$STUB_LOG")"
check "room-create saves the solvr_rt_ token" \
    "$(is '[ "$(stored_token stub-room)" = "solvr_rt_AGENT" ]')" "stored: $(stored_token stub-room)"
check "room-create never stores the shared token" \
    "$(is '! grep -q solvr_rm_ "$SOLVR_CONFIG_DIR/rooms.json" 2>/dev/null')"
check "room-create never prints the shared token" \
    "$(is '! echo "$out" | grep -q solvr_rm_SHARED')" "output: $out"

# 2. room-join with no stored credential handshakes first, then joins as the agent.
fresh_env join
out=$(run_solvr room-join stub-room --name stub-agent 2>&1) || true
check "room-join handshakes when no token is stored" \
    "$(is 'head -1 "$STUB_LOG" | grep -q "/v1/rooms/stub-room/handshake"')" "log: $(cat "$STUB_LOG")"
check "room-join authenticates with the per-agent token" \
    "$(is 'grep -q "POST http://stub.local/r/stub-room/join | Bearer solvr_rt_AGENT" "$STUB_LOG"')" "log: $(cat "$STUB_LOG")"
check "room-join succeeds" "$(is 'echo "$out" | grep -q "Joined room: stub-room"')" "output: $out"

# 3. A legacy stored shared token is replaced by handshake, and never sent.
fresh_env legacy
echo '{"stub-room":{"token":"solvr_rm_OLD","created_at":"2026-01-01T00:00:00Z"}}' > "$SOLVR_CONFIG_DIR/rooms.json"
out=$(run_solvr room-message stub-room hello --name stub-agent 2>&1) || true
check "legacy shared token is never sent" \
    "$(is '! grep -q solvr_rm_OLD "$STUB_LOG"')" "log: $(cat "$STUB_LOG")"
check "room-message posts with the per-agent token" \
    "$(is 'grep -q "POST http://stub.local/r/stub-room/message | Bearer solvr_rt_AGENT" "$STUB_LOG"')" "log: $(cat "$STUB_LOG")"
check "legacy shared token is replaced in rooms.json" \
    "$(is '[ "$(stored_token stub-room)" = "solvr_rt_AGENT" ]')" "stored: $(stored_token stub-room)"

# 4. The handshake command does not bootstrap with a stored shared token.
fresh_env handshake
echo '{"stub-room":{"token":"solvr_rm_OLD","created_at":"2026-01-01T00:00:00Z"}}' > "$SOLVR_CONFIG_DIR/rooms.json"
run_solvr handshake stub-room >/dev/null 2>&1 || true
check "handshake does not send a stored shared token" \
    "$(is '! grep -q solvr_rm_OLD "$STUB_LOG"')" "log: $(cat "$STUB_LOG")"

# 5. A stored per-agent token is reused without another handshake.
fresh_env reuse
echo '{"stub-room":{"token":"solvr_rt_AGENT","created_at":"2026-01-01T00:00:00Z"}}' > "$SOLVR_CONFIG_DIR/rooms.json"
run_solvr room-message stub-room hi --name stub-agent >/dev/null 2>&1 || true
check "stored per-agent token is reused (no handshake)" \
    "$(is '! grep -q handshake "$STUB_LOG"')" "log: $(cat "$STUB_LOG")"
check "reused token authenticates the message" \
    "$(is 'grep -q "Bearer solvr_rt_AGENT" "$STUB_LOG"')" "log: $(cat "$STUB_LOG")"

echo ""
echo "Passed: ${PASSED}  Failed: ${FAILED}"
[ "$FAILED" -eq 0 ]
