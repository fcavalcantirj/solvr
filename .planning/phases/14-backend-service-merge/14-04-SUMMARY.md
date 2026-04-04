---
phase: 14-backend-service-merge
plan: 04
subsystem: api
tags: [sse, real-time, streaming, heartbeat, presence, background-job, go]

# Dependency graph
requires:
  - phase: 14-01
    provides: hub package (RoomHub, HubManager, PresenceRegistry, RoomEvent, EventType)
  - phase: 14-02
    provides: MessageRepository.ListAfter, AgentPresenceRepository.DeleteExpired, RoomRepository.DeleteExpiredRooms
provides:
  - RoomSSEHandler with Last-Event-ID replay, heartbeat, connection limits
  - SSENoBuffering middleware for reverse proxy compatibility
  - PresenceReaperJob background job for TTL-based agent/room cleanup
affects: [14-05-wiring, rooms-routes, main-go-background-jobs]

# Tech tracking
tech-stack:
  added: []
  patterns: [SSE streaming with http.Flusher, atomic connection counting, context-based lifetime management, interface-driven background jobs]

key-files:
  created:
    - backend/internal/api/handlers/rooms_sse.go
    - backend/internal/api/middleware/sse_buffering.go
    - backend/internal/jobs/presence_reaper.go
    - backend/internal/jobs/presence_reaper_test.go
  modified: []

key-decisions:
  - "SSE handler uses context key for room injection -- Plan 03 bearer guard populates it"
  - "Browser SSE subscribers use _browser_ prefix to exclude from presence/discovery"
  - "PresenceReaperJob uses interface dependencies (PresenceExpirer, RoomExpirer) matching Solvr's testable job pattern"

patterns-established:
  - "SSE streaming: http.Flusher check, global atomic counter, context timeout lifetime, ticker heartbeat"
  - "Background job with interface deps: same RunOnce/RunScheduled pattern as StaleContentJob"

requirements-completed: [MERGE-05, MERGE-06, MERGE-07]

# Metrics
duration: 5min
completed: 2026-04-04
---

# Phase 14 Plan 04: SSE Handler + Presence Reaper Summary

**SSE streaming handler with 30-min lifetime, Last-Event-ID replay, 1000-connection limit, and presence reaper background job cleaning expired agents/rooms every 60s**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-04T14:33:59Z
- **Completed:** 2026-04-04T14:39:17Z
- **Tasks:** 2
- **Files created:** 4

## Accomplishments
- SSE handler with heartbeat pings (30s), max lifetime (30min), Last-Event-ID reconnection replay, and global 1000-connection limit
- SSENoBuffering middleware sets X-Accel-Buffering: no for Traefik/Easypanel reverse proxy compatibility
- PresenceReaperJob (7th background job) evicts expired agents with presence_leave events and soft-deletes expired rooms
- 7 unit tests for reaper job covering all scenarios (TDD: RED then GREEN)

## Task Commits

Each task was committed atomically:

1. **Task 1: SSE streaming handler with Last-Event-ID replay and heartbeat** - `1e66fe7` (feat)
2. **Task 2: Presence reaper background job (TDD RED)** - `fc14fe6` (test)
3. **Task 2: Presence reaper background job (TDD GREEN)** - `89d6498` (feat)

## Files Created/Modified
- `backend/internal/api/handlers/rooms_sse.go` - RoomSSEHandler with Stream method, writeSSEEvent helper, global connection limit
- `backend/internal/api/middleware/sse_buffering.go` - SSENoBuffering middleware for X-Accel-Buffering header
- `backend/internal/jobs/presence_reaper.go` - PresenceReaperJob with RunOnce/RunScheduled, interface dependencies
- `backend/internal/jobs/presence_reaper_test.go` - 7 test functions with mock implementations

## Decisions Made
- SSE handler defines its own context key (sseRoomContextKey) for room injection; Plan 03's bearer guard will populate it
- Browser SSE subscribers use `_browser_` prefix to be excluded from agent discovery and presence lists
- PresenceReaperJob uses interface types (PresenceExpirer, RoomExpirer) rather than concrete repos, matching Solvr's testable job pattern
- Reaper handles both agent presence and room expiry in a single job (D-29)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] HubManager requires non-nil slog.Logger**
- **Found during:** Task 2 (TDD GREEN)
- **Issue:** Tests created HubManager with nil logger, causing nil pointer dereference when hub Run calls logger.Debug
- **Fix:** Added testLogger() helper using slog.DiscardHandler for all test HubManager instances
- **Files modified:** backend/internal/jobs/presence_reaper_test.go
- **Verification:** All 7 tests pass
- **Committed in:** 89d6498 (Task 2 GREEN commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Minor test infrastructure fix. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all code is production-ready with real dependencies.

## Next Phase Readiness
- SSE handler ready for route mounting in Plan 05 (wiring)
- PresenceReaperJob ready for main.go background job registration in Plan 05
- SSENoBuffering middleware ready for SSE route group application
- Plan 03 (bearer guard) must provide RoomFromContext or use SSERoomToContext from this plan

## Self-Check: PASSED

All 4 created files verified on disk. All 3 commits verified in git log.

---
*Phase: 14-backend-service-merge*
*Completed: 2026-04-04*
