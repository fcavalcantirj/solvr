---
phase: 16-frontend-rooms-human-commenting
plan: 03
subsystem: ui
tags: [nextjs, react, vitest, typescript, json-ld, seo, ssr, rooms, tailwind]

# Dependency graph
requires:
  - phase: 16-frontend-rooms-human-commenting
    provides: "APIRoom, APIRoomMessage, APIAgentPresenceRecord types; fetchRoom, fetchRoomMessages API client; roomJsonLd helper"
provides:
  - "/rooms/[slug] SSR page with generateMetadata, JSON-LD, ISR (revalidate=300)"
  - "MessageBubble component: agent (blue, Bot icon, left), human (green, User icon, right), system (centered, dashed)"
  - "RoomHeader component: room name h1, description, category/tag badges, owner link"
  - "PresenceSidebar component: desktop sticky sidebar + mobile horizontal strip (layout prop)"
  - "MessageList component: ScrollArea wrapper, cursor-based Load older pagination"
  - "RoomDetailClient: use client hydration shell wiring MessageList + PresenceSidebar"
  - "24 Vitest tests for MessageBubble (11) and roomJsonLd (6) + existing blogPostJsonLd (7)"
affects:
  - 16-04

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "layout prop pattern on PresenceSidebar: parent controls mobile/desktop placement via CSS wrapper divs"
    - "Messages reversed server-side: API returns newest-first, page.tsx reverses to oldest-first for chat display"
    - "RoomDetailClient as thin state shell: initialMessages/initialAgents from SSR, setMessages/setAgents ready for SSE wiring in Plan 04"
    - "node_modules symlink from main repo into worktree for shared Vitest infrastructure"

key-files:
  created:
    - frontend/app/rooms/[slug]/page.tsx
    - frontend/components/rooms/room-detail-client.tsx
    - frontend/components/rooms/room-header.tsx
    - frontend/components/rooms/message-bubble.tsx
    - frontend/components/rooms/message-bubble.test.tsx
    - frontend/components/rooms/message-list.tsx
    - frontend/components/rooms/presence-sidebar.tsx
  modified:
    - frontend/components/seo/json-ld.test.tsx

key-decisions:
  - "layout prop on PresenceSidebar controls rendering mode — parent (RoomDetailClient) chooses which mode appears at which breakpoint via CSS, avoiding double-render"
  - "RoomDetailClient exposes setMessages via onMessagesLoaded callback on MessageList — Plan 04 SSE hook will update message state without restructuring the component tree"
  - "node_modules symlinked from /frontend to worktree/frontend — worktrees share git objects but not node_modules, symlink resolves Vitest runner dependency"

patterns-established:
  - "SSR room page: cache(getRoom) deduplicates fetch between generateMetadata and page component"
  - "Author link routing: agent author_id -> /agents/{id}, human author_id -> /users/{id}"
  - "MessageBubble: null-safe author_id check before rendering link vs plain text"

requirements-completed:
  - ROOMS-02
  - ROOMS-03
  - ROOMS-04
  - COMMENT-03

# Metrics
duration: 18min
completed: 2026-04-04
---

# Phase 16 Plan 03: Room Detail Page Summary

**SSR /rooms/[slug] page with DiscussionForumPosting JSON-LD, MessageBubble (agent/human/system), RoomHeader, cursor-paginated MessageList, and PresenceSidebar with mobile/desktop layout prop**

## Performance

- **Duration:** 18 min
- **Started:** 2026-04-04T22:04:00Z
- **Completed:** 2026-04-04T22:22:00Z
- **Tasks:** 3
- **Files modified:** 8 (7 created, 1 modified)

## Accomplishments

- Created `/rooms/[slug]/page.tsx` as SSR server component with `generateMetadata` (title, description, OG), `notFound()` for invalid slugs, 5-min ISR, and DiscussionForumPosting JSON-LD (ROOMS-03)
- Created `MessageBubble` with three rendering modes: agent (left-aligned, blue tint, Bot icon, markdown support), human (right-aligned, green tint, User icon), system (centered, dashed border)
- Created `PresenceSidebar` with `layout` prop — parent uses CSS wrapper divs to show mobile strip or desktop sticky sidebar without double-rendering
- Created `MessageList` with ScrollArea, cursor-based "Load older messages" button, `bottomRef` scroll anchor for Plan 04 SSE
- Created `RoomDetailClient` as thin client hydration shell: receives SSR state as props, exposes `setMessages` as `onMessagesLoaded` callback for Plan 04 SSE wiring
- 24 Vitest tests: 11 for MessageBubble, 6 for roomJsonLd, 7 existing blogPostJsonLd — all pass; zero TypeScript errors

## Task Commits

1. **Task 1: MessageBubble, RoomHeader, roomJsonLd tests** - `403d5c6` (feat)
2. **Task 2: PresenceSidebar, MessageList** - `081d8e5` (feat)
3. **Task 3: SSR page, RoomDetailClient** - `b593a11` (feat)

## Files Created/Modified

- `frontend/app/rooms/[slug]/page.tsx` — SSR room detail page with generateMetadata, JSON-LD, notFound, ISR
- `frontend/components/rooms/room-detail-client.tsx` — Client hydration shell for messages/agents state
- `frontend/components/rooms/room-header.tsx` — Room name h1, description, category badge, tag badges, owner link
- `frontend/components/rooms/message-bubble.tsx` — Three-mode message renderer (agent/human/system)
- `frontend/components/rooms/message-bubble.test.tsx` — 11 Vitest tests for all rendering modes
- `frontend/components/rooms/message-list.tsx` — ScrollArea wrapper + Load older cursor pagination
- `frontend/components/rooms/presence-sidebar.tsx` — Desktop sticky + mobile horizontal strip (layout prop)
- `frontend/components/seo/json-ld.test.tsx` — Added roomJsonLd describe block (6 tests)

## Decisions Made

- `layout` prop on `PresenceSidebar` controls which rendering mode is used. The parent `RoomDetailClient` wraps each usage in CSS breakpoint divs (`lg:hidden` and `hidden lg:block`) to show/hide each mode — cleaner than internal breakpoint detection.
- `RoomDetailClient` exposes `setMessages` as `onMessagesLoaded` callback to `MessageList`. This allows Plan 04's SSE hook to push new messages into the same state without restructuring the component.
- Symlinked `node_modules` from the main `frontend/` into the worktree's `frontend/` since git worktrees share file history but not installed packages. This is required for Vitest to run in the worktree.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Symlinked node_modules from main repo into worktree**
- **Found during:** Task 1 (RED phase Vitest run)
- **Issue:** Worktree's `frontend/` had no `node_modules/`, Vitest could not resolve `vitest/config` module
- **Fix:** `ln -sf /home/clawdbot/development/solvr/frontend/node_modules node_modules` in worktree frontend directory
- **Files modified:** None (symlink only, gitignored)
- **Verification:** `npx vitest run` executed successfully after symlink
- **Committed in:** Not committed (runtime symlink, not tracked in git)

---

**Total deviations:** 1 auto-fixed (Rule 3 - blocking)
**Impact on plan:** Trivial worktree setup issue. No scope change.

## Issues Encountered

- `json-ld.test.tsx` already existed (blogPostJsonLd tests) despite Glob not finding it — the Glob tool had a path issue. Resolved by using `ls` to confirm existence, then reading and appending the roomJsonLd describe block. Existing tests unaffected.

## Known Stubs

None — all components wire to real API data from SSR props or API client calls. `MessageList` loads older messages via `api.fetchRoomMessages()`. No hardcoded empty values.

## Threat Flags

No new threat surface beyond what the plan's threat model covers (T-16-10 through T-16-13 all addressed).

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- `/rooms/[slug]` SSR page ready for Plan 04 (SSE live updates, comment input)
- `RoomDetailClient` state management (`messages`, `agents`) ready for Plan 04 SSE hook wiring via `onMessagesLoaded` callback
- `MessageList.bottomRef` scroll anchor ready for Plan 04 new-message badge scroll behavior
- All 24 Vitest tests green, TypeScript clean

## Self-Check: PASSED

All 7 created files and 1 modified file verified present. Three commits (403d5c6, 081d8e5, b593a11) confirmed in git log.

---
*Phase: 16-frontend-rooms-human-commenting*
*Completed: 2026-04-04*
