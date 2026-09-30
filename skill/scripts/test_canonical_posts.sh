#!/usr/bin/env bash
#
# test_canonical_posts.sh - Offline tests for the canonical post and reply commands of solvr.sh.
#
# Posts take no type, contributions are replies (POST /v1/posts/{id}/replies), and the retired
# answer/approach commands and the get --include flag are refused before any request is sent.
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
# Stub curl: logs "METHOD URL" to $STUB_LOG and the compact JSON body (or "-") to $STUB_LOG.data,
# then prints body + "\n" + status code like `curl -w "\n%{http_code}"`.
method="GET"; url=""; data=""
while [ $# -gt 0 ]; do
    case "$1" in
        -X) method="$2"; shift 2 ;;
        -H) shift 2 ;;
        -d) data="$2"; shift 2 ;;
        -w) shift 2 ;;
        -s) shift ;;
        *) url="$1"; shift ;;
    esac
done
echo "${method} ${url}" >> "$STUB_LOG"
if [ -n "$data" ]; then printf '%s' "$data" | jq -c . >> "$STUB_LOG.data"; else echo "-" >> "$STUB_LOG.data"; fi
case "${method} ${url}" in
    "POST "*/v1/posts)
        printf '%s\n%s' '{"data":{"id":"post_new","type":"post","title":"stub title","status":"pending_review"}}' 201 ;;
    "POST "*/v1/posts/*/replies)
        parent=$(printf '%s' "$data" | jq -c '.parent_reply_id // null')
        printf '%s\n%s' "{\"data\":{\"id\":\"reply_new\",\"post_id\":\"p1\",\"parent_reply_id\":${parent},\"author_type\":\"agent\",\"author_id\":\"agent_stub\",\"body\":\"stub reply body\",\"upvotes\":0,\"downvotes\":0,\"score\":0}}" 201 ;;
    "GET "*/v1/posts/empty/replies*)
        printf '%s\n%s' '{"data":[],"meta":{"total":0,"has_more":false}}' 200 ;;
    "GET "*/v1/posts/*/replies\?*limit=1*)
        printf '%s\n%s' '{"data":[{"id":"r3","post_id":"p1","author_type":"human","author_id":"human_1","body":"third reply","score":0}],"meta":{"total":3,"has_more":true,"next_cursor":"c2"}}' 200 ;;
    "GET "*/v1/posts/*/replies)
        printf '%s\n%s' '{"data":[{"id":"r1","post_id":"p1","author_type":"agent","author_id":"agent_a","body":"Tried pooling.\nIt worked.","score":2,"legacy_type":"approach","legacy_id":"a1"},{"id":"r2","post_id":"p1","parent_reply_id":"r1","author_type":"agent","author_id":"agent_b","body":"Confirmed here.","score":0}],"meta":{"total":2,"has_more":false}}' 200 ;;
    "GET "*/v1/posts/*)
        printf '%s\n%s' '{"data":{"id":"p1","type":"post","title":"Stub post","status":"open","posted_by_id":"agent_a","posted_by_type":"agent","upvotes":1,"downvotes":0,"tags":["go"],"description":"Stub description"}}' 200 ;;
    *)
        printf '%s\n%s' '{"error":{"message":"stub: unexpected request"}}' 500 ;;
esac
STUB
chmod +x "$WORK/bin/curl"

# fresh_env resets the request logs for one test case.
fresh_env() {
    export STUB_LOG="$WORK/log-$1"
    rm -f "$STUB_LOG" "$STUB_LOG.data"
    : > "$STUB_LOG"
    : > "$STUB_LOG.data"
}

# run_solvr runs solvr.sh against the stub; it sets $out and $code.
run_solvr() {
    code=0
    out=$(PATH="$WORK/bin:$PATH" SOLVR_API_KEY="solvr_agentkey" SOLVR_API_URL="http://stub.local/v1" \
        SOLVR_CONFIG_DIR="$WORK/config" "$SOLVR_SH" "$@" 2>&1) || code=$?
}

requests() { wc -l < "$STUB_LOG" | tr -d ' '; }
payload() { sed -n "${1:-1}p" "$STUB_LOG.data"; }

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
echo "Solvr canonical post/reply tests (stub curl)"
echo "========================================="

# 1. post creates a canonical post: no type in the payload, no type in the output.
fresh_env post
run_solvr post "Title of a canonical post" "Body of a canonical post, long enough." --tags "go,postgres"
check "post exits 0" "$(is '[ "$code" -eq 0 ]')" "exit $code, output: $out"
check "post sends one POST /v1/posts" \
    "$(is '[ "$(cat "$STUB_LOG")" = "POST http://stub.local/v1/posts" ]')" "log: $(cat "$STUB_LOG")"
check "post payload has no type" "$(is '[ "$(payload | jq "has(\"type\")")" = false ]')" "payload: $(payload)"
check "post payload carries title, description and tags" \
    "$(is '[ "$(payload | jq -c "[.title, .description, .tags]")" = "[\"Title of a canonical post\",\"Body of a canonical post, long enough.\",[\"go\",\"postgres\"]]" ]')" \
    "payload: $(payload)"
check "post payload omits visibility when not given" "$(is '[ "$(payload | jq "has(\"visibility\")")" = false ]')" "payload: $(payload)"
check "post prints the new id" "$(is 'echo "$out" | grep -q "ID: post_new"')" "output: $out"
check "post prints no Type line" "$(is '! echo "$out" | grep -q "^Type:"')" "output: $out"

# 2. post --visibility is forwarded as given (the API validates it).
fresh_env visibility
run_solvr post "Title of a family post" "Body of a family post, long enough." --visibility family
check "post forwards --visibility" "$(is '[ "$(payload | jq -r .visibility)" = family ]')" "payload: $(payload)"

# 3. A legacy type argument is refused before any request.
for legacy in problem question idea; do
    fresh_env "type-$legacy"
    run_solvr post "$legacy" "Some title here" "Some body here"
    check "post $legacy is refused" "$(is '[ "$code" -ne 0 ]')" "exit $code, output: $out"
    check "post $legacy names the refused type" \
        "$(is 'echo "$out" | grep -qF "posts take no type: \"$legacy\" is not accepted"')" "output: $out"
    check "post $legacy sends nothing" "$(is '[ "$(requests)" -eq 0 ]')" "log: $(cat "$STUB_LOG")"
done

# 4. Any third positional argument is a type, and refused.
fresh_env type-other
run_solvr post invalid "Some title here" "Some body here"
check "post with a third argument is refused" \
    "$(is '[ "$code" -ne 0 ] && echo "$out" | grep -qF "posts take no type: \"invalid\""')" "exit $code, output: $out"
check "post with a third argument sends nothing" "$(is '[ "$(requests)" -eq 0 ]')" "log: $(cat "$STUB_LOG")"

# 5. post needs a title and a body.
fresh_env post-args
run_solvr post "Only a title"
check "post without a body is refused" \
    "$(is '[ "$code" -ne 0 ] && echo "$out" | grep -qF "post requires a title and a body"')" "exit $code, output: $out"
check "post without a body sends nothing" "$(is '[ "$(requests)" -eq 0 ]')" "log: $(cat "$STUB_LOG")"

# 6. reply creates a canonical reply: POST /v1/posts/{id}/replies with only the body.
fresh_env reply
run_solvr reply p1 "A reply body with details."
check "reply exits 0" "$(is '[ "$code" -eq 0 ]')" "exit $code, output: $out"
check "reply sends one POST /v1/posts/p1/replies" \
    "$(is '[ "$(cat "$STUB_LOG")" = "POST http://stub.local/v1/posts/p1/replies" ]')" "log: $(cat "$STUB_LOG")"
check "reply payload is only the body" \
    "$(is '[ "$(payload)" = "{\"body\":\"A reply body with details.\"}" ]')" "payload: $(payload)"
check "reply prints the reply and post ids" \
    "$(is 'echo "$out" | grep -q "Reply created" && echo "$out" | grep -q "ID: reply_new" && echo "$out" | grep -q "Post ID: p1"')" \
    "output: $out"
check "reply points to the replies command" "$(is 'echo "$out" | grep -qF "solvr replies p1"')" "output: $out"
check "top-level reply prints no parent" "$(is '! echo "$out" | grep -q "In reply to"')" "output: $out"

# 7. reply --parent threads under another reply.
fresh_env reply-parent
run_solvr reply p1 "A threaded reply body." --parent r1
# (compared as a jq array: a quoted {a,b} literal is brace-expanded by bash 3.2 inside $(...))
check "reply --parent sends parent_reply_id" \
    "$(is '[ "$(payload | jq -c "[keys, .body, .parent_reply_id]")" = "[[\"body\",\"parent_reply_id\"],\"A threaded reply body.\",\"r1\"]" ]')" \
    "payload: $(payload)"
check "reply --parent prints the parent" "$(is 'echo "$out" | grep -q "In reply to: r1"')" "output: $out"

# 8. reply --json prints the API response.
fresh_env reply-json
run_solvr reply p1 "A reply body for json." --json
check "reply --json prints the raw response" "$(is '[ "$(echo "$out" | jq -r .data.id)" = reply_new ]')" "output: $out"

# 9. reply needs a post id and a body.
fresh_env reply-args
run_solvr reply p1
check "reply without a body is refused" \
    "$(is '[ "$code" -ne 0 ] && echo "$out" | grep -qF "reply requires a post ID and a body"')" "exit $code, output: $out"
check "reply without a body sends nothing" "$(is '[ "$(requests)" -eq 0 ]')" "log: $(cat "$STUB_LOG")"

# 10. replies lists a post's replies, labeling migrated and threaded ones.
fresh_env replies
run_solvr replies p1
check "replies sends one GET /v1/posts/p1/replies with no query" \
    "$(is '[ "$(cat "$STUB_LOG")" = "GET http://stub.local/v1/posts/p1/replies" ]')" "log: $(cat "$STUB_LOG")"
check "replies prints the count" "$(is 'echo "$out" | grep -q "^2 replies"')" "output: $out"
check "replies labels a migrated reply" "$(is 'echo "$out" | grep -qF "1. r1 [migrated approach]"')" "output: $out"
check "replies labels a threaded reply" "$(is 'echo "$out" | grep -qF "2. r2 (in reply to r1)"')" "output: $out"
check "replies prints author and score" "$(is 'echo "$out" | grep -qF "By: agent_a (agent)  Score: 2"')" "output: $out"
check "replies prints every body line" \
    "$(is 'echo "$out" | grep -q "^   Tried pooling.$" && echo "$out" | grep -q "^   It worked.$"')" "output: $out"
check "replies without more pages prints no cursor hint" "$(is '! echo "$out" | grep -q "More:"')" "output: $out"

# 11. replies pages with --limit and --cursor.
fresh_env replies-page
run_solvr replies p1 --limit 1 --cursor c1
check "replies sends limit and cursor" \
    "$(is '[ "$(cat "$STUB_LOG")" = "GET http://stub.local/v1/posts/p1/replies?limit=1&cursor=c1" ]')" "log: $(cat "$STUB_LOG")"
check "replies prints the page size and total" "$(is 'echo "$out" | grep -q "^Showing 1 of 3 replies"')" "output: $out"
check "replies prints the next cursor" "$(is 'echo "$out" | grep -qF "More: solvr replies p1 --cursor c2"')" "output: $out"

# 12. replies on a post without replies.
fresh_env replies-empty
run_solvr replies empty
check "replies on an empty post says so" "$(is '[ "$code" -eq 0 ] && echo "$out" | grep -q "No replies yet."')" "exit $code, output: $out"

# 13. replies --json prints the page.
fresh_env replies-json
run_solvr replies p1 --json
check "replies --json prints the raw page" "$(is '[ "$(echo "$out" | jq -c "[(.data | length), .meta.total]")" = "[2,2]" ]')" "output: $out"

# 14. The retired answer and approach commands are refused and point to reply.
for retired in answer approach; do
    fresh_env "retired-$retired"
    run_solvr "$retired" p1 "Some content here"
    check "$retired is refused" "$(is '[ "$code" -ne 0 ]')" "exit $code, output: $out"
    check "$retired points to solvr reply" \
        "$(is 'echo "$out" | grep -qF "solvr $retired was retired" && echo "$out" | grep -qF "solvr reply <post_id> <body>"')" "output: $out"
    check "$retired sends nothing" "$(is '[ "$(requests)" -eq 0 ]')" "log: $(cat "$STUB_LOG")"
done

# 15. get reads the post with one request and points to replies.
fresh_env get
run_solvr get p1
check "get sends one GET /v1/posts/p1" \
    "$(is '[ "$(cat "$STUB_LOG")" = "GET http://stub.local/v1/posts/p1" ]')" "log: $(cat "$STUB_LOG")"
check "get prints the post" "$(is 'echo "$out" | grep -q "Stub post"')" "output: $out"
check "get points to the replies command" "$(is '[ "$(echo "$out" | tail -1)" = "Replies: solvr replies p1" ]')" "output: $out"

# 16. get --include was removed.
fresh_env get-include
run_solvr get p1 --include answers
check "get --include is refused" \
    "$(is '[ "$code" -ne 0 ] && echo "$out" | grep -qF "get --include was removed; use: solvr replies p1"')" "exit $code, output: $out"
check "get --include sends nothing" "$(is '[ "$(requests)" -eq 0 ]')" "log: $(cat "$STUB_LOG")"

echo ""
echo "Passed: ${PASSED}  Failed: ${FAILED}"
[ "$FAILED" -eq 0 ]
