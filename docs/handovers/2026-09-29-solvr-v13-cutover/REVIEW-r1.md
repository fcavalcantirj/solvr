---
verdict: APPROVED
round: 1
reviewer_session: the author session that wrote HANDOVER.md (ralph scaffold → 69/96, four harness fixes, three production migrations)
date: 2026-09-29
---

# Review r1 — APPROVED

The plan satisfies all eight acceptance items. More than that: it caught six real defects in my
handover, three of which I re-verified against the live systems just now and confirm. A plan that
corrects its handover is doing exactly what the builder role is for.

## Acceptance checklist — verdict per item

1. **Restate the five blocking constraints** — SATISFIED. All five restated in the builder's own
   words with independent evidence (`rooms.go` lines 73/75/149/161/174 for constraint 1;
   `router.go:692` + `router_homepage.go:49` for constraint 3). Constraint 5 was extended with a
   lesson I had not written down — the loop's `git add .` swept HANDOVER.md into `619b44b8`.
2. **Re-verify against live systems, report discrepancies** — SATISFIED, and then some. Twelve
   discrepancies, each with evidence. Ledger, both local DB versions, prod `replies` absence and
   prod counts all re-checked and matching.
3. **Backup before any cutover step, restored into a scratch DB** — SATISFIED. Step A1 restores
   into `solvr_cutover_rehearsal`, never `solvr` or `solvr_test`, with a `TEMPLATE` pristine copy
   for reruns. Correctly refuses `scripts/restore-local-db.sh` because it hard-codes
   `DB_NAME="solvr"`, drops it, and `kill -9`s whatever holds port 8080 — I had pointed at that
   script in the handover without noticing that trap. Good catch.
4. **Rehearse before proposing any production write** — SATISFIED. A3–A8 cover force-84 → up →
   full cutover sequence → second idempotency pass → old-vs-new diff → new-code smoke on real data
   → rollback → old-code smoke on the rolled-back DB. Nine explicit pass criteria. The rollback
   rehearsal measuring *what is lost* (room tokens, post-cutover replies) is the part I would have
   under-specified.
5. **Hand-work vs ralph-work per task** — SATISFIED. All 27 open tasks in one table with reasons.
6. **Engine per block, opus or better for 93** — SATISFIED. Task 93 stays in-session on Opus 5.5
   and is never handed to the loop; kimi, codex and opencode excluded with reasons.
7. **Known-baseline vs new failures, by name** — SATISFIED, and it corrected my baseline (D9).
   Three named sets with a standing rule for classifying later failures from the saved log.
8. **No production write without an approval step** — SATISFIED. Gates G1–G7, each an explicit
   stop, with the correct note that approving one gate never approves the next.

## Discrepancies I re-verified myself (all confirmed)

- **D5 — 207 unpushed commits.** `git rev-list --count origin/main..main` → **207**; origin/main is
  `c03734ae` (2026-07-06). Shipping requires pushing these first. This was a genuine hole in my
  handover and it is the most dangerous omission, because G1's secret scan now sits in front of it.
- **D7 — no production entry point.** `backend/cmd/` holds api, backfill-embeddings,
  migrate-quorum, moderate-existing, test-groq. Nothing in `cmd/` or `internal/api` calls
  `MigrateContributions` or `RemapLegacyRelations`. The cutover cannot be run at all today.
  I had written "run `MigrateContributions`" as if a runner existed.
- **D3 — production has no `schema_migrations`.** `to_regclass('public.schema_migrations')` →
  **ABSENT**; `messages.client_entry_id` → 0 columns. So "prod is at 84" is inferred from markers,
  as the plan says, and `force 84` is itself the first production write. My `[REAL]` label on the
  schema version was wrong; it should have been `[UNVERIFIED]` with the marker evidence.

Accepted without re-verification: **D1** (HEAD moved to `619b44b8`; my "loop is STOPPED" was true
when I ran `pgrep` and stale by the time it was read — the failure mode is real and my label
should have carried a timestamp), **D4** (`/health` version is a stale constant — correct, the
404 on `/v1/overview` is the better evidence), **D6** (file names for the replies-based counts:
`stats_canonical.go`/`knowledge_totals.go`, not `stats.go` — substance of constraint 3 unchanged),
**D9**, **D11**, **D2**, **D8**, **D10**, **D12**.

## Answers to the builder's questions

1. **What does "done" mean?** Your proposal is accepted as the working definition — *launch-stage
   done* = every build and verification task passes and the cutover ships, with dated gates left
   open for 68's +90-day table drop, 52 step 5, 85's week-8 review, and the outcome tasks 87/89.
   **Whether to re-scope the ledger instead IS FELIPE'S CALL** — put it to him with your D11
   evidence; do not decide it yourself.
2. **Room tokens at 000098.** I do not know whether cutting off `solvr_rm_` was deliberate — it was
   built by the loop across idx 69's slices and I audited the code, not the product intent. **IS
   FELIPE'S CALL.** Bring it with the A2 number (how many of the 56 rooms were active in the last
   7/30 days and how many distinct agents hold tokens). Note for him: only one room has been active
   recently, so the blast radius is probably small — but measure before saying so.
3. **Where do rehearsal results go?** Both. Append **one short dated entry** to `progress.txt`
   (append-only, `>>`, only while the loop is stopped) so the next iteration knows 93 was
   rehearsed, and keep the full numbers in the task 93 runbook plus chat. Do not paste the full
   report into the journal — it is already 840 KB and every iteration reads it.
4. **Who builds `cmd/cutover`?** You do, by hand, in this session. Agreed for your stated reason:
   it is production-critical and gated. The loop has never been given a task whose failure mode is
   data loss, and this is not the place to start.
5. **Felipe's pending decisions.** I do not have his answers to any of them — task 76's
   disposition, the claim-referral mount, staged vs all-at-once cleanup, required If-Match, the
   24h search tests. All still open. Batch them after the rehearsal as you propose, with the
   rehearsal numbers attached so he decides from evidence.

## Concerns I endorse, with a note

- **[HIGH] 000098 cutting off live agents** — this is the one I would have missed, and it directly
  contradicts task 93 step 5 ("token validity through the stated transition"). Treat the A2
  measurement as a gate input, not a footnote.
- **[HIGH] 96/96 unreachable by build alone** — correct, and it means the ledger meter will stall
  near the end by design. Say so to Felipe early; he has been reading the meter as the progress
  signal for six days.
- **[MEDIUM] moderator comments surfacing as replies** — 2,678 comments by `solvr-moderator` on
  1,836 posts becoming top-level replies would be visible on every one of those pages. A6 checking
  the listing is the right gate. If they do surface, that is a product decision, not a bug fix.

One correction to your plan, not blocking: under Phase C you list G4 as "Run `migrate force 84` +
`up` against production". Per D3 that `force` **creates** `schema_migrations` on production, so it
is a production write and belongs behind its own approval — fold it into G4's stop rather than
treating it as setup.

## Status

`HANDOVER.md` frontmatter is updated to `status: approved`. **You are now the primary session.**

Your `next_action`: Phase A0/A1 — create the worktree at `619b44b8` and restore
`db-backups/solvr_prod_2026-09-29_15-49-50.dump` (sha256 `e20dacdc…0bcf5`) into
`solvr_cutover_rehearsal`. Nothing touches production until the Phase A report exists and Felipe
approves each gate.
