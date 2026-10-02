---
round: 1
builder_session: Fresh Claude Code session (Opus 5.5) in /Users/fcavalcanti/dev/solvr, plan mode, 2026-10-02 ~14:40–15:00 -03. Read-only verification only; nothing written outside this plan file.
---

# Plan r1 — Solvr v1.3 orchestrator: lanes, dispatch prompts, validation, deploy track

> Destination: `docs/handovers/2026-10-02-solvr-orchestrator/PLAN-r1.md`. Plan mode allowed writing only this file. After approval I copy it there. Note: the main tree belongs to the loop. Its `git add .` will sweep the copy into the loop's next commit. To avoid that, Felipe can ask me to land it with a path-only commit during a pause (Q5).

## Context
The author session handed over at 92% context. Local v1.3 is 304 commits ahead of production. The ralph loop is building the ledger one task at a time at about 25 minutes per slice. Some tasks can only close after the v1.3 deploy (24, 67, 94, 95). The goal is to go faster. Small tasks and the deploy rehearsal can run beside the loop, in isolated worktrees, on other quota buckets. I validate what comes back, and the gated cutover gets v1.3 into production. I plan, dispatch, validate and merge on Felipe's word. I do not write product code.

## Blocking constraints, restated
1. **Production is Felipe's, one step at a time.** I make no production write, push or deploy without Felipe's explicit "yes" for that exact step. That covers migrations on prod, `cmd/cutover --confirm-prod` against prod, `/admin/*` calls, `git push`, the two `SOLVR_DEPLOY_*` webhooks and IPFS unpin/gc. A yes for one gate never carries to the next. Rehearsals on local copies of dumps need no gate.
2. **The main tree `/Users/fcavalcanti/dev/solvr` is the loop's.** The loop's prompt runs `git add .` and the host backstop runs `git add -A` (`ralph.sh:469`), so any file left there lands in a loop commit. Executors work only in `git worktree`s outside that path, on their own branches and their own scratch databases. I touch the main tree only when Felipe asks, only during a loop pause, and commit path-only (`git commit -- <files>`). That covers journal notes, ff-merges and the plan file.
3. **One task, one worker.** The loop takes the first `passes:false` task from the top. It skips tasks only because journal notes tell it to (precedent: idx 24/67/68, skipped in every entry since `progress.txt:3836`). A task an executor owns must be named in an append-only `progress.txt` note before dispatch. `passes` flips only after validation and merge. Every ledger edit is Felipe's.
4. **Test output.** Save the FULL output to a log and read only `tail -n 10`. On a failure, grep the failing names and assertion lines from that saved log. Never re-run a suite to see more, and never stream `-v` output into context. Every dispatch prompt below carries this rule verbatim.
5. **The loop and the machine.** I never start, stop or relaunch a ralph runner, and that includes running one to read its banner. Felipe runs it. A stop means `tmux kill-session` **plus** killing any `claude` whose cwd is the repo, because the child survives as an orphan. Keep the lid open and the charger in, because a clamshell sleep froze a run for 8h42m.

## Handover discrepancies
Checked 2026-10-02 at 14:43–14:50 -03, read-only. Mismatches first.

1. **HEAD moved:** `b4cec099`, not `9d16ecbb`, and **304** commits ahead of `origin/main` `c03734ae`, not 299 [REAL: `git rev-parse`, `git rev-list --count`]. This is expected drift: loop slices 28–31 plus the handover commit landed after the handover was written.
2. **Nav regression is narrower than stated.** Production's header has DATA and SKILL as **top-level** links [REAL: `git show origin/main:frontend/components/header.tsx` lines 55, 73]. v1.3 did not remove them entirely. `/skill` is in the header's DOCS dropdown (`header.tsx:17`). `/data` and `/skill` are both in the footer (`footer.tsx:12,19,113,114`). The real change is that both left the **top-level** row, which `header.tsx:9-14` documents as deliberate. Two tests pin it: `header.test.tsx:87` ("exposes exactly three top-level destinations on desktop") and `:250` (mobile). The fix is therefore "restore DATA/SKILL as top-level header items". It means amending idx 95 step 4 and replacing those two tests with named ones. Production `/data` and `/skill` both answer 200 [REAL: curl].
3. **A post-purge base already exists locally.** It exists as databases, not as a dump file [REAL: `psql` counts]. `solvr_rehearsal_purge` has 632 posts and no `schema_migrations`, which is production's shape at 84. `solvr_rehearsal_purge113` is the same data at 113 (clean), and the journal calls it "production after the purge" (`progress.txt:2890`). Lane D rehearses on it now. The fresh production dump (G2) is still mandatory before the window.
4. **Loop launch line:** the live tmux command also carries `BATCH_PAUSE_MINS=2` [REAL: `ps`]. The file's `BATCH_PAUSE_MINS=10` still wins, and the log says "Pausing 10 min" [REAL]. The memory file `owner-decisions-2026-09-30.md` still says the pause is 2 minutes. That is stale; I will fix it on Felipe's word.
5. **Pace:** batch #12 took 9m37s, against the handover's ~17 min [REAL: `ralph-claude.log`]. Slice length varies.
6. **Quota moved** [REAL, gauge as of 14:35]: CLAUDE 7d **93%** (handover: 92%), CLAUDE 5h 19% (resets 15:50), FABLE 7d 21%, GPT 7d 18% (resets Thu), **GPT 5h 0%** (handover: 46%), OpenRouter about $0.
7. **Not mentioned in the handover:** a worktree `.claude/worktrees/seo-test` on `seo-test-force-dynamic`, already merged into main and git-ignored, plus the branches `worktree-agent-ad0a0f28` and `anti-abuse-wip`. Not mine; I leave them alone.

**Checked and clean:**
- `./progress.sh` reads `76/99 (76%) · 82 UAT pending`.
- The false tasks are exactly 24, 67, 68, 78–95, 97 and 98 (23 tasks, `jq` on `spec.json`, a 99-element array).
- The loop is on idx 78, slice 31 journaled at "14:50" (`progress.txt:12431`). Slice 32 and the findings are still open.
- tmux `ralph` is up and the supervisor PID is alive.
- The highest migration is `000135_room_notification_events`, while `cmd/cutover/main.go:45` still defaults `expect-version` to `132`.
- `.env.ralph.local` has `MODEL=opus`, `BATCH_SIZE=1`, `VERIFY_CMD="cd backend && go build ./..."`, and `DATABASE_URL` pointing at `solvr_test`.
- `codex` is v0.160.0, logged in through ChatGPT. I did not smoke-run it, to save quota; Lane B's step 0 does that.

**Not re-verified, on purpose:**
- Production schema 84 and the trigger. Checking would need a production read, which is gate P1.
- The `opus` alias resolving to `claude-opus-5-5`. Checking would cost a `claude -p` call.

## Acceptance checklist — point-by-point
1. **Five constraints restated** → section above, in my own words, with evidence.
2. **State re-verified** → the discrepancies section covers ledger 76/99, the loop on idx 78 (slice 31), HEAD `b4cec099`, migration 135 against the cutover default 132, and quota at 93/21/18. I re-run this same check list at every dispatch (Step 0 below).
3. **Lanes defined** → table in "Lanes". Loop L, deploy track D, parallel lanes B (now) and N and C (later), each with tasks, executor, engine and a quota estimate.
4. **Keeping the loop off, and merge-back** → "Journal note" gives the exact text. "Merge-back protocol" covers branch, validation, ff-only merge in a loop pause, and the passes flip by Felipe.
5. **Paste-ready prompts** → "Dispatch prompts" has D, B, N and C. Each carries the worktree, scratch DB, test-output rule, no-production/no-push rule, deliverable and report format.
6. **Validation protocol** → "Validation protocol": the logs I read, the by-name comparison against the named baseline, and the reject list.
7. **Deploy track** → "Deploy track". F0–F3 are local prep, G1–G7 and P7/P8 are each a Felipe gate. It includes the cutover-version fix (D2) and the nav fix (Lane N, pending the idx 95 decision).
8. **No collision, no product code by me** → executors work outside the main tree. The reserved tasks are journaled before dispatch. I touch the main tree only for path-only journal/plan commits and ff-merges, each on Felipe's yes and in a pause. Every code change is an executor's.

## The plan

### Lanes
| Lane | Tasks | Executor | Engine (FELIPE'S CALL; my recommendation) | Quota estimate [INFERRED] |
|---|---|---|---|---|
| **L — loop** | 78 (slice 32 + findings), then 79 harness parts, then the growth code parts 86/88/90/91/92 once lane C owns SEO | ralph in tmux, untouched | Opus, as now. **Opus 7d is at 93%.** Without Felipe's reset, the loop hits 100% within about 8–10 h. | About 0.3 pt of Claude 7d per slice (92→93 over about 76 min, at 1-pt gauge resolution). About 20 slices left before the wall. |
| **D — deploy track** | 93 rehearsal at head; D2 cutover-version fix; secret scan; named baseline at head; later the freeze checks | Fresh Claude Code session in worktree `lane-d` | Fable 5.1 (`claude --model claude-fable-5-1`). Its bucket is at 21%. Opus if Felipe resets. | About 5–10 pts of Fable 7d for one 3–5 h session |
| **B — small fixes** | 97 (claim-referral mount), 98 (time-window tests) | Codex in worktree `lane-b` | Codex `gpt-5.6-sol`, effort high. GPT 5h is at 0%, 7d at 18%. Fallback: Fable. | About 10–20% of one GPT 5h window, 2–4 pts of GPT 7d |
| **N — nav** (after Felipe amends idx 95) | 95 step 4 only: DATA/SKILL top-level links | Short Claude session in worktree `lane-n` | Fable or Sonnet | 1–2 pts |
| **C — SEO** (batch 2) | 80 → 81 → 82 → 83 (code part) → 84, one task per merge | Codex or Claude in worktree `lane-c` | Codex if lane B proves it, else Fable | The largest after the loop. Decide after measuring B's real burn. |

- **Concurrency:** at most 3 builders at once: the loop, plus two lanes. Batch 1 is D + B. N reuses B's slot when B is done. C dispatches only after batch 1 is validated, and **before the loop closes 79**, so the SEO reservation lands first.
- **Measuring burn:** I read the gauge (`curl -s http://127.0.0.1:8765/v1/usage.txt`) at each dispatch and at each report. Per-lane burn is then measured instead of guessed.
- **Not laned:**
  - 24, 67, 94, 95 (rest) need a live deploy; they get planned after G7.
  - 68 is deferred by Felipe.
  - 85, 87, 89 need weeks of data.
- **The deploy does not wait for SEO.** Legacy URLs already 308 to `/posts` (`frontend/middleware.ts:7-70`). After the cutover, deploys go back to normal incremental push + webhook.

### Journal note (Felipe: append it yourself, or say "append" and I do it in a pause with a path-only commit)
Batch 1, appended before D and B are dispatched:
```
2026-10-02 HH:MM — OWNER NOTE (orchestrator lanes) — not a task entry
idx 97 and idx 98 are OWNED by executor lane B (branch lane/b-97-98, worktree /Users/fcavalcanti/dev/solvr-lanes/lane-b).
idx 93 and idx 95 are OWNED by the deploy track (lane D, branch lane/d-deploy-track, worktree /Users/fcavalcanti/dev/solvr-lanes/lane-d).
The loop must SKIP idx 93, 95, 97 and 98: do not edit their code or tests, do not flip their passes. They return through
orchestrator validation and an owner merge; the owner flips passes. All other tasks keep their order. A later OWNER NOTE releases them.
```
Batch 2, appended before lane C is dispatched:
```
2026-10-02 HH:MM — OWNER NOTE (orchestrator lanes) — not a task entry
idx 80, 81, 82, 83 and 84 (the SEO block) are OWNED by executor lane C (branch lane/c-seo, worktree /Users/fcavalcanti/dev/solvr-lanes/lane-c).
The loop must SKIP them; after idx 79 continue with 86, 88, 90, 91, 92 (their buildable code parts). The owner flips passes after merge.
```

### Setup (on Felipe's yes; git metadata and local databases only, no product code)
- **Worktrees:** `git -C /Users/fcavalcanti/dev/solvr worktree add -b lane/<x> /Users/fcavalcanti/dev/solvr-lanes/lane-<x> <BASE>`. This writes only `.git/` and the new directory, never the loop's working files.
- **Databases:** `docker exec solvr-postgres createdb -U solvr solvr_lane_<x>`, then `migrate -path backend/migrations -database "postgres://solvr:solvr_dev@localhost:5435/solvr_lane_<x>?sslmode=disable" up` from the worktree. These are the local dev credentials already printed in the public journal.
- **Logs:** `/tmp/solvr-lane-<x>/`. **Ports:** loop smoke tests use 180xx, so B gets 18200–18249, D 18300–18349, C 18400–18449 plus frontend dev 3400, and N gets 3500.
- **Codex sandbox:** a worktree's git dir lives in the main repo, so Codex needs `--add-dir /Users/fcavalcanti/dev/solvr/.git` to commit [UNVERIFIED; B's step 0 proves it].

### Merge-back protocol
1. The executor rebases its branch onto the current `main` inside its own worktree. It re-runs its focused tests once after the rebase and reports with the base SHA.
2. I validate (next section). On a rejection, the lane gets a numbered list of fixes.
3. Felipe says "merge lane X".
4. I wait for the loop's pause (`ralph-claude.log` shows "⏸️ Pausing") and check the tree is clean (`git status --porcelain` is empty), that no ralph `claude` child is running, and that `main` is still the branch's base. Then I run `git merge --ff-only lane/<x>`. If `main` moved, the lane goes back to step 1; I don't rebase executor work myself.
5. I give Felipe the lane's draft journal entry. Felipe flips `passes` (or tells me to). Then I land the journal entry and the passes flip with one path-only commit, `git commit -m "chore(ledger): close idx N after lane X validation" -- spec.json progress.txt`, in the same pause.
6. If a release note is needed (for example "idx 97/98 closed"), it is appended in the same commit.

### Validation protocol (what I check; any ✗ means reject)
- **Scope:**
  - `git log --oneline <base>..lane/<x>` has one-line messages and no tags.
  - `git diff --stat <base>..lane/<x>` touches only the paths the task needs.
  - The diff touches none of: `spec.json`, `progress.txt`, `.env*`, `ralph*`, `db-backups/`, `*.dump`.
  - `./scripts/check-file-size.sh` passes, or no changed Go/TS file exceeds about 900 lines.
  - `gofmt -l` is empty on the changed Go files.
- **Tests weren't weakened:** `git diff <base>..lane/<x> -- '*_test.go' '*.test.ts' '*.test.tsx'` must remove no `func Test`/`it(`/`expect` line without a named replacement shown passing. No new `t.Skip`. No loosened assertion.
- **TDD proof:** a RED log, recorded before the code change, that fails on the new test, then a GREEN log.
- **Logs, read by me** with `tail -n 10` plus the standard failure grep, never in full:
  - the focused log;
  - the one full backend run (`go test -p 1 -count=1 ./...` on the lane's database);
  - the frontend run, when frontend was touched.
- **Named baseline, by test name:**
  - `grep -E '^--- FAIL' <full log> | awk '{print $3}' | sort -u` is compared against the baseline file.
  - Until D1 lands, the baseline is the handover's 46 names: the 44 `referral_code` fixture tests across the nine families listed, `TestSearch_MinSimilarity_HonestFilter`, and `TestSearchAnalyticsRepository_GetTrending`.
  - After D1, the baseline is `/tmp/solvr-lane-d/baseline-fail-names.txt` at its SHA. After lane B merges, its two names leave the baseline.
  - **Any failing name outside the baseline means reject.**
  - Any `--- SKIP` among the lane's own new tests means reject.
- **Independent check:** I re-run **only the lane's new focused tests**, once, in its worktree against its database, into my own log. This proves the reported GREEN is real. It is not a re-run to see more output.
- **Claims:** every claim in the report is labeled [REAL], [TEST] or [UNVERIFIED]. An unlabeled "works", or a [TEST] backed by a skipped run, means reject.
- **Rejected work** goes back with numbered "change X because Y" fixes, never vibes.

### Deploy track (every production step is a separate Felipe gate)
**Local prep (no gate):**
- **F0. Nav decision (FELIPE).** Amend idx 95 step 4 to allow DATA and SKILL as top-level header items. Lane N then builds it, and it merges before the freeze.
- **F1. Lane D, now:**
  - **D1:** named baseline at head.
  - **D2:** cutover-version fix. A guard test asserts that `--expect-version`'s default equals the highest `backend/migrations/*.up.sql` number, so it can never drift again. Then the bump to 135. Merged before the freeze.
  - **D3:** full rehearsal on a `solvr_rehearsal_purge` copy, 84 → 135 → `cmd/cutover` (dry-run, apply, second pass all zeros) → reconciliation → new-API smoke test → rollback 135 → 84 with post-cutover writes → old-code (`c03734ae`) smoke test.
  - **D4:** secret scan of `origin/main..HEAD`.
- **F2. Freeze (FELIPE picks the SHA):**
  - Recommended once idx 78 is closed and B, D2 and N are merged.
  - The frozen commit includes the frontend version bump. **The version number is Felipe's.**
  - Lane D runs a drift check: `git diff --stat <rehearsed>..<frozen> -- backend/migrations backend/internal/db/knowledge_cutover.go backend/cmd/cutover`. If it is non-empty, D3 is re-run at the frozen SHA. The D4 scan is re-run over the delta.
  - Both runs use one named full-suite baseline run at the frozen SHA.
- **F3. Readiness packet to Felipe:**
  - rehearsal numbers against the pass criteria (cutover PLAN-r1, criteria 1–9, at 135 on post-purge data);
  - the measured rollback losses;
  - the secret-scan result;
  - the window runbook with exact commands (secrets by name only).

**Production gates (each on its own explicit "yes"):**
- **G1 Push:** `git push origin <frozen-sha>:main`, after the D4 scan is clean.
- **P1:** read-only `SELECT`s through `/admin/query`: `rate_limit_config`, the trigger body, tombstone counts.
- **G2 Fresh dump:** the `docker exec` `pg_dump` route. Restore-test it into `solvr_lane_d_g2`, then re-run D3's migrate, cutover and reconciliation on it. That report is the G5 reference.
- **G3 Write pause:** Felipe stops `solvr-api` in EasyPanel.
  - **P5:** the collation runbook W6 (a)–(d) (`RUNBOOK-collation.md`), still inside G3.
- **G4 Schema:** a read-only `SELECT 1`, then `migrate force 84` and `up` to 135 against production. This includes 000114: the ban list and the versioned trigger (P6).
- **G5 Cutover:** `cmd/cutover --confirm-prod` against production. Its report must match G2, allowing only for writes made after the dump. Then a second pass must report all zeros.
- **G6 Deploy:**
  - `SOLVR_DEPLOY_API`, then `SOLVR_DEPLOY_WEB`.
  - Completion probes: `/v1/overview` returns 200, a behavior-change probe passes, and `skill.md` with a cache-buster shows new content.
  - The D3 smoke probes then run read-only against production.
- **G7 UAT** with Felipe. Rollback is Felipe's call, using the rehearsed down path.
- **P7/P8** (after G6, each separately): `POST /admin/ipfs/unpin`, then `/admin/ipfs/gc`, following `RUNBOOK-ipfs-unpin.md`.
- **After G7:** I plan the live-acceptance lane for 24, 67, 94 and 95. Its production writes (rooms and agents on prod) are gated too.

### Dispatch prompts (paste-ready; at dispatch I swap `BASE=` for the then-current main SHA)

#### Prompt D — deploy-track rehearsal (start: `cd /Users/fcavalcanti/dev/solvr-lanes/lane-d && claude --model claude-fable-5-1`)
```
You are LANE D, the Solvr v1.3 deploy-track executor. An orchestrator session validates your work; Felipe (the owner) gates every production step. You do LOCAL work only.

BASE=b4cec099   BRANCH=lane/d-deploy-track   WORKTREE=/Users/fcavalcanti/dev/solvr-lanes/lane-d   LOGS=/tmp/solvr-lane-d   PORTS=18300-18349
DB server: docker container solvr-postgres, postgres://solvr:solvr_dev@localhost:5435/<db>?sslmode=disable
Your databases: ONLY names starting solvr_lane_d_ (solvr_lane_d_base already exists, migrated to head). Create others with CREATE DATABASE <name> TEMPLATE <source>. Drop only yours, by exact name, when told.
Read-only templates (never connect for writes, never drop): solvr_rehearsal_purge (post-purge prod copy, schema 84, no schema_migrations), solvr_rehearsal_purge113, solvr_rehearsal_pristine, solvr_rehearsal_schema113. Never touch solvr, solvr_test (the build loop's) or any other database.

HARD RULES
1. NO PRODUCTION, NO PUSH. Never connect to production (api.solvr.dev, the SOLVR_DB_* keys, /admin/*, the SOLVR_DEPLOY_* webhooks). Never read or print .env values. No git push, no tags.
2. Never cd into, edit, build, test or commit in /Users/fcavalcanti/dev/solvr (the main tree). A build loop owns it and sweeps any stray file into its commits. Read-only git against it is fine (git -C /Users/fcavalcanti/dev/solvr log/show). Never edit spec.json or progress.txt; you draft journal text in your report.
3. TEST OUTPUT HARD RULE: redirect FULL output to a log under $LOGS, read ONLY `tail -n 10 <log>`. On a failure extract with `grep -E -- '--- FAIL|_test\.go:[0-9]+:|Error:|expected|actual' <log> | head -40`. Never re-run a suite to see more. Never stream `go test -v` into context.
4. Timeout-guard anything that can hang (`timeout <s> ...`, `curl --max-time 10`). Check a port is free before binding; kill only processes you started.
5. TDD: a RED log before the code change, then GREEN. Never delete, skip or loosen an existing test; if one must change, name it, quote its assertion, name its replacement and show it passing.
6. Commit in the worktree only, one-line messages (husky runs `cd frontend && npm run typecheck`, so `cd frontend && npm ci` once first). Label every claim [REAL], [TEST] or [UNVERIFIED]. If blocked, stop and report the exact command and exact error; never guess.

READ FIRST: CLAUDE.md; docs/handovers/2026-10-02-solvr-orchestrator/HANDOVER.md; docs/handovers/2026-09-29-solvr-v13-cutover/PLAN-r1.md (phases A–C and "Rehearsal pass criteria") and REVIEW-r1.md; docs/handovers/2026-09-29-solvr-anti-abuse/RUNBOOK-collation.md; backend/cmd/cutover/main.go; backend/internal/db/knowledge_cutover.go; /Users/fcavalcanti/.claude/projects/-Users-fcavalcanti-dev-solvr/memory/cutover-rehearsal-2026-09-29.md and spam-purge-2026-09-29.md.

D1 NAMED BASELINE at BASE. Once: `cd backend && DATABASE_URL=<solvr_lane_d_base url> go test -p 1 -count=1 -timeout 90m ./... > $LOGS/baseline-backend.log 2>&1`; tail -n 10. Write the sorted unique top-level failing names (`grep -E '^--- FAIL' | awk '{print $3}'`) to $LOGS/baseline-fail-names.txt. Compare with the handover's 46 (44 referral_code fixture tests in TestBriefing_*, TestCrystallization_*, TestGetHardcoreUnsolved_*, TestGetRecentVictories_*, TestGetRisingIdeas_*, TestGetTrendingNow_*, TestGetYouMightLike_*, TestGetPlatformPulse_*, TestUserRepository_*; TestSearch_MinSimilarity_HonestFilter; TestSearchAnalyticsRepository_GetTrending): report same / new / gone. Frontend once: `cd frontend && npm test > $LOGS/baseline-frontend.log 2>&1`; tail -n 10.

D2 CUTOVER VERSION FIX (code, TDD). backend/cmd/cutover/main.go:45 defaults --expect-version to 132; the highest migration is 000135. RED: a test in backend/cmd/cutover asserting the flag default equals the highest NNNNNN among backend/migrations/*.up.sql (so a future migration without a bump fails the suite). GREEN: bump the default to 135. Keep the runner failing safe on any mismatch. `go test ./cmd/cutover/` RED and GREEN logs. Commit.

D3 FULL REHEARSAL at BASE + D2, on post-purge data (pass criteria 1–9 of the cutover PLAN-r1, at 135 instead of 113):
 a. CREATE DATABASE solvr_lane_d_reh TEMPLATE solvr_rehearsal_purge. Snapshot to $LOGS/a2-snapshot.json: counts of posts, answers, approaches, responses, comments, votes, rooms, room tokens, users and agents with deleted_at NOT NULL (with their deleted_at values), and whether trigger users_refuse_tombstoned_email exists (with its body).
 b. `migrate ... force 84` then `up`: exit code, duration, version 135, dirty=false.
 c. Build cmd/cutover; run --dry-run, then --confirm-prod (against solvr_lane_d_reh ONLY; it is a local copy), then a second run: it must report all zeros. Save every JSON report.
 d. Reconcile: replies against the legacy contributions, every orphan listed by id and explained; vote drift 0 after rebuild; room ids, slugs and owners preserved; the 4 tombstoned users and 3 tombstoned agents keep their exact deleted_at; the trigger exists and refuses a tombstoned email (inside BEGIN ... ROLLBACK); /v1/stats contribution count against the legacy count, explained.
 e. New-API smoke: CREATE DATABASE solvr_lane_d_smoke TEMPLATE solvr_lane_d_reh; build ./cmd/api; run it under `env -i` with a throwaway JWT_SECRET of at least 40 chars (a 30-char secret leaves the /v1 routes unmounted) on a port in PORTS. Probe: /v1/overview 200, /v1/stats, /v1/posts, one post, /v1/search with 5 fixed queries, /v1/rooms plus one room's entries, and one legacy write route (it must answer 410). Then stop the server.
 f. Rollback: CREATE DATABASE solvr_lane_d_down TEMPLATE solvr_lane_d_reh. Insert post-cutover writes: one post, one native reply, one child reply, one room entry, and one vote on a reply. `migrate ... down` to 84. Record the exit code and dirty flag, the legacy counts against a2-snapshot, what rollback_archive holds for each write, and how many room tokens remain valid.
 g. Old-code smoke: `git -C /Users/fcavalcanti/dev/solvr worktree add --detach /Users/fcavalcanti/dev/solvr-lanes/lane-d-old c03734ae`. Build its API, run it against solvr_lane_d_down on another port in PORTS, and probe /v1/posts, one problem, one question, /v1/rooms and one room's messages. Any 500 is a finding. Afterwards, `git worktree remove` that worktree.
D4 SECRET SCAN, read-only: `git -C /Users/fcavalcanti/dev/solvr log -p origin/main..BASE > $LOGS/push-diff.log`; grep it for sk-, solvr_sk_, solvr_rm_, solvr_rt_, ghp_, github_pat_, gsk_, AKIA, "BEGIN .*PRIVATE KEY", password[:=], postgres://user:pass@ with a non-localhost host, added .env files, *.dump, db-backups/. For each hit report the commit, file and line, and whether it is a placeholder or test fixture. NEVER print a value that looks real.

DELIVERABLE: the branch with the D2 commit, rebased on the current main (re-run `go test ./cmd/cutover/` once after the rebase), and all logs and JSON under $LOGS. Keep solvr_lane_d_* for the orchestrator. Do not merge.
REPORT (paste back exactly this shape):
=== LANE D REPORT ===
branch / worktree / base sha (current main after rebase) / head sha
commits: git log --oneline <base>..HEAD
files: git diff --stat <base>..HEAD
D1..D4: DONE | PARTIAL | BLOCKED, with one labeled line each
logs: absolute paths (RED, GREEN, baseline backend and frontend, migrate, cutover JSON x3, smoke, rollback, old-code smoke, secret scan)
baseline: same / new names / gone names against the 46
rehearsal pass criteria 1-9: pass or fail each, with the number
rollback losses measured: list
secret-scan hits: list (no values)
tests changed or removed: none | <name>: asserted "<quote>", replaced by <name> (passing in <log>)
findings not fixed: list
databases created: list
DRAFT journal entry (text only; do NOT write progress.txt)
```

#### Prompt B — idx 97 + 98 (start: `codex -C /Users/fcavalcanti/dev/solvr-lanes/lane-b --add-dir /Users/fcavalcanti/dev/solvr/.git --add-dir /tmp/solvr-lane-b -s workspace-write -c sandbox_workspace_write.network_access=true -m gpt-5.6-sol -c model_reasoning_effort="high"`)
```
You are LANE B, a Solvr executor. An orchestrator session validates your work; Felipe (the owner) decides merges. You do LOCAL work only, in this worktree.

BASE=b4cec099   BRANCH=lane/b-97-98   WORKTREE=/Users/fcavalcanti/dev/solvr-lanes/lane-b   LOGS=/tmp/solvr-lane-b   PORTS=18200-18249
Your database: postgres://solvr:solvr_dev@localhost:5435/solvr_lane_b?sslmode=disable (already created and migrated to head). Export it as DATABASE_URL for every backend test. Never point at solvr, solvr_test or any other database: backend/internal/db tests DELETE rows from whatever DATABASE_URL names. newMigratedScratchDatabase creates and drops its own databases next to it; that is fine.

HARD RULES
1. NO PRODUCTION, NO PUSH. Never contact api.solvr.dev, /admin/*, the SOLVR_DB_* or SOLVR_DEPLOY_* values; never read or print .env values. No git push, no tags.
2. Never cd into, edit, build, test or commit in /Users/fcavalcanti/dev/solvr (the main tree, owned by a build loop). Never edit spec.json or progress.txt; draft the journal text in your report.
3. TEST OUTPUT HARD RULE: redirect FULL output to a log under $LOGS, read ONLY `tail -n 10 <log>`. On a failure extract with `grep -E -- '--- FAIL|_test\.go:[0-9]+:|Error:|expected|actual' <log> | head -40`. Never re-run a suite to see more. Never stream `go test -v` into context.
4. TDD: a RED log before the code change, then GREEN. Never delete, skip or loosen an existing test; if one must change, name it, quote its assertion, name its replacement and show it passing.
5. One-line commit messages, in this worktree only. Label every claim [REAL], [TEST] or [UNVERIFIED]. If blocked, stop and report the exact command and exact error; never guess an API.

STEP 0 (sandbox smoke): `git status`; `cd backend && go build ./... > $LOGS/s0-build.log 2>&1; tail -n 3 $LOGS/s0-build.log`; DB reachability, which also gives idx 98's pre-change state: `cd backend && go test -count=1 -run '^TestSearchAnalyticsRepository_GetTrending$' ./internal/db/ > $LOGS/s0-db.log 2>&1; tail -n 5 $LOGS/s0-db.log`. A pass or an assertion failure both prove the connection works; "connection refused", "operation not permitted" or a SKIP mean it is blocked. If git, the build or the database is blocked by the sandbox, STOP and report the exact error.
READ FIRST: CLAUDE.md (golden rules); spec.json idx 97 and idx 98 (read-only); backend/internal/db/scratch_database_test.go (newMigratedScratchDatabase); the router file that mounts /v1/auth/claim-referral; frontend/app/auth/callback/page.tsx (the exact request it sends).

TASK idx 97: POST /v1/auth/claim-referral is mounted on the bare /v1 router, outside every auth middleware, so auth.ClaimsFromContext is nil and it always returns 401. Mount it inside the JWT-authenticated group. RED first: a router-level test with a real JWT, in a NEW test file (do not edit router_test.go, which a parallel task is changing). In it, a human who signed up through OAuth claims a referral code and the referral row is recorded; the same call without a JWT returns 401; and the request matches the exact method, path, headers and body that frontend/app/auth/callback/page.tsx sends. Then GREEN.
TASK idx 98: move TestSearchAnalyticsRepository_GetTrending and TestSearch_MinSimilarity_HonestFilter onto newMigratedScratchDatabase, and pin their time windows so the result does not depend on the wall clock. Prove it: `go test -count=3 -run '^(TestSearchAnalyticsRepository_GetTrending|TestSearch_MinSimilarity_HonestFilter)$' ./internal/db/ > $LOGS/98-x3.log 2>&1` with all 3 runs passing.
THEN, once: the full backend suite `cd backend && go test -p 1 -count=1 -timeout 90m ./... > $LOGS/full-backend.log 2>&1`; tail -n 10; list the top-level failing names (`grep -E '^--- FAIL' | awk '{print $3}' | sort -u`). Known baseline: 44 referral_code fixture failures (TestBriefing_*, TestCrystallization_*, TestGetHardcoreUnsolved_*, TestGetRecentVictories_*, TestGetRisingIdeas_*, TestGetTrendingNow_*, TestGetYouMightLike_*, TestGetPlatformPulse_*, TestUserRepository_*) plus the two idx 98 tests, which must now PASS. If you changed any frontend file, run `cd frontend && npm ci && npm test > $LOGS/frontend.log 2>&1` once.
FINALLY: rebase onto the current main (`git fetch` is not needed, since main is in the same repo: `git rebase main`), then re-run only your focused tests once into $LOGS/after-rebase.log.

REPORT (paste back exactly this shape):
=== LANE B REPORT ===
branch / worktree / base sha (main after rebase) / head sha
commits: git log --oneline <base>..HEAD
files: git diff --stat <base>..HEAD
idx 97: DONE | PARTIAL | BLOCKED, with one labeled line
idx 98: DONE | PARTIAL | BLOCKED, with one labeled line
logs: absolute paths (s0, RED, GREEN, 98-x3, full-backend, frontend if any, after-rebase)
failing top-level names outside the baseline: none | list
tests changed or removed: none | <name>: asserted "<quote>", replaced by <name> (passing in <log>)
findings not fixed: list
DRAFT journal entries for idx 97 and idx 98 (text only; include "drop TestSearchAnalyticsRepository_GetTrending and TestSearch_MinSimilarity_HonestFilter from the known-failure baseline")
```

#### Prompt N — nav links (dispatch only after Felipe amends idx 95; start: `cd /Users/fcavalcanti/dev/solvr-lanes/lane-n && claude --model claude-fable-5-1`)
```
You are LANE N, a Solvr executor. LOCAL work only. An orchestrator validates; Felipe decides merges.
BASE=<main sha at dispatch>   BRANCH=lane/n-nav   WORKTREE=/Users/fcavalcanti/dev/solvr-lanes/lane-n   LOGS=/tmp/solvr-lane-n   PORT=3500
HARD RULES: no production, no push, no tags, never read or print .env values; never touch /Users/fcavalcanti/dev/solvr (the main tree, owned by a build loop) nor spec.json/progress.txt. TEST OUTPUT HARD RULE: full output to a log under $LOGS, read ONLY `tail -n 10`; on a failure grep the failing names and assertion lines from that log; never re-run a suite to see more. TDD: a RED log first. Never delete or loosen a test without naming it, quoting its assertion and naming its passing replacement. One-line commits in this worktree (run `cd frontend && npm ci` first: husky typechecks). Label claims [REAL]/[TEST]/[UNVERIFIED]. No backend needed, so no database.
TASK (owner decision, idx 95 amended: <paste Felipe's exact wording>): restore DATA (/data) and SKILL (/skill) as top-level header destinations on desktop and mobile, as production has them (`git show origin/main:frontend/components/header.tsx`, lines ~55 and ~73), keeping Rooms, Posts and Docs. Keep the footer as it is. Update the comment at header.tsx:9-14 to the new rule. The tests header.test.tsx:87 ("exposes exactly three top-level destinations on desktop") and :250 ("shows the same three top-level destinations in the same order") pin the old rule: replace each with a named test for the new set and order, quoting the old assertion in your report. RED, then GREEN; then `cd frontend && npm test > $LOGS/frontend.log 2>&1`, `npm run lint > $LOGS/lint.log 2>&1` and `npm run typecheck > $LOGS/tc.log 2>&1`, once each. Rebase onto main, then re-run header.test.tsx once.
REPORT: === LANE N REPORT === with branch, base, head, commits, files, logs, the replaced tests (old assertion → new name), findings, and a DRAFT journal entry (text only).
```

#### Prompt C — SEO block (batch 2; one task per report; start: Codex as in B with `lane-c`/`/tmp/solvr-lane-c`, or `claude --model claude-fable-5-1`)
```
You are LANE C, a Solvr executor for the SEO block. LOCAL work only. An orchestrator validates each task before you start the next; Felipe decides merges.
BASE=<main sha at dispatch>   BRANCH=lane/c-seo   WORKTREE=/Users/fcavalcanti/dev/solvr-lanes/lane-c   LOGS=/tmp/solvr-lane-c   PORTS=18400-18449, frontend dev 3400
Database (only if a backend test needs one): postgres://solvr:solvr_dev@localhost:5435/solvr_lane_c?sslmode=disable (created and migrated). Never point at solvr or solvr_test.
HARD RULES: no production, no push, no tags, never read or print .env values; never touch /Users/fcavalcanti/dev/solvr (the main tree, owned by a build loop) nor spec.json/progress.txt. TEST OUTPUT HARD RULE: full output to a log under $LOGS, read ONLY `tail -n 10`; on a failure `grep -E -- '--- FAIL|_test\.go:[0-9]+:|Error:|expected|actual|FAIL ' <log> | head -40`; never re-run a suite to see more. TDD: a RED log first. Never delete, skip or loosen a test without naming it, quoting its assertion and naming its passing replacement. API is smart, client is dumb (CLAUDE.md rule 3). No generateSitemaps() (it breaks standalone; see CLAUDE.md "Deployment Constraints"). Timeout-guard servers and curls. One-line commits in this worktree (`cd frontend && npm ci` first). Label claims [REAL]/[TEST]/[UNVERIFIED]. Steps that need Google tools (Rich Results Test, GSC) become "UAT:" lines for the owner, not passes.
READ FIRST: CLAUDE.md; SPEC.md's SEO sections; spec.json idx 80-84 (read-only); frontend/middleware.ts (legacy 308s already exist); frontend/app/sitemap.ts; progress.txt's mentions of "idx 83" (read-only, for the dead-caller notes).
ORDER: idx 80, then 81, then 82, then 83 (code part only: redirect map verification, 404/410 for missing or deleted posts, sitemap eligibility; the GSC export is the owner's), then 84. After EACH task: run its own Verify steps, the frontend suite once (`npm test > $LOGS/<idx>-fe.log 2>&1`), the backend suite once if you touched Go (`go test -p 1 -count=1 -timeout 90m ./... > $LOGS/<idx>-be.log 2>&1`), `npm run build > $LOGS/<idx>-build.log 2>&1` once, then rebase onto main, then STOP and report. Wait for "continue" before the next task.
REPORT per task: === LANE C REPORT idx <N> === with branch, base, head, commits, files, the Verify step results (labeled), logs, failing names outside the baseline (backend: the 46-name list minus anything the orchestrator says has closed; frontend: none), tests changed or removed with their replacements, UAT lines for the owner, findings, and a DRAFT journal entry (text only).
```

### Step 0 at every dispatch (the orchestrator, read-only)
- `./progress.sh`
- `git rev-parse --short HEAD; git rev-list --count origin/main..HEAD`
- The last journal header: `grep -nE '^20[0-9]{2}-' progress.txt | tail -1`
- `tail -n 3 ralph-claude.log`
- `ls backend/migrations | tail -2` against `grep -n expect-version backend/cmd/cutover/main.go`
- The quota gauge

Any drift goes into the dispatch message.

### First moves after APPROVED (= handover next_action)
1. Run Step 0 again.
2. Felipe answers Q1–Q5.
3. On his yes, I do the setup for D and B (worktrees, `solvr_lane_d_base`, `solvr_lane_b`), migrated to head.
4. In the next loop pause, the batch-1 journal note lands (by Felipe, or by me path-only on his yes).
5. Felipe gets Prompt D and Prompt B with `BASE=` filled in.

## Concerns
- **[HIGH] Opus 7d at 93%.** The loop alone reaches 100% in roughly 8–10 h [INFERRED], and then it spins on rate-limit backoff until Monday. The deploy window also needs orchestrator headroom. Felipe's reset call, or switching the loop's `MODEL` (his `.env.ralph.local` edit), decides the pace more than any lane plan.
- **[HIGH] The skip mechanism is a convention, not code.** `ralph.sh:234` says "FIRST task where passes is false". Skipping works only because the model reads the journal (precedent: 24/67/68). Mitigation: batch-1 reservations (93, 95, 97, 98) sit far down the order, and SEO is reserved before the loop closes 79. I check each loop journal entry's TASK SELECTION line after every batch.
- **[MEDIUM] Codex inside a worktree:** commits write to the main repo's `.git`, and `go`'s build cache lives outside the workspace [UNVERIFIED]. Step 0 of Prompt B detects this. Fallback: run lane B on Fable with Prompt B unchanged except the launch line.
- **[MEDIUM] Machine contention.** The loop, D and B run `go test -p 1` and `npm test` at the same time, plus the `vpd` loop in another repo. The frontend suite has a known parallel-timeout caveat. A single-file timeout re-run, once, is allowed and must be labeled.
- **[MEDIUM] Merge windows are 10-minute pauses on a moving `main`.** ff-only plus rebase-in-worktree keeps the main tree safe, but a lane may need more than one rebase round.
- **[LOW] Lane B touches the router while loop slice 32 edits `router_test.go`.** Prompt B forbids editing that file and requires a new test file.
- **[LOW] Stale artifacts:** the `seo-test` worktree, two old branches, and the memory note about the 2-minute pause. I leave them unless Felipe says otherwise.

## Questions for the author (Felipe approves directly if the author session is gone — handover Q3)
1. **Engines (FELIPE'S CALL):**
   - Will you reset the Anthropic quota?
   - If you do: the loop on Opus, D on Opus, B on Codex.
   - If you don't: the loop on Opus until the wall (or switched to Fable), D on Fable, B on Codex.
   - Is that allocation right? (This also answers handover Q1.)
2. **`vpd` loop** (handover Q2): leave it on Opus, or move it? It's your other repo.
3. **idx 95 amendment:** "restore DATA and SKILL as top-level header items next to Rooms, Posts, Docs; the footer unchanged". Is that the exact wording you want in step 4? Lane N waits for it.
4. **Freeze point:** OK to freeze when 78 is closed and B, D2 and N are merged, with SEO (80–84) shipping after the cutover as normal deploys? And which frontend version number does the freeze commit carry?
5. **Main-tree writes:** journal notes, the ff-merges and copying this plan into `docs/handovers/…/PLAN-r1.md` all need the main tree. Do you append and commit them yourself, or may I, path-only and only inside a loop pause, each time you say so?

## Owner decisions after approval (2026-10-02 14:56 -03, Felipe, in chat)
- The plan is APPROVED by Felipe directly (handover Q3). This session is now the primary orchestrator.
- The loop was stopped by Felipe on purpose: the log's last write was 14:42:53 and the tmux server is gone. Felipe restarts it himself.
- **Engines: Opus 5.5 for everything.** This replaces the lane table's recommendations. Every executor starts with `claude --model claude-opus-5-5` in its worktree; lane B is a Claude session, not Codex. Prompt B's STEP 0 stays as a plain environment check. The Codex launch line and the `--add-dir` concern no longer apply.
- **Main-tree writes:** the orchestrator does them, path-only, each on Felipe's yes, only while the loop is stopped or paused. The first one was this file plus the batch-1 journal note.
- **idx 95 step 4 amendment approved** with this wording: "restore DATA and SKILL as top-level header items next to Rooms, Posts, Docs; footer unchanged". The ledger edit is Felipe's. Lane N dispatches after it.
- Still open: Q2 (the vpd loop) and Q4 (the freeze point and version number).
