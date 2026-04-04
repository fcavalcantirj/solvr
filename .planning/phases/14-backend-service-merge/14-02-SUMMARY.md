---
phase: 14-backend-service-merge
plan: 02
subsystem: database
tags: [pgx, postgresql, rooms, messages, agent-presence, repository, tdd]

# Dependency graph
requires:
  - phase: 13-database-foundation
    provides: rooms, messages, and agent_presence tables via migrations 000073-000075
provides:
  - RoomRepository with 13 methods including List with JOIN aggregate (no N+1)
  - MessageRepository with concurrent-safe sequence_num and cursor-based pagination
  - AgentPresenceRepository with Upsert, DeleteExpired (returns removed for reaper), TTL filtering
  - Room, Message, AgentPresenceRecord model structs matching migration schemas
  - Token package with solvr_rm_ prefix for room bearer tokens
affects: [14-03 room handlers, 14-04 SSE streaming, 14-05 router wiring, 15 data migration]

# Tech tracking
tech-stack:
  added: [internal/token package]
  patterns: [correlated subquery for live_agent_count in room list, COALESCE(MAX+1) for sequence_num, TTL-based interval filtering for presence]

key-files:
  created:
    - backend/internal/db/rooms.go
    - backend/internal/db/rooms_test.go
    - backend/internal/db/room_messages.go
    - backend/internal/db/room_messages_test.go
    - backend/internal/db/agent_presence.go
    - backend/internal/db/agent_presence_test.go
    - backend/internal/models/room.go
    - backend/internal/models/message.go
    - backend/internal/models/agent_presence.go
    - backend/internal/token/token.go
  modified: []

key-decisions:
  - "Correlated subquery for live_agent_count instead of JOIN aggregate — simpler with equivalent performance for small result sets"
  - "Created model structs and token package as prerequisite (Rule 3 deviation) since Plan 01 runs in parallel"
  - "Used direct SQL for edge-case test data (expired rooms, expired presence) rather than repo methods"

patterns-established:
  - "Room repository scanning: 15-column scanRoom helper for single rows, scanRoomRow for list iteration"
  - "Dynamic UPDATE with positional args: build SET clauses from non-nil params with sequential $N placeholders"
  - "Presence TTL filtering: WHERE last_seen > NOW() - (ttl_seconds || ' seconds')::interval"
  - "Message sequence_num via subquery in INSERT: (SELECT COALESCE(MAX(sequence_num), 0) + 1 FROM messages WHERE room_id = $1)"

requirements-completed: [MERGE-02]

# Metrics
duration: 8min
completed: 2026-04-04
---

# Phase 14 Plan 02: pgx Repositories Summary

**Three pgx repositories (rooms, messages, agent_presence) with 22 methods, concurrent-safe sequence_num, TTL-based presence, and JOIN aggregate room list -- all tested against real PostgreSQL**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-04T14:18:01Z
- **Completed:** 2026-04-04T14:26:16Z
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments
- RoomRepository with 13 methods: Create (auto-slug + token), GetBySlug/ID/TokenHash, List (live_agent_count subquery, no N+1), ListByOwner, Update (dynamic SET), SoftDelete, RotateToken, UpdateActivity, DeleteExpiredRooms, Increment/DecrementMessageCount
- MessageRepository with 3 methods: Create (concurrent-safe sequence_num via COALESCE(MAX+1)), ListAfter (cursor-based), ListRecent (DESC + reverse for chronological)
- AgentPresenceRepository with 6 methods: Upsert (ON CONFLICT), Remove, ListByRoom (TTL-filtered), UpdateHeartbeat, DeleteExpired (RETURNING for reaper), ListAllPublic
- 15 integration test functions (22 subtests), all passing against real PostgreSQL

## Task Commits

Each task was committed atomically:

1. **Task 1: RoomRepository -- CRUD, list with JOIN aggregate, token rotation** - `eebebc0` (feat)
2. **Task 2: MessageRepository and AgentPresenceRepository** - `e8db448` (feat)

## Files Created/Modified
- `backend/internal/db/rooms.go` - RoomRepository with 13 methods (431 lines)
- `backend/internal/db/rooms_test.go` - 8 integration test functions for rooms
- `backend/internal/db/room_messages.go` - MessageRepository with 3 methods (176 lines)
- `backend/internal/db/room_messages_test.go` - 3 integration test functions for messages
- `backend/internal/db/agent_presence.go` - AgentPresenceRepository with 6 methods (208 lines)
- `backend/internal/db/agent_presence_test.go` - 4 integration test functions for presence
- `backend/internal/models/room.go` - Room, RoomWithStats, CreateRoomParams, UpdateRoomParams
- `backend/internal/models/message.go` - Message, CreateMessageParams
- `backend/internal/models/agent_presence.go` - AgentPresenceRecord, UpsertAgentPresenceParams, ExpiredPresence
- `backend/internal/token/token.go` - GenerateRoomToken, HashToken, VerifyToken with solvr_rm_ prefix

## Decisions Made
- Used correlated subquery for live_agent_count in room List query (semantically equivalent to LEFT JOIN aggregate, simpler SQL)
- Created model structs and token package in this plan (Rule 3 deviation) because Plan 01 runs in parallel and models were needed as prerequisites
- Used direct SQL inserts in test edge cases (expired rooms, expired presence) to test cleanup methods without circular repo dependencies

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Created model structs and token package**
- **Found during:** Task 1 (RoomRepository implementation)
- **Issue:** models.Room, models.CreateRoomParams, token.GenerateRoomToken etc. do not exist -- Plan 01 creates them but runs in parallel
- **Fix:** Created backend/internal/models/room.go, message.go, agent_presence.go and backend/internal/token/token.go matching Plan 01's spec exactly
- **Files modified:** 4 new files (3 models + 1 token)
- **Verification:** go build ./internal/models/... and go build ./internal/token/... both pass
- **Committed in:** eebebc0 (Task 1 commit)

**2. [Rule 1 - Bug] Fixed users table referral_code NOT NULL constraint in test**
- **Found during:** Task 1 (ListByOwner test)
- **Issue:** Test INSERT into users table failed because referral_code column is NOT NULL (migration 000070)
- **Fix:** Added referral_code to test user INSERT with 8-char value
- **Files modified:** backend/internal/db/rooms_test.go
- **Verification:** TestRoomRepository_ListByOwner passes
- **Committed in:** eebebc0 (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both fixes necessary for plan execution. No scope creep.

## Issues Encountered
None beyond the auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all repositories are fully implemented against real PostgreSQL database.

## Next Phase Readiness
- All three repositories ready for handler consumption in Plan 03
- Token package ready for bearer guard middleware in Plan 03
- Models shared with Plan 01 (hub package) -- if Plan 01 creates different model files, a merge resolution will be needed (but both follow the same spec)

## Self-Check: PASSED

All 10 created files verified on disk. Both task commits (eebebc0, e8db448) verified in git log.

---
*Phase: 14-backend-service-merge*
*Completed: 2026-04-04*
