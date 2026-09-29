---
slug: solvr-v13-cutover
date: 2026-09-29
status: open
round: 0
author_session: Claude Code session that scaffolded the ralph loop, audited every batch 0/95 → 69/96, fixed four harness defects and two production-class bugs, and migrated production to schema 84
---

# Handover — Solvr v1.3: finish the ledger and ship the knowledge-model cutover

## Mission

Solvr is mid-rebuild (v1.3). Two consolidations are in flight: **rooms** (one membership
authority, per-agent tokens, one ordered timeline) and **knowledge** (problems/questions/ideas →
one `posts` model; approaches/answers/responses/comments → one `replies` table). An autonomous
build loop ("ralph") works a 96-task ledger, `spec.json`, one task per iteration.

Your job: **revalidate everything in this document, decide per remaining task whether it is
hand-work or ralph-work, oversee all of it, take a production backup, and deliver to the end** —
96/96 with the cutover shipped.

The ledger has converged on ONE gate. Seven open tasks (52, 53, 67, 68, 73, 74, 76) are blocked on
dropping the legacy knowledge tables, which is task 68 → task **93, the cutover**. The loop spent
43 slices on task 76 proving this, then stopped and asked the owner to decide. **Your first real
job is the cutover rehearsal, not restarting the loop.**

## Where things stand (verified)

### Ledger and repo
- [REAL 2026-09-29] `./progress.sh` → `69/96 (71%) · 69 UAT pending`.
- [REAL] HEAD `8a7eb2bd` — "test(data): verify the host-auto-committed room activity projection …
  (idx 77 slice 1)".
- [REAL] Working tree is NOT clean — uncommitted loop work: `M backend/internal/db/posts.go`,
  `M backend/internal/db/replies.go`, `?? backend/internal/db/vote_scores_test.go`,
  `?? backend/migrations/000113_vote_score_projection.{up,down}.sql`. Leave it; the loop resumes
  from the tree.
- [REAL] Loop is STOPPED: `pgrep -f 'ralph-continuous.sh'` → 0, `pgrep -f 'ralph.sh 1'` → 0.
- [REAL] `spec.json` schema is exactly `category, description, passes, steps` — `progress.sh` and
  the loop prompt both assume it.

### Databases
- [REAL] local `solvr` v102 dirty=false; local `solvr_test` v113 dirty=false; latest migration on
  disk `000113_vote_score_projection.up.sql`. Docker: `solvr-postgres` on host port **5435**
  (moved off 5433, which `bart-v2-temporal-postgres` owns permanently).
- [REAL] **Production schema is at migration 84.** `replies` table = MISSING. `rooms.owner_id`
  still present.
- [REAL] Production code is OLD: `curl https://api.solvr.dev/health` →
  `{"status":"ok","version":"0.2.0"}`. `/v1/overview` and `/v1/homepage/overview` → 404.

### Production data (the cutover's payload), measured 2026-09-29 via `/admin/query`
```
posts 2225 · comments 2716 (system-authored 2678) · approaches 412 · answers 200
responses 21 · messages 2452 · rooms 56 (not deleted)
```
3,349 contributions must become `replies` rows. **2,678 of the comments are `author_type='system'`** —
that is 98.6% of comments and it already broke the migration once (see Refuted below).

### Harness — verbatim from `.env.ralph.local` (mode 600, git-ignored; values are not secrets)
```
PRD_FILE=spec.json
RALPH_PUSH=0
BATCH_SIZE=1
BATCH_PAUSE_MINS=15
WAIT_TIME_MINS=15
VERIFY_CMD="cd backend && go build ./..."
MODEL=opus
CODEX_EFFORT=medium
DATABASE_URL=postgres://solvr:solvr_dev@localhost:5435/solvr_test?sslmode=disable
```
- [REAL] Engines: `./ralph-claude.sh`, `./ralph-codex.sh`, `./ralph-kimi.sh`,
  `./ralph-opencode.sh` (one task each) and a `-continuous` wrapper per engine.
  `./ralph-overnight.sh [engine] [HH:MM]` stops at a wall-clock time, between tasks.
- [REAL] Guards added this session, all smoke-tested with stub engines:
  per-iteration timeout `RALPH_ITER_TIMEOUT` (default 45m, `timeout -k 30s`, exit 124/137 →
  journaled + batch ends); in-batch stall guard (`STALL_LIMIT`, exit **2**); cross-batch stall
  detection in the supervisor (`NO_PROGRESS_LIMIT`, default 4 — use 20 for large tasks);
  journal-restore backstop; host git auto-commit.
- [REAL] Journal was rotated 2026-09-24: `progress.txt` 110 KB (recent entries + all carried
  pending checks), `progress-archive.txt` 702 KB (92 older entries, READ-ONLY, never attached).
  This cut per-iteration cost from ~$11.54 to ~$1.76.

### Test baseline — [TEST] serialized (`go test -p 1`) on a FRESH database
```
44 × "null value in column referral_code"   (pre-existing fixture defect, migration 000070)
 3 × TestCommentsCount_*
 1 × TestTypeSpecificListEndpoints
 1 × TestIPFSHealthEndpoint
 4 × GitHub OAuth integration
 5 × "cannot scan NULL" (agents list)
 n × room-slug collisions when packages share one DB
```
**Anything outside that list is new and yours to investigate.**
- [REAL] Frontend passes fully in isolation (~1,600 tests). Under parallel load you will see many
  `Test timed out in 5000ms` — always re-run the failing files alone before believing them.

## Blocking constraints (builder: restate these before planning)

1. **Migrations 97 and 98 drop `rooms.owner_id` and `rooms.token_hash`.** Production runs 0.2.0,
   which reads both columns. Applying them without deploying the new code takes the rooms API down
   immediately. Migrations ship WITH the deploy, never before it.
2. **Migrations are linear.** You cannot cherry-pick the rooms migrations (93–98, 101–113) without
   also applying 88/89, the knowledge-model ones. There is no partial path.
3. **`replies` is empty on production.** `stats.go`, `homepage_knowledge.go` and
   `legacy_type_stats.go` already count contributions from it. Deploying before the contribution
   migration runs makes every contribution counter read **zero** — silently wrong, not a visible
   failure.
4. **NEVER run a loop runner.** `ralph.sh`, `ralph-*.sh` are the owner's to execute — always.
   Reading them is fine; piping one to `head` counts as running it and has burned this before.
5. **NEVER test the live tree while an iteration is in flight.** Use `git worktree add --detach`
   at HEAD plus a scratch database. A mid-edit tree produces fake failures — this session wasted a
   cycle concluding "all three tests fail" when the tree was half-written.

## Accepted residuals / Refuted — don't fix

- **kimi** (`openrouter/poolside/laguna-s-2.1:free`): completed one heavy task well, then made a
  confidently FALSE schema claim, silently deleted 11 backend tests, and finally **hung for 13.5
  hours** at 2% CPU with zero output. The timeout now caps the hang and the test-deletion rule is
  in the prompt. Acceptable for seo/growth only.
- **codex** (`gpt-5.6-sol`): authenticated (`codex login status` → "Logged in using ChatGPT",
  token valid to 2026-10-05) and passed a clean smoke task. Then every call started returning
  `401 Unauthorized: Incorrect API key provided: sk-svcacct…`. That key survives `env -i`, is not
  in any env var here (ours are `sk-proj…`), is not in `~/.codex/config.toml`, and appears for both
  `gpt-5.6-sol` and the default model. **Root cause never found.** `ralph.sh` now does
  `unset OPENAI_API_KEY` in the codex lane — a real bug (`~/.zshrc` exports one, which hijacks
  subscription auth under tmux) but NOT the cause of this 401. If revisited: `codex login`, then
  `RUST_LOG=debug codex exec … 2>&1 | grep -iE "auth|key|token"`.
- **Per-(room, issue) sequence: REFUTED on data, don't reopen.** A consumer asked for it. Measured:
  58% of production events carry no issue at all; the busiest room has 26 issues across 38 events
  (~1.5 events/issue). Shipped instead: per-room `sequence` in the envelope + issue-filtered
  hole-free opaque cursor + 400 on unknown query params + `sequence`/`id` on every SSE frame.
- **Stall-guard false positive: already fixed.** The supervisor used to `grep "RALPH BLOCKED"` in
  the batch log; the stall entry lands in `progress.txt`, `ralph.sh` builds the host auto-commit
  message from the last dated journal line, and that message printed into the log — so the harness
  matched its own words and stopped a healthy batch. It now keys off `exit_code -eq 2`. **Do not
  revert to text matching.**
- **`000089` constraint bug: fixed, don't re-introduce.** `replies.author_type` was
  `CHECK IN ('human','agent')` while migration 000054 had widened `comments.author_type` to include
  `'system'`. `MigrateContributions` aborted on the first system comment. Now includes `'system'`,
  proven end-to-end by seeding a live system comment and asserting a `system`/`comment` reply row.
- **`BATCH_PAUSE_MINS=15`** makes the loop idle ~60% of wall-clock. The owner set it; left alone.
- **`progress.sh` counts `UAT:` unanchored** (66 vs 62 line-anchored). The journal rotation
  deliberately carried all 66 forward so the meter stayed honest. Don't "fix" the grep.

## Hard rules & human-reserved decisions

- **Every production write stops for the owner's approval — each time.** Applying migrations
  85→113, running the contribution migration, deploying. THIS IS FELIPE'S CALL. Rehearsing
  locally (dump, restore, migrate, verify) needs no permission.
- **Task 76's disposition IS FELIPE'S CALL, and is still unanswered.** The loop asked explicitly
  "whether idx 76 may close with its remaining dispositions owned by idx 52/68/73/78/83/93".
- Deploys are **manual on EasyPanel**. No auto-deploy on push.
- Commit and push only when asked. Never create a release tag. Never edit `.env` or
  `.env.ralph.local` without being told.
- `spec.json` is append-only truth for the loop; ledger edits are the owner's.
- Admin route for production SQL: `POST https://api.solvr.dev/admin/query` with header
  `X-Admin-API-Key`, value in `.env` (key name only — never inline the value). Verified working.

## Acceptance checklist (the author approves the plan ONLY against these)

1. Restates all five blocking constraints correctly, in the builder's own words.
2. Re-verifies against live systems and reports discrepancies: ledger count, both local DB
   versions, production `version`, production schema version, `replies` presence.
3. Starts from the existing verified backup (`db-backups/solvr_prod_2026-09-29_15-49-50.dump`)
   and says how it will be restored into a scratch database — not into `solvr` or `solvr_test`.
4. Rehearses the cutover on a restored copy — apply 85→113, run `MigrateContributions`, diff
   old-model vs new counts, and exercise the **down** path — before proposing any production write.
5. States, for each remaining task, whether it is hand-work or ralph-work, with a reason.
6. Names the engine per block and keeps opus (or better) for task 93.
7. Distinguishes known-baseline test failures from new ones, by name.
8. Schedules no production write without an explicit owner-approval step.

## next_action

**The production backup is already taken** — see below. Your first step is the cutover REHEARSAL:
restore that dump into a scratch database, apply 85→113, run `MigrateContributions`, diff
old-model vs new counts, and exercise the down path. Nothing touches production until that is
clean and the owner approves.

### The backup [REAL 2026-09-29 15:49]
```
db-backups/solvr_prod_2026-09-29_15-49-50.dump    16.0 MB, pg_dump custom format
415 TOC entries · 42 tables with data · --no-owner --no-privileges
```
Taken with **pg_dump 17.11 from inside the `solvr-postgres` container**, which sidesteps the host's
pg_dump 18.0 vs production PostgreSQL 17 mismatch — do it the same way, do NOT install
postgresql@17:
```
docker exec -e PGPASSWORD=<SOLVR_DB_PASSWORD> solvr-postgres \
  pg_dump -h <SOLVR_DB_HOST> -p <SOLVR_DB_PORT> -U <SOLVR_DB_USER> -d <SOLVR_DB_NAME> \
  --format=custom --no-owner --no-privileges -f /tmp/<name>.dump
docker cp solvr-postgres:/tmp/<name>.dump db-backups/
```
Correction to an earlier belief: **`SOLVR_DB_HOST/PORT/NAME/USER/PASSWORD` ARE present in `.env`**
(five keys, names only here). `db-backups/` is git-ignored. The dump is verified by
`pg_restore --list`, not by having been written.

## Open questions

1. Task 76's disposition — close it with items reassigned, or keep it open until 68/93 land?
2. Deploy shape: one window (code + 85→113 + contribution migration), or code first with the
   contribution migration immediately after? Constraint 3 makes "code first, migrate later" unsafe.
3. Engine for the seo/growth block (80–92): kimi is free but unreliable; sonnet is cheap and
   reliable. Claude 7-day window was at 75% on 2026-09-25.
4. `BATCH_PAUSE_MINS=15` — intentional, or worth dropping to 2 to reclaim ~60% of wall-clock?

## Pointers

- Ledger `spec.json`; journal `progress.txt`; older journal `progress-archive.txt` (read-only).
- Harness: `ralph.sh` (core, engine-agnostic), `ralph-continuous.sh` (supervisor),
  `ralph-overnight.sh` (wall-clock stop), `progress.sh` (meter), `.env.ralph.local` (knobs).
- Project rules: `CLAUDE.md`. Spec: `SPEC.md`. Prod DB helpers: `scripts/backup-prod-db.sh`,
  `scripts/restore-local-db.sh`, `scripts/validate-local-schema.sh`.
- Cutover code: `backend/internal/db/contribution_migration.go`
  (`MigrateContributions`, `VerifyContributionMigration`), tests in
  `backend/internal/db/contribution_migration_test.go`.
- Secrets live in `.env` (`ADMIN_API_KEY`) and `.env.ralph.local`. Names only — never inline values.
