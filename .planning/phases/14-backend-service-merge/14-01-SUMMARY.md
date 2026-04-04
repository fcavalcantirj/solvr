---
phase: 14-backend-service-merge
plan: 01
subsystem: api
tags: [hub, sse, a2a, websocket-alternative, presence, token, sha256, models]

# Dependency graph
requires:
  - phase: 13-database-foundation
    provides: rooms/agent_presence/messages migration schemas (000073-075)
provides:
  - internal/hub package (broadcast hub, presence registry, hub manager, event types, message store)
  - internal/token package (solvr_rm_ room bearer token generation and verification)
  - Room, Message, AgentPresenceRecord model structs matching Phase 13 schemas
affects: [14-02 repositories, 14-03 handlers, 14-04 sse-streaming, 14-05 router-wiring]

# Tech tracking
tech-stack:
  added: [a2a-go v0.3.12, go-chi/httprate v0.15.0, goleak v1.3.0]
  patterns: [per-room hub goroutine with command channels, double-checked locking in HubManager, atomic SSE connection counting]

key-files:
  created:
    - backend/internal/hub/roomid.go
    - backend/internal/hub/event.go
    - backend/internal/hub/registry.go
    - backend/internal/hub/messages.go
    - backend/internal/hub/hub.go
    - backend/internal/hub/manager.go
    - backend/internal/hub/hub_test.go
    - backend/internal/token/token.go
    - backend/internal/models/room.go
    - backend/internal/models/message.go
    - backend/internal/models/agent_presence.go
  modified:
    - backend/go.mod
    - backend/go.sum

key-decisions:
  - "EventPresenceJoin/EventPresenceLeave as canonical D-06 event names (legacy EventAgentJoined/EventAgentLeft are aliases)"
  - "RoomEvent.ID field added for Last-Event-ID SSE reconnection support per D-07"
  - "solvr_rm_ token prefix distinguishes room tokens from solvr_ agent keys and solvr_sk_ user API keys"

patterns-established:
  - "Hub command channel pattern: subscribe/unsubscribe/broadcast via buffered channels processed in a single goroutine (no external locks)"
  - "Presence registry with RWMutex for many-readers/few-writers concurrency"
  - "Room model structs use json:\"-\" for sensitive fields (TokenHash, DeletedAt)"

requirements-completed: [MERGE-05]

# Metrics
duration: 8min
completed: 2026-04-04
---

# Phase 14 Plan 01: Hub + Token + Models Summary

**Ported Quorum hub package (7 files, 15 tests passing) with D-06 event types and D-07 Last-Event-ID support, SHA256 room token package with solvr_rm_ prefix, and 3 model structs matching Phase 13 migration schemas**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-04T14:17:22Z
- **Completed:** 2026-04-04T14:25:30Z
- **Tasks:** 2
- **Files modified:** 13

## Accomplishments
- Hub package ported from Quorum with Solvr import paths, all 15 tests passing (subscribe, broadcast, isolation, presence, SSE limits, goroutine leak detection)
- Four canonical SSE event types established (EventPresenceJoin, EventPresenceLeave, EventMessage, EventRoomUpdate) per D-06
- Token package generates solvr_rm_ prefixed room bearer tokens with SHA256 hashing and constant-time verification
- Three model files (Room, Message, AgentPresenceRecord) with structs matching migrations 000073-000075 exactly

## Task Commits

Each task was committed atomically:

1. **Task 1: Port hub package + token package from Quorum** - `a09cc93` (feat)
2. **Task 2: Create Room, Message, and AgentPresence model structs** - `7c161fd` (feat)

## Files Created/Modified
- `backend/internal/hub/roomid.go` - RoomID type-safe UUID wrapper
- `backend/internal/hub/event.go` - EventType constants and RoomEvent struct with ID field
- `backend/internal/hub/registry.go` - PresenceRegistry thread-safe in-memory agent store (257 lines)
- `backend/internal/hub/messages.go` - MessageStore per-room ring buffer for polling agents
- `backend/internal/hub/hub.go` - RoomHub per-room goroutine with subscribe/unsubscribe/broadcast (247 lines)
- `backend/internal/hub/manager.go` - HubManager with lazy GetOrCreate double-checked lock (85 lines)
- `backend/internal/hub/hub_test.go` - 15 tests covering all hub functionality
- `backend/internal/token/token.go` - GenerateRoomToken, HashToken, VerifyToken with solvr_rm_ prefix
- `backend/internal/models/room.go` - Room struct (15 fields), RoomWithStats, CreateRoomParams, UpdateRoomParams
- `backend/internal/models/message.go` - Message struct (11 fields), CreateMessageParams
- `backend/internal/models/agent_presence.go` - AgentPresenceRecord, UpsertAgentPresenceParams, ExpiredPresence

## Decisions Made
- Used EventPresenceJoin/EventPresenceLeave as canonical D-06 names; kept EventAgentJoined/EventAgentLeft as aliases for internal backward compat
- Added int64 ID field to RoomEvent for D-07 Last-Event-ID SSE reconnection support
- Chose solvr_rm_ prefix for room tokens (distinct from solvr_ agent keys and solvr_sk_ user API keys)
- Ran go mod tidy after adding dependencies to fix transitive checksum issues (go upgraded 1.23.0 to 1.24.4)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added goleak dependency for hub tests**
- **Found during:** Task 1 (porting hub_test.go)
- **Issue:** Hub tests import go.uber.org/goleak for goroutine leak detection, not listed in plan's go get commands
- **Fix:** Added `go get go.uber.org/goleak` alongside the other dependencies
- **Files modified:** backend/go.mod, backend/go.sum
- **Verification:** go test ./internal/hub/... passes with goleak enabled
- **Committed in:** a09cc93 (Task 1 commit)

**2. [Rule 3 - Blocking] Ran go mod tidy to fix transitive dependency checksums**
- **Found during:** Task 2 (full build verification)
- **Issue:** `go build ./...` failed with missing go.sum entry for golang.org/x/text/secure/precis (pgx transitive dep)
- **Fix:** Ran `go mod tidy` which also upgraded go version from 1.23.0 to 1.24.4
- **Files modified:** backend/go.mod, backend/go.sum
- **Verification:** `go build ./...` succeeds
- **Committed in:** 7c161fd (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both auto-fixes necessary to compile and test. No scope creep.

## Issues Encountered
None - all ported code compiled and tests passed on first attempt after fixing dependencies.

## Known Stubs
None - all files contain real implementations, no placeholder data or TODO markers.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Hub package ready for injection into room handlers (Plan 14-03)
- Token package ready for bearer guard middleware (Plan 14-03)
- Model structs ready for pgx repository methods (Plan 14-02)
- Dependencies (a2a-go, httprate) available for handler and middleware implementation

## Self-Check: PASSED

All 11 created files verified on disk. Both task commits (a09cc93, 7c161fd) verified in git log.

---
*Phase: 14-backend-service-merge*
*Completed: 2026-04-04*
