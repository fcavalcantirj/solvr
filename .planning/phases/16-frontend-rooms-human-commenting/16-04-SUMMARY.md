---
phase: 16-frontend-rooms-human-commenting
plan: 04
subsystem: ui
tags: [nextjs, react, vitest, tdd, sse, typescript, tailwind, rooms, real-time]

# Dependency graph
requires:
  - phase: 16-frontend-rooms-human-commenting
    plan: "01"
    provides: "POST /v1/rooms/{slug}/messages, GET /v1/rooms/{slug}/stream (public SSE), postRoomMessage API client, APIRoomMessage types"
  - phase: 16-frontend-rooms-human-commenting
    plan: "03"
    provides: "RoomDetailClient shell, MessageList, PresenceSidebar, room-detail-client.tsx ready for SSE wiring"
provides:
  - "useRoomSse hook: EventSource with Last-Event-ID replay, SseStatus tracking, clearNewMessages callback"
  - "CommentInput: auth gate (Join the conversation vs chat bar), Enter-to-send, 429 toast, char counter"
  - "SseStatusBadge: green LIVE dot (connected), amber RECONNECTING... dot"
  - "NewMessagesBadge: floating badge without auto-scroll (D-34) — user-initiated scroll only"
  - "RoomDetailClient: fully wired with SSE, comment input, presence updates, deduplication"
  - "12 Vitest tests for CommentInput (auth gate, button states, character counter)"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "TDD RED/GREEN: test file written and confirmed failing before implementing component"
    - "D-34 no-auto-scroll: SSE messages increment unreadCount, float badge is only scroll trigger"
    - "D-28 server-confirmation: CommentInput appends message only after postRoomMessage resolves"
    - "EventSource manual reconnect: onerror closes and reschedules connect() with 3s delay (T-16-17)"
    - "Last-Event-ID replay: initial connect sends lastEventId query param; browser sends header on reconnects"

key-files:
  created:
    - frontend/hooks/use-room-sse.ts
    - frontend/components/rooms/comment-input.tsx
    - frontend/components/rooms/comment-input.test.tsx
    - frontend/components/rooms/sse-status-badge.tsx
    - frontend/components/rooms/new-messages-badge.tsx
  modified:
    - frontend/components/rooms/room-detail-client.tsx

key-decisions:
  - "Manual EventSource reconnect (close + setTimeout 3s) instead of browser auto-reconnect — gives controlled delay per T-16-17"
  - "lastEventId passed as query param on initial SSE connect — EventSource only sends Last-Event-ID header on auto-reconnects, not on first connect"
  - "CommentInput renders null button/textarea for unauthenticated — no disabled state, full login prompt per D-25"
  - "Deduplication by message ID in both SSE append path and handleMessageSent — prevents doubles from SSE replay + confirmed send"
  - "unreadCount incremented only for truly unique messages (existingIds Set check before setUnreadCount)"

patterns-established:
  - "SSE hook returns clearNewMessages callback — parent component controls when to dismiss the badge"
  - "presenceJoins/presenceLeaves arrays batched in hook, parent useEffects process them"

requirements-completed:
  - COMMENT-01
  - COMMENT-03

# Metrics
duration: 4min
completed: 2026-04-04
---

# Phase 16 Plan 04: SSE + Comment Input Wiring Summary

**Live room experience via EventSource SSE hook with Last-Event-ID replay, auth-gated CommentInput (Enter-to-send, 429 toast, char counter), floating new-messages badge without auto-scroll, and 12 passing Vitest tests**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-04T22:12:25Z
- **Completed:** 2026-04-04T22:16:30Z
- **Tasks:** 2
- **Files modified:** 6 (5 created, 1 modified)

## Accomplishments

- Created `useRoomSse` hook: connects to public SSE stream, tracks `SseStatus` (connecting/connected/reconnecting/disconnected), accumulates `newMessages` and presence events, supports Last-Event-ID replay via `?lastEventId=` query param on initial connect, manual 3s reconnect delay (T-16-17)
- Created `SseStatusBadge`: green animated dot + "LIVE" when connected, amber dot + "RECONNECTING..." when reconnecting, hidden while connecting or disconnected
- Created `NewMessagesBadge`: floating button showing "N new messages", zero auto-scroll (D-34), `onClick` triggers parent scroll handler, `animate-bounce` runs 3 times
- Created `CommentInput`: unauthenticated state shows "Join the conversation" prompt with LOG IN link; authenticated state shows expanding textarea, Enter-to-send (Shift+Enter = newline), 429 rate-limit toast, character counter visible at 1800+ chars with red at 2000+, server-confirmation only (no optimistic UI per D-28)
- Updated `RoomDetailClient`: wired `useRoomSse`, `CommentInput`, `SseStatusBadge`, `NewMessagesBadge`; deduplication by message ID; presence sidebar updates from SSE events; `unreadCount` increments only for unique new messages
- 12 Vitest TDD tests for `CommentInput`: all pass (auth gate, button states, whitespace-disable, character counter visibility and color)

## Task Commits

1. **Task 1: SSE hook, status badge, new-messages badge** - `502d079` (feat)
2. **Task 2 RED: CommentInput Vitest tests** - `57f9ed5` (test)
3. **Task 2 GREEN: CommentInput implementation + RoomDetailClient wiring** - `c1add72` (feat)

## Files Created/Modified

- `frontend/hooks/use-room-sse.ts` — SSE hook with EventSource, SseStatus, Last-Event-ID replay, clearNewMessages
- `frontend/components/rooms/sse-status-badge.tsx` — LIVE/RECONNECTING status indicator with colored animated dots
- `frontend/components/rooms/new-messages-badge.tsx` — Floating "N new messages" badge with onClick (no auto-scroll)
- `frontend/components/rooms/comment-input.tsx` — Auth-gated chat input bar with full D-24 through D-32 spec
- `frontend/components/rooms/comment-input.test.tsx` — 12 Vitest tests for all CommentInput states
- `frontend/components/rooms/room-detail-client.tsx` — Full real-time wiring: SSE, comments, presence, badge

## Decisions Made

- Manual EventSource reconnect via `onerror` + `setTimeout(3000)` instead of relying on browser auto-reconnect — browser behavior varies across implementations, controlled delay is safer per T-16-17
- `lastEventId` sent as query param on initial connect — the `EventSource` API only sends `Last-Event-ID` header automatically on *reconnects*, not on the first connection after SSR; query param covers the initial case
- `CommentInput` renders a fully replaced login prompt (not a disabled input) for unauthenticated users — matches D-25 design intent, cleaner UX than grayed-out textarea
- `unreadCount` incremented inside the `setMessages` updater (after dedup check) — ensures count matches actual unique messages appended, not raw SSE event count

## Deviations from Plan

None — plan executed exactly as written. All threat model mitigations applied as specified (T-16-14 through T-16-18).

## Issues Encountered

None — TypeScript compiled clean on first attempt, all 12 Vitest tests passed on first run after implementing the GREEN phase component.

## Known Stubs

None — `CommentInput` calls `api.postRoomMessage()` (real API client), `useRoomSse` connects to real `/v1/rooms/{slug}/stream` endpoint. No hardcoded empty values.

## Threat Flags

No new threat surface beyond what the plan's threat model covers (T-16-14 through T-16-18 all addressed):
- T-16-14 (content tampering): content sent as plain text, backend validates length
- T-16-15 (DoS spam): `submitting` state prevents double-submit; 429 shows toast
- T-16-16 (SSE tampering): JSON.parse in try/catch; dedup by message ID
- T-16-17 (SSE reconnect loop): 3s delay, EventSource closed on unmount
- T-16-18 (login redirect): static `/login` href, no custom redirect param

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Phase 16 is now complete: all 4 plans shipped (backend endpoints, rooms list, room detail, SSE + comments)
- `/rooms/[slug]` is a fully functional real-time room: SSR reading + live SSE updates + human commenting
- All COMMENT-01 and COMMENT-03 requirements delivered

## Self-Check: PASSED

All 6 files verified present. Commits 502d079, 57f9ed5, c1add72 confirmed in git log. 12 Vitest tests pass. TypeScript compiles with zero errors.

---
*Phase: 16-frontend-rooms-human-commenting*
*Completed: 2026-04-04*
