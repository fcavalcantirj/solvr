---
verdict: REVISE
round: 1
reviewer_session: the author session that wrote HANDOVER.md (the production purge, the emergency ban trigger, the cutover rehearsal)
date: 2026-09-29
---

# Review r1 — REVISE

A strong plan. It re-verified the handover and found real gaps the handover missed. I checked
two of them against the code myself:

- **D5(a) is real.** In `handlers/posts.go`, a PATCH that changes only `status` goes through
  `IsValidPostStatus` and nothing else (`:687-705`). Re-moderation happens only when the title or
  description changes (`:726-734`). So any author can publish their own rejected or
  pending_review post with `{"status":"open"}`, and that defeats W2.
- **D3 is real.** The legacy create routes insert into `answers`, `approaches` and `responses`
  (`db/answers.go:157`, `db/approaches.go:50`, `db/responses.go:157`). No trigger mirrors those
  rows into `replies`, and the routes are still mounted.

It needs one revision round. One checklist item is not met, and two of the builder's own
questions change the scope in ways I have to direct.

## Checklist verdicts

1. **Restate the six constraints:** satisfied. All six are correct. Constraint 3 is rightly sharpened to "not only rows in the new ban table".
2. **Re-verify facts, red tests first for W0/W3:** satisfied. There is a verification list with discrepancies D1–D11. W3 is red through the real router with a real agent key. W0 is red through the real `OAuthUserService` on a real database with a deadline and a hang guard, including the orphan-row twin.
3. **Every create path tested with the gate:** **NOT satisfied.**
   - W1 is fine. The route table covers 12 routes, "one gate test per remaining route", and all five required cases are named.
   - **W2 names no test per path.** Only a prompt test and the D5(a) red test are listed. There is nothing showing:
     - that legacy typed creates now return `pending_review` and trigger moderation;
     - that a rejected contribution is soft-deleted, flagged and notified;
     - that `responses` and `progress_notes` get a flag only.
4. **W3 DB-counted, through the real router:** satisfied. The counts come from the authoritative tables and are exercised through `NewRouter`, with no injected identity.
5. **W4:** satisfied.
   - 114 up and down, with the down archiving.
   - 9 entry points, each with a test. Handler tests for the two OAuth callbacks are acceptable, since their base URLs are hardcoded.
   - The seed covers the 7 accounts.
   - The version bump and rollback test are included, and the trigger's fate is stated.
6. **W5:** satisfied. The unpin endpoint refuses in-use CIDs, has a dry run, reports per CID, and comes with a CSV runbook. The crystallization skip is tested.
7. **W6:** satisfied. The rehearsal is faithful: build the indexes under glibc 2.41, then read them under 2.36, which is a better design than the handover's stamp-only idea. There is a duplicate pre-check, measured timings, and a gated placement in G3. No schema or code change.
8. **Test evidence against the baseline:** satisfied. Full suites run once, on a fresh database, with logs saved and failures diffed by name against the 50.
9. **No unguarded production action:** satisfied. Gates P1–P10 each need a separate "yes"; no loop run, no `spec.json` edit, no tags.
10. **File sizes and stores:** satisfied. No new file reaches 800 lines; the plan's rule for files already over the limit is accepted (next line); `duplicate.go` is deleted; no in-memory data store is added.

   On file size: I accept "touched pre-existing violators get net ≤ 0 lines" as the reading of "every file stays under 800". The 34 existing violators predate this work.

## Changes required

1. **Enumerate the W2 tests, per path, in PLAN-r2** (checklist item 3). At minimum:
   - **Routes 3–5** (`POST /v1/problems|questions|ideas`): the response carries `status: pending_review`, and moderation runs. Assert the call through `MockContentModerationService.GetCalls`. An approved result flips the post to `open`; a rejected one flips it to `rejected`.
   - **Routes 2, 6, 8 and 10** (replies, approaches, answers, comments): a Groq reject soft-deletes the row, creates the `flags` row (`reporter_type system`, `moderation_rejected`) and notifies the author. An approve leaves the row live.
   - **Routes 7 and 9** (progress notes, responses): a reject creates the flag only, and the row stays.
   - Every one of these runs through the real router, with the moderation service mocked at its interface.
2. **Bring D5(a) and D5(c) into scope as required work, and D5(b) as moderation, not a restriction.**
   - **(a) PATCH self-approve** must be closed, with a red test first. Without it, W2 is decorative.
   - **(c) Room publication** goes through `pending_review` and moderation.
   - **(b) Blog:** run Groq on a `status:"published"` create or update. A reject sets `draft` and notifies the author. Restricting blog creation to operators stays FELIPE'S CALL and is not part of this plan.
3. **W3: enforce only the DB-counted create limits. Do not switch on the general or search per-minute limits for anyone.**
   - Those limits have never fired in production. Turning them on for authenticated agents at 60/min is untested against room collaboration traffic (agents polling entries and streams), which is the product's core.
   - That is unmeasured behavior shipping with the cutover. Enabling them later, from measured request rates, is FELIPE'S CALL.
   - Mount the create limiter on the create routes only, for example with `r.With(...)` next to the idempotency middleware. Don't wrap every auth group.
   - Keep the per-request single-count test.
4. **Fold `000115` into `000114`.** That gives one migration for this release and one version bump.
   - `--expect-version` becomes **114**.
   - `cutover_rollback_test.go` expects **30** down files (114..85).
5. **OAuth callbacks: refuse with a 302 redirect** to the frontend's existing error route, `/auth/callback?error=account_suspended`, not 403 JSON.
   - The callbacks are browser navigations, so JSON would render raw.
   - The GitHub OAuth integration tests already expect 302 on callback errors (they are 4 of the named baseline's failures).
   - Every non-browser entry point keeps 403 `ACCOUNT_SUSPENDED` JSON.
6. **Day-counter rule: series-only.**
   - Block a title that matches the counter pattern only when the same author already has a live or rejected title that also matches it.
   - The handover's rule says "templated day/milestone **series**". Your own measurement says series-only catches 972 of 973 with no absolute false-positive risk.
   - The heartbeat and `[Watchdog]`/`Agent death:` rules stay absolute (Felipe's decision).
7. **Trigger option A: state its behavior on a fresh database, and cover the tests it may break.**
   - `000114.down` restoring the original function body leaves the function and trigger in place on databases where 114 created them. That is acceptable; write it into the migration header.
   - Run the account-deletion and signup test packages in your targeted runs. They include `router_deleted_account_test.go` and any test that re-registers a deleted email. The trigger and the gate now refuse a soft-deleted email with `account suspended`, where the old results were `DUPLICATE_EMAIL` or the OAuth loop.

## Answers to the builder's questions

1. **Accepted.** Soft-delete + flag + notify for rejected contributions; flag-only for `responses` and `progress_notes`. A few seconds of visibility is an accepted residual.
2. **FELIPE'S CALL.** Present both at gate P2. The author recommends **A** (versioned and extended), with change 7 above.
3. **Yes.** Apply the new-account halving to humans. That applies to the create limits only, per change 3.
4. **Series-only.** See change 6.
5. **Creates only**, for both humans and agents. General and search enforcement stays as it is today; see change 3. Turning them on later is FELIPE'S CALL, with measurements.
6. **Yes to all three**, as in change 2. Restricting blog to operators is FELIPE'S CALL and out of this plan.
7. **302 redirect** for the two callbacks, 403 JSON elsewhere. See change 5.
8. **The legacy routes stay mounted after the cutover until the ledger's adapter task** (idx 52, "route existing agent integrations into the canonical model during a defined transition") replaces them. The visibility gap belongs to the cutover, not to you.
   - I'm recording it as a new cutover blocker: before G6, either the adapters land or legacy writes are mirrored.
   - `MigrateContributions` is idempotent, so a post-deploy re-run of `cmd/cutover` converts anything written before the adapters land.
   - Your gates and moderation must still cover those routes while they are mounted.
9. **OK.** Room messages and entries are excluded from W1. Room save-as-post is included, as you planned.
10. **Fold them.** See change 4.
11. **Escape every `arg=` site** in `ipfs.go`. It is small, and it's the same bug class.
12. **Yes.** `POST /admin/ipfs/gc` as its own endpoint, called only at its own Felipe gate (P8).
13. **Out of scope.** Once W4 ships, ban rows survive a hard delete, which makes hard delete safe for bans. A guard on `DELETE /admin/users|agents` is FELIPE'S CALL.

## Next round

Write `PLAN-r2.md` with changes 1–7. Change only what they touch; everything else in r1 stands
as approved. This is round 1 of 3.
