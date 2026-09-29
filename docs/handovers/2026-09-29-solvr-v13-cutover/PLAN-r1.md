---
round: 1
builder_session: fresh Claude Code session, Opus 5.5 (claude-opus-5-5), 2026-09-29 ~16:00-16:10 -03, plan mode. Read-only checks against the repo, local Docker, and production (SELECT-only via /admin/query).
---

# Plan r1 — Solvr v1.3: rehearse and ship the knowledge-model cutover

## Context
Solvr v1.3 has 27 open tasks in the ledger, and all of them wait on one event: shipping the
knowledge-model cutover. Production is at schema 84 and runs old code. Local `main` has the full
rebuild at migration 113. The handover gives the builder three jobs: revalidate every claim, take
over running the work, and rehearse the cutover on a restored production dump before anything
touches production.

## Blocking constraints, restated
1. **The old code breaks if the schema moves first.** Production runs origin/main `c03734ae`. Its
   `backend/internal/db/rooms.go` INSERTs and SELECTs `rooms.owner_id` and `rooms.token_hash`
   (lines 73, 75, 149, 161, 174). Migration 000097 drops the first column and 000098 drops the
   second. If the schema moves without the new code, every rooms query fails. Migrations are
   therefore applied only inside the deploy window, after the old API is stopped.
2. **Migrations are all-or-nothing.** golang-migrate applies files in order, with no subsets. The
   rooms work (93–98, 101–113) comes after the knowledge migrations 88/89, so reaching any of it
   means applying everything from 85. Production makes one move: 84 → 113.
3. **The new code reads contribution counts only from `replies`.** `router.go:692` and
   `router_homepage.go:49` wire up `NewCanonicalStatsRepository`, which is replies-based. On
   production `replies` does not exist. After 000089 it exists but stays empty until
   `MigrateContributions` runs. If the new code serves traffic before that, every contribution
   counter reads 0 and nothing errors. So the migrations, the contribution migration and the
   relation remap must all finish, and be verified, before the new code takes traffic.
4. **I never execute a loop runner.** That covers `ralph.sh` and every `ralph-*.sh`: no `--help`,
   no `| head`, no tmux. I only read them. Felipe runs them.
5. **I never test or build in the live tree.** All my builds, tests and rehearsal runs happen in
   `git worktree add --detach` checkouts under my scratchpad, against scratch databases I create.
   One lesson added this session: the loop commits with `git add .`, so any file I leave in the
   main tree while it runs ends up in its commit. HANDOVER.md itself was swept into `619b44b8`
   this way. Nothing of mine goes into the main tree while the loop runs.

## Handover discrepancies
Checked 2026-09-29 18:58–19:10Z.

**Confirmed:**
- `./progress.sh` prints `69/96 (71%) · 69 UAT pending`.
- Local databases: `solvr` is at v102 (dirty=false), `solvr_test` at v113 (dirty=false).
- The latest migration on disk is `000113`, and all 29 down files for 85–113 are present.
- `solvr-postgres` listens on 5435. `bart-v2-temporal-postgres` is on 5433.
- On production, `/v1/overview` and `/v1/homepage/overview` both return 404.
- Production `replies` table: `to_regclass` returns NULL (absent).
- Production `rooms` still has 2 legacy columns: `owner_id` and `token_hash`.
- Production counts match the handover exactly (19:01:54Z): posts 2225 · comments 2716 (system 2678) · approaches 412 · answers 200 · responses 21 · messages 2452 · live rooms 56. All 2678 system comments are by `solvr-moderator`, on 1,836 posts.
- The backup is present:
  - 16,795,054 B, sha256 `e20dacdc8bde88ce221d852e2cd947266b4b96b090b120833c265f2caca0bcf5`.
  - Custom format, pg_dump 17.11, dumped from PG 17.9.
  - 42 TABLE DATA entries; `pg_restore --list` exits 0.
- `.env.ralph.local` values match the handover. It is mode 600 and git-ignored. The handover's "verbatim" block left out the inline comments, but the values are the same.
- The loop guards the handover lists are all in place:
  - `ralph.sh`: `RALPH_ITER_TIMEOUT` and `timeout -k 30s` (lines 38 and 55), `STALL_LIMIT` (37), and `unset OPENAI_API_KEY` in the codex lane (361).
  - `ralph-continuous.sh`: stops on `exit_code -eq 2` (212), not on log text, and has `NO_PROGRESS_LIMIT` (37).
- `000089` now allows `'system'` in `replies.author_type` (line 20).
- No up migration from 85 to 113 drops a legacy knowledge table.
- The loop is stopped now: `pgrep -fl ralph` shows no processes.
- `codex login status` prints "Logged in using ChatGPT". I did not re-test the 401, because that would mean running codex.

**Mismatches:**
- **D1. HEAD moved, and the loop was running when the handover was written.** HEAD is `619b44b8` (15:56:37), not `8a7eb2bd`, and the tree is clean.
  - An iteration (idx 77 slice 2) was running while the handover was written. Its journal says: "HANDOVER.md … written 15:49-15:50 by another (owner's) Claude session while this iteration ran".
  - So the handover's [REAL] claim "Loop is STOPPED" was false when written.
  - The 000113 and vote files the handover lists as uncommitted are now committed, together with HANDOVER.md.
- **D2. The journal has grown back.** `progress.txt` is 840,448 B, not ~110 KB; the archive is 718,874 B. The loop prompt tells each iteration to read `progress.txt` first, so per-iteration cost has likely crept back up [INFERRED, not measured].
- **D3. "Production schema is at 84" is inferred from markers, not read from a version table.** Production has no `schema_migrations` table: selecting from it returns 42P01.
  - Present on production: `search_queries.public_scope` (82), `api_request_events` (83, 0 rows), `messages.reply_to_entry_id` and `messages.addressed_member_ids` (84).
  - Absent: `messages.client_entry_id` (85), `rooms.archived_at` (86), `messages.pinned_at` (87).
  - Consequence: `migrate up` on the restored dump, and on production, first needs `migrate force 84`. On production that step creates `schema_migrations`, which is itself a production write.
- **D4. The `/health` version is not evidence of old code.** It returns `"0.2.0"`, which is a stale constant. The real evidence:
  - `/v1/overview` returns 404 on production.
  - origin/main's migrations stop at `000081`, while migrations 82–84 were applied to production by hand.
- **D5. Missing from the handover: 207 unpushed commits.** Local `main` is 207 commits ahead of origin/main (`c03734ae`, 2026-07-06).
  - EasyPanel builds from the GitHub `main` archive, so shipping needs those 207 commits pushed to a **public** repo first.
- **D6. Constraint 3 names the wrong files, but its substance holds.**
  - The replies-based counts live in `internal/db/stats_canonical.go`, `knowledge_totals.go` and related files.
  - `internal/db/stats.go` still counts the legacy tables but is not what the router uses for stats.
- **D7. The cutover is more than `MigrateContributions`, and none of it can run on production yet.**
  - Other cutover code that exists:
    - `VerifyPostMigration` and `RemapPostStates` (`post_migration.go`).
    - `RemapLegacyRelations`. It covers FreezeLegacyReputation, accepted answers, votes, reports and flags, approach relationships, progress notes, notification links and embeddings.
    - `rebuild_vote_scores()`, which the header of 000113 requires after votes are retargeted.
  - **None of it has a production entry point:** no `cmd/` program and no admin route calls these functions (checked with grep).
- **D8. There are more open and blocked tasks than the handover says.**
  - 27 tasks are open. idx 24 is also blocked, per the journal.
  - idx 74 is waiting on Felipe's decision about step 5 ("require If-Match"), not on the cutover.
  - The journal has more unanswered questions for Felipe that the handover does not carry:
    - the claim-referral mount (slice 31)
    - staged vs all-at-once legacy cleanup (slice 32)
    - the two 24h-window search tests on shared data (slice 32)
- **D9. The test-failure baseline is out of date.** In `/tmp/solvr_idx77_s2_full.log` (shared `solvr_test`, `-p 1`), there are 96 failing top-level test names.
  - Now passing, though the handover lists them as known failures: `TestCommentsCount_*` ×3, `TestTypeSpecificListEndpoints`, `TestIPFSHealthEndpoint`.
  - "cannot scan NULL" appears 0 times.
  - Still failing as the handover says: 44 `referral_code` failures and the 4 GitHub OAuth tests.
  - New failures the handover does not list:
    - ~24 duplicate-fixture failures (`TestAgentRepository_*`, `TestBadges_*`).
    - ~25 failures caused by leftover data in the shared database: `TestServiceCheckRepository_*`, `TestSearchAnalyticsRepository_*`, `TestHomepageSearch_*`, `TestSearchPulse_*`, `TestSearch_MinSimilarity_*`, `TestListPosts_AuthorSeesOwnHidden` (27 rows instead of 1), `TestGetTrendingPosts_*`, and `…_PerformanceTarget` (181 ms against a 100 ms limit).
- **D10. The dump's entry count differs.** Its header says `TOC Entries: 404` (400 are listed), not 415. This is cosmetic.
- **D11. The ledger itself puts legacy table removal 90 days after cutover.**
  - idx 93 step 6 says legacy tables are removed "after the 90-day compatibility period"; idx 95 step 6 calls "90-day legacy removal … a later cleanup gate"; idx 52 steps 3 and 5 say the same.
  - More tasks can only pass with time or real outcomes:
    - idx 85 needs SEO reviews at weeks 1, 4 and 8.
    - idx 87 and 89 need real user cohorts; 89 says "shipping the website alone cannot mark these outcome requirements passed".
  - So 96/96 cannot be reached by build work alone, and idx 68 cannot honestly pass at cutover time.
- **D12. Not re-verified:**
  - the frontend's ~1,600 tests (that needs a full run)
  - the codex 401
  - kimi's behavior
  - the handover's ~60% idle figure for `BATCH_PAUSE_MINS`

## Acceptance checklist — point-by-point
1. **Restate the five blocking constraints.** See the restated section above. Each one is in my own words, with the evidence I checked.
2. **Re-verify against live systems.**
   - Ledger: 69/96.
   - Local `solvr` is at 102 and `solvr_test` at 113.
   - Production version: `/health` returns "0.2.0", but that is a stale constant. The 404 on `/v1/overview` is the real evidence (D4).
   - Production schema is at 84 by markers, with no `schema_migrations` table (D3).
   - `replies` is absent on production.
   - Discrepancies D1–D12 are listed above.
3. **Restore the verified backup into a scratch database.** Step A1 restores `db-backups/solvr_prod_2026-09-29_15-49-50.dump` (sha256 above) into a **new** database, `solvr_cutover_rehearsal`. It uses `docker cp` and `pg_restore` inside `solvr-postgres`, so the PG17 client is used.
   - `scripts/restore-local-db.sh` is **not** used. It hard-codes `DB_NAME="solvr"`, drops that database, and runs `kill -9` on whatever is on port 8080.
   - Every command names its target database explicitly, and the rehearsal runner refuses any database name it does not expect.
4. **Rehearse before proposing any production write.** Steps A3–A8 do this in order:
   - `force 84`, then migrate up to 113.
   - The **full** cutover sequence from D7, plus a second pass to prove it can be re-run.
   - A diff of old-model vs new counts, content hashes and relationships.
   - A smoke test of the new API on real data.
   - Simulated writes after the cutover.
   - `down 29` (113 → 84).
   - A smoke test of the old code on the rolled-back database.

   No production write is proposed until the Phase A report is done and Felipe approves.
5. **Hand-work or ralph-work for each remaining task.** See the task table below.
6. **Engine per block, with Opus (or better) for task 93.** See the engine section below. Task 93 is done in this session on Opus 5.5 and is never handed to the loop.
7. **Tell known-baseline failures from new ones, by name.** Phase B runs the full backend suite once, on a fresh database, and saves the output. Failures are extracted by name and sorted into groups. Any later failure whose name is not in that baseline counts as new.
8. **No production write without Felipe's approval.** Phase C has gates G1–G7. Each one stops and waits for Felipe's explicit "yes" in chat; approving one gate never approves the next.

## The plan

### Phase A — cutover rehearsal (the handover's next_action; all local, no permission needed)
- **A0. Worktrees.**
  - `git worktree add --detach $SCRATCH/rehearsal 619b44b8` for the new code.
  - Later, `$SCRATCH/oldcode` at origin/main `c03734ae` for the old code.
  - Every command runs once, and its full output is saved to `$SCRATCH/logs/<step>.log`.
- **A1. Restore.**
  - `docker exec solvr-postgres createdb -U solvr solvr_cutover_rehearsal`
  - `docker cp` the dump into the container at `/tmp/rehearsal.dump`
  - `docker exec solvr-postgres pg_restore -U solvr -d solvr_cutover_rehearsal --no-owner --no-acl /tmp/rehearsal.dump`, with output saved to `restore.log`
  - Every error line must be 0 or explained.
  - Then `CREATE DATABASE solvr_rehearsal_pristine TEMPLATE solvr_cutover_rehearsal`, so a rerun never needs a second restore.
- **A2. Pre-cutover snapshot.** `$SCRATCH/checks-pre.sql` exports its results to CSV with `\copy`. It records:
  - posts by type, status and deleted flag
  - approaches, answers, responses and comments, live and deleted; comments by `target_type` and `author_type`
  - messages and room_events
  - rooms (id, slug, owner_id, md5(token_hash)) and room_members
  - votes by target type and confirmed flag
  - reports, flags, and notifications whose links point at legacy pages
  - progress_notes and approach_relationships
  - md5 of each legacy content body
  - rooms with activity in the last 7 and 30 days, and their distinct agent authors (for the token-impact concern below)
  - the legacy public contribution count, using the same SQL as `stats.go:130`
- **A3. Schema.**
  - `migrate -path backend/migrations -database "postgres://solvr:solvr_dev@localhost:5435/solvr_cutover_rehearsal?sslmode=disable" force 84`
  - then `… up`, timed
  - Expect version 113 with dirty=false. On any failure: stop, and record the exact error as the finding.
- **A4. Cutover runner.**
  - A throwaway `backend/cmd/cutover-rehearsal/main.go` that exists **only in the worktree** and is never committed. It is the prototype for task 93's real runner.
  - Guard: it refuses to start unless the host is localhost and the database name starts with `solvr_rehearsal` or is exactly `solvr_cutover_rehearsal`.
  - It runs these steps in order, timing each one and printing its report as JSON:
    1. `VerifyPostMigration` → `RemapPostStates` → `VerifyPostMigration` again (must report OK)
    2. `VerifyContributionMigration` before migrating (exceptions grouped by kind)
    3. `MigrateContributions` (expect 3,349 minus the orphans that step 2 reports)
    4. `RemapLegacyRelations` (counts, plus the list of unresolved references)
    5. `SELECT count(*) FROM vote_score_drift()` → `SELECT rebuild_vote_scores()` → drift must be 0
    6. `VerifyContributionMigration` after migrating
    7. A second pass of steps 1–5. Every count must be 0, which proves a production retry is safe.
- **A5. Old vs new diff.** `checks-post.sql` compares:
  - replies by `legacy_type` against the legacy table counts (approach 412, answer 200, response 21, comment 2716, of which 2678 must have `author_type='system'`)
  - deleted rows stay deleted
  - comments on a contribution become child replies of that contribution's reply
  - md5 of legacy bodies against `replies.body` for answers, responses and comments. Approach bodies are rebuilt from several fields, so they are checked through their provenance fields instead.
  - no vote, report or flag still targets a legacy type, except those the remap listed as unresolved
  - `room_entries` against `legacy_messages` + `legacy_room_events`
  - rooms keep the same ids and slugs
  - each room's active human owner membership matches its pre-migration `owner_id`
  - the stored `message_count` against the actual count
  - how long each migration took
- **A6. New-code smoke test.**
  - It runs on a **copy** (`CREATE DATABASE solvr_rehearsal_api TEMPLATE solvr_cutover_rehearsal`), because the API's cleanup and health-check jobs write rows.
  - Build the worktree backend and check `lsof -i :18082` first; I kill only my own earlier run.
  - Start it with `env -i` and only `DATABASE_URL`, `JWT_SECRET=dummy`, `PORT=18082`, and `IPFS_API_URL=http://127.0.0.1:9`. The unreachable IPFS address makes crystallization skip. No Groq, Voyage, Resend or SMTP keys are set.
  - Probes:
    - `/v1/overview`.
    - `/v1/stats`: contributions must equal A2's legacy public count. This checks constraint 3 on real data.
    - `/v1/posts`.
    - `/v1/posts/{id}/replies` for three posts: a problem with approaches, a question with an accepted answer, and a post with a `solvr-moderator` comment. For the last, record whether system replies are shown.
    - `/v1/search` with 5 fixed queries, keyword-only.
    - `/v1/rooms` and one room's entries.
  - Afterwards, stop the server and drop the copy.
- **A7. Rollback rehearsal.**
  - On a copy of the post-cutover database, `solvr_rehearsal_down`, insert writes that would happen after the cutover: one post, one native reply, one child reply, one room entry and one vote.
  - Then run `migrate … down 29` (113 → 84) and measure:
    - the exit code and dirty flag
    - legacy counts must equal the A2 snapshot
    - what happens to each post-cutover write. I expect [UNVERIFIED] that native replies are lost when `replies` is dropped. If so, that becomes the export step of task 93's rollback procedure.
    - `rooms.owner_id` as rebuilt by `097.down` against the A2 snapshot
    - `token_hash`: I expect 0 of 56 to match, because `098.down` fills in random hashes
- **A8. Old-code smoke test.**
  - Build origin/main and run it against `solvr_rehearsal_down` on a free port.
  - Probe `/v1/posts`, one problem, one question, `/v1/rooms`, and one room's messages.
  - Any 500 means rolling back with `down` is broken, which is a finding.
- **A9. Report and clean up.**
  - The numbers go to Felipe and the author in chat (see Q3 for whether they also go into `progress.txt`).
  - I drop only the databases I created, each by exact name, and `git worktree remove` my worktrees.
  - `solvr_audit`, `solvr_probe` and the older prunable worktrees are not mine, and I leave them alone.

**Rehearsal pass criteria.** All of these must hold before I propose any production write:
1. Migrating 84 → 113 is clean.
2. Every exception in the verify reports is listed by id and explained.
3. The replies count equals 3,349 minus the reported orphans.
4. The second pass reports all zeros.
5. Vote drift is 0 after the rebuild.
6. The new API's contribution count equals the legacy count.
7. Room ids, slugs and owners are preserved.
8. Rolling back 113 → 84 is clean, and the old code serves without any 500.
9. Every loss (room tokens, post-cutover replies) is measured and written into the rollback procedure.

### Phase B — named test baseline (after Phase A; local only)
- **B1. One full run.** In a worktree at HEAD, against a fresh `solvr_baseline_<hhmm>` migrated from the files, run `go test -p 1 -count=1 -v -timeout 60m ./... > $SCRATCH/baseline-full.log 2>&1` **once**.
- **B2. Group the failures by name.** Extract every failing top-level test name and its first error line. Then sort them into three sets:
  1. **Fresh-database baseline:**
     - the `referral_code` fixture failures (from migration 000070)
     - the four GitHub OAuth tests: `TestGitHubCallback_{GitHubError,InvalidCode,MissingCode}_Integration` and `TestGitHubOAuthRedirect_Integration`
     - room-slug collisions, and anything else that shows up on a fresh database
  2. **Shared-database-only failures (D9):**
     - duplicate fixtures in `TestAgentRepository_*` and `TestBadges_*`
     - leftover data in `TestServiceCheckRepository_*`, `TestSearchAnalyticsRepository_*`, `TestHomepageSearch_*` and `TestSearchPulse_*`
     - `TestRoomExpiry_ExpiredRoomLeavesEveryListingBeforeTheReaper` (load-sensitive) and `TestSearchRepository_Search_PerformanceTarget`
  3. **New:** anything else.

  Rule from then on: a failing name outside sets 1 and 2 is new, and I investigate it from the saved log without re-running the suite.
- **B3. Frontend.** Run `npm test` once in the worktree and save the output. Any failing file is re-run alone once, because of the parallel-timeout caveat in the handover.

### Phase C — production (each G is a stop for Felipe's explicit approval)
**Prerequisite: task 93 code, written by hand, test-first, with the loop stopped while I touch the main tree.** Nothing is committed until Felipe asks. The code:
- `backend/cmd/cutover/`:
  - runs the A4 sequence
  - has `--dry-run`
  - prints a JSON report
  - requires both `--database-url` and `--confirm-prod`
- A migration ledger (93 step 4), designed from the rehearsal numbers.
- A rollback procedure that includes exporting writes made after the cutover (93 step 4).

**Gates:**
- **G1. Push.** Scan `git log -p origin/main..main` for secrets: `sk-`, `solvr_sk_`, `solvr_rm_`, `ghp_`, `AKIA`, `BEGIN PRIVATE KEY`, password literals. Then Felipe approves pushing the 207 commits to the public repo.
- **G2. Fresh dump.** Take a new production dump the same way the handover did (`docker exec` pg_dump). Restore-test it into a scratch database and run A2–A5 on it.
- **G3. Write pause.** Felipe stops `solvr-api` in EasyPanel. This is the documented brief write pause that 93 step 2 allows.
- **G4. Schema.** Run `migrate force 84` + `up` against production, using the `SOLVR_DB_*` keys in `.env`. Before that, check the connection with a read-only `SELECT 1`.
- **G5. Cutover.** Run `cmd/cutover` against production. Its report must match the G2 run, allowing only for writes made after the dump.
- **G6. Deploy.**
  - Trigger `SOLVR_DEPLOY_API`, then `SOLVR_DEPLOY_WEB`.
  - Completion probe: `/v1/overview` returns 200. `/health` is not used, because its version string is stale.
  - Then run the A6 probes against production, read-only.
- **G7. After deploy.** Walk the pending UAT items with Felipe. If something fails, whether to roll back is Felipe's call, using the rehearsed procedure.

**Deploy shape (open question 2): my recommendation is one window.**
- Deploying the code first is unsafe: it breaks rooms (constraint 1) and makes counters read 0 (constraint 3).
- Migrating first without the new code is also unsafe (constraint 1).
- The only safe order is: stop the old API → migrate → cutover → verify → deploy the new code. That is G3 → G6, in one contiguous window.

### Task disposition — hand-work or ralph-work (27 open)
| idx | Who | Why |
|---|---|---|
| 77 | Ralph, now | In progress. Its slices can be built before the cutover. I feed step 5 (reconciliation) the rehearsal numbers. |
| 74 | Felipe decides, then ralph | Only step 5 is left: whether If-Match is *required*. That is a product decision. |
| 76 | Felipe decides | The loop says there is no buildable slice left. The question is whether it closes with its remaining items reassigned to 52/68/73/78/83/93. |
| 93 | Hand (this session) | It writes to production. It needs the rehearsal, a runner, and Felipe's gates. |
| 68 | Hand | It drops tables on production, a destructive change. The timing depends on D11 and Q1. |
| 53 | Hand, then ralph for fixes | The before/after query comparison needs the restored production snapshot (A2 vs A6). |
| 52, 73 | Ralph, after the cutover | The code parts are buildable. 52's sunset date is Felipe's call, and its step 5 waits for the 90-day window to end. |
| 78 | Ralph | SDK, CLI and MCP work. Its "upgraded schema" consumer test can use a copy of the rehearsal database. |
| 79 | Ralph for the harness and targets; hand for the load test | The load test and backup/restore checks run on the production-like rehearsal database, and Phase A supplies the evidence. |
| 80, 81, 82, 84 | Ralph | Bounded frontend, SSR and metadata work that can be tested locally. |
| 83 | Ralph, plus Felipe | Ralph builds the redirect map, which must match the migration. Felipe exports GSC data (step 1). |
| 88 | Ralph | Product feature: "Try this workflow" and source attribution. |
| 86, 90, 91, 92 | Ralph builds the definitions, model and examples | Passing needs observed data over time. Listing submissions are Felipe's. |
| 85, 87, 89 | Felipe, gated by time | GSC access and reviews at weeks 1, 4 and 8. Real cohorts. 89 says shipping alone cannot pass it. |
| 24, 67 | Hand (me with Felipe), after deploy | Needs real agents on several clients, two API instances, and a logged-out browser against the deployed product. 24 also needs 78 first. |
| 94, 95 | Hand (me with Felipe), last | Acceptance and release gate, checked against the deployed product. |

### Engines per block
- **Hand block** — 93, 68, 24, 67, 94, 95, and the rehearsal and verification parts of 53 and 79. This session runs it on Opus 5.5 (`claude-opus-5-5`). The loop never gets task 93.
- **Loop, data/api block** — 77, 74 (after the decision), 52/53/73/78 code, 79's harness, 88, 83. `./ralph-claude*.sh` with `MODEL=opus`, as configured now.
- **Loop, seo/growth block** — 80, 81, 82, 84, 86, 90, 91, 92. I recommend the claude lane with Sonnet. This is Felipe's call (open question 3). He would set it with a `MODEL=sonnet` override when starting the run, since I do not edit `.env.ralph.local`.
- **Excluded engines:**
  - kimi: it hung for 13.5 hours, deleted 11 tests, and made a false schema claim.
  - codex: the 401 is still unexplained.
  - opencode: there is no track record for it in the handover.
- **Before any loop restart:** Felipe checks the Claude 7-day quota (it was at 75% on 09-25) and decides whether to rotate the 840 KB journal (D2).

## Concerns
- **[HIGH] 96/96 cannot be reached by build work alone (D11).**
  - The ledger itself makes several tasks wait on time or outcomes: the 90-day legacy removal, the SEO reviews at weeks 1, 4 and 8, and the cohort outcomes.
  - We need a definition of "done" (Q1).
- **[HIGH] 000098 cuts off every agent in a live room at deploy.**
  - It retires shared room tokens with no transition period. The handshake stops accepting `solvr_rm_`, so every agent holding one in the 56 live rooms loses access when the new code deploys.
  - This conflicts with 93 step 5, which asks for "token validity through the stated transition". A2 measures how many rooms are recently active (Q2).
- **[HIGH] Constraint 2 conflicts with 93 step 5.** Step 5 asks that room downtime not be coupled to the knowledge conversion, but the linear migration history forces them into one window.
- **[HIGH] No production runner exists for the cutover functions (D7).** `cmd/cutover` has to be built and tested before G5.
- **[HIGH] 207 unpushed commits go to a public repo (D5).** The secret scan in G1 comes first.
- **[MEDIUM] Production has no `schema_migrations` table (D3).**
  - `force 84` creates it. That is the first production write.
  - Applying 29 files by hand through the admin route would be riskier, given the plpgsql bodies and the rename-and-view swap in 094.
  - I recommend the migrate CLI.
- **[MEDIUM] Moderator comments may start showing as replies [UNVERIFIED].**
  - The 2,678 `solvr-moderator` comments become top-level replies on 1,836 posts.
  - The only system filter grep found is in `posts_reply_counts.go:14`, which covers counts, not the replies listing.
  - Whether the listing shows them is unknown. A6 checks it.
- **[MEDIUM] Public post scores may change.** `rebuild_vote_scores` recomputes post scores from confirmed votes only, so scores that counted unconfirmed votes will drop. A4 step 5 measures the real drift; how much change is acceptable is Felipe's call.
- **[MEDIUM] The journal is 840 KB (D2), and every iteration reads it.** Rotating it is harness state, so it is Felipe's call.
- **[MEDIUM] The dump is from 15:49 and production keeps taking writes.** The fresh dump in G2 is mandatory.
- **[LOW] Both rollback paths lose something.**
  - Migrating down makes room tokens unrecoverable.
  - Restoring the dump loses writes made after the cutover.
  - A7 measures both.
- **[LOW] Migrated comments and responses have no embeddings.** `copyLegacyEmbeddings` covers answers and approaches only. Comments and responses need `cmd/backfill-embeddings --content-types replies`, which calls the Voyage API and costs money. That is a step after the cutover, and it is Felipe's call.

## Questions for the author
1. **What does "done" mean (D11)?** My proposal:
   - Launch-stage done means every build and verification task passes and the cutover ships.
   - These stay open with dated gates:
     - 68's table drop (+90 days)
     - 52 step 5
     - 85's week-8 review
     - the outcome tasks 87 and 89
   - Or should Felipe re-scope the ledger instead?
2. **Room tokens:** was cutting off shared tokens at 000098 an explicit decision? If yes, I plan a re-invite message for affected rooms. If no, does 93 step 5 need a transition window in which the handshake still accepts `solvr_rm_`?
3. **Where do the rehearsal results go?** Should I append them to `progress.txt` (append-only, while the loop is stopped) so the loop knows task 93 was rehearsed? Or only chat plus the task 93 runbook?
4. **Who builds `cmd/cutover`?** I plan to write it by hand, because it is production-critical and gated by Felipe. Do you agree, or should the loop take it once the rehearsal numbers exist?
5. **Felipe's pending decisions:** do you already have his answers to the D8 questions (claim-referral mount; staged vs all-at-once cleanup; required If-Match; the 24h search tests) and to the task 76 disposition? If not, I bring them to him as one batch after the rehearsal.

## Critical files
- Cutover logic:
  - `backend/internal/db/contribution_migration.go`
  - `legacy_relation_remap.go`
  - `post_migration.go`
  - `reputation_history.go`
- Migrations 000085–000113, especially `094`, `097`, `098` and `113`, and their down files.
- `backend/cmd/api/main.go`, which starts the background jobs that matter for A6.
- `scripts/restore-local-db.sh`: I only read it; it is **not used** (see checklist item 3).
- New, worktree only: `backend/cmd/cutover-rehearsal/main.go` and `$SCRATCH/checks-{pre,post}.sql`.

## Verification
- **Phase A:** passes only if all 9 rehearsal pass criteria above hold, each with a saved log.
- **Phase B:** produces the named baseline and its three sets, and every later test failure is classified against it.
- **Phase C:** each gate has its own read-only check: the report equals the G2 counts, `/v1/overview` returns 200, and `/v1/stats` contributions equal the legacy count.
