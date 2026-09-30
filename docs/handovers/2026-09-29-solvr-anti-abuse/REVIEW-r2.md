---
verdict: APPROVED
round: 2
reviewer_session: the author session that wrote HANDOVER.md (the production purge, the emergency ban trigger, the cutover rehearsal)
date: 2026-09-29
---

# Review r2 — APPROVED

All seven changes from REVIEW-r1 are in. I checked both of the new factual claims myself:
- **D12 holds.** On a schema-113 copy of the production data, `flags_reason_check` allows only
  `spam, offensive, duplicate, incorrect, low_quality, other`, and `flags_target_type_check` has
  no `progress_note`. The existing `posts_moderation.go:62` already writes the out-of-range
  `moderation_failed`.
- **D15 holds.** `oauth_integration_test.go:181-187` expects `302` with `error=access_denied`
  in `Location`.

## Checklist verdicts (PLAN-r1 + PLAN-r2 as merged by r2's "Consolidated execution order")

1. **Six constraints restated:** satisfied. Unchanged. Ledger idx 93 and `spec.json` hands-off are acknowledged.
2. **Facts re-verified, W0/W3 red first:** satisfied. The red tests are the first step in the consolidated order. New discrepancies D12–D16 are reported, with evidence.
3. **Every create path tested with its gate:** satisfied.
   - W1 routes 1–12 each get a gate test. The five required cases are present, with the day counter now series-based: first counter title → 201, second → 422.
   - W2 is enumerated as T-M1…T-M12, through `NewRouter` with the 6.1 seam: legacy typed creates, each contribution kind on approve and on reject, flag-only for responses and progress notes, D5(a) (T-M9, red first), D5(b) and D5(c), and the prompt.
4. **W3 DB-counted, through the real router:** satisfied.
   - The create limiter is mounted per route and counts from the authoritative tables, including deleted and rejected rows.
   - The root limiter and the general and search limits are untouched, and the "70 authenticated GETs succeed" test pins that down.
   - The idempotency ordering is handled.
5. **W4:** satisfied.
   - 114 alone carries the ban table, the seed, the author index, the `flags` CHECK widening and the trigger (option A or B at P2). Its down archives the ban rows and the flags that use the new values into `rollback_archive`, and restores the CHECKs and the function body verbatim.
   - There is one test per entry point, and the OAuth callbacks get a 302 to `/auth/callback?error=account_suspended`.
   - `--expect-version` becomes 114. The rollback test expects 30 down files and the new archive map, and asserts that the trigger survives at 84.
6. **W5:** satisfied. As r1, plus escaping at every `arg=` site with a URL test.
7. **W6:** satisfied. Unchanged from r1.
8. **Test evidence against the named baseline:** satisfied. Targeted runs come before the single full run. Fixture-only collisions get fixed and listed. **An asserted behavior change goes to the author before it is changed.** That is exactly the right rule.
9. **No unguarded production action:** satisfied. P1–P10 are unchanged.
10. **File sizes and stores:** satisfied. The `router.go` edits are in place (net 0), new code goes in new files, and no in-memory data store is added.

## Answers to the builder's questions

1. **Yes.** Use `redirectWithError` at every GitHub/Google callback error site, not only for `account_suspended`.
   - The four named-baseline failures (`TestGitHubCallback_{GitHubError,InvalidCode,MissingCode}_Integration`, `TestGitHubOAuthRedirect_Integration`) are specifications of exactly that behavior. The frontend already renders `?error=`.
   - Any of the four that flip to PASS go in the final report as **"baseline failures fixed"**, never mixed with new results.
   - If one of them fails for a reason unrelated to the redirect (for example, needing live GitHub), leave it in the baseline and say why.
2. **Yes, widen the `flags` CHECKs inside 000114,** as planned (D12). It is a schema change, so it belongs in the migration, with the archive-then-restore down you described. This also removes the latent `moderation_failed` violation.
   - `reason 'other'` plus a text marker would hide the real reason from anyone querying `flags`.
   - Wiring `SetFlagCreator` (D11) stays out of scope.
3. **Your MEDIUM concern about the series rule:** add `existing_id` (the earlier counter-titled post) to the 422 body, the same as the 409 does. It is cheap and saves the author a search.

## Closing

`HANDOVER.md` frontmatter is set to `status: approved`. **The builder is now the primary session for this work.**

Your first move is the handover's `next_action`, which is also step 1 of your consolidated order:
- in a detached worktree at `6efe181b` against a scratch database, write and run once, with logs saved:
  - **W3 red:** an authenticated agent's `effective+1`th post create through the real router is not refused today;
  - **W0 red:** a soft-deleted user's OAuth identity makes `FindOrCreateUser` loop, and the same-provider-new-email twin leaves an orphan row.

Report both red outputs before any fix.

Standing rules:
- Every production step (P1–P10) waits for Felipe's explicit "yes", each on its own.
- Commits happen only when Felipe asks, with one-line messages and no push.
- Never run a loop runner, and never touch `spec.json`.

The author validates your results (the red and green logs, the targeted runs, and the full-run diff against the named baseline) and does not write code.

## Post-approval rulings (2026-09-30, author, with Felipe's decisions)

The author checked the builder's logs directly: `backend-full.log`, `fix-reruns*.log` and `full-new.txt`.

- Tests run: 4,878 vs the baseline's 4,809. Nothing was skipped.
- Of the 14 new failures, 11 are fixed in re-runs. Four are open, and they are handled below.

1. **Agents of a tombstoned human are shut out: 401. FELIPE decided.**
   - Update `TestRoomFamily_DeletedHumansSiblingLosesClosedRoomAndToken` to assert 401. Keep the scenario, and add a one-line comment saying why: the owner is gone, so the agent cannot authenticate anywhere.
   - `DELETE /v1/me` still unclaims agents first, so self-deleted users' agents keep working. Do not change that.
2. **Every GitHub/Google callback error redirects to `/auth/callback?error=…`. FELIPE decided.**
   - Update the 12 currently passing tests that assert JSON errors so they assert the redirect, keeping each case and its error code.
   - Report the 4 named-baseline OAuth tests that flip as "baseline failures fixed".
3. **No exemption for family (private) posts from the W1 hard checks. FELIPE decided.** The heartbeat and watchdog titles, same-author repeats and the N-day series rule all apply. Groq still skips family posts, as today.
4. **Registry entry for the cutover runner. The author's defect, from commit `f803d727`.**
   - Add `"code:internal/db/knowledge_cutover.go"` to `internal/db/legacy_dependency_registry.go`, mirroring the existing `contribution_migration.go` entry: `pending(LegacyActionRetire, "the cutover runner reads legacy tables by design; remove with them")`.
   - This fixes `TestLegacyDependencyRegistry_CoversTheSourceTree` and the line-102 failure in `TestLegacyDroppedDatabase_StaticQueriesExposeOnlyRegisteredDependencies`.
   - Also check that test's line 117 ("pending code entries with no failing static statement: content_duplicates.go, create_counts.go, …"). If it is an assertion, fix the registrations it names.
5. **Fix your `TestContentGate_LegacyTypedCreates`** (`router_content_gate_test.go:103`: expected 201, got 409). Most likely the repeat gate fires between the test's own cases. Fix the test data or the gate, and say which.
6. **Then ONE final full backend run and one frontend run** on the finished branch, with the logs saved.
   - Diff the failures by name against the named baseline.
   - Log `TestSearchAnalyticsRepository_GetTrending` as "baseline test passed (time-window test)", not as fixed.
   - Report to the author before any commit.
7. **Landing: FELIPE decided.** After the author validates that final log, and only when Felipe says commit, `anti-abuse-wip` merges into `main`. No push. Production gates P1–P10 are unchanged.

The other validator's point about `cmd/cutover --expect-version` is already done: the branch default is 114 (`cmd/cutover/main.go:41`).

## Final validation (2026-09-30, author): PASSED

Checked against the builder's saved logs and the diff, not against its summary.

- **Backend.** `backend-final.log` (fresh DB at 114, one run): 4,879 run, 4,816 PASS (including subtests), 46 FAIL, 17 SKIP.
  - Only `internal/db` fails. No panic, timeout or build failure.
  - Failing names vs the named baseline: **0 new**.
  - 4 baseline failures fixed: `TestGitHubCallback_{GitHubError,InvalidCode,MissingCode}_Integration` and `TestGitHubOAuthRedirect_Integration`.
  - `TestSearchAnalyticsRepository_GetTrending` failed this run, as in the baseline. It is time-window dependent.
- **Frontend.** `frontend-final.log`: 183 files, 1,663 tests, all pass.
- **Deleted tests (17), all accounted for.**
  - 13 in `services/duplicate_test.go`, the deleted dead in-memory store (approved).
  - 4 in `moderation*_test.go`, covering only the deleted hash-store path.
  - The surviving DB-backed duplicate path keeps its tests (`AutoFlag_DuplicatePostFromCanonicalPosts`, `…ReplyOnTheSamePost`, `CheckContentDuplicate_*`, `…FinderErrorIsReturned`).
  - 49 test functions were added.
- **Rulings 1–6, verified in the diff:**
  1. The room-family test asserts 401, with a comment.
  2. The OAuth callback tests assert the redirect through `requireCallbackErrorRedirect` (302, `/auth/callback`, exact `error=` code, no code or token leak), with each case's code kept. The 4 integration tests only switched to a non-following client; their assertions are unchanged.
  3. The content gate runs outside the family-visibility block (`posts.go:524`), so there is no family exemption.
  4. The registry has `code:internal/db/knowledge_cutover.go` at `legacy_dependency_registry.go:88`.
  5. The gate tests use UUID agent names.
- **Merge check.** Base `6efe181b`. `main` changed only handover documents since then, and the branch touches none of them, so the merge will be clean.

**Ready to land:** commit the builder's 17 uncommitted files onto `anti-abuse-wip`, then merge into `main`. This waits for Felipe's "commit"; nothing is pushed. Production gates P1–P10 are unchanged and still each need Felipe's "yes".
