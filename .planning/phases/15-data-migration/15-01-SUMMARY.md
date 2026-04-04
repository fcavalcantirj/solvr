---
phase: 15-data-migration
plan: 01
subsystem: database
tags: [go, postgresql, migration, cli, tdd, pgx, quorum]

# Dependency graph
requires:
  - phase: 14-backend-service-merge
    provides: rooms/messages/agents tables in Solvr DB (migrations 000073-075)
provides:
  - Standalone Go CLI at backend/cmd/migrate-quorum/ that migrates 5 Quorum rooms + messages to Solvr
  - Unit tests (13) + integration tests (6) covering all migration behaviors
  - Idempotent migration with ON CONFLICT DO NOTHING (safe to re-run)
affects:
  - phase-15-plan-02: enrichment step reads migrated rooms; migration must complete first

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "migrationDB interface abstracts dual-connection reads/writes for unit testability"
    - "txInterface subset wraps pgx Tx for mock injection in tests"
    - "Global map mutation + t.Cleanup restore for integration test email patching"

key-files:
  created:
    - backend/cmd/migrate-quorum/main.go
    - backend/cmd/migrate-quorum/migrate_test.go
    - backend/cmd/migrate-quorum/migrate_integration_test.go

key-decisions:
  - "txInterface defined in main package (not using db.Tx) so mockTx can implement it without pgx dependency in tests"
  - "Integration tests patch slugOwnerEmail global map with per-test emails, restore in t.Cleanup"
  - "Sequence numbers computed in Go (index+1) not SQL ROW_NUMBER() — messages already ordered from Quorum query"
  - "schemaQuorumMigrationDB seeds quorum data in memory rather than a real schema to avoid DDL complexity"

patterns-established:
  - "CLI tool with dual pgxpool connections: quorum (read-only) + solvr (transactional)"
  - "Dry-run skips BeginTx entirely — no write methods called at all"
  - "Single transaction wraps all 5 rooms + agents + messages — rollback on any error"

requirements-completed: [DATA-01, DATA-02, DATA-03]

# Metrics
duration: ~8min
completed: 2026-04-04
---

# Phase 15 Plan 01: Quorum-to-Solvr Migration CLI Summary

**Standalone Go CLI with dual pgx connections migrates 5 Quorum rooms + 215 messages into Solvr DB via single transaction, idempotent via ON CONFLICT DO NOTHING, with 13 unit tests (mock interface) + 6 integration tests (local Docker PostgreSQL)**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-04-04T19:56:00Z
- **Completed:** 2026-04-04T19:02:39Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Built `backend/cmd/migrate-quorum/main.go` — 588-line standalone CLI tool that reads from QUORUM_DB_URL and writes to DATABASE_URL in a single atomic transaction
- Implemented all 13 unit test cases using a `mockMigrationDB` interface — no real DB required for unit tests
- Implemented 6 integration tests against local Docker PostgreSQL seeding real test users and verifying count accuracy, owner mapping, sequence numbers, skipped room exclusion, idempotency, and slug transform

## Task Commits

Each task was committed atomically:

1. **Task 1: TDD migration script -- tests first, then implementation** - `9f009df` (feat)
2. **Task 2: Integration tests against local Docker PostgreSQL** - `d080bdd` (test)

**Plan metadata:** (docs commit — see below)

## Files Created/Modified

- `backend/cmd/migrate-quorum/main.go` — Migration CLI: migrationDB interface, migrator struct, detectContentType, targetSlug, agentID, pgMigrationDB implementation, main()
- `backend/cmd/migrate-quorum/migrate_test.go` — 13 unit tests covering all 13 behavioral specs via mockMigrationDB
- `backend/cmd/migrate-quorum/migrate_integration_test.go` — 6 integration tests (TestIntegration_ prefix) against local Docker PostgreSQL

## Decisions Made

- `txInterface` defined in the main package with only the methods needed (Exec, Commit, Rollback) — avoids importing db.Tx in tests and allows mockTx to implement it cleanly using `pgconn.CommandTag`
- Sequence numbers computed in Go as `index+1` on the slice (messages already ordered by created_at from Quorum query) — simpler than SQL ROW_NUMBER() inside the TX
- Integration tests patch the `slugOwnerEmail` global map with per-test unique emails, restoring in `t.Cleanup()` — avoids needing real prod emails in tests while exercising the real FindSolvrUserByEmail path

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed expected truncation string in TestAgentID**
- **Found during:** Task 1 (RED → GREEN phase)
- **Issue:** Test expected `agent_ThisIsAVeryLongAgentNameThatExceedsFiftyCha` (49 chars) but `id[:50]` produces 50 chars
- **Fix:** Corrected the expected string to `agent_ThisIsAVeryLongAgentNameThatExceedsFiftyChar` (50 chars)
- **Files modified:** backend/cmd/migrate-quorum/migrate_test.go
- **Committed in:** 9f009df (Task 1 commit)

**2. [Rule 1 - Bug] Fixed mockTx.Exec return type mismatch**
- **Found during:** Task 1 (GREEN phase — first compile attempt)
- **Issue:** Mock Exec returned `interface{RowsAffected() int64}` instead of `pgconn.CommandTag`
- **Fix:** Changed mockTx to return `pgconn.NewCommandTag("INSERT 0 1")` — matches txInterface signature exactly
- **Files modified:** backend/cmd/migrate-quorum/migrate_test.go
- **Committed in:** 9f009df (Task 1 commit)

**3. [Rule 1 - Bug] Fixed users INSERT in integration tests (wrong column names)**
- **Found during:** Task 2 (first integration test run)
- **Issue:** Used `provider_id` and `auth_provider` — actual column is `auth_provider_id`; also missing required `referral_code` NOT NULL column
- **Fix:** Updated INSERT to use `auth_provider_id` and added `referral_code` with unique suffix
- **Files modified:** backend/cmd/migrate-quorum/migrate_integration_test.go
- **Committed in:** d080bdd (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (3 Rule 1 bugs)
**Impact on plan:** All auto-fixes were test-correctness issues discovered during compilation and first runs. No scope creep. No plan changes needed.

## Issues Encountered

- Docker DB password is `solvr_dev` (not `solvr`) — discovered from docker-compose.yml inspection before attempting migrations
- Integration tests run against existing local DB (all migrations already applied) — no migration step needed for test setup

## Known Stubs

None — migration writes real data to real tables using real pgx connections.

## Next Phase Readiness

- Migration CLI is ready to run at cutover when QUORUM_DB_URL is available
- Phase 15 Plan 02 (metadata enrichment) can proceed — it reads migrated rooms and enriches via LLM
- Pre-requisite for cutover: apply migrations 000073-075 to production via admin query route (D-46)

## Self-Check: PASSED

- FOUND: backend/cmd/migrate-quorum/main.go
- FOUND: backend/cmd/migrate-quorum/migrate_test.go
- FOUND: backend/cmd/migrate-quorum/migrate_integration_test.go
- FOUND: .planning/phases/15-data-migration/15-01-SUMMARY.md
- FOUND commit: 9f009df (feat(15-01): implement Quorum-to-Solvr migration CLI with TDD)
- FOUND commit: d080bdd (test(15-01): add integration tests against local Docker PostgreSQL)

---
*Phase: 15-data-migration*
*Completed: 2026-04-04*
