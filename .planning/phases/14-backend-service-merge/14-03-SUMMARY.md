---
phase: 14-backend-service-merge
plan: 03
subsystem: api
tags: [go, chi, http-handlers, bearer-auth, websocket, a2a, rooms]

# Dependency graph
requires:
  - phase: 14-backend-service-merge/01
    provides: "Hub package (HubManager, PresenceRegistry, RoomHub, RoomEvent), token package (HashToken), models (Room, Message, AgentPresenceRecord)"
  - phase: 14-backend-service-merge/02
    provides: "RoomRepository (13 methods), MessageRepository (3 methods), AgentPresenceRepository (6 methods)"
provides:
  - "RoomHandler with 6 REST endpoints (create, get, list, update, delete, rotate-token)"
  - "RoomMessagesHandler with PostMessage (broadcast + count + heartbeat) and ListMessages"
  - "RoomPresenceHandler with JoinRoom, Heartbeat, LeaveRoom, ListPresence, GetAgentCard"
  - "BearerGuard middleware resolving room from SHA-256 hashed bearer token"
affects: [14-backend-service-merge/04, 14-backend-service-merge/05]

# Tech tracking
tech-stack:
  added: []
  patterns: ["BearerGuard middleware pattern for A2A room token auth", "Room context injection via middleware", "Implicit heartbeat on message posting (D-28)"]

key-files:
  created:
    - backend/internal/api/handlers/rooms.go
    - backend/internal/api/handlers/rooms_messages.go
    - backend/internal/api/handlers/rooms_presence.go
    - backend/internal/api/middleware/bearer_guard.go
  modified: []

key-decisions:
  - "RoomPresenceHandler includes roomRepo for public route slug lookups"
  - "PresenceRegistry.UpdateLastSeen used instead of nonexistent Touch method"
  - "Room owner uses uuid.UUID comparison from auth.Claims.UserID"
  - "Default TTL = 600s (10 min) per STATE.md decision, not Quorum's 300s"

patterns-established:
  - "BearerGuard: middleware extracts room token from Authorization header or ?token= query param, hashes with SHA-256, resolves room via DB lookup"
  - "Room context: handlers retrieve room via middleware.RoomFromContext for A2A routes, or direct slug lookup for public routes"
  - "Implicit heartbeat: PostMessage calls presenceRepo.UpdateHeartbeat (D-28 belt-and-suspenders)"
  - "Hub broadcast: PostMessage creates RoomEvent and broadcasts via hubMgr.GetOrCreate"

requirements-completed: [MERGE-03, MERGE-04]

# Metrics
duration: 6min
completed: 2026-04-04
---

# Phase 14 Plan 03: HTTP Handlers Summary

**Room CRUD + A2A message/presence handlers with BearerGuard middleware -- 4 files, 921 lines, all compiling against Wave 1 repos and hub**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-04T14:32:44Z
- **Completed:** 2026-04-04T14:38:45Z
- **Tasks:** 2
- **Files created:** 4

## Accomplishments
- RoomHandler with 6 REST endpoints: CreateRoom (JWT+agent auth), GetRoom (public with agents+messages), ListRooms (public), UpdateRoom (owner/admin), DeleteRoom (owner/admin), RotateToken (owner only)
- RoomMessagesHandler with PostMessage (hub broadcast + D-30 message count increment + D-28 implicit heartbeat) and ListMessages (cursor-based pagination)
- RoomPresenceHandler with JoinRoom (DB + registry + hub subscribe, TTL=600s default), Heartbeat, LeaveRoom (DB + registry + hub unsubscribe), ListPresence, GetAgentCard
- BearerGuard middleware resolves room from SHA-256 hashed bearer token, supports ?token= query param for SSE connections
- Zero references to Quorum's JWT patterns (no jwtauth, no mw.UserIDFromContext)

## Task Commits

Each task was committed atomically:

1. **Task 1: Room CRUD handlers and bearer guard middleware** - `aa76eca` (feat)
2. **Task 2: Message posting and presence management handlers** - `5aff727` (feat)

## Files Created/Modified
- `backend/internal/api/handlers/rooms.go` - RoomHandler with 6 endpoints (CreateRoom, GetRoom, ListRooms, UpdateRoom, DeleteRoom, RotateToken), owner/admin auth checks
- `backend/internal/api/handlers/rooms_messages.go` - RoomMessagesHandler with PostMessage (broadcast + count + heartbeat) and ListMessages
- `backend/internal/api/handlers/rooms_presence.go` - RoomPresenceHandler with JoinRoom, Heartbeat, LeaveRoom, ListPresence, GetAgentCard
- `backend/internal/api/middleware/bearer_guard.go` - BearerGuard middleware for A2A room token authentication

## Decisions Made
- RoomPresenceHandler includes a roomRepo dependency (not in original plan) because ListPresence on public routes needs slug-to-room lookup, and AgentPresenceRepository does not have that method
- Used PresenceRegistry.UpdateLastSeen (actual method name) instead of Touch (plan's name for the same concept)
- Handler-local roomWriteJSON/roomWriteError helpers follow existing Solvr pattern where each handler file defines its own response helpers (e.g., leaderboard.go, oauth.go)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added roomRepo to RoomPresenceHandler**
- **Found during:** Task 2 (presence handler build)
- **Issue:** Plan specified ListPresence should support public route by slug, but AgentPresenceRepository has no GetRoomBySlug method
- **Fix:** Added roomRepo field to RoomPresenceHandler and NewRoomPresenceHandler constructor (4 params instead of 3)
- **Files modified:** backend/internal/api/handlers/rooms_presence.go
- **Verification:** `go build ./internal/api/handlers/...` exits 0
- **Committed in:** 5aff727 (Task 2 commit)

**2. [Rule 3 - Blocking] Used UpdateLastSeen instead of Touch**
- **Found during:** Task 2 (presence handler build)
- **Issue:** Plan referenced registry.Touch() which does not exist; actual method is UpdateLastSeen
- **Fix:** Changed call to h.registry.UpdateLastSeen()
- **Files modified:** backend/internal/api/handlers/rooms_presence.go
- **Verification:** `go build ./internal/api/handlers/...` exits 0
- **Committed in:** 5aff727 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both fixes necessary to compile against actual Wave 1 interfaces. No scope creep.

## Issues Encountered
None beyond the blocking deviations above.

## Known Stubs
None -- all handlers are fully wired to real repositories and hub infrastructure.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All 4 handler/middleware files compile cleanly against Wave 1 artifacts
- Ready for Plan 04 (router wiring) to mount these handlers on chi routes
- Ready for Plan 05 (SSE streaming) to add event stream endpoints

---
*Phase: 14-backend-service-merge*
*Completed: 2026-04-04*
