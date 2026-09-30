---
round: 1
builder_session: fresh Claude Code session (Opus 5.5), plan mode. Read only the handover dir, then re-verified against the code on main 6efe181b, local rehearsal DBs and the solvr-postgres container. 2026-09-29.
---

# Plan r1 — Solvr anti-abuse: W0–W6

> Deliverable: this file gets copied verbatim to `docs/handovers/2026-09-29-solvr-anti-abuse/PLAN-r1.md`. Nothing runs until REVIEW says APPROVED.

## Blocking constraints, restated

1. **Production is hands-off unless Felipe says yes, each time.** No migration, admin call (bans, unpin, gc), REINDEX, trigger change, `git push` or deploy without his explicit "yes" for that step. Local rehearsals on restored dumps are free.
2. **All of this ships only inside the v1.3 cutover.** Production is origin/main `c03734ae`, schema 84, no `schema_migrations`. No hotfixing of the old code, no push, unless Felipe decides.
3. **Tombstones are the only ban until mine is live.** Never hard-delete or un-delete the 6 soft-deleted users or 3 tombstoned agents. Never drop `users_refuse_tombstoned_email` until its replacement is deployed and backfilled. Every new check must still refuse a tombstoned identity (not only rows in the new ban table).
4. **Migrations are linear and ride the cutover.** Next is `000114`. I bump `cmd/cutover --expect-version` and its test, update `cutover_rollback_test.go` (29 down files today), and every new down migration **archives into `rollback_archive`** (000088/000089/000109 style), never silently drops.
5. **Never run a loop runner** (`ralph*.sh`), not even `--help` or `| head`.
6. **Work in a `git worktree add --detach` checkout against scratch databases.** Never touch `spec.json` or the main tree. `ralph` commits with `git add .`, so nothing may be left lying in the main tree.

## Handover discrepancies

What I checked (by reading code, running read-only SQL on local rehearsal DBs, or running read-only commands):

**Confirmed as written:**
- W0 recursion at `oauth_user.go:136-139`.
- Both finders filter `deleted_at` (`db/users.go:146,160`).
- `ErrDuplicateEmail` is mapped by substring (`db/users.go:103`).
- Orphan-user path at `oauth_user.go:161-170`.
- W3 mount order: the limiter is at `router.go:68` and auth only inside groups (`:536,543,586,…,817`). The identity-less pass-through is at `ratelimit.go:128-132`. `DetectOperation` is at `:342-367`. Human `CreatedAt` is empty.
- Legacy `Status: open`: `problems.go:261`, `questions.go:314`, `ideas.go:292`, fed through `DeriveStates` (`models/post.go:52-54`).
- Dead duplicate code: no production caller of `NewModerationService`, `NewInMemoryDuplicateStore` or `NewContentDuplicateRepository`.
- Groq runs only at `posts.go:578-580` and `:770`. `BlogHandler` is given the service and never calls it.
- Agent key auth has no owner check (`db/agents.go:785,806`).
- Two `ErrAgentNotFound` values (`handlers/agents.go:24` vs `db/agents.go:20`) cause a 500 at `agents_claim.go:226-230`.
- `Unpin` does not escape the CID (`ipfs.go:113`).
- The crystallization candidate query ignores the author (`db/post_crystallization.go:27-50`).
- CSV: 87 unique CIDs, 67 pins (62 + 5) and 20 crystallized-post sources.
- Local `solvr-postgres`: glibc 2.36, every local DB at `datcollversion` 2.36, `amcheck` 1.4 available.
- The emergency trigger is absent from the local rehearsal DBs (expected: it exists only on production).

**Discrepancies and additions:**
- **D1 (path).** `legacy_dependency_registry.go:61` lives in `internal/db/`, not `handlers/`.
- **D2 (count).** Local main is 210 ahead of origin/main, not 209. The extra commit is the handover commit `6efe181b` itself. Harmless.
- **D3 — important. Legacy contributions never reach `replies`.**
  - In v1.3 code, `POST …/answers`, `…/approaches`, `…/responses`, `…/comments` and `…/progress` write the legacy tables `answers`, `approaches`, `responses`, `comments` and `progress_notes`, not `replies`.
  - No trigger mirrors them. Verified on `solvr_rehearsal_purge113`: `pg_trigger` shows none on those tables.
  - Consequences: (a) my repeat gate must search the legacy tables and `replies` together, because the NaoParis case was 187 legacy answers; (b) a cutover-level issue outside my scope, in Questions.
- **D4. Hard-delete endpoints exist.** `DELETE /admin/users/{id}` and `DELETE /admin/agents/{id}` (`router.go:114-115`) run a plain `DELETE`. Used on a tombstoned spammer, they free the email that the trigger anchors on. This is a constraint-3 trap. My ban rows are not FK-bound, so a ban survives a hard delete.
- **D5. Three more moderation bypasses, not in the handover** (all REAL by reading, none run yet):
  - (a) `PATCH /v1/posts/{id}` with only `{"status":"open"}` on a pending_review, rejected or draft post: `needsReModeration` is false (`posts.go:727-735`), so the post goes open and `DeriveStates` makes it approved.
  - (b) `POST /v1/blog` accepts `status:"published"` from any authenticated identity (`blog.go:246-253`), with no moderation.
  - (c) `POST /v1/rooms/{slug}/posts/{postID}/publish` (the room owner) publishes and approves without Groq (`db/posts_outcomes.go:82-88`).
- **D6. `POST /v1/mcp` creates nothing.** The `solvr_post` and `solvr_answer` tools are stubs (`h/mcp.go:321,342`). The real MCP server (`mcp-server/src/api.ts`) calls the REST routes, so it is covered by them. `POST /v1/auth/moltbook` has a nil service and persists nothing.
- **D7. `db.ClaimAgent` does not filter `deleted_at`** (`db/claim_tokens.go:194-219`).
- **D8. `responses` and `progress_notes` have no `deleted_at`,** and `progress_notes` has no author columns. This limits how a rejection can hide them (see W2).
- **D9. Human API keys bypass the posts/answers limits.** When a human uses a `solvr_sk_` key with a tier, `getLimitAndWindowWithAPIKey` replaces the per-operation limits with the general per-minute limit.
- **D10. The file-size rule is already broken in 34 files.** Among them: `router.go` 1244, `handlers/agents.go` 1082, `db/agents.go` 937, `db/posts.go` 868, `posts_moderation_test.go` 947, and `ratelimit_test.go` at exactly 800. See checklist item 10 for how I handle it.
- **D11.** `SetFlagCreator` is never called, so exhausted Groq retries leave no flag. Noted only; I don't fix it.
- **Not re-run:** the named baseline (4,747 PASS / 50 FAIL at `619b44b8`). I compare against it by name at the end (checklist item 8).

**Spike: denylist measured against real data.** Read-only SQL on local restores.

| Measurement | Result |
|---|---|
| R2 + R3 + day-counter rule, xiezhen's 1,321 posts (`solvr_rehearsal_prepurge`) | **973 caught** |
| Same rule, the 524 live survivors (`solvr_rehearsal_purge`) | **0 false positives** |
| Adding "N hours" to the counter | 5 legit survivor false positives ("every 2-4 hours"), so hours are excluded |
| Normalized-title same-author repeat, xiezhen | only 59 of 1,321 |
| Rate limit: posts beyond 3/hour | only 14 (p50 1/hour, max 8/hour, spread over 1,039 hours) |
| Rate limit: posts beyond 1/hour (the halved new-account limit) | 282 |
| NaoParis: 187 answers | 2 distinct bodies, so the reply-repeat gate blocks 185 |
| Same-author reply-body repeats among the 459 live survivor replies | 0 |
| Same-author normalized-title repeats among the 524 live survivor posts | 0 |

The conclusion is **MEASURED**: the rate limiter does not stop drip spam. The denylist, the repeat gate and Groq with the author's history are the real defenses.

## Acceptance checklist — point-by-point

1. **Six constraints restated** → section above.
2. **Facts re-verified; W0 and W3 red first** → see Discrepancies. Phase 1 writes two red tests before any fix:
   - W3 goes through the real router with a real agent key.
   - W0 goes through the real `OAuthUserService` on a real DB, with a context deadline and a hang guard.
   - Each runs once, with output saved.
3. **Every create path named, each with a gate test, including the five required cases** → the W1/W2 route table and test list below. The required cases: NaoParis cross-post reply (legacy answer and canonical reply, plus cross-kind), N-Day title, heartbeat, `[Watchdog]`, and a legit unique post that passes.
4. **W3 limits are DB-counted and tested through the real router** → the count queries the authoritative tables over a trailing hour. Tests use `NewRouter` + httptest with a real `POST /v1/agents/register` key and a real JWT. No injected context.
5. **W4** → `000114_banned_identities` up/down, with the down archiving to `rollback_archive`. One test per entry point (all 9). The seed covers the 7 accounts. `--expect-version` is bumped and the rollback test updated. The trigger's fate is stated (option A recommended; Felipe decides).
6. **W5** → `POST /admin/ipfs/unpin {cids, dry_run}` refuses in-use CIDs, escapes the CID and reports per CID. A runbook uses the CSV. Crystallization skips deleted or banned authors, with a test.
7. **W6** → rehearsed runbook with measured timings and a duplicate pre-check on the 34 unique indexes. The production step sits in the G3 write-pause window, gated by Felipe. No schema or code change.
8. **Test evidence** → the full backend and frontend suites each run once at the end, on a fresh scratch DB, logs saved. Failures are diffed by name against the handover's 50. New failures are listed separately.
9. **No unguarded production action** → every production-touching step is in the gate list (Phase 10). None runs without Felipe's "yes".
10. **Files and stores** → no new file reaches 800 lines. Any file already over 800 that I touch (`router.go`, `handlers/agents.go`, `db/agents.go`, `db/posts.go`) gets **net ≤ 0 lines**: call sites only, logic in new files. New tests go in new files. No in-memory data store is added, and `services/duplicate.go` is deleted.

## The plan

### Phase 0 — setup
Create the worktree and a scratch database:
```
git worktree add --detach <scratchpad>/antiabuse 6efe181b
```
- Scratch DB `solvr_antiabuse` on `solvr-postgres:5435`, migrated to head. `DATABASE_URL` points at it for every run.
- The migration tests use `newMigratedScratchDatabase`, which makes a DB per test.
- I kill any stale API or test processes before each run.

### Phase 1 — red tests (the `next_action`)

**W3 red** — `internal/api/router_create_ratelimit_test.go`
- Setup: `NewRouter(pool,nil,nil)` behind httptest. Register a real agent with `POST /v1/agents/register`.
- Action: create `effective+1` posts with unique legitimate titles, where `effective = loadRateLimitConfig(pool).AgentPostsPerHour/2` (halved because the account is new).
- Assert: the last request gets 429 `RATE_LIMITED`. Today it gets 201, so the test is red.
- A human-JWT twin uses `createLiveTestUser`.

**W0 red** — `internal/services/oauth_user_db_test.go`
- Setup: real `db.UserRepository` and `db.AuthMethodRepository`. Create a user with a github auth method, then soft-delete it.
- Action: call `FindOrCreateUser` with a 3-second context deadline, inside a goroutine guarded by a 10-second `time.After`.
- Assert: `errors.Is(err, ErrAccountSuspended)` within 1 second. Today it loops until the deadline, so the test is red.
- A second test: same provider id, new email. Assert `ErrAccountSuspended` **and** that no new `users` row exists. Today this leaves an orphan and a 500, so it is red.

Each test runs once, with output saved to `<scratchpad>/red-w3.log` and `red-w0.log`.

### Phase 2 — W0: account-path correctness

**`oauth_user.go`**
- Before step 1, call `identityGate.CheckOAuth(provider, providerID, email)`. It refuses when the identity is banned, or when the provider id or `lower(email)` belongs to a tombstoned user. The result is `ErrAccountSuspended`.
- Replace the recursion with a bounded loop (at most 2 passes: a race with a *live* user resolves on pass 2). Otherwise return the error.
- The new-user path becomes `UserRepository.CreateWithAuthMethod` in a single transaction, in a new file `db/users_oauth_create.go`.
- `users.Create` also maps P0001 `account suspended` (the trigger) to `ErrAccountSuspended`.

**`handlers/oauth.go` callbacks** map `ErrAccountSuspended` to 403 `ACCOUNT_SUSPENDED` (JSON, like the other callback errors). The helper goes in a new file.

**Claim flow**
- In handlers, `var ErrAgentNotFound = db.ErrAgentNotFound`, so both match `errors.Is`. `agents_claim.go:226` then returns 404.
- `db.ClaimAgent` filters `deleted_at`.

**Agent owner check** — `auth/apikey_validate.go` gets an optional `AccountChecker`. If `agent.HumanID != nil` and the owner is not active (existing `db/users_active.go`), it returns 401 `INVALID_API_KEY`. That is the same status as a forged key, which keeps the `router_deleted_account_test` contract. Wired at the existing validator construction.

**Tests**
- Owner-deleted agent key gets 401 on 3 routes, through the real router.
- Claim of a soft-deleted agent gets 404.
- Callback 403 in a handler test: real service, mock GitHub/Google servers via `NewOAuthHandlersWithDeps`. The base URLs are hardcoded in the router, so a router-level test cannot reach a mock provider.

### Phase 3 — W4: ban list

**`000114_banned_identities.up.sql`**
```sql
CREATE TABLE banned_identities (
  id BIGSERIAL PRIMARY KEY,
  kind     TEXT COLLATE "C" NOT NULL CHECK (kind IN ('email','oauth','agent_id')),
  provider TEXT COLLATE "C" NOT NULL DEFAULT '',   -- oauth only: github|google
  value    TEXT COLLATE "C" NOT NULL,              -- email lowercased; provider id; agent id
  reason TEXT NOT NULL, source_type TEXT, source_id TEXT,
  created_by TEXT NOT NULL DEFAULT 'operator', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT banned_identities_unique UNIQUE (kind, provider, value),
  CONSTRAINT banned_identities_email_lower CHECK (kind <> 'email' OR value = lower(value)));
```
- `COLLATE "C"` keeps the new unique index out of the glibc problem (W6).
- **Seed:** emails and `auth_methods` rows of users `xiezhen223600`, `gongli0929`, `shan_he`, `xu_wei`, plus the `agent_id` of `agent_NaoParis`, `agent_frogtrader`, `agent_openclaw_mack`. Selected by name/id, `ON CONFLICT DO NOTHING`, with a `RAISE NOTICE` of the counts. `srcjj777` is not seeded.
- **Trigger, option A (recommended; FELIPE'S CALL):** the migration `CREATE OR REPLACE`s `users_refuse_tombstoned_email` with the verbatim body **plus** `OR EXISTS (banned email)`, then re-creates the trigger. This is idempotent over production's copy and puts the trigger under version control.
- **Option B:** leave production's unversioned trigger as it is, and drop it later in a separate Felipe-gated step once the ban list is deployed and backfilled.
- The replacement lands at cutover gate G4. Under both options the trigger is never dropped by 114, up or down.
- **`000114.down.sql`:** archive every `banned_identities` row into `rollback_archive` (reason `'000114 down: ban list'`), then drop the table. Under option A, restore the **verbatim** original function body. The trigger stays.

**Code**
- `db/banned_identities.go` holds a single `IsRefused(ctx, IdentityQuery{Email, Provider, ProviderID, AgentID})`. It is the union of:
  - the ban table;
  - tombstoned users by `lower(email)` or by `auth_methods` of a deleted user;
  - tombstoned agents by id.

  The union is what satisfies constraint 3 for the 2 older tombstones without seeding them.
- `services/identity_gate.go` returns `ErrAccountSuspended`.
- `handlers/identity_gate.go` writes the 403 and holds the `SetIdentityGate` methods.

**Entry points, each with a test (403 `ACCOUNT_SUSPENDED`):**

| Entry point | Test style |
|---|---|
| GitHub callback `oauth.go:176` | handler test (see W0) |
| Google callback `:306` | handler test (see W0) |
| `POST /v1/auth/register` (`auth.go:115`) | router test |
| `POST /v1/auth/login` (`:287`) | router test |
| `POST /v1/auth/oauth/exchange` (`oauth_login_code.go:72`): live user whose email is banned | router test |
| `POST /v1/agents/register` (`agents.go:225`): agent id and email | router test |
| `POST /v1/agents/me/claim` (`agents_claim.go:76`) | router test |
| `POST /v1/agents/claim` (`:178`) | router test |
| Room handshake (`rooms_handshake.go:45`) | router test |

**`POST /admin/bans`**
- Body: `{account_type, account_id, reason, include_owner:false, dry_run:false}`.
- One transaction:
  - Tombstone the account (keeping an existing `deleted_at`).
  - Delete its API keys and refresh tokens (users) or its key hash (agents).
  - Insert ban rows: user → lowercased email plus each `auth_methods` row; agent → agent id.
  - With `include_owner`, do the same for the owner. That is FELIPE'S case-by-case call; the default is off.
- Response: the rows and the tombstone state.
- Wiring: a new `router_admin_abuse.go` mounts it with `operatorOnly`, called by one line in `router.go`. It is added to the `route_families.go` administration list.
- Tests: the transaction rolls back on failure; `dry_run` writes nothing.
- There is no unban or restore endpoint: FELIPE'S CALL.

**Cutover wiring**
- `cmd/cutover` default `--expect-version` becomes 115, and `main_test.go` asserts 115.
- `cutover_rollback_test.go` expects 31 down files (115..85). It inserts a ban row and expects `"banned_identities": 1` in the archive map.
- **`000115_contribution_author_indexes`:** `replies(author_type, author_id, created_at)`. `replies` has no author index today, and the gate and limiter query by author on every create. The down migration drops the index, which is not data. I would fold it into 114 if the author prefers (see Questions).

**Rehearsal**
- Copy `solvr_rehearsal_purge113` via `CREATE DATABASE … TEMPLATE`, then run 114 and 115 up.
- Seed counts must equal 4 emails + N auth methods + 3 agents, where N is measured and reported.
- Run down (archive counts) and up again. Timings are recorded.

### Phase 4 — W1: deterministic gates (synchronous, before the INSERT and before Groq)

**`services/content_gate.go`**
- Denylist, ported 1:1 from the measured SQL:
  - R2: `(?i)^\s*(\[watchdog\]|agent death:)`
  - R3: `(?i)heartbeat`
  - day-counter: `(?i)(\d+(\.\d+)?\+?(st|nd|rd|th)?[- ]?days?([^a-z]|$))|((^|[^a-z])day[- ]?\d+(\.\d+)?([^a-z0-9]|$))|(\d+(\.\d+)?天)`
- A hit returns 422 `CONTENT_NOT_ALLOWED` `{rule}`.
- Repeat check: 409 `DUPLICATE_CONTENT` `{existing_id, existing_type}`.

**`db/content_duplicates.go` (extended; normalization is done in SQL on both sides, one source of truth)**
- `FindAuthorPostByTitle`: live posts by the same `posted_by_*` where R4-normalized titles are equal (lowercase, each digit → `#`, whitespace collapsed, trimmed).
- `FindAuthorContribution`: the normalized body (lowercase, trimmed, whitespace collapsed; digits kept) compared across `replies` (non-system), `answers.content`, `approaches.method`, `responses.content` and `comments.content` by the same author, on **any** post.
- `FindAuthorBlogByTitle`: the same idea for blog posts.
- "Live" means `deleted_at IS NULL` in any moderation state. A rejected post still blocks a verbatim resubmit, and the 409 points the author at it to edit.

**Hook:** each handler gets `SetContentGate(g)`; nil means a no-op for existing mock tests. The call goes right after `GetAuthInfo` and validation. The system moderation writer is untouched.

| # | Route | Gate |
|---|---|---|
| 1 | `POST /v1/posts` | title denylist + author title repeat |
| 2 | `POST /v1/posts/{id}/replies` | contribution repeat |
| 3–5 | `POST /v1/problems`, `/v1/questions`, `/v1/ideas` | title denylist + repeat |
| 6 | `POST /v1/problems/{id}/approaches` | contribution repeat (method) |
| 7 | `POST /v1/approaches/{id}/progress` | repeat against the author's notes, joined through `approaches.author` |
| 8 | `POST /v1/questions/{id}/answers` | contribution repeat |
| 9 | `POST /v1/ideas/{id}/responses` | contribution repeat |
| 10 | `POST /v1/{approaches,answers,responses,posts}/{id}/comments` | contribution repeat |
| 11 | `POST /v1/blog` | title denylist + author blog repeat |
| 12 | `POST /v1/rooms/{slug}/save-as-post` | title denylist + repeat |

Excluded, with reasons:
- `/ideas/{id}/evolve` carries no text.
- `/v1/mcp` is a stub.
- Room messages and entries are coordination chat, where repeats like "ok" are normal. Question 9.
- Pins and checkpoints carry no user prose.

**Tests** (`router_content_gate_test.go` and `router_content_gate_contrib_test.go`, real router, real keys):
- **NaoParis:** an agent answers question A with body X, then question B with X → 409 with A's answer id. Also answer-then-`/replies` on another post with X → 409 (cross-kind).
- Titles: "Quantum Monitoring Persistence Breakthrough: 47-Day Continuous Operation Verification" → 422 `day_counter`; "Heartbeat Check - Tuesday Morning" → 422 `heartbeat`; "[Watchdog] gateway down" → 422 `watchdog`.
- A unique legit post → 201. A legit "every 2-4 hours" title → 201.
- One gate test per remaining route in the table.
- Unit table tests on the regex: the sampled spam titles, plus legit ones such as "Go 1.22", "30 seconds" and "Tuesday".

**Cleanup**
- Delete `services/duplicate.go` and its test.
- Remove `DuplicateDetectionService` from `ModerationService`'s constructor, `CheckDuplicate` and its tests. `ModerationService` stays otherwise untouched (unused; removing it is not asked).
- Correct `legacy_dependency_registry.go:61` to point at the real content gate.

### Phase 5 — W3: rate limiter

**Identity before the limiter**
- Remove the root `r.Use(rateLimiter.Middleware)`.
- A new `router_ratelimit.go` adds `withLimit(authMW) = func(next){ return authMW(rl.Middleware(next)) }`. Every `r.Use(auth.X(...))` group line and the room auth-middleware variables (`router.go:216-221`) use it, as line edits with net 0 lines.
- A test asserts one count per request: `X-RateLimit-Remaining` decrements by exactly 1.

**DB-counted creates** — a new `middleware/ratelimit_create.go` adds `CreateCounter` and `db/create_counts.go`.
- Operation `posts` counts `posts` and `blog_posts` rows by author in the last hour.
- Operation `contributions` counts `replies` (non-system), `answers`, `approaches`, `responses`, `comments`, and `progress_notes` via the approach author.
- Deleted and rejected rows **are counted**, so deleting does not reset the budget.
- Allow when `count < limit`, using the existing `Agent/HumanPostsPerHour` and `Agent/HumanAnswersPerHour` config.
- The same query returns the account's `created_at`, so the new-account halving can apply to humans (Question 3; recommended yes). The API-key tier override does not apply to create operations (fixes D9).
- Concurrent requests can overshoot by the number in flight. Accepted; noted.

**`DetectOperation`** gains `contributions` for POST `…/replies|approaches|answers|responses|comments|progress`, and counts `POST /v1/blog` and `…/save-as-post` as `posts`.

**The in-memory per-minute store stays** for general and search limits. It holds throttle counters, not data. See Question 5 on turning general limits on for humans.

**Tests:** the W3 red test turns green, plus the human twin, a contributions twin, a per-request single-count test, and a human-with-`solvr_sk_`-key create-limit test.

**Existing router tests** that send more than 30–60 requests a minute with one live identity may start seeing 429s. I run those packages (targeted runs) before the final full run and raise the limits in test setup through `rate_limit_config` where needed.

### Phase 6 — W2: moderation coverage

**Legacy typed creates (routes 3–5)**
- Start at `pending_review`, like `/v1/posts` (`posts.go:515`).
- Moderate by calling the existing flow: export `PostsHandler.StartModeration(...)` as a thin wrapper over `moderatePostAsync`. The router wires `problems|questions|ideasHandler.SetPostModerator(postsHandler.StartModeration)`. No duplicated flow.

**Prompt**
- Add rule 6: automated status, heartbeat or watchdog reports, and templated day/milestone series.
- Add rule 7: near-repeats of the author's own recent titles.
- The user message gains `Author's recent titles:` (the last 20 live titles, read by a small DB reader set on `PostsHandler`).
- `ModerationInput` gains `AuthorRecentTitles` in both services and handlers, and the adapter maps it.
- Test: an httptest Groq server asserts the prompt text and the user message.

**Replies and contributions (routes 2, 6–10)** — **my answer to Open Question 1: soft-delete on rejection.**
- Why: every read path already filters `deleted_at`. A hidden-pending state would need a new column on `replies`, plus filters in stats, homepage, search, crystallization and counts. That is a large blast radius for a cutover release.
- A new `handlers/contribution_moderation.go`: after a 201, run Groq asynchronously with Title = the parent post title and Description = the body.
- On reject:
  - Soft-delete the row (`replies`, `answers`, `approaches`, `comments`).
  - Create a `flags` row (`reporter_type system`, reason `moderation_rejected`, with the explanation).
  - Notify the author.
- `responses` and `progress_notes` have no `deleted_at` (D8): they get a **flag only**.
- Trade-off: a rejected reply is visible for the few seconds Groq takes.

**D5 bypasses (propose to include; Question 6):**
- **(a) PATCH.** If the requested status derives published and the current `moderation_state` is not approved, force `pending_review` and re-moderate (non-family only). Red test first through the real router.
- **(b) Blog.** A `status:"published"` create or update runs Groq, and a reject sets the post back to `draft` and notifies the author. Or, FELIPE'S CALL: restrict blog creation to operators.
- **(c) Room publication.** Owner approval sets `pending_review` and moderates, instead of approving directly.

All of the above only when `GROQ_API_KEY` is set, which is the existing condition.

**Measured, optional (costs Groq calls):** run 20 xiezhen titles with their recent history plus 20 survivor titles through the new prompt, and report the confusion matrix. I do this only if Felipe OKs the spend.

### Phase 7 — W5: IPFS

**`POST /admin/ipfs/unpin` `{cids:[…≤500], dry_run}`**
- Per-CID results:

| Status | Meaning |
|---|---|
| `invalid` | fails the CIDv0/v1 format check |
| `in_use_pin` | any `pins` row references it |
| `in_use_post` | any `posts.crystallization_cid` references it, regardless of `deleted_at` |
| `would_unpin` | dry run only |
| `unpinned` | pin removed |
| `not_pinned` | Kubo's "not pinned" error |
| `error` | anything else |

- Plus a summary.
- `Unpin` uses `url.QueryEscape`. I propose escaping the other `arg=` sites in `ipfs.go` too; Question 11.

**`POST /admin/ipfs/gc`** is a separate, explicit endpoint for Kubo `repo/gc`, with a long timeout and a count of removed keys. It is called only as its own Felipe-gated step.

**Crystallization:** `ListCrystallizationCandidates` adds `NOT EXISTS` on a tombstoned human or agent author, and on an `agent_id` ban. A DB test proves the post of a deleted author, and the post of a banned author, are skipped.

**Tests**
- Handler tests with a fake IPFS: dry run, in-use refusal, an escaped CID with `+` or `/` rejected as invalid.
- One integration test against the local `solvr-ipfs` container (Kubo 0.33.2): add, pin, unpin, then unpin again → `not_pinned`. It skips when `IPFS_API_URL` is unset.

**Runbook** (`docs/handovers/2026-09-29-solvr-anti-abuse/RUNBOOK-ipfs-unpin.md`):
1. Build the JSON from the CSV with `jq`.
2. Dry run. Expect 87 `would_unpin` and 0 `in_use`.
3. **[Felipe gate]** The real run; save the report.
4. **[separate Felipe gate]** gc.

### Phase 8 — W6: collation (runbook + rehearsal; no production write, no code)

**1. Reproduce faithfully**
- Stamping `datcollversion='2.41'` only reproduces the *warning*: every local index was built under 2.36, so `amcheck` would pass trivially.
- A faithful copy: restore the pre-purge dump into a **glibc 2.41** container (`pgvector/pgvector:pg17-trixie`, or `postgres:17-trixie` + `postgresql-17-pgvector`) on a throwaway volume. Stop it, then start the **same volume** under the 2.36 bookworm image. That is exactly production's condition: indexes built under 2.41, read under 2.36.
- Fallback if trixie can't be pulled: stamp only, and label the results "warning reproduced, mis-order unmeasured".

**2. Duplicate pre-check**
- A `DO` block iterates over the unique indexes that depend on the default collation (from `pg_depend`/`pg_index`; expect 34).
- It builds `GROUP BY <key exprs> HAVING count(*)>1` from `pg_get_indexdef(indexrelid,k,true)` plus the partial predicate.
- It runs with `enable_indexscan`, `enable_bitmapscan` and `enable_indexonlyscan` off.
- Then `bt_index_check(idx, heapallindexed=>true)` on all 92.

**3. Time the repair.** `REINDEX DATABASE`, then `ALTER DATABASE … REFRESH COLLATION VERSION`. I also time a targeted `REINDEX` of just the 92, because `REINDEX DATABASE` rebuilds the HNSW vector indexes too, and recommend whichever is faster.

**4. Production placement.** Inside cutover G3, with the API stopped, before G4 migrations:
- (a) Read-only duplicate pre-check. Any duplicate means **STOP**; Felipe decides.
- (b) Optional `amcheck`. It needs `CREATE EXTENSION`, which is its own Felipe gate.
- (c) REINDEX.
- (d) `REFRESH COLLATION VERSION`.
- (e) G4.

**5. Recommend pinning** the production Postgres image to a Debian-suffixed tag or digest, set in EasyPanel by Felipe.

**Runbook:** `RUNBOOK-collation.md`, with measured timings.

### Phase 9 — verification
- `bash scripts/check-file-size.sh`: no new violator, and touched pre-existing violators show net ≤ 0 lines.
- `grep -rn InMemory` over the new code: nothing.
- Run once each, logs saved:
  - `go test -p 1 ./... 2>&1 | tee <scratchpad>/backend-full.log` on a **fresh** scratch DB;
  - `cd frontend && npm test 2>&1 | tee <scratchpad>/frontend-full.log`.
- Failures are extracted from the log with `awk` and diffed by name against the handover's 50. New failures are listed separately, with their assertion text.
- `go build ./...` and `golangci-lint run` on the touched packages.

### Phase 10 — production gates (none run without Felipe's explicit "yes", each separately)

| Gate | Action |
|---|---|
| P1 | Read-only `SELECT`s via `/admin/query`: `rate_limit_config` values, and a preview of the seed identities |
| P2 | Trigger option A or B |
| P3 | Commits in the worktree (one-line messages) and how they reach `main` |
| P4 | Push and deploy: the existing cutover gates G1/G6 |
| P5 | W6 steps (a)–(d) inside G3 |
| P6 | 114/115 applied with G4 |
| P7 | `POST /admin/ipfs/unpin`, real run |
| P8 | `POST /admin/ipfs/gc` |
| P9 | Any `POST /admin/bans` beyond the seed |
| P10 | Optional Groq evaluation spend |

No loop runner, no `spec.json` edit, no tags.

## Concerns
- **[HIGH] D3 (cutover scope):** after cutover, the still-mounted legacy routes write to tables v1.3 no longer reads. That content becomes invisible and uncounted. My gates cover those routes, but the visibility gap belongs to the cutover owner.
- **[MEDIUM]** Enabling the limiter after auth turns on **general per-minute limits for authenticated users for the first time in production** (default: agents 60/min, humans 30/min). Browser sessions or bursty agents could get 429s. Unmeasured.
- **[MEDIUM]** The absolute day-counter rule blocks legitimate titles like "Building a 30-day retention job". The measured false-positive rate is 0/524 today, but it is a real risk. A "series" variant (block only an author's second counter title) would catch 972 of 973.
- **[MEDIUM]** Legacy creates switching to `pending_review` changes responses for the CLI, MCP server and skill, which may assume `open`.
- **[LOW]** Soft-deleting rejected replies leaves them publicly visible for a few seconds. Very short repeated bodies ("thanks") by one author get 409.
- **[LOW]** Unpin refuses any CID referenced by *any* `pins` or `posts` row, dead ones included. That is safe, but a future ban's content needs its pin rows removed first.
- **[LOW]** Hard delete (D4) of a tombstoned user frees the email for the trigger. The ban rows persist.

## Questions for the author
1. **Open Question 1:** soft-delete + flag + notify for rejected contributions; flag-only for `responses` and `progress_notes`. Accept?
2. **Open Question 2 (FELIPE'S CALL):** trigger option A (version it and extend it with the ban email; recommended) or B (drop it later, gated)?
3. **Open Question 3:** apply the new-account halving to humans? I recommend yes: 4 of the 7 purged accounts were humans, and `created_at` comes from the same query.
4. Day-counter: absolute (the handover's wording) or series-only?
5. General per-minute limits for authenticated **humans**: enable, or limit creates only? Agents get both either way.
6. Include the D5 bypass fixes (PATCH self-approve, blog, room publish) in this scope? Blog restricted to operators is FELIPE'S CALL.
7. OAuth callback refusal: 403 JSON, as the handover says and like the other callback errors, or a redirect to `/auth/callback?error=…`, which the frontend already renders?
8. D3: are the legacy contribution routes staying mounted after cutover, and who owns the visibility gap?
9. Room messages and entries excluded from the W1 gates. OK?
10. `000115` as a separate index migration, or fold it into 114?
11. Escape every `arg=` in `ipfs.go`, or only `Unpin`?
12. Is `POST /admin/ipfs/gc` an acceptable way to "offer repo/gc" without SSH?
13. Should `DELETE /admin/users|agents/{id}` refuse tombstoned or banned accounts? That is FELIPE'S CALL.
