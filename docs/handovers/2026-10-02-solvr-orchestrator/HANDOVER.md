---
slug: solvr-orchestrator
date: 2026-10-02
status: open
round: 0
author_session: The Claude Code session that rehearsed the v1.3 cutover, ran the production bot purge, validated the anti-abuse build, and set up the ralph loop. Its context was at 92%, so it is handing over.
---

# Handover — Solvr: an ORCHESTRATOR to finish v1.3 and ship it, faster

## Mission

You are the **orchestrator**: planner and validator. **You do not write product code.** Your job:

1. **Understand** what production runs today and what local v1.3 is.
2. **Plan parallel lanes**: the running ralph loop, fresh Claude Code executor sessions in git worktrees, and Codex. Decide who builds what, on which engine, within the quota.
3. **Write the exact prompts** Felipe pastes into each executor. Felipe is the courier.
4. **Validate** what comes back: saved logs, diffs, and the named test baseline. Then merge, or send it back.
5. **Run the deploy track.** Get v1.3 safely to production through Felipe's gates.

**"Done" for you:** every task the loop and executors can build is built and validated, and v1.3 is deployed through the gated cutover. The deploy unblocks the tasks that need a live system.

## Where things stand (verified)

All `[REAL]` facts below were checked by the author session on 2026-10-02, around 13:35 -03, unless dated otherwise.

### Ledger and loop
- [REAL] `./progress.sh` reads `76/99 (76%) · 82 UAT pending`.
  - The ledger is `spec.json`, at the repo root, 99 tasks.
  - The journal is `progress.txt`; older entries are in `progress-archive.txt`, which is read-only.
- [REAL] The loop is running in tmux session `ralph`. Felipe started it with:
  ```
  cd /Users/fcavalcanti/dev/solvr && tmux new-session -d -s ralph 'caffeinate -dimsu env NO_PROGRESS_LIMIT=60 RALPH_ITER_TIMEOUT=60m ./ralph-claude-continuous.sh >> ralph-claude.log 2>&1'
  ```
  - It is on idx 78, with its latest journal entry at 13:20.
  - Model: `MODEL=opus`, which resolves to `claude-opus-5-5` [REAL: verified through the `claude -p` JSON `modelUsage`].
  - **The pause is 10 minutes.** `.env.ralph.local` has `BATCH_PAUSE_MINS=10`. The supervisor sources that file *after* the environment (`ralph-continuous.sh:24`), so the file wins over any env override.
  - The log is `ralph-claude.log`. `ralph-continuous.log` is a stale file from 09-25; ignore it.
- [REAL] **Pace:** about 17 minutes per iteration plus the pause; about 25–30 minutes per slice.
  - Big tasks take many slices: idx 52 took 8, idx 77 about 25, and idx 78 is at 20+.
  - `NO_PROGRESS_LIMIT` counts batches without a task **closing**, not batches without commits. The old limit of 20 stopped the loop mid-task on 10-02. It is 60 now.
- [REAL] **Remaining false tasks**, from `jq` on `spec.json`, and who can close them:

  | Who | Tasks |
  |---|---|
  | Loop-buildable | 78 (in progress), 97 (claim-referral mount), 98 (time-window search tests), 79 (ops gates: the harness parts) |
  | SEO, loop-buildable | 80, 81, 82, 84, and 83's redirect part |
  | Growth: code and instrumentation can be built; the outcomes need real data | 86, 88, 90, 91, 92 |
  | **Need a live deploy and real agents** | 24, 67, 94, 95 |
  | **Production cutover, Felipe's gates** | 93 |
  | **Deferred by Felipe** (drop the old tables later, all at once) | 68 |
  | **Need weeks of traffic or cohorts** | 85, 87, 89 |

  Expect the loop to stall around 85–88/99 until the deploy happens. This is an estimate.

### Production vs local
- [REAL] Production runs origin/main `c03734ae` at **schema 84**. It has no `schema_migrations` table.
  - It also has the hand-installed trigger `users_refuse_tombstoned_email`. Migration `000114` versions it, as Felipe decided.
  - Deploys are manual on EasyPanel, which builds from the **GitHub `main` archive**. The webhooks are `SOLVR_DEPLOY_API` and `SOLVR_DEPLOY_WEB` in `.env`; names only here.
- [REAL] Local `main` is `9d16ecbb`, **299 commits ahead** of origin/main. Nothing is pushed.
- [REAL] **Migrations run up to `000135`, but `cmd/cutover` defaults to `--expect-version 132`.** On production the runner would refuse to start. That fails safe, but it would block the deploy window. Bump it, or pass `--expect-version 135`.
- [REAL] **Rehearsals.** The full author rehearsal ran at schema 113 on a pre-purge dump. A loop slice ran the cutover on a post-purge copy at 131.
  - **No rehearsal exists at the current head.**
  - Local templates: `solvr_rehearsal_pristine` (pre-purge, schema 84) and `solvr_rehearsal_schema113`.
  - Dumps are in `db-backups/` (git-ignored). The newest is `solvr_prod_2026-09-29_19-28-15_prepurge.dump`; a **post-purge** dump does not exist yet.
- [REAL] **Production data after the 09-29 bot purge** (see `docs/handovers/2026-09-29-solvr-anti-abuse/`):
  - 632 posts.
  - 4 users and 3 agents tombstoned with soft deletes. Never hard-delete or un-delete them.
- [REAL] **Nav regression Felipe noticed.** v1.3 removed the `/data` and `/skill` links from `frontend/components/header.tsx` and `footer.tsx`.
  - Production's `header.tsx` and `footer.tsx` still have them.
  - The pages still exist in v1.3 (`frontend/app/data`, `frontend/app/skill`), and production serves both with 200.
  - The removal follows ledger idx 95 ("final navigation contains only Rooms, Posts, and Docs"). **Felipe wants the links back. That means amending idx 95, which is Felipe's call.** Plan it as a small executor task.

### Quota (shared across ALL sessions and loops)
- [REAL 13:19, from the ai-usage gauge `curl -s http://127.0.0.1:8765/v1/usage.txt`]

  | Bucket | Used | Resets |
  |---|---|---|
  | **Claude Opus 7d** | **92%** | Mon |
  | Fable 7d | 21% | Mon |
  | GPT/Codex 7d | 18% | Thu |
  | GPT 5h | 46% | — |
  | OpenRouter | about $0 | — |

  - Felipe says he can **reset** the Anthropic quota.
  - [REAL] `claude --model claude-fable-5-1 -p` works headless on its own bucket.
  - Codex had an unexplained `401 sk-svcacct…` in September (see the earlier cutover handover). Smoke-test codex before relying on it.
- [REAL] **A second loop shares the same quota:** `/Users/fcavalcanti/dev/m5/sticks3-vpd-gennie` (tmux `vpd`, a different repo), with routes defaulting to `claude-opus-5-5`.

### Test baseline (named)
- [TEST] A fresh database, `go test -p 1` at the anti-abuse merge, gave **46 known failures, all in `internal/db`**:
  - 44 `referral_code` fixture failures (`TestBriefing_*`, `TestCrystallization_*`, `TestGetHardcoreUnsolved_*`, `TestGetRecentVictories_*`, `TestGetRisingIdeas_*`, `TestGetTrendingNow_*`, `TestGetYouMightLike_*`, `TestGetPlatformPulse_*`, `TestUserRepository_*`);
  - `TestSearch_MinSimilarity_HonestFilter`;
  - `TestSearchAnalyticsRepository_GetTrending`, which is time-window flaky. Ledger idx 98 fixes both search tests.
- The shared `solvr_test` database adds more failures from leftover data. Frontend: 183 files and 1,663 tests passed.

## Blocking constraints (orchestrator: restate these before planning)

1. **No production write, push or deploy without Felipe's explicit "yes" for that step.** That covers migrations, the cutover runner, admin calls, `git push`, the deploy webhooks and IPFS unpins. Local rehearsals on restored dumps are free.
2. **Executors never work in `/Users/fcavalcanti/dev/solvr` itself.** The loop owns that tree, and its `git add .` sweeps any stray file into its own commit. Executors use `git worktree add` on a branch, with their own scratch databases. Commit to the main tree only path-only (`git commit -- <files>`), and only when Felipe asks.
3. **The loop and an executor must never work the same ledger task.** The loop takes the first `passes:false` task from the top, with URGENT first. To keep it off a task an executor owns, tell it in `progress.txt` with an append-only entry such as "idx N owned by executor X — skip". Only flip `passes` after validation and merge. Ledger edits are Felipe's.
4. **The test-output HARD RULE** (`CLAUDE.md` golden rule 8; also `ralph.sh` step 4 and the ralph-brow skill):
   - save the full output to a log;
   - read only `tail -n 10`;
   - on a failure or error, `grep` the failing names and assertion lines;
   - never re-run a suite just to see more.
   Put this rule in every dispatch prompt.
5. **The loop and its machine.**
   - Never run a loop runner yourself unless Felipe explicitly tells you to; Felipe starts it.
   - `tmux kill-session` leaves the running `claude` alive as an orphan. A stop must also kill any `claude` whose cwd is the repo.
   - Keep the lid open and the charger in: on 10-01 at 23:50 a clamshell sleep froze a run for 8h42m.

## Accepted residuals / Refuted — don't redo

- **The production purge is done** (2,225 → 632 posts). Don't redo it. Its rules and tombstones are in the anti-abuse handover and the memory files.
- **Headroom (context compression) was spiked and NOT wired in.** It saved only about 10% on normal test logs and dropped 1 of 7 key lines.
- **The 09-25 codex 401 in `ralph-continuous.log` is history,** not a current fault.
- **"Engine hung" or a 45-minute timeout on 10-01 was a run that hit the time cap,** with its work kept in the tree. It was not a hang.
- **Settled owner decisions,** recorded in `progress.txt` in the 2026-09-30 OWNER DECISIONS entries:
  - legacy write routes return 410 (done);
  - room tokens are cut at deploy, with no grace period;
  - the old tables stay, to be dropped later all at once;
  - If-Match is required;
  - idx 76 is closed;
  - blog stays open but moderated;
  - the loop model is Opus;
  - the purge's heartbeat, repeat and watchdog rules apply to everyone.

## Hard rules & human-reserved decisions

- **These are FELIPE'S CALL:**
  - every production gate and the deploy window;
  - any `git push`;
  - which engine or model each lane uses, and the quota reset;
  - **amending ledger idx 95 to restore the `/data` and `/skill` nav links**;
  - any ledger edit.
- **The deploy path is all of v1.3 in one window.** No partial deploy is possible: a single codebase, linear migrations, and the old code reads columns that 097, 098 and 108 drop. The sequence:
  1. Pick a frozen commit.
  2. Re-rehearse on a fresh post-purge dump: 84 → head migration, `cmd/cutover`, smoke tests of the old and new API, the rollback.
  3. Fix the cutover version.
  4. Secret-scan the commits; the repo is public.
  5. Gated: `git push origin <frozen-sha>:main`.
  6. In the window: stop the API → `migrate force 84` + `up` → `cmd/cutover` → verify → the two deploy webhooks → UAT.

  The gates G1–G7 are in `docs/handovers/2026-09-29-solvr-v13-cutover/PLAN-r1.md`, and the P1–P10 gates in the anti-abuse PLAN-r1/r2.
- Commit only when asked, with one-line messages. No tags.

## Acceptance checklist (your PLAN-r1 is approved ONLY against these)

1. Restates the five blocking constraints in its own words.
2. Re-verifies the ledger count, the loop's current task, HEAD, the head migration vs `cmd/cutover`'s version, and the quota. It reports discrepancies.
3. Defines **lanes**. Each lane has its tasks (idx numbers), its executor type (the loop, a fresh Claude session, or Codex), its engine and model, and a quota estimate. There must be at least the loop lane, the **deploy track**, and one parallel executor lane.
4. For each parallel lane, says **how the loop is kept off those tasks** (the exact journal note) and how merge-back works (branch → validation → merge → `passes` flip by whom).
5. Includes the **exact, paste-ready prompt** for every executor it proposes. Each prompt must include the worktree, the scratch DB, the test-output rule, the "no production, no push" rule, the expected deliverable, and the report format.
6. Defines the **validation protocol**: what logs it checks, the comparison against the named baseline by test name, and what makes it reject work.
7. Lays out the **deploy track** step by step, with every production step marked as a Felipe gate. Includes the nav-links fix (pending Felipe's idx 95 decision) and the cutover-version fix.
8. Nothing in the plan collides with the running loop, or makes the orchestrator write product code itself.

## next_action

After APPROVED:
1. Re-verify the state (checklist item 2).
2. Give Felipe the first batch of dispatch prompts, the deploy-track rehearsal executor and one parallel build lane, plus the journal note that keeps the loop off the reserved tasks.

## Open questions

1. Which lane gets Fable and which gets Opus after Felipe's reset? A proposal: orchestrator and deploy track on Opus; the loop or one executor on Fable to spread the buckets.
2. Should the `vpd` loop also move off Opus? That is another repo, and Felipe's call.
3. Who reviews this PLAN-r1 if the author session is gone? Then Felipe approves directly.

## Pointers

- **Earlier handovers:**
  - `docs/handovers/2026-09-29-solvr-v13-cutover/`: the cutover rehearsal, runner, rollback, gates G1–G7.
  - `docs/handovers/2026-09-29-solvr-anti-abuse/`: the purge, prevention, P1–P10, runbooks for the collation fix and the IPFS unpin.
- **The harness:**
  - `ralph.sh`: the prompt; step 4 is the test-output rule.
  - `ralph-continuous.sh`: the supervisor.
  - `.env.ralph.local`: the knobs; Felipe edits it.
  - `progress.sh`: the meter.
  - The skill source is `/Users/fcavalcanti/dev/ralph-brow`.
- **Code:**
  - The cutover runner: `backend/cmd/cutover/`, `backend/internal/db/knowledge_cutover.go`.
  - The test helper `newMigratedScratchDatabase` in `backend/internal/db/scratch_database_test.go`.
- **Memory** (`~/.claude/projects/-Users-fcavalcanti-dev-solvr/memory/`):
  - `owner-decisions-2026-09-30.md`
  - `spam-purge-2026-09-29.md`
  - `cutover-rehearsal-2026-09-29.md`
  - `feedback-test-output-tail.md`
  - `deploy-process.md`
- **Secrets:** `.env` holds `ADMIN_API_KEY`, the `SOLVR_DB_*` keys and the `SOLVR_DEPLOY_*` webhooks. Names only; never inline the values.
