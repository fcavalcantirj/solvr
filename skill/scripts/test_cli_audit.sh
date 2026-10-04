#!/usr/bin/env bash
#
# test_cli_audit.sh - Offline regressions from the live audit of every solvr.sh command (2026-10-04).
#
# A stub `curl` on PATH records every request and answers like the API does:
#   - the health check lives at the API root (/health); /v1/health does not exist (404);
#   - a public room's entries are readable without any credential; a closed room answers 403.
#

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
# Stub curl: logs "METHOD URL | AUTH" and prints body + "\n" + status code.
method="GET"; url=""; auth=""
while [ $# -gt 0 ]; do
    case "$1" in
        -X) method="$2"; shift 2 ;;
        -H) case "$2" in Authorization:*) auth="${2#Authorization: }" ;; esac; shift 2 ;;
        -d|-w|-D) shift 2 ;;
        -s|-sN|-N) shift ;;
        *) url="$1"; shift ;;
    esac
done
echo "${method} ${url} | ${auth}" >> "$STUB_LOG"
case "${method} ${url}" in
    "GET http://stub.local/health")
        printf '%s\n%s' '{"status":"ok"}' 200 ;;
    "GET http://stub.local/v1/rooms/open-room/entries"*)
        printf '%s\n%s' '{"data":[{"id":7,"kind":"message","author_id":"agent_x","actor_label":"agent_x","body":"hello from the open room"}],"meta":{"has_more":false}}' 200 ;;
    "GET http://stub.local/v1/rooms/closed-room/entries"*)
        printf '%s\n%s' '{"error":{"code":"FORBIDDEN","message":"this room is closed to non-members"}}' 403 ;;
    *)
        printf '%s\n%s' '{"error":{"code":"NOT_FOUND","message":"not found"}}' 404 ;;
esac
STUB
chmod +x "$WORK/bin/curl"

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
echo "Solvr CLI audit regressions (stub curl)"
echo "========================================="

# 1. `solvr.sh test` checks the API's health at the root, not under /v1 (which is 404).
fresh_env health
code=0; out=$(run_solvr test 2>&1) || code=$?
check "test succeeds against a healthy API" "$(is '[ "$code" -eq 0 ]')" "exit $code, output: $out"
check "test asks /health at the API root" \
    "$(is 'grep -q "^GET http://stub.local/health " "$STUB_LOG"')" "log: $(cat "$STUB_LOG")"

# 2. A public room is readable without a room token: no credential is sent.
fresh_env open
code=0; out=$(run_solvr room read open-room --limit 5 2>&1) || code=$?
check "room read of a public room without a token succeeds" "$(is '[ "$code" -eq 0 ]')" "exit $code, output: $out"
check "room read without a token sends no credential" \
    "$(is 'grep -q "^GET http://stub.local/v1/rooms/open-room/entries?limit=5 | $" "$STUB_LOG"')" "log: $(cat "$STUB_LOG")"
check "room read prints the entries" "$(is 'echo "$out" | grep -q "hello from the open room"')" "output: $out"

# 3. A closed room still needs a join: the API's 403 comes with the command that fixes it.
fresh_env closed
code=0; out=$(run_solvr room read closed-room 2>&1) || code=$?
check "room read of a closed room without a token fails" "$(is '[ "$code" -ne 0 ]')" "exit $code"
check "the closed-room error says to join" "$(is 'echo "$out" | grep -q "solvr.sh room join closed-room"')" "output: $out"

echo ""
echo "Passed: ${PASSED}  Failed: ${FAILED}"
[ "$FAILED" -eq 0 ]
