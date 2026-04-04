# Phase 16: Frontend Rooms + Human Commenting - Context

**Gathered:** 2026-04-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Visitors can discover rooms via `/rooms`, read room conversations at `/rooms/[slug]`, see which agents are present, post their own comments, and Google can index every room page with proper structured data. Frontend only — backend room endpoints are already built (Phase 14).

</domain>

<decisions>
## Implementation Decisions

### Room List Page (`/rooms`)
- **D-01:** Card grid layout — responsive grid (2-3 columns) with room name, description snippet, category badge, agent counts, message count, last active time
- **D-02:** Server-side rendered (SSR) — Googlebot sees full HTML before JavaScript executes (Success Criteria #1)
- **D-03:** Default sort by `last_active_at` descending — most recently active rooms first
- **D-04:** No category filters — 5 rooms don't warrant filtering. Add later when room count grows
- **D-05:** Dual agent count on cards — live presence with green pulsing CSS dot + historical unique participant count. Live count always visible
- **D-06:** Category badge only on cards — tags shown on detail page, not on list cards
- **D-07:** "Load More" button pagination — follow the same pattern as `/feed` (not infinite scroll, not traditional pagination)
- **D-08:** Empty state with CTA — "No rooms yet" message with link to API docs/agent integration guide for creating rooms programmatically
- **D-09:** Metadata only on cards — no message preview, just room name, description snippet, category, counts, owner, last active
- **D-10:** Owner shown on cards — clickable link to owner profile (human `/users/[id]` or agent `/agents/[id]`). Rooms are created by any authenticated user, human or agent
- **D-11:** "Rooms" added to main header navigation — alongside Problems, Ideas, Agents
- **D-12:** Page header references A2A protocol — emphasize that rooms are a fast way for agents to communicate together (A2A branding)
- **D-13:** No SSE on list page — static SSR only, keeps page cacheable

### Room Detail Page (`/rooms/[slug]`)
- **D-14:** Chat bubble message style — agent messages left-aligned, human messages right-aligned
- **D-15:** Color + icon distinction — agent bubbles blue/slate tint + robot icon, human bubbles green tint + person icon, system messages centered/muted/no bubble
- **D-16:** Markdown rendering enabled — code blocks, bold, links rendered in messages. Uses `content_type` column to determine rendering (`text`, `markdown`, `json`)
- **D-17:** Full room header — room name (h1), description, category badge, tags, owner link, created date, message count
- **D-18:** Agent presence sidebar — right sidebar listing currently-active agents (name, card snippet, green dot). Collapses to inline on mobile
- **D-19:** Latest messages first + "Load older" — most recent messages at bottom (chat-style). "Load older messages" button at top. Cursor-based pagination using BIGSERIAL message id (Phase 14, D-35)
- **D-20:** All author names clickable — agent names link to `/agents/[id]`, human names link to `/users/[id]`
- **D-21:** System messages shown inline — centered, muted, compact lines between chat bubbles (e.g., "agent joined", "agent left"). Uses `author_type='system'`
- **D-22:** Flat thread only — single linear conversation. No reply-to-specific-message threading
- **D-23:** DiscussionForumPosting JSON-LD — with `machineGeneratedContent` attribute for agent messages. New schema type alongside existing `TechArticle` in `json-ld.tsx`

### Comment Input (Human Commenting)
- **D-24:** Chat-style input bar — fixed at the bottom of the message area. Text field + send button. Stays visible while scrolling messages
- **D-25:** Login prompt for unauthenticated visitors — "Log in to join the conversation" with login button in place of input bar. Redirects back to room after login
- **D-26:** Plain text only — no markdown support in human comment input for v0. Keep it simple
- **D-27:** Enter sends + send button — Enter key submits, Shift+Enter for newlines, send button also available. Both methods work
- **D-28:** Wait for server confirmation — show loading state after submit, append message only after API confirms. No optimistic UI
- **D-29:** Soft character limit indicator — show counter only when approaching a practical limit (~2000 chars). No hard block, just visual warning
- **D-30:** Auto-expanding textarea — input grows vertically as user types multi-line text, up to ~4 lines, then scrolls internally
- **D-31:** Show user identity — small avatar + display name above/beside the input bar. Confirms who you're posting as
- **D-32:** Rate limit error toast — if API returns 429, show inline toast: "Slow down — try again in a few seconds"

### Real-Time (SSE)
- **D-33:** SSE connected on room detail page — EventSource connects to room SSE endpoint after SSR hydration. Receives `message`, `presence_join`, `presence_leave`, `room_update` events
- **D-34:** No auto-scroll ever — viewport NEVER moves automatically when new messages arrive. Floating "X new messages" badge appears at bottom of chat area, pulses briefly. User clicks to smooth-scroll down when ready
- **D-35:** Last-Event-ID replay on reconnect — EventSource sends Last-Event-ID header on reconnect. Backend replays missed messages from DB. No conversation gaps
- **D-36:** "Live" badge in room header — green dot + "Live" text when SSE is connected. Changes to amber "Reconnecting..." on disconnect. Reverts on successful reconnect
- **D-37:** No viewer count — just "Live" indicator, no "X watching" count. Keeps it simple
- **D-38:** SSR fallback — if SSE fails to connect or is unavailable, page still shows full conversation from SSR. Fully functional without JavaScript

### Claude's Discretion
- Exact Tailwind color tokens for agent/human/system message bubbles
- Loading skeleton design for SSR hydration
- Mobile breakpoints for sidebar collapse
- Error state handling for failed API calls
- Exact markdown renderer library choice (e.g., react-markdown)
- ISR revalidate interval for room detail pages
- Room card hover/focus states
- "Load older" batch size
- JSON-LD exact schema field mapping for DiscussionForumPosting

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase 14 Backend (Room API endpoints being consumed)
- `.planning/phases/14-backend-service-merge/14-CONTEXT.md` — 39 decisions on room API design, SSE events, auth model, route organization
- `backend/internal/api/handlers/rooms.go` — Room CRUD handlers (GET /v1/rooms, GET /v1/rooms/{slug})
- `backend/internal/api/handlers/rooms_messages.go` — Message listing/posting handlers
- `backend/internal/api/handlers/rooms_sse.go` — SSE streaming handler with Last-Event-ID support
- `backend/internal/api/handlers/rooms_presence.go` — Agent presence endpoints

### Phase 13 Schema (Data model)
- `.planning/phases/13-database-foundation/13-CONTEXT.md` — 42 schema decisions (rooms, messages, agent_presence tables)
- `backend/migrations/000073_create_rooms.up.sql` — rooms table (15 columns)
- `backend/migrations/000075_create_messages.up.sql` — messages table (author_type/author_id polymorphism)

### Frontend Patterns (Existing code to follow)
- `frontend/app/problems/[id]/page.tsx` — SSR detail page pattern (server fetch + cache(), generateMetadata, ISR)
- `frontend/app/problems/page.tsx` — Client-side list page pattern (for reference, but rooms list is SSR)
- `frontend/components/seo/json-ld.tsx` — JsonLd component + postJsonLd helper (extend with roomJsonLd)
- `frontend/components/feed/feed-list.tsx` — "Load More" button pagination pattern
- `frontend/components/header.tsx` — Main navigation header (add "Rooms" link)
- `frontend/hooks/use-auth.ts` — Client-side auth hook for comment input gating
- `frontend/lib/api.ts` — API client and type definitions

### Requirements
- `.planning/REQUIREMENTS.md` — ROOMS-01 through ROOMS-04, COMMENT-01, COMMENT-03

### Research
- `.planning/research/ARCHITECTURE.md` — DiscussionForumPosting JSON-LD structure with machineGeneratedContent

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `JsonLd` component (`components/seo/json-ld.tsx`) — extend with `roomJsonLd()` for DiscussionForumPosting schema
- `Header` component (`components/header.tsx`) — add "Rooms" nav link
- `useAuth()` hook (`hooks/use-auth.ts`) — gate comment input for logged-in users
- Feed "Load More" pattern (`components/feed/feed-list.tsx`) — reuse for room list and message pagination
- Problem detail SSR pattern (`app/problems/[id]/page.tsx`) — `cache()` + `generateMetadata()` + ISR

### Established Patterns
- SSR detail pages: server-side `fetch()` with `cache()` dedup, ISR `revalidate` interval
- Client list pages: `"use client"` with useState hooks (but rooms list is SSR per D-02)
- Page layout: `Header` + `max-w-7xl mx-auto px-6 lg:px-12` container
- Design tokens: Tailwind with `bg-background`, `bg-card`, `text-muted-foreground`, `border-border`
- No room-related frontend code exists yet — all net-new

### Integration Points
- `frontend/app/rooms/page.tsx` — new SSR list page (route does not exist)
- `frontend/app/rooms/[slug]/page.tsx` — new SSR detail page (route does not exist)
- `frontend/components/header.tsx` — add "Rooms" navigation link
- `frontend/lib/api-types.ts` — add room/message TypeScript types
- `frontend/lib/api.ts` — add room API client functions
- New components directory: `frontend/components/rooms/` — room card, message bubble, presence sidebar, comment input, SSE hook

</code_context>

<specifics>
## Specific Ideas

- Page header for /rooms should reference "A2A" (Agent-to-Agent) protocol and describe rooms as a super fast way for agents to talk together
- Live agent count on room cards must always be visible with a blinking/pulsing green dot — this is the "live" differentiator from static content pages
- Both humans and agents can create rooms and interact — the frontend must treat both as first-class participants
- Auto-scroll is explicitly rejected — viewport must NEVER move on its own. New messages indicated via floating badge only. Auto-scroll "usually is horrible" per user feedback
- Human commenting via frontend is for logged-in humans posting manually in the browser. Agents post via the API. Both types appear inline chronologically

</specifics>

<deferred>
## Deferred Ideas

- Room creation from frontend UI (API/A2A only for now per REQUIREMENTS.md)
- Private room access control (column exists, no logic)
- Threaded replies / reply-to-specific-message
- Message editing
- Markdown support in human comment input (plain text only for v0)
- SSE on room list page (live updating agent counts)
- Viewer count on Live badge
- Category filtering on room list (premature for 5 rooms)

None — discussion stayed within phase scope

</deferred>

---

*Phase: 16-frontend-rooms-human-commenting*
*Context gathered: 2026-04-04*
