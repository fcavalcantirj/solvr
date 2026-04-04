---
phase: 16-frontend-rooms-human-commenting
plan: 01
subsystem: api
tags: [go, rooms, sse, jwt, typescript, json-ld, seo]

# Dependency graph
requires:
  - phase: 14-backend-service-merge
    provides: "RoomSSEHandler, RoomMessagesHandler, HubManager, router_rooms.go, all room backend infrastructure"
  - phase: 13-database-foundation
    provides: "rooms, messages, agent_presence tables; RoomRepository, MessageRepository"
provides:
  - "POST /v1/rooms/{slug}/messages — JWT-auth human commenting, author_type=human"
  - "GET /v1/rooms/{slug}/stream — public browser SSE stream (no bearer token)"
  - "RoomWithStats.UniqueParticipantCount (D-05) and OwnerDisplayName (D-10) on room list"
  - "Frontend TypeScript types: APIRoom, APIRoomWithStats, APIRoomMessage, APIAgentPresenceRecord"
  - "Frontend API client: fetchRooms, fetchRoom, fetchRoomMessages, postRoomMessage"
  - "roomJsonLd() helper for DiscussionForumPosting schema with machineGeneratedContent"
affects:
  - 16-02
  - 16-03
  - 16-04

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Test hook fields (testRoomLookup, testMsgCreate) on handler structs for unit testing without DB"
    - "streamRoom() shared private method for SSE logic reuse between Stream and PublicStream"
    - "resolveSSERoomBySlug / resolveRoomBySlug indirection for testability"

key-files:
  created:
    - backend/internal/api/handlers/rooms_messages_test.go
    - backend/internal/api/handlers/rooms_sse_test.go
  modified:
    - backend/internal/models/room.go
    - backend/internal/db/rooms.go
    - backend/internal/api/handlers/rooms_messages.go
    - backend/internal/api/handlers/rooms_sse.go
    - backend/internal/api/router_rooms.go
    - frontend/lib/api-types.ts
    - frontend/lib/api.ts
    - frontend/components/seo/json-ld.tsx

key-decisions:
  - "Test hook fields (testRoomLookup, testMsgCreate) added to handler structs — enables unit testing without interface extraction or DB"
  - "streamRoom() extracted to shared private method — Stream (A2A bearer) and PublicStream (JWT-free browser) share identical SSE logic"
  - "PostHumanMessage AgentName set to 'human:{userID}' — deterministic identifier, not displayed to users"
  - "roomRepo added to RoomSSEHandler constructor (breaking change to NewRoomSSEHandler signature) — required for slug resolution in PublicStream"
  - "httprate.LimitByIP(10, time.Minute) for human comment route — stricter than agent 60/min per T-16-02"

patterns-established:
  - "Test injection pattern: nil-check function fields (testRoomLookup) on handler struct bypass real DB in unit tests"
  - "PublicStream always inside /v1/rooms group, outside auth group — ensures no auth middleware applied to browser SSE"

requirements-completed:
  - ROOMS-03
  - COMMENT-01

# Metrics
duration: 12min
completed: 2026-04-04
---

# Phase 16 Plan 01: Backend Endpoints + Frontend Types Summary

**Human commenting (JWT-auth POST) and public browser SSE stream added to rooms backend, with TypeScript types, API client, and DiscussionForumPosting JSON-LD helper for downstream frontend plans**

## Performance

- **Duration:** 12 min
- **Started:** 2026-04-04T21:47:33Z
- **Completed:** 2026-04-04T21:59:18Z
- **Tasks:** 2
- **Files modified:** 8 (+ 2 created)

## Accomplishments

- Added `PostHumanMessage` handler: JWT-auth, `author_type="human"`, text-only content (T-16-01/03), rate-limited 10 req/min per IP (T-16-02)
- Added `PublicStream` handler: public SSE stream resolved by slug (no bearer token), supports `?lastEventId=` query param, reuses `streamRoom()` shared logic
- Extended `RoomWithStats` with `UniqueParticipantCount` (D-05) and `OwnerDisplayName` (D-10); updated `List` query with subquery + LEFT JOIN users
- Defined 8 TypeScript interfaces (`APIRoom`, `APIRoomWithStats`, `APIRoomMessage`, `APIAgentPresenceRecord`, `APIRoomDetailResponse`, `APIRoomListResponse`, `APIRoomMessagesResponse`, `APIPostRoomMessageResponse`)
- Added 4 API client methods to `SolvrAPI` (`fetchRooms`, `fetchRoom`, `fetchRoomMessages`, `postRoomMessage`)
- Added `roomJsonLd()` for `DiscussionForumPosting` schema with `machineGeneratedContent` property

## Task Commits

1. **Task 1: Backend endpoints + room list stats** - `6199329` (feat)
2. **Task 2: Frontend types, API client, roomJsonLd** - `d30acb5` (feat)

## Files Created/Modified

- `backend/internal/models/room.go` — Added `UniqueParticipantCount int` and `OwnerDisplayName *string` to `RoomWithStats`
- `backend/internal/db/rooms.go` — Extended `List` query with `COUNT(DISTINCT author_id)` subquery + `LEFT JOIN users` for owner name
- `backend/internal/api/handlers/rooms_messages.go` — Added `PostHumanMessage`, `postHumanMessageRequest`, test hook fields, `resolveRoomBySlug`, `createMessage`
- `backend/internal/api/handlers/rooms_messages_test.go` — 5 tests for PostHumanMessage (success, 401, empty, too-long, 404)
- `backend/internal/api/handlers/rooms_sse.go` — Added `PublicStream`, `streamRoom()` shared method, `roomRepo` field, updated `NewRoomSSEHandler` constructor
- `backend/internal/api/handlers/rooms_sse_test.go` — 2 tests for PublicStream (404, SSE headers without auth)
- `backend/internal/api/router_rooms.go` — Wired `GET /{slug}/stream` (PublicStream) and `POST /{slug}/messages` (PostHumanMessage with rate limit), passed `roomRepo` to SSE handler
- `frontend/lib/api-types.ts` — Added 8 room/message/presence TypeScript interfaces
- `frontend/lib/api.ts` — Added 4 room API methods to SolvrAPI; added new types to import block
- `frontend/components/seo/json-ld.tsx` — Added `roomJsonLd()` returning DiscussionForumPosting schema

## Decisions Made

- Test hook fields (`testRoomLookup`, `testMsgCreate`) on handler structs — avoids interface extraction while enabling unit testing without a real DB. Consistent with the handlers package pattern.
- `streamRoom()` extracted as shared private method — `Stream` (A2A bearer route) and `PublicStream` (browser JWT-free route) are identical except for how they resolve the room.
- `NewRoomSSEHandler` constructor signature changed to accept `roomRepo` — breaking change from phase 14 but necessary for `PublicStream`. Router updated accordingly.
- `PostHumanMessage` rate limit set to 10/min (stricter than agent 60/min) per threat model T-16-02.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed unused `uuid` import from rooms_messages.go**
- **Found during:** Task 1 (PostHumanMessage implementation)
- **Issue:** `uuid` was imported but only used via a blank-identifier placeholder var
- **Fix:** Removed the import and placeholder; PostHumanMessage uses `auth.Claims.UserID` (string) directly, not uuid package
- **Files modified:** backend/internal/api/handlers/rooms_messages.go
- **Committed in:** 6199329 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - unused import)
**Impact on plan:** Trivial cleanup, no scope change.

## Issues Encountered

None — all 7 new backend tests pass on first run, TypeScript compiles with zero errors.

## Known Stubs

None — all API client functions call real endpoints. TypeScript types match the backend Go models exactly.

## Threat Flags

No new threat surface beyond what the plan's threat model already covers (T-16-01 through T-16-06 all mitigated in implementation).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `PostHumanMessage` (POST /v1/rooms/{slug}/messages) ready for Plan 02 (comment form component)
- `PublicStream` (GET /v1/rooms/{slug}/stream) ready for Plan 02 (SSE hook)
- `fetchRooms`, `fetchRoom`, `fetchRoomMessages`, `postRoomMessage` ready for Plan 03 (rooms list page) and Plan 04 (room detail page)
- `roomJsonLd()` ready for Plan 04 (room detail page SEO)
- `APIRoomWithStats.unique_participant_count` and `owner_display_name` ready for D-05 and D-10 room cards

## Self-Check: PASSED

All 11 files verified present. Both commits (6199329, d30acb5) confirmed in git log.

---
*Phase: 16-frontend-rooms-human-commenting*
*Completed: 2026-04-04*
