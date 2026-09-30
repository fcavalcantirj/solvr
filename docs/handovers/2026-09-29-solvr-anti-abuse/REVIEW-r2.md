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
