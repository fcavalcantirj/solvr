---
phase: 13-database-foundation
plan: 01
subsystem: backend/database
tags: [migration, postgresql, rooms, agent_presence, messages, a2a, quorum-merge]
dependency_graph:
  requires: []
  provides:
    - rooms table (UUID PK, 15 columns, slug UNIQUE, tags CHECK, 4 indexes)
    - agent_presence table (UUID PK, 7 columns, UNIQUE(room_id, agent_name), 2 indexes)
    - messages table (BIGSERIAL PK, 11 columns, author_type CHECK, content CHECK, 2 indexes)
  affects:
    - Phase 14 (Backend Service Merge) — depends on these tables existing
    - Phase 15 (Data Migration) — migrates Quorum data into these tables
    - Phase 16 (Frontend Rooms) — reads from rooms and messages tables
tech_stack:
  added: []
  patterns:
    - golang-migrate format (.up.sql / .down.sql, pure SQL, no annotations)
    - UUID primary keys for entity tables, BIGSERIAL for ordered sequences
    - Partial indexes with WHERE clauses for soft-deleted tables (migration 000030 pattern)
    - Integration tests using os.Getenv("DATABASE_URL") with skip-on-missing pattern
    - Pre-cleanup + t.Cleanup teardown pattern for idempotent DB integration tests
key_files:
  created:
    - backend/migrations/000073_create_rooms.up.sql
    - backend/migrations/000073_create_rooms.down.sql
    - backend/migrations/000074_create_agent_presence.up.sql
    - backend/migrations/000074_create_agent_presence.down.sql
    - backend/migrations/000075_create_messages.up.sql
    - backend/migrations/000075_create_messages.down.sql
    - backend/internal/db/migrations_rooms_test.go
  modified:
    - backend/internal/db/migrations_test.go
decisions:
  - "Unified messages table (no separate room_comments) — satisfies COMMENT-02 via author_type/author_id columns"
  - "agent_presence TTL default 900s (15min) — more forgiving than Quorum's 300s"
  - "Pre-cleanup pattern in integration tests — idempotent across multiple test runs"
  - "No IF NOT EXISTS guards in migrations — fail loudly on conflicts per CONTEXT.md guidance"
metrics:
  duration_minutes: 5
  completed_date: "2026-04-04"
  tasks_completed: 2
  tasks_total: 2
  files_created: 8
  files_modified: 1
requirements_satisfied:
  - MERGE-01
  - COMMENT-02
---

# Phase 13 Plan 01: Database Foundation Summary

**One-liner:** Three golang-migrate migrations (000073-000075) adding rooms/agent_presence/messages tables with constraints, indexes, and integration tests verifying schema correctness.

## What Was Built

### Migration 000073: rooms table
- 15 columns: id (UUID PK), slug (UNIQUE + regex CHECK), display_name, description, category, tags (TEXT[] + array_length CHECK <=10), is_private, owner_id (FK to users ON DELETE SET NULL), token_hash, message_count, created_at, updated_at, last_active_at, expires_at, deleted_at
- 4 indexes: idx_rooms_owner_id, idx_rooms_expires_at (partial WHERE expires_at IS NOT NULL), idx_rooms_active (partial WHERE deleted_at IS NULL), idx_rooms_deleted (partial WHERE deleted_at IS NULL)

### Migration 000074: agent_presence table
- 7 columns: id (UUID PK), room_id (FK to rooms ON DELETE CASCADE), agent_name, card_json (JSONB + length CHECK <=16384), joined_at, last_seen, ttl_seconds (DEFAULT 900)
- UNIQUE(room_id, agent_name) constraint
- 2 indexes: idx_agent_presence_room_id, idx_agent_presence_last_seen

### Migration 000075: messages table
- 11 columns: id (BIGSERIAL PK), room_id (FK to rooms ON DELETE CASCADE), author_type (CHECK IN ('human', 'agent', 'system')), author_id, agent_name, content (TEXT + length CHECK <=65536), content_type (CHECK IN ('text', 'markdown', 'json')), metadata (JSONB DEFAULT '{}'), sequence_num, created_at, deleted_at
- 2 indexes: idx_messages_room_created (composite), idx_messages_room_active (partial WHERE deleted_at IS NULL)

### Integration Tests
- New file `migrations_rooms_test.go` (436 lines) with 5 test functions
- Updated `migrations_test.go` AllTablesExist to include 3 new tables (20 total, 836 lines)
- All tests pass green, idempotent across multiple runs via pre-cleanup pattern

## Decisions Made

1. **Unified messages table** — No separate `room_comments` table. The `author_type` (human/agent/system) and `author_id` columns on `messages` satisfy COMMENT-02 without schema complexity.

2. **agent_presence TTL = 900s** — Overrode Quorum's 300s for more forgiving presence tracking in Solvr's context.

3. **No IF NOT EXISTS guards** — Migrations fail loudly on conflicts; this is intentional per CONTEXT.md decision. Keeps migrations semantically clear.

4. **Pre-cleanup in tests** — Added `roomsTestCleanup()` helper called both at test start AND registered with `t.Cleanup`. This makes tests safe to run repeatedly without manual DB cleanup between runs.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed non-idempotent integration tests**
- **Found during:** Task 2 GREEN phase, second test run
- **Issue:** First test run created rooms in DB; second run failed with duplicate key error because `t.Cleanup` from first run had already cleaned up (cleanup ran at the right time), but the issue was that cleanup only registered for END-of-test, leaving prior run data if the test binary crashed or was killed. More critically, test slugs like `presence-test-room` and `msg-test-room` were not covered by the cleanup pattern `DELETE FROM rooms WHERE slug LIKE 'test-%'`.
- **Fix:** Added `roomsTestCleanup()` helper called at BOTH test start (pre-cleanup) AND via `t.Cleanup`. Updated cleanup coverage to include all test slugs used in each test function.
- **Files modified:** backend/internal/db/migrations_rooms_test.go
- **Commit:** 7ae267e

## Verification Results

All success criteria passed:

1. Migration files exist and are syntactically correct
2. No goose annotations in any of the 6 migration files
3. Migrations applied cleanly (000073, 000074, 000075 all applied successfully)
4. All 6 integration tests pass green (TestMigrations_RoomsTable, TestMigrations_RoomsConstraints, TestMigrations_AgentPresenceTable, TestMigrations_MessagesTable, TestMigrations_MessagesConstraints, TestMigrations_AllTablesExist)
5. File size compliance: migrations_test.go = 836 lines, migrations_rooms_test.go = 436 lines (both < 900)
6. Migrations are reversible (down 3 + up applied cleanly)

## Known Stubs

None — migrations are pure SQL schema definitions, no stub data or placeholder values.

## Self-Check: PASSED

- backend/migrations/000073_create_rooms.up.sql: EXISTS
- backend/migrations/000073_create_rooms.down.sql: EXISTS
- backend/migrations/000074_create_agent_presence.up.sql: EXISTS
- backend/migrations/000074_create_agent_presence.down.sql: EXISTS
- backend/migrations/000075_create_messages.up.sql: EXISTS
- backend/migrations/000075_create_messages.down.sql: EXISTS
- backend/internal/db/migrations_rooms_test.go: EXISTS
- backend/internal/db/migrations_test.go (updated): EXISTS
- Commit 9a5eece (migrations): FOUND
- Commit 7ae267e (tests): FOUND
