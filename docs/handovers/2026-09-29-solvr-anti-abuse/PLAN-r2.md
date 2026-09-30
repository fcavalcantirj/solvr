---
round: 2
builder_session: same builder session as r1 (Opus 5.5). Read REVIEW-r1 and the handover's SUPERSEDED section, then re-verified the facts each change depends on. 2026-09-29.
---

# Plan r2 — Solvr anti-abuse (delta on PLAN-r1)

**How to read this.** PLAN-r1 stands as approved except for the sections below. Each section marked **REPLACES** substitutes for the r1 section it names, in full. Anything not mentioned here is unchanged from r1. The consolidated execution order at the end removes any ambiguity from the merge.

| Review change | Where it lands |
|---|---|
| 1. W2 tests enumerated per path | Phase 6 (REPLACES r1 Phase 6), test matrix T-M1…T-M12 |
| 2. D5(a), (b), (c) required | Phase 6, items 6.4–6.6, each with a red test first |
| 3. W3: create limits only, per route | Phase 5 (REPLACES r1 Phase 5) |
| 4. Fold 000115 into 000114; version 114; 30 down files | Phase 3 cutover wiring (REPLACES the r1 subsection) |
| 5. OAuth callbacks: 302 to `/auth/callback?error=account_suspended` | Phase 2 (amends the callbacks bullet) and the W4 table |
| 6. Series-only day counter | Phase 4 denylist (amends) |
| 7. Option A on a fresh database; targeted runs of affected tests | Phase 3 migration header, Phase 9 targeted runs |

## Blocking constraints, restated
Unchanged from r1; the review marked all six correct. One addition from the handover's SUPERSEDED section: the cutover is ledger **idx 93** again, and `spec.json` is still Felipe's and still uncommitted. I don't touch it.

## Handover discrepancies (new in r2; r1's D1–D11 stand)

Checked this round:

- **D12. `flags` CHECK constraints conflict with the review's spec, and with existing code.** Verified on `solvr_rehearsal_purge113` with `pg_get_constraintdef`:
  - `flags_reason_check` allows only `spam`, `offensive`, `duplicate`, `incorrect`, `low_quality`, `other`. So `moderation_rejected` (change 1) would violate it.
  - It already rejects the `moderation_failed` that `posts_moderation.go:62` writes. That bug is latent only because `SetFlagCreator` is never wired (D11).
  - `flags_target_type_check` allows `post`, `answer`, `response`, `approach`, `comment`, `reply`. There is no `progress_note`, which the flag-only path for route 7 needs.
  - **Resolution, inside 000114** (it is a schema change):
    - Widen `reason` with `moderation_rejected` and `moderation_failed`, and `target_type` with `progress_note`.
    - The down migration archives any `flags` row using a new value into `rollback_archive`, deletes it, and restores the original CHECKs verbatim.
- **D13. The room save-post test uses the D5(a) path as its legitimate publish flow.**
  - `router_rooms_save_post_test.go:151-160`: "Author publishes via the ordinary post-update flow", a PATCH to `open` on a room draft, which is expected to be published immediately.
  - With D5(a) closed, that PATCH goes to `pending_review` and moderation. The test changes: it wires an approving mock through the seam in 6.1, then waits for the post to become published.
  - The same applies to the owner-approve publish tests (`:191+`) under D5(c).
  - These are expectation changes that follow directly from change 2. They are not new behavior.
- **D14. `MockContentModerationService` is invisible to router tests.**
  - It is declared in `handlers/posts_moderation_test.go:37`, a `_test.go` file in package `handlers`. Package `api` router tests cannot import it.
  - The router builds moderation only from `GROQ_API_KEY` (`router.go:303-309`) and has no injection seam.
  - I add a seam (6.1) and a package-`api` recording twin with the same `GetCalls()` / `QueueResults` shape.
- **D15. The OAuth callback error convention is confirmed.**
  - `handlers/oauth_integration_test.go:181-187` expects `302` with `error=access_denied` in `Location`. The frontend renders `searchParams.get("error")` (`frontend/app/auth/callback/page.tsx:26-31`).
  - The redirect base is `h.config.FrontendURL` (`oauth_login_code.go:56-60`), which I reuse.
- **D16. Series-only counter, now MEASURED** on `solvr_rehearsal_prepurge`, in creation order per author:

  | Author | Posts | Blocked, series rule | Blocked, absolute rule |
  |---|---|---|---|
  | xiezhen223600 | 1,321 | 972 | 973 |
  | shan_he | 12 | 11 | 11 |
  | agent_ClaudiusThePirateEmperor | 244 | 24 | 25 |

  Live survivors: 0 counter matches, so 0 hits. r1's "972 of 973" was an estimate; it is now measured.

## Acceptance checklist — point-by-point (changed items only)
- **3.** W1 is unchanged from r1. W2 is now enumerated per path in the Phase 6 test matrix T-M1…T-M12. Every test goes through the real router with moderation mocked at `handlers.ContentModerationServiceInterface`.
- **4.** The limit is DB-counted per create route, and exercised through the real router with real keys and a real JWT. General and search limits are untouched.
- **5.** Migration 114 only. `--expect-version` 114. The rollback test expects 30 down files. OAuth callbacks refuse with a 302 redirect; every other entry point returns 403 JSON.
- **1, 2, 6, 7, 8, 9, 10:** as in r1. Item 10 keeps the net ≤ 0 lines rule on pre-existing violators: the `r.With(...)` edits in `router.go` change lines in place, they don't add any.

## Phase 2 — W0 (amended bullet only)
- **OAuth callbacks (`oauth.go:176`, `:306`).** On `ErrAccountSuspended`, redirect with `302` to `{FrontendURL}/auth/callback?error=account_suspended`, using a helper `redirectWithError(w, r, code)` in a new file.
- The other callback error paths are unchanged. They stay JSON. The 4 named-baseline OAuth failures stay in the baseline; see Question 1.
- **Test:** a handler test with the real `OAuthUserService`, a scratch DB and a mock GitHub/Google server asserts `302` and `Location` containing `error=account_suspended`, and that no user row and no login code were created.

## Phase 3 — W4 (REPLACES r1's "Cutover wiring"; amends the migration)

**`000114_banned_identities.up.sql`**
1. `banned_identities`: as r1, `COLLATE "C"`.
2. Seed: as r1 (4 users' emails and `auth_methods`, 3 agent ids; not `srcjj777`).
3. Author index, folded in from the former 115:
   ```sql
   CREATE INDEX IF NOT EXISTS idx_replies_author_created ON replies (author_type, author_id, created_at);
   ```
4. D12: widen the `flags` CHECKs by dropping and re-adding them, with the values listed in D12.
5. **Trigger, option A** (still FELIPE'S CALL at P2): `CREATE OR REPLACE FUNCTION users_refuse_tombstoned_email` with the verbatim prod body plus an OR on a banned email; `DROP TRIGGER IF EXISTS` and `CREATE TRIGGER`.

   The **header comment states the fresh-database behavior (change 7), verbatim intent:**
   > On production this replaces the emergency trigger installed by hand on 2026-09-29. On a fresh database (tests, new environments) 114 CREATES it. 114 down restores the original function body and leaves the function and the trigger in place on both kinds of database: the trigger is the pre-ban-list emergency ban and is never removed by a rollback. Consequence: once 114 has run, any INSERT into users whose email matches (case-insensitively) a soft-deleted user's email, or a banned email, fails with 'account suspended' (P0001).

   Under **option B**, step 5 is omitted and the header says so.

**`000114_banned_identities.down.sql`** runs in reverse order:
1. Restore the verbatim original function body (option A).
2. Archive and delete the `flags` rows that use the new values, then restore the original CHECKs verbatim.
3. Drop the index.
4. Archive every `banned_identities` row, then drop the table.

It raises NOTICEs with the counts.

**Cutover wiring**
- `cmd/cutover` `--expect-version` default goes 113 → **114**. `cmd/cutover/main_test.go` asserts 114; its `checkSchema` cases use 114, and 84 is still rejected.
- `cutover_rollback_test.go`:
  - `require.Equal(t, 30, applied, "down migrations 000114..000085")`.
  - It inserts one `banned_identities` row and one `flags` row with reason `moderation_rejected` on the legacy answer.
  - The archive map becomes `{"votes": 2, "reports": 1, "replies": 3, "flags": 2, "posts": 1, "banned_identities": 1}`.
  - It asserts that `users_refuse_tombstoned_email` still exists at 84.
- Rehearsal: as r1, with 114 only.

## Phase 4 — W1 (amended: denylist)
- **Absolute rules, 422 `CONTENT_NOT_ALLOWED`:** R2 `watchdog` (`(?i)^\s*(\[watchdog\]|agent death:)`) and R3 `heartbeat` (`(?i)heartbeat`).
- **Series rule, 422 `CONTENT_NOT_ALLOWED` `{rule: "day_counter_series"}`:** the title matches the day-counter pattern **and** the same author already has a post (`deleted_at IS NULL`, any moderation state, so live or rejected) whose title matches it too.
  - For `POST /v1/blog` the lookup is on `blog_posts`.
  - For room save-as-post the lookup is on `posts`.
- **Regex parity.** The Go RE2 pattern and the Postgres pattern (used in `FindAuthorCounterTitle`) are the measured ones. One test asserts that both engines agree on a fixture list: the sampled spam titles plus legit ones ("Building a 30-day retention job", "every 2-4 hours", "Go 1.22", "Tuesday").
- **Required tests** (checklist item 3), updated:
  - "Quantum Monitoring … 47-Day …": the author's **first** such title → 201; the **second** → 422 `day_counter_series`. A second author's first counter title → 201.
  - Heartbeat → 422. `[Watchdog]` → 422. The NaoParis cross-post case, cross-kind case and legit unique post: unchanged from r1.

## Phase 5 — W3: create limits only (REPLACES r1 Phase 5)
- **Root limiter untouched.** `router.go:68` stays, and so does its behavior for everyone (the review's change 3; answer 5). `DetectOperation` is unchanged. The per-minute store is untouched.
- **New `middleware/create_ratelimit.go`: `CreateRateLimit(counter CreateCounter, cfg *RateLimitConfig, op string)`.**
  - It reads identity from context, since it runs after the group's auth.
  - It asks `db/create_counts.go` for the author's `count` over the trailing hour **and** the account's `created_at`, in one query.
  - It allows the request when `count < limit`, where the limit is `Agent/HumanPostsPerHour` for `posts` and `Agent/HumanAnswersPerHour` for `contributions`.
  - The limit is halved for accounts younger than `NewAccountThreshold`, **humans included** (answer 3).
  - API-key tiers are ignored for creates (D9).
  - On refusal it returns 429 `RATE_LIMITED` with `Retry-After` and sets `X-RateLimit-*`.
  - Anonymous requests pass through, and the auth group returns 401.
  - Counted rows include deleted and rejected ones, as in r1.
- **Mounted only on the create routes, with net 0 lines** (in-place edits):

  | Operation | Routes (`router.go` unless noted) |
  |---|---|
  | `posts` | `/v1/posts` :820, `/v1/problems` :957, `/v1/questions` :964, `/v1/ideas` :972, `/v1/blog` :837, `router_rooms.go:160` save-as-post |
  | `contributions` | `/v1/posts/{id}/replies` :831, `…/approaches` :958, `…/progress` :960, `…/answers` :965, `…/responses` :973, the four `…/comments` routes :977-981 |

  Where an idempotency middleware exists, the limiter goes **inside** it (`r.With(Idempotency(...), createLimit)`), so replaying a stored 201 never counts against the limit or gets refused by it.
- **Tests** (`router_create_ratelimit_test.go`, real router):
  - W3 red, then green: the agent key's `effective+1`th post gets 429.
  - The human JWT twin, with halving.
  - A contributions twin (answers).
  - A human `solvr_sk_` key twin (D9).
  - **Single count per request:** two sequential creates show `X-RateLimit-Remaining` dropping by exactly 1.
  - An idempotent replay of a successful create at the limit returns the stored 201, not 429.
  - General limits unchanged: 70 authenticated GETs in a minute all succeed. That pins the "not switched on" decision.

## Phase 6 — W2: moderation coverage (REPLACES r1 Phase 6)

**6.1 Seam (D14).**
- `internal/api/moderation_adapter.go` gains `var wrapContentModerator = func(svc *services.ContentModerationService) handlers.ContentModerationServiceInterface { return NewContentModerationAdapter(svc) }`.
- `router.go:309` calls it instead, as an in-place edit.
- Router tests do `t.Setenv("GROQ_API_KEY", "test")`, then swap `wrapContentModerator` for a recording mock and restore it in `t.Cleanup`.
- The mock lives in the new file `router_moderation_mock_test.go`. It records each input and has `GetCalls()`, `QueueResults()`, and `LastInput()` so prompt inputs can be asserted.
- Async results are awaited by polling the DB (at most 5 s, 50 ms tick) under a hard `time.After` guard.

**6.2 Legacy typed creates (routes 3–5):** start at `pending_review`, then run `postsHandler.StartModeration`, wired by `SetPostModerator`, as in r1.

**6.3 Contributions (routes 2, 6–10):**
- A new `handlers/contribution_moderation.go`, wired to `RepliesHandler`, `ProblemsHandler` (approaches, progress), `QuestionsHandler` (answers), `IdeasHandler` (responses) and `CommentsHandler`.
- Groq input: `Title` is the parent post's title, `Description` is the body.
- On reject, soft-delete where the table allows it (replies, answers, approaches, comments). Always write the `flags` row: `reporter_type system`, `reporter_id content-moderation`, `reason moderation_rejected` (D12), `details` = the explanation. Always notify the author.
- Responses and progress notes get the flag and the notification only; the row stays. Progress notes use the D12 `progress_note` target type.

**6.4 D5(a) — PATCH self-approve (required).**
- In `PostsHandler.Update`, *after* the existing room-owner gate: if the requested status derives `published` and the post's `moderation_state` is not `approved`, and it is not a family post, set `pending_review` and start moderation.
- Fail-safe: when moderation is not configured the post still stays `pending_review`, the same as `POST /v1/posts` (`posts.go:515`).
- Red test first (T-M9).

**6.5 D5(b) — Blog (required, moderation only).**
- A `status:"published"` create, or an update that sets or keeps `published` with changed title or body, runs Groq asynchronously.
- A reject sets the post back to `draft` and notifies the author.
- Restricting blog creation to operators is FELIPE'S CALL and not part of this plan.

**6.6 D5(c) — Room publication (required).**
- `ApprovePublication` moves an undecided draft to `pending_review` (new repo method `SubmitDraftForModeration`, replacing the call to `PublishDraftOutcome`) and starts moderation. An approved result publishes it.
- A retried approval while it is already `pending_review` returns 200 without writing again.
- A post that moderation rejected still returns `409 PUBLICATION_STATE_CONFLICT`, as today.

**6.7 Prompt:** rules 6 and 7, plus the author's recent titles, as in r1. The prompt test is T-M12.

**Test matrix.** Every row runs through `NewRouter` with real keys or JWTs and the mocked moderator (6.1).

| # | Route(s) | Mock result | Assertions |
|---|---|---|---|
| T-M1 | `POST /v1/problems`, `/v1/questions`, `/v1/ideas` (table-driven) | approve | Response `status: pending_review`; `GetCalls()==1`; post becomes `open` (`published`/`approved`) |
| T-M2 | same three | reject | Post becomes `rejected`; a `solvr-moderator` system reply exists; the author is notified |
| T-M3 | `POST /v1/posts/{id}/replies` | reject | Reply `deleted_at` set; `flags` row (`reply`, `system`, `moderation_rejected`); author notified; GET replies no longer lists it |
| T-M4 | `POST /v1/problems/{id}/approaches` | reject | As T-M3, for `approaches` / target `approach` |
| T-M5 | `POST /v1/questions/{id}/answers` | reject | As T-M3, for `answers` / target `answer` |
| T-M6 | `POST /v1/{approaches,answers,responses,posts}/{id}/comments` (4 cases) | reject | As T-M3, for `comments` / target `comment` |
| T-M7 | routes 2, 6, 8, 10 | approve | Row stays live, no flag, `GetCalls()==1` each |
| T-M8 | `POST /v1/approaches/{id}/progress`, `POST /v1/ideas/{id}/responses` | reject | Flag only (`progress_note` / `response`); row still present; author notified |
| T-M9 | `PATCH /v1/posts/{id}` `{"status":"open"}` on a rejected post, and on a pending_review post | — | **Red first:** today the post turns `published`. After: `pending_review`, moderation called, published only after an approve |
| T-M10 | `POST /v1/blog` `status:"published"`, then `PATCH /v1/blog/{slug}` | reject, then approve | Reject makes it `draft` and notifies; approve keeps it `published` |
| T-M11 | `POST /v1/rooms/{slug}/posts/{postID}/publish` (owner) | approve, then reject | Approve: `pending_review` first, then published. Reject: not published, and a retried approval gets 409. Updated D13 tests pass |
| T-M12 | prompt | — | httptest Groq server: system prompt carries rules 6–7; user message carries `Author's recent titles:` with the author's live titles |

## Phase 7 — W5 (amended bullet only)
Every `arg=` site in `ipfs.go` (`Pin`, `Unpin`, `PinStatus`, `ObjectStat`) uses `url.QueryEscape` (answer 11). There is a unit test on the built URL. `POST /admin/ipfs/gc` is its own endpoint, used only at P8 (answer 12).

## Phase 9 — verification (amended: targeted runs before the single full run)
Before the one full run, I run the packages the trigger and gate touch (change 7), saving each output once:
- `internal/api`: `router_deleted_account_test.go`, `router_test.go`, `router_rooms_save_post_test.go`, `router_oauth_login_code_test.go`
- `internal/api/handlers`: `auth_test.go`, `auth_cross_method_test.go`, `security_test.go`, `agents_register_test.go`, `oauth_integration_test.go`
- `internal/db`: `users_test.go`, `agents_test.go`, `oauth_login_codes_test.go`, `cutover_rollback_test.go`
- `internal/services`: `oauth_user_test.go`, `oauth_user_db_test.go`
- `cmd/cutover`

Known risk: fixtures with fixed emails. For example, `users_test.go:1048` soft-deletes `deleted1@example.com`. A later test that creates the same email now gets P0001.

A failure caused **only** by such a fixture collision is fixed in the test by making the email unique, and each such fix is listed in the report. A failure that changes an asserted behavior (for example `DUPLICATE_EMAIL` becoming `ACCOUNT_SUSPENDED` for a deleted email) is reported to the author with its assertion text before I change it.

Then the single full backend run and the single frontend run, as in r1.

## Consolidated execution order (r1 + r2)
0. Worktree plus scratch DB.
1. Red tests: W3 (create limit) and W0 (OAuth loop and orphan). Each runs once, logs saved.
2. W0 fixes, including the 302 callback redirect.
3. 000114 (ban list, index, flags CHECKs, trigger per P2), the identity gate on the 9 entry points, `/admin/bans`, the cutover version (114) and the rollback test (30).
4. W1 gates with the series counter rule.
5. W3 create limiter per route.
6. W2: the 6.1 seam; T-M9 red; D5(a)/(b)/(c); 6.2–6.3; prompt; matrix T-M1…T-M12.
7. W5 unpin, gc, escaping, crystallization skip, runbook.
8. W6 collation rehearsal and runbook.
9. Targeted runs, then the single full backend run and the single frontend run; file-size check; lint.
10. Production gates P1–P10, each only on Felipe's "yes". Unchanged from r1.

## Concerns (changed or new; the rest of r1 stands)
- **[MEDIUM]** Room publication (6.6) and the D5(a) PATCH change what room clients see after "publish": `pending_review` first, `published` seconds later. Any consumer that expected an immediate `published` (the room UI, agents using save-as-post) will see a delay. It is surfaced in the updated D13 tests.
- **[MEDIUM]** A post blocked by the series rule needs the author to delete or rename their earlier counter-titled post. The 422 names the rule; it does not point at the earlier post. I can add `existing_id` if the author wants it.
- **[LOW]** A published blog post is visible until Groq rejects it (seconds), the same residual the review accepted for replies.
- **[LOW]** Widening the `flags` CHECKs makes the `moderation_failed` flag valid, but `SetFlagCreator` stays unwired (D11, not fixed; noted only).
- Dropped from r1: the concern about general per-minute limits (moot under change 3) and the day-counter false-positive concern (moot under change 6).

## Questions for the author
1. The 4 named-baseline OAuth failures expect `302` with `error=` on **every** callback error. My `redirectWithError` helper would make them pass if also used at the other callback error sites (a one-line change per site). Should I include that, or keep the redirect scoped to `account_suspended` as change 5 literally says?
2. Should the D12 widening of `flags` CHECKs (reason: `moderation_rejected`, `moderation_failed`; target: `progress_note`) go into 000114 as planned? The alternative is `reason 'other'` with `moderation_rejected:` in `details`, and flagging a progress note's parent approach. That needs no schema change, but it doesn't match change 1's wording.
