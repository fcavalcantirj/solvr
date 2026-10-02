#!/usr/bin/env bash
# Cutover window smoke probes against $SOLVR_API_BASE.
#   probes.sh new <out-dir>   G6: v1.3 is serving (after the cutover)
#   probes.sh old <out-dir>   R6: c03734ae is serving again (after the rollback)
# Read-only. The one POST is a retired legacy write: v1.3 answers 410 without reading the body or
# touching storage; old code answers 401 without a credential. Prints one line per probe and a summary;
# exits 1 when any status differs from the expected one.
set -u
MODE=${1:?new|old}; OUT=${2:?out-dir}; B=${SOLVR_API_BASE:?}
mkdir -p "$OUT"
# Fixed targets from the post-purge production data (2026-09-29 dump).
PROBLEM_ID=950a61c8-3dc1-4a89-956f-795c058c70fe   # live public problem, 10 approaches
QUESTION_ID=0b2ecbed-86f6-48c2-b7bf-daae19169394  # live public question, 2 answers
ROOM_SLUG=secure-my-supa-360                      # live public room, 195 messages
QUERIES=("moltbook: api error during nightly tasks" "outlook email alerts with natural telegram notifications"
         "calendar alerts via python and telegram" "continuous operation" "log optimization")
fails=0
p() { # name expected-status curl-args...
  local name=$1 want=$2; shift 2
  local got
  got=$(curl -s --max-time 15 -o "$OUT/$name.json" -w '%{http_code} %{time_total}s' "$@")
  [ "${got%% *}" = "$want" ] || fails=$((fails + 1))
  printf '%-22s want %s got %s\n' "$name" "$want" "$got"
}
if [ "$MODE" = new ]; then
  p overview 200 "$B/v1/overview"
  p stats 200 "$B/v1/stats"
  p posts 200 "$B/v1/posts"
  p post 200 "$B/v1/posts/$PROBLEM_ID"
  p post-replies 200 "$B/v1/posts/$PROBLEM_ID/replies"
  i=0; for q in "${QUERIES[@]}"; do i=$((i + 1)); p "search-$i" 200 -G --data-urlencode "q=$q" "$B/v1/search"; done
  p rooms 200 "$B/v1/rooms"
  p room-entries 200 "$B/v1/rooms/$ROOM_SLUG/entries"
  p legacy-write-410 410 -X POST -H 'Content-Type: application/json' -d '{"title":"probe","description":"probe"}' "$B/v1/problems"
else
  p overview-404 404 "$B/v1/overview"
  p stats 200 "$B/v1/stats"
  p posts 200 "$B/v1/posts"
  p problem 200 "$B/v1/problems/$PROBLEM_ID"
  p problem-approaches 200 "$B/v1/problems/$PROBLEM_ID/approaches"
  p question 200 "$B/v1/questions/$QUESTION_ID"
  p rooms 200 "$B/v1/rooms"
  p room-messages 200 "$B/v1/rooms/$ROOM_SLUG/messages"
  p search-1 200 -G --data-urlencode "q=${QUERIES[0]}" "$B/v1/search"
fi
echo "total_contributions $(jq -r '.data.total_contributions' "$OUT/stats.json" 2>/dev/null)"
echo "probes failed: $fails"
[ "$fails" -eq 0 ]
