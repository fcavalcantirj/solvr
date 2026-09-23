#!/bin/bash
set -e

# ─────────────────────────────────────────────────────────────────────────────
# ralph.sh — bounded per-task build loop for Solvr (engine-agnostic core).
#
# Runs a headless coding agent N times. Each run: pick the first spec.json
# task with passes=false, do ONLY that task (tests-first), run the task's own
# Verify steps, flip passes=true, journal to progress.txt, commit, then STOP.
# Statelessness means every run re-reads the JSON and picks up the next task.
#
# Engines (pick via wrapper script or ENGINE env var):
#   ./ralph-claude.sh 3         # Claude Code CLI (default engine)
#   ./ralph-codex.sh 1          # Codex CLI, gpt-5.6-sol
#   ./ralph-kimi.sh 1           # Kimi Code CLI, KIMI_MODEL alias
#   ./ralph-opencode.sh 1       # OpenCode Zen
#
# Knobs (env or git-ignored .env.ralph.local):
#   ENGINE=claude|codex|opencode|kimi   MODEL=<claude pin>
#   CODEX_MODEL=gpt-5.6-sol   CODEX_EFFORT=max
#   OPENCODE_MODEL=...   KIMI_MODEL=...   RALPH_PUSH=0
#   PRD_FILE=spec.json   VERIFY_CMD=<host verify command>
# ─────────────────────────────────────────────────────────────────────────────

# Load git-ignored local config if present (provider keys, RALPH_PUSH, MODEL…)
if [ -f .env.ralph.local ]; then set -a; . ./.env.ralph.local; set +a; fi

ENGINE="${ENGINE:-claude}"
PRD_FILE="${PRD_FILE:-spec.json}"
RALPH_PUSH="${RALPH_PUSH:-0}"          # 1 = git push after each task (needs a remote)
MODEL="${MODEL:-}"                     # optional Claude model pin
CODEX_MODEL="${CODEX_MODEL:-gpt-5.6-sol}"
CODEX_EFFORT="${CODEX_EFFORT:-max}"
CUSTOM_MODEL="${CUSTOM_MODEL:-opencode/muse-spark-1.3-contributor-free}"
CUSTOM_EFFORT="${CUSTOM_EFFORT:-}"     # empty = don't send reasoning effort
VERIFY_CMD="${VERIFY_CMD:-}"           # host-side batch verify (empty = off); runs OUTSIDE the engine sandbox
STALL_LIMIT="${STALL_LIMIT:-2}"        # consecutive iterations with no ledger progress before the batch stops
RALPH_ITER_TIMEOUT="${RALPH_ITER_TIMEOUT:-45m}"   # wall clock per engine call; empty = off

# Per-iteration timeout. The stall guard only counts iterations that END, so an
# engine that hangs is never counted and eats the whole run: kimi wedged for
# 13.5 h on 2026-09-22 with zero output, zero files touched, and the supervisor
# never noticed. This kills the engine call itself.
TIMEOUT_BIN=""
if [ -n "$RALPH_ITER_TIMEOUT" ]; then
  if command -v timeout >/dev/null 2>&1; then TIMEOUT_BIN="timeout"
  elif command -v gtimeout >/dev/null 2>&1; then TIMEOUT_BIN="gtimeout"
  else echo "Note: no timeout/gtimeout on PATH — per-iteration timeout disabled (brew install coreutils)"; fi
fi

# run_engine <cmd...> — runs the engine under the timeout when one is available.
# -k 30s sends KILL 30 s after TERM for an engine that ignores TERM.
run_engine() {
  if [ -n "$TIMEOUT_BIN" ]; then
    "$TIMEOUT_BIN" -k 30s "$RALPH_ITER_TIMEOUT" "$@"
  else
    "$@"
  fi
}

case "$ENGINE" in
  claude|codex|opencode|kimi) ;;
  *) echo "Unknown ENGINE '$ENGINE' (expected claude, codex, opencode, or kimi)"; exit 1 ;;
esac

# opencode guard: credentials live in opencode's own auth store (`opencode auth`),
# not in an env var, so probe the CLI instead of a key. OPENCODE_MODEL is
# overridable from the environment so a rate-limited free model can be swapped
# without editing this script.
OPENCODE_MODEL="${OPENCODE_MODEL:-$CUSTOM_MODEL}"
if [ "$ENGINE" = "opencode" ]; then
  if ! command -v opencode >/dev/null 2>&1; then
    echo "ENGINE=opencode requires the opencode CLI on PATH"
    exit 1
  fi
  if ! opencode models >/dev/null 2>&1; then
    echo "opencode is installed but not authenticated — run: opencode auth login"
    exit 1
  fi
fi

# kimi guard: the Kimi Code CLI keeps provider keys in ~/.kimi-code/config.toml;
# `kimi doctor` validates that file offline. KIMI_MODEL is a config alias
# (the [models."..."] keys), overridable from the environment or .env.ralph.local.
KIMI_MODEL="${KIMI_MODEL:-openrouter/poolside/laguna-s-2.1:free}"
KIMI_EXTRA_FLAGS="${KIMI_EXTRA_FLAGS:-}"
KIMI_RETRIES="${KIMI_RETRIES:-0}"          # 0 = retry forever on transient provider errors
KIMI_BACKOFF_S="${KIMI_BACKOFF_S:-60}"     # first wait; doubles each retry up to the cap
KIMI_BACKOFF_MAX_S="${KIMI_BACKOFF_MAX_S:-300}"
KIMI_FAST_FAIL_S="${KIMI_FAST_FAIL_S:-20}" # a retry that dies this fast made no progress
KIMI_FAST_FAILS="${KIMI_FAST_FAILS:-6}"    # that many in a row = the daily free quota is gone
KIMI_RESUME_INPUT="A provider error interrupted you. Continue the SAME task exactly where you left off in this session: do not start over, do not switch tasks. Re-read only what you need, finish the task's steps, run its Verify steps, append the progress.txt entry, set passes=true, and commit as instructed. Then stop."
if [ "$ENGINE" = "kimi" ]; then
  if ! command -v kimi >/dev/null 2>&1; then
    echo "ENGINE=kimi requires the Kimi Code CLI on PATH (~/.kimi-code/bin/kimi)"
    exit 1
  fi
  if ! kimi doctor >/dev/null 2>&1; then
    echo "kimi is installed but its config is invalid — run: kimi doctor"
    exit 1
  fi
fi

MODEL_FLAG=""
if [ -n "$MODEL" ]; then
  MODEL_FLAG="--model $MODEL"
fi

# Colors
CYAN='\033[0;36m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'
MAGENTA='\033[0;35m'; BLUE='\033[0;34m'; RED='\033[0;31m'
BOLD='\033[1m'; DIM='\033[2m'; NC='\033[0m'

# Push instruction injected into the prompt based on RALPH_PUSH.
if [ "$RALPH_PUSH" = "1" ]; then
  PUSH_STEP="8. PUSH: run 'git push' to publish the commit (only if a git remote exists)."
else
  PUSH_STEP="8. Do NOT push — leave the commit local (RALPH_PUSH=0)."
fi

format_time() {
  local secs=$1
  printf "%02d:%02d:%02d" $((secs/3600)) $((secs%3600/60)) $((secs%60))
}

# Passed-task count, used by the stall guard below.
count_passed() {
  if command -v jq >/dev/null 2>&1; then
    jq '[.[] | select(.passes == true)] | length' "$PRD_FILE" 2>/dev/null || echo 0
  else
    grep -cE '"passes"[[:space:]]*:[[:space:]]*true' "$PRD_FILE" 2>/dev/null || echo 0
  fi
}

# Host-side verification (runs OUTSIDE the engine sandbox, once per batch).
# The engine's sandbox can silently skip checks it cannot run (port binds,
# browsers, git) — this is the ground truth. On failure: journal the tail to
# progress.txt and inject ONE URGENT ledger task (deduped against open tasks).
run_host_verify() {
  [ -n "$VERIFY_CMD" ] || return 0
  echo -e "${CYAN}🔍 Host verify: ${VERIFY_CMD}${NC}"
  local vout
  vout=$(mktemp)
  if bash -c "$VERIFY_CMD" > "$vout" 2>&1; then
    echo -e "${GREEN}✅ Host verify passed${NC}"
    rm -f "$vout"
    return 0
  fi
  echo -e "${RED}${BOLD}❌ Host verify FAILED — output tail:${NC}"
  tail -20 "$vout"
  {
    echo ""
    echo "$(date '+%Y-%m-%d %H:%M'): HOST VERIFY FAILED — \`$VERIFY_CMD\` exited nonzero. Tail:"
    tail -20 "$vout" | sed 's/^/    /'
  } >> progress.txt
  if command -v jq >/dev/null 2>&1; then
    local desc="URGENT: host verification failed — run '$VERIFY_CMD' on the host, fix every failure, re-run until it exits 0"
    if ! jq -e --arg d "$desc" 'any(.[]; .description == $d and .passes == false)' "$PRD_FILE" >/dev/null 2>&1; then
      local tprd
      tprd=$(mktemp)
      if jq --arg d "$desc" --arg cmd "$VERIFY_CMD" \
        '[{category: "infra", description: $d,
           steps: [("Run on the host: " + $cmd + " and read every failure"),
                   "Fix the root causes — do NOT weaken, skip, or sandbox-attest the checks",
                   ("Re-run " + $cmd + " until it exits 0")],
           passes: false}] + .' "$PRD_FILE" > "$tprd"; then
        mv "$tprd" "$PRD_FILE"
        echo -e "${YELLOW}⚠️  Injected URGENT task at top of ${PRD_FILE}${NC}"
      else
        rm -f "$tprd"
      fi
    fi
  else
    echo -e "${YELLOW}⚠️  jq not found — cannot inject URGENT task; see progress.txt${NC}"
  fi
  rm -f "$vout"
  return 1
}

if [ -z "${1:-}" ] || ! [ "$1" -ge 1 ] 2>/dev/null; then
  echo "Usage: $0 <iterations>"
  exit 1
fi

# Host git backstop: some engine sandboxes cannot create .git, and a run with no
# commits loses every task it finished.
if [ ! -d .git ] && command -v git >/dev/null 2>&1; then
  git init -b main >/dev/null 2>&1 || git init >/dev/null 2>&1
  echo -e "${DIM}Initialized git repo (host backstop).${NC}"
fi

case "$ENGINE" in
  claude) ENGINE_DESC="claude (${MODEL:-CLI default})"; ENGINE_CMD="claude" ;;
  codex)  ENGINE_DESC="codex (${CODEX_MODEL}, effort ${CODEX_EFFORT})"; ENGINE_CMD="codex" ;;
  opencode) ENGINE_DESC="opencode (${OPENCODE_MODEL})"; ENGINE_CMD="opencode" ;;
  kimi) ENGINE_DESC="kimi (${KIMI_MODEL})"; ENGINE_CMD="kimi" ;;
esac

echo -e "${DIM}PRD: ${PRD_FILE}   engine: ${ENGINE_DESC}   push: ${RALPH_PUSH}${NC}"
# Show only the current engine's agent processes (claude -> claude, codex -> codex).
running_pids=$(pgrep -il "$ENGINE_CMD" 2>/dev/null || true)
if [ -n "$running_pids" ]; then
  echo -e "${DIM}Running ${ENGINE_CMD} processes:${NC}"
  echo "$running_pids" | awk '{print "  PID: " $1}'
else
  echo -e "${DIM}No ${ENGINE_CMD} processes running.${NC}"
fi
echo ""

# Shared prompt: project golden rules + one-task workflow.
# NB: assigned via `read`, not $(cat <<heredoc) — macOS bash 3.2 cannot parse a
# heredoc inside $() when the body contains an apostrophe.
read -r -d '' PROMPT <<EOF || true
=== GOLDEN RULES (MUST FOLLOW) ===
- PROJECT: Solvr — a knowledge base for developers and AI agents (Go API in \`backend/\`, Next.js 15 App Router frontend in \`frontend/\`, PostgreSQL). The ledger you are given IS the specification for this build. \`CLAUDE.md\`, \`SPEC.md\` and \`PROJECT_KNOWLEDGE.md\` are REFERENCE — read them when a task needs context; edit \`SPEC.md\` only when the current task's steps say to document an API change there.
- THE LEDGER IS APPEND-ONLY TRUTH: never edit, reorder, delete or renumber tasks in $PRD_FILE. The ONLY field you may change is \`passes\`, one task per iteration, after that task's own Verify steps pass. Never add tasks.
- TDD, ALWAYS: write the failing test FIRST (RED), then the minimum code (GREEN), then clean up (REFACTOR). Backend tests are \`_test.go\` run with \`cd backend && go test ./...\`; frontend tests use VITEST, never Jest (\`vi.mock\`/\`vi.fn\`/\`vi.mocked\`, imports from 'vitest'), run with \`cd frontend && npm test\`. Coverage floor is 80%.
- API IS SMART, CLIENT IS DUMB: 100% of business logic lives in the Go API. The frontend never validates, transforms, calculates or decides — it calls endpoints, renders what comes back, and shows loading/error states.
- NO STUBS, NO IN-MEMORY REPOSITORIES in production paths. Use \`db.New*Repository(pool)\`; never \`NewInMemory*Repository()\`, never "temporary until the DB lands". If you cannot implement real storage, do not implement the feature.
- NO CODE FILE OVER 800 LINES — CI enforces it via \`./scripts/check-file-size.sh\`. Split by responsibility before you get there. Markdown is exempt.
- LOCAL ONLY. NEVER touch production: no \`/admin/query\` calls against api.solvr.dev, no email broadcasts, no deploys, no release tags, no \`git push\` unless this prompt's PUSH step says to. NEVER modify \`.env\` or any env file. The local stack is \`docker compose up -d\` (PostgreSQL on port 5435, IPFS on 5001/8081); kill any previous process before starting another.
- DO NOT CREATE DOCUMENTS: no summary files, no reports, no new .md files unless the current task explicitly names the file.
- LABEL EVERY CLAIM \`[REAL]\` (verified against the running system), \`[TEST]\` (a test that ACTUALLY RAN and passed — never a skipped one) or \`[UNVERIFIED]\` (reasoned but not checked). "It should work now" is not a status.
- VERIFICATION IS NOT OPTIONAL — A SKIPPED TEST IS NOT A PASSED TEST. Never set passes=true on a task whose checks did not actually EXECUTE and come back green. \`SKIP\`, \`[no tests to run]\`, "it builds", "it vets clean" and "the suite is green because everything skipped" are NOT evidence of behavior. If the tests you wrote report SKIP (integration tests skip when DATABASE_URL is unset), the task is UNVERIFIED — leave passes=false.
- ENVIRONMENT BLOCKED = STUCK, NOT DONE. If the database, Docker, a service or any dependency needed to RUN the checks is unavailable, journal the exact commands and their exact errors to progress.txt, leave passes=false, and STOP the iteration for a human. Never mark a task passed because the environment could not prove it. Point DATABASE_URL only at a local, isolated test database — never at production, never at a database holding real data.
- OWNER-ONLY LAST MILE IS THE ONLY THING THAT DEGRADES: a task may pass with something outstanding ONLY when the remaining step is inherently the owner's — a production migration, a deploy, an email broadcast, a physical or visual confirmation — AND every automated check for that task already RAN GREEN locally. Append a \`UAT:\` line naming exactly what the owner must run or confirm, then set passes=true. This clause NEVER covers "the tests did not run".
- NEVER DELETE, SKIP OR WEAKEN AN EXISTING TEST. Every test in the tree was written by an earlier iteration to pin behavior that is still live. Do not delete it, comment it out, rename it away, mark it skipped, or loosen its assertions to make your task pass. If a test genuinely describes behavior THIS task replaces, you must (a) name the test in progress.txt, (b) quote what it asserted, (c) name the new test that covers the same behavior, and (d) show that new test passing. A test removed with no named replacement is a REGRESSION, not a cleanup — the behavior it guarded silently loses its only guard.
- THE JOURNAL MUST MATCH THE DIFF. Never write "no production code changes were needed" when the commit changes production files, and never describe work you did not do. Before you write the entry, run \`git status --porcelain\` and \`git diff --stat\` and describe what they actually show, file by file. The journal is the only memory the next iteration has: a wrong entry sends it down a wrong path.
- ACCEPTANCE TASKS ARE NOT CHECKLISTS. If a task's checks span behavior that other tasks still have to build, writing tests for it does not complete it — leave it open and take the first task that is actually buildable now.
- WHEN STUCK: journal the exact command and its exact error to progress.txt, leave the task open (do NOT flip passes), and stop the iteration so a human can look. Never guess an API, never invent one.

=== WORKFLOW ===
1. Read $PRD_FILE (the task ledger) and progress.txt (the build journal) before anything else.
2. In $PRD_FILE, find the FIRST task (top-to-bottom order = priority; do any task whose description starts with the URGENT marker before others) where passes is false. Work ONLY on that one task. Honor its 'DEPENDS ON:' / 'PREREQUISITE:' notes.
3. Follow that task's 'steps' exactly. Write tests first.
4. Validate by RUNNING that task's own 'Verify:' steps plus \`cd backend && go test ./...\` and/or \`cd frontend && npm test\` for what you touched, and READ THE OUTPUT: every test you wrote for this task must report ok/PASS, not SKIP. Backend integration tests need a local isolated database — export DATABASE_URL (the local stack is \`docker compose up -d\`, Postgres on 5435) before running them, and say in the journal which database they ran against. Do NOT mark the task done until those checks pass. If the last remaining step is inherently the owner's (production migration, deploy, broadcast, visual confirmation) and everything automatable already ran green, append a 'UAT:' line to progress.txt and then set passes=true. If instead the checks could not run at all, leave passes=false and stop (see ENVIRONMENT BLOCKED above).
5. APPEND a dated entry to progress.txt describing what you did. progress.txt is APPEND-ONLY: use >> and NEVER > — do not overwrite, truncate, rewrite or reformat it, and never delete lines you did not add. Its history is the only memory the next iteration has.
6. In $PRD_FILE, set that task's "passes" to true.
7. COMMIT: if .git exists, run 'git add .' to stage ALL files (including new ones), then 'git commit -m "<type>(<scope>): <task description>"' using the project's prefixes (feat, fix, refactor, test, docs, chore). If the repo is not git-initialized, note that in progress.txt and skip committing this once.
$PUSH_STEP
9. If, and ONLY IF, every task in $PRD_FILE now has passes=true, output the exact line: <promise>COMPLETE</promise>

CRITICAL:
- ONE TASK ONLY, then STOP. Do NOT continue to another task.
- Always 'git add .' (include NEW files) before committing.
- After commit, you are DONE. Exit immediately.
- HARNESS IS INFRASTRUCTURE, NOT DELIVERABLE: never create, modify, or replace ralph.sh, ralph-*.sh, progress.sh, .env.ralph.local, or the ledger schema unless the current task explicitly names them.
EOF

# Claude attaches the ledger/journal; codex-family engines are told to read them first.
CLAUDE_INPUT="@$PRD_FILE @progress.txt $PROMPT"
CODEX_INPUT="FIRST: read ./$PRD_FILE and ./progress.txt in this repository — they are the task ledger and build journal.

$PROMPT"

# opencode takes the same "read them first" preamble as codex, for the -f reason
# documented at the invocation below.
OPENCODE_INPUT="$CODEX_INPUT"

tmpfile=$(mktemp)
errfile=$(mktemp)
cleanup() { rm -f "$tmpfile" "$errfile"; }
trap cleanup EXIT

overall_start=$(date +%s)
prev_passed=$(count_passed)
stall_streak=0
total_iteration_time=0
completed_iterations=0
total_cost=0
total_input_tokens=0
total_output_tokens=0

for ((i=1; i<=$1; i++)); do
  echo ""
  echo -e "${CYAN}${BOLD}═══════════════════════════════════════════════════════════${NC}"
  echo -e "${CYAN}${BOLD}  Iteration $i of $1 — ${ENGINE_DESC}${NC}"
  echo -e "${CYAN}${BOLD}═══════════════════════════════════════════════════════════${NC}"
  echo ""
  iter_start=$(date +%s)
  engine_exit=0

  # Journal backstop: progress.txt is append-only, but an engine may rewrite it.
  # Snapshot it so the history can be restored if the engine truncates.
  progbak=""
  if [ -f progress.txt ]; then
    progbak=$(mktemp)
    cp progress.txt "$progbak"
  fi

  if [ "$ENGINE" = "claude" ]; then
    # Headless Claude Code on ONE task; JSON output carries result + usage/cost.
    run_engine claude $MODEL_FLAG --dangerously-skip-permissions --no-session-persistence \
      -p --output-format json "$CLAUDE_INPUT" > "$tmpfile" 2>&1 || engine_exit=$?
  elif [ "$ENGINE" = "opencode" ]; then
    # opencode run: headless one-shot. --auto auto-approves permissions (the loop
    # cannot answer a prompt).
    # Do NOT pass the ledger with -f: --file is declared [array] in opencode's
    # yargs parser, and an array option greedily swallows the positional that
    # follows it, so `-f progress.txt "$PROMPT"` parsed the ENTIRE prompt as a
    # second filename and exited 1. The prompt tells it to read the files itself,
    # exactly like the codex lane. Keep the message the ONLY positional.
    run_engine opencode run --auto -m "$OPENCODE_MODEL" ${OPENCODE_VARIANT:+--variant "$OPENCODE_VARIANT"} $OPENCODE_EXTRA_FLAGS \
      "$OPENCODE_INPUT" < /dev/null > "$tmpfile" 2> "$errfile" || engine_exit=$?
  elif [ "$ENGINE" = "kimi" ]; then
    # kimi -p: one prompt, non-interactive, response on stdout (text mode adds a
    # leading bullet, harmless to the <promise> grep). Prompt mode runs tool
    # calls without asking and REFUSES --auto/-y ("Cannot combine --prompt with
    # --auto", measured 2026-09-16), so no approval flag is passed. stdin
    # </dev/null as for codex.
    # Free-tier providers answer 429 mid-task. A dead prompt run keeps its
    # session, and `-p ... -c` resumes the last session of this directory with
    # its memory intact, so a retry continues the task instead of restarting it.
    run_engine kimi -p "$OPENCODE_INPUT" -m "$KIMI_MODEL" --output-format text $KIMI_EXTRA_FLAGS \
      < /dev/null > "$tmpfile" 2> "$errfile" || engine_exit=$?
    kimi_attempt=1
    kimi_backoff="$KIMI_BACKOFF_S"
    kimi_fast_fails=0
    kimi_daily_quota=""
    # Retry forever on transient provider errors; stop only for the daily free
    # quota: either the error text names it, or KIMI_FAST_FAILS retries in a row
    # died within KIMI_FAST_FAIL_S seconds (a per-minute limit clears after one
    # backoff; a dead daily quota fails every retry instantly).
    while [ "$engine_exit" -ne 0 ] \
          && grep -qiE '429|rate_limit|rate limit|empty response|overloaded|502|503|timed out|ECONNRESET' "$errfile"; do
      if grep -qiE 'per-day|per day|daily|quota' "$errfile"; then
        kimi_daily_quota="provider said: $(grep -oiE 'free-models-per-day[a-z-]*|rate limit exceeded[^.]*' "$errfile" | head -1)"
        break
      fi
      if [ "$KIMI_RETRIES" -gt 0 ] && [ "$kimi_attempt" -ge "$KIMI_RETRIES" ]; then break; fi
      kimi_attempt=$((kimi_attempt + 1))
      echo -e "${YELLOW}  ⏳ kimi hit a provider limit — resuming the session in ${kimi_backoff}s (attempt ${kimi_attempt})${NC}"
      sleep "$kimi_backoff"
      engine_exit=0
      kimi_retry_start=$(date +%s)
      run_engine kimi -p "$KIMI_RESUME_INPUT" -c -m "$KIMI_MODEL" --output-format text $KIMI_EXTRA_FLAGS \
        < /dev/null >> "$tmpfile" 2> "$errfile" || engine_exit=$?
      kimi_retry_ran=$(( $(date +%s) - kimi_retry_start ))
      if [ "$engine_exit" -ne 0 ] && [ "$kimi_retry_ran" -lt "$KIMI_FAST_FAIL_S" ]; then
        # Died at once: no work happened, the limit is still on — wait longer.
        kimi_fast_fails=$((kimi_fast_fails + 1))
        kimi_backoff=$((kimi_backoff * 2))
        [ "$kimi_backoff" -gt "$KIMI_BACKOFF_MAX_S" ] && kimi_backoff="$KIMI_BACKOFF_MAX_S"
      else
        # It worked for a while (or finished): the limit had cleared, so the
        # next wait starts from the base again instead of the last wait.
        kimi_fast_fails=0
        kimi_backoff="$KIMI_BACKOFF_S"
      fi
      if [ "$kimi_fast_fails" -ge "$KIMI_FAST_FAILS" ]; then
        kimi_daily_quota="${KIMI_FAST_FAILS} retries in a row died within ${KIMI_FAST_FAIL_S}s"
        break
      fi
    done
  else
    # codex exec: one task then exit; final answer -> stdout, progress -> stderr.
    codex_args=(exec --skip-git-repo-check --sandbox workspace-write
      -c 'sandbox_workspace_write.network_access=true'
      -m "$CODEX_MODEL" -c "model_reasoning_effort=\"$CODEX_EFFORT\"")
    # stdin MUST be /dev/null. `codex exec` inherits the parent's stdin otherwise:
    # interactively that is a TTY and it works, but under a detached tmux job with
    # output redirected it died with exit 141 (SIGPIPE), and with an inherited pipe
    # it simply HUNG until killed. Measured 2026-09-10: same invocation, stdin
    # inherited = exit 124 after a 90 s timeout; stdin </dev/null = exit 0.
    run_engine codex "${codex_args[@]}" "$CODEX_INPUT" < /dev/null > "$tmpfile" 2> "$errfile" || engine_exit=$?
  fi

  # Restore the journal if the engine overwrote rather than appended: the last
  # non-empty line of the snapshot must still be present in the new file.
  if [ -n "$progbak" ] && [ -s "$progbak" ] && [ -f progress.txt ]; then
    last_prev=$(grep -v '^[[:space:]]*$' "$progbak" | tail -1)
    if [ -n "$last_prev" ] && ! grep -qF "$last_prev" progress.txt; then
      { cat "$progbak"; echo ""; cat progress.txt; } > progress.txt.restored \
        && mv progress.txt.restored progress.txt
      echo -e "${YELLOW}⚠️  progress.txt was overwritten, not appended — history restored by host${NC}"
    fi
  fi
  [ -n "$progbak" ] && rm -f "$progbak"

  # Timed out? 124 = timeout sent TERM, 137 = it had to KILL. Either way the
  # engine produced nothing usable: journal it and end the batch so a human (or
  # the supervisor's backoff) decides what runs next.
  if [ "$engine_exit" -eq 124 ] || [ "$engine_exit" -eq 137 ]; then
    echo ""
    echo -e "${RED}${BOLD}  ⏱️  RALPH ITERATION TIMEOUT — ${ENGINE} produced nothing within ${RALPH_ITER_TIMEOUT}.${NC}"
    echo -e "${RED}  Killed it. The ledger was not touched; the working tree is whatever the engine left.${NC}"
    {
      echo ""
      echo "$(date '+%Y-%m-%d %H:%M'): RALPH ITERATION TIMEOUT — ${ENGINE} was killed after ${RALPH_ITER_TIMEOUT} with no result (iteration $i). Ledger untouched."
    } >> progress.txt
    echo -e "${GREEN}📊 $(./progress.sh)${NC}"
    exit 1
  fi

  iter_end=$(date +%s)
  iter_time=$((iter_end - iter_start))
  total_iteration_time=$((total_iteration_time + iter_time))
  completed_iterations=$((completed_iterations + 1))

  if [ "$ENGINE" = "claude" ]; then
    if jq -e . "$tmpfile" > /dev/null 2>&1; then
      result_text=$(jq -r '.result // "No result"' "$tmpfile")
      cost=$(jq -r '.total_cost_usd // 0' "$tmpfile")
      input_tokens=$(jq -r '.usage.input_tokens // 0' "$tmpfile")
      cache_read=$(jq -r '.usage.cache_read_input_tokens // 0' "$tmpfile")
      cache_create=$(jq -r '.usage.cache_creation_input_tokens // 0' "$tmpfile")
      output_tokens=$(jq -r '.usage.output_tokens // 0' "$tmpfile")
      iter_context=$((input_tokens + cache_read + cache_create))

      total_cost=$(echo "$total_cost $cost" | awk '{printf "%.4f", $1 + $2}')
      total_input_tokens=$((total_input_tokens + iter_context))
      total_output_tokens=$((total_output_tokens + output_tokens))

      echo "$result_text"
      echo ""
      echo -e "${BLUE}───────────────────────────────────────────────────────────${NC}"
      echo -e "${BLUE}  🔢 CONTEXT: ${BOLD}${iter_context}${NC}${BLUE} tokens (in=${input_tokens} cache_read=${cache_read} cache_create=${cache_create})${NC}"
      echo -e "${BLUE}  📤 OUTPUT:  ${BOLD}${output_tokens}${NC}${BLUE} tokens${NC}"
      echo -e "${BLUE}  💰 COST:    ${BOLD}\$${cost}${NC}"
      echo -e "${BLUE}───────────────────────────────────────────────────────────${NC}"
    else
      echo -e "${YELLOW}Warning: Could not parse JSON output${NC}"
      cat "$tmpfile"
    fi
  else
    # Codex-family engines: stdout IS the final answer; no usage JSON in text mode.
    cat "$tmpfile"
    if [ "$engine_exit" -ne 0 ]; then
      echo ""
      if [ "$ENGINE" = "kimi" ] && [ -n "$kimi_daily_quota" ]; then
        # Not a failure of the task: the free tier's daily request quota is
        # gone. Say when it comes back and how to resume; skip the stderr tail.
        now_utc=$(date -u +%s)
        reset_utc=$(( (now_utc / 86400 + 1) * 86400 ))
        reset_local=$(date -r "$reset_utc" '+%H:%M %Z' 2>/dev/null || date -d "@$reset_utc" '+%H:%M %Z' 2>/dev/null || echo "00:00 UTC")
        left=$(( reset_utc - now_utc ))
        dirty=$(git status --porcelain 2>/dev/null | grep -c '^' || echo 0)
        echo -e "${RED}${BOLD}  ⛔ Kimi free tier: daily request quota reached${NC}"
        echo -e "${RED}     ${kimi_daily_quota}${NC}"
        echo -e "${YELLOW}     Resets at 00:00 UTC = ${reset_local} local, in $((left / 3600))h$(( (left % 3600) / 60 ))m.${NC}"
        echo -e "${YELLOW}     Work in progress is kept in the working tree: ${dirty} file(s) modified, nothing committed, nothing lost.${NC}"
        echo -e "${YELLOW}     Resume after the reset with:  ./ralph-kimi.sh $(( $1 - i + 1 ))   (the engine continues from the tree)${NC}"
        echo -e "${YELLOW}     Or now, on a paid alias:      KIMI_MODEL=openrouter/z-ai/glm-5.3 ./ralph-kimi.sh $(( $1 - i + 1 ))${NC}"
      else
        echo -e "${RED}${BOLD}  🚨 ${ENGINE} exited with code ${engine_exit} — stderr tail:${NC}"
        tail -20 "$errfile"
      fi
      echo -e "${GREEN}📊 $(./progress.sh)${NC}"
      exit 1   # let ralph-continuous.sh back off
    fi
    echo ""
    if [ "$ENGINE" = "opencode" ]; then
      echo -e "${DIM}  💰 usage/cost: n/a inline — run 'opencode stats' for token totals${NC}"
    elif [ "$ENGINE" = "kimi" ]; then
      echo -e "${DIM}  💰 usage/cost: n/a inline — billed by the provider behind ${KIMI_MODEL}${NC}"
    else
      echo -e "${DIM}  💰 usage/cost: n/a (${ENGINE} engine — codex exec text mode reports no usage)${NC}"
    fi
  fi

  # Host commit backstop: if the engine's sandbox couldn't commit, do it here.
  if [ -d .git ] && command -v git >/dev/null 2>&1 && [ -n "$(git status --porcelain 2>/dev/null)" ]; then
    commit_msg=$(grep -E '^[0-9]{4}-[0-9]{2}-[0-9]{2}' progress.txt 2>/dev/null | tail -1 | head -c 72)
    git add -A >/dev/null 2>&1 || true
    git commit -m "${commit_msg:-ralph: host auto-commit after iteration $i}" >/dev/null 2>&1 \
      && echo -e "${DIM}📦 Host auto-commit: ${commit_msg:-iteration $i}${NC}" || true
  fi

  echo ""
  echo -e "${YELLOW}⏱  Iteration $i took ${BOLD}$(format_time $iter_time)${NC}"
  echo -e "${GREEN}📊 $(./progress.sh)${NC}"

  if grep -q "<promise>COMPLETE</promise>" "$tmpfile"; then
    # The engine's claim of completeness only stands if the HOST agrees.
    if ! run_host_verify; then
      echo ""
      echo -e "${RED}${BOLD}  🚫 Engine claims COMPLETE but host verify FAILED — banner withheld.${NC}"
      echo -e "${RED}  An URGENT task was injected; the next batch will pick it up first.${NC}"
      echo -e "${GREEN}📊 $(./progress.sh)${NC}"
      exit 1
    fi
    overall_end=$(date +%s)
    overall_time=$((overall_end - overall_start))
    avg_time=$((total_iteration_time / completed_iterations))
    echo ""
    echo -e "${MAGENTA}${BOLD}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${MAGENTA}${BOLD}  🎉 PRD COMPLETE after $i iterations!${NC}"
    echo -e "${MAGENTA}${BOLD}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${MAGENTA}  ⏱  Overall time: ${BOLD}$(format_time $overall_time)${NC}"
    echo -e "${MAGENTA}  ⏱  Average per iteration: ${BOLD}$(format_time $avg_time)${NC}"
    echo -e "${BLUE}  🔢 Total context: ${BOLD}${total_input_tokens}${NC}${BLUE} tokens (claude iterations only)${NC}"
    echo -e "${BLUE}  📤 Total output: ${BOLD}${total_output_tokens}${NC}${BLUE} tokens (claude iterations only)${NC}"
    echo -e "${BLUE}  💰 Total cost: ${BOLD}\$${total_cost}${NC}"
    echo -e "${GREEN}  📊 $(./progress.sh)${NC}"
    exit 0
  fi

  # Stall guard: a task that cannot be verified is left open on purpose, which
  # means the next iteration picks up the SAME task. Without this the loop burns
  # tokens all night re-attempting a blocked task. Two iterations with no ledger
  # progress = stop and let a human look.
  now_passed=$(count_passed)
  if [ "$now_passed" = "$prev_passed" ]; then
    stall_streak=$((stall_streak + 1))
  else
    stall_streak=0
  fi
  prev_passed="$now_passed"
  if [ "$stall_streak" -ge "$STALL_LIMIT" ]; then
    echo ""
    echo -e "${RED}${BOLD}  ⛔ RALPH BLOCKED — ${stall_streak} iterations with no ledger progress.${NC}"
    echo -e "${RED}  The current task is not passing and is not being flipped. Read the tail of${NC}"
    echo -e "${RED}  progress.txt for the exact command and error that stopped it.${NC}"
    echo -e "${GREEN}📊 $(./progress.sh)${NC}"
    {
      echo ""
      echo "$(date '+%Y-%m-%d %H:%M'): RALPH BLOCKED — ${stall_streak} iterations with no ledger progress (engine ${ENGINE}). Batch stopped for a human."
    } >> progress.txt
    exit 2
  fi
done

# Batch ended without COMPLETE: verify anyway so drift is caught (and an URGENT
# task injected) as early as possible. Informational — the batch itself succeeded.
run_host_verify || true

overall_end=$(date +%s)
overall_time=$((overall_end - overall_start))
avg_time=$((total_iteration_time / completed_iterations))

echo ""
echo -e "${MAGENTA}${BOLD}═══════════════════════════════════════════════════════════${NC}"
echo -e "${MAGENTA}${BOLD}  Completed $1 iterations${NC}"
echo -e "${MAGENTA}${BOLD}═══════════════════════════════════════════════════════════${NC}"
echo -e "${MAGENTA}  ⏱  Overall time: ${BOLD}$(format_time $overall_time)${NC}"
echo -e "${MAGENTA}  ⏱  Average per iteration: ${BOLD}$(format_time $avg_time)${NC}"
echo -e "${BLUE}  🔢 Total context: ${BOLD}${total_input_tokens}${NC}${BLUE} tokens (claude iterations only)${NC}"
echo -e "${BLUE}  📤 Total output: ${BOLD}${total_output_tokens}${NC}${BLUE} tokens (claude iterations only)${NC}"
echo -e "${BLUE}  💰 Total cost: ${BOLD}\$${total_cost}${NC}"
echo -e "${GREEN}  📊 $(./progress.sh)${NC}"
