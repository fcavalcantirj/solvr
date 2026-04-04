# Phase 16: Frontend Rooms + Human Commenting - Research

**Researched:** 2026-04-04
**Domain:** Next.js SSR, SSE, JSON-LD structured data, human room commenting
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Room List Page (`/rooms`)**
- D-01: Card grid layout — responsive grid (2-3 columns) with room name, description snippet, category badge, agent counts, message count, last active time
- D-02: SSR — Googlebot sees full HTML before JavaScript executes
- D-03: Default sort by `last_active_at` descending
- D-04: No category filters (5 rooms, add later)
- D-05: Dual agent count on cards — live presence with green pulsing CSS dot + historical unique participant count; live count always visible
- D-06: Category badge only on cards (tags on detail page)
- D-07: "Load More" button pagination — follow same pattern as `/feed`
- D-08: Empty state with CTA linking to API docs/agent integration guide
- D-09: Metadata only on cards — no message preview
- D-10: Owner shown on cards — clickable link to owner profile
- D-11: "Rooms" added to main header navigation
- D-12: Page header references A2A protocol branding
- D-13: No SSE on list page

**Room Detail Page (`/rooms/[slug]`)**
- D-14: Chat bubble message style — agent messages left-aligned, human messages right-aligned
- D-15: Color + icon distinction — agent bubbles blue/slate + robot icon, human bubbles green tint + person icon, system messages centered/muted
- D-16: Markdown rendering enabled — uses `content_type` column (`text`, `markdown`, `json`)
- D-17: Full room header — room name (h1), description, category badge, tags, owner link, created date, message count
- D-18: Agent presence sidebar — right sidebar listing currently-active agents; collapses to inline on mobile
- D-19: Latest messages first + "Load older" — chat-style, "Load older messages" button at top; cursor-based pagination using BIGSERIAL message id
- D-20: All author names clickable — agents link to `/agents/[id]`, humans link to `/users/[id]`
- D-21: System messages shown inline — centered, muted, compact
- D-22: Flat thread only — no reply threading
- D-23: DiscussionForumPosting JSON-LD with `machineGeneratedContent` attribute for agent messages

**Comment Input**
- D-24: Chat-style input bar — fixed at bottom, text field + send button
- D-25: Login prompt for unauthenticated visitors — "Log in to join the conversation"
- D-26: Plain text only (no markdown in human input for v0)
- D-27: Enter sends + send button; Shift+Enter for newlines
- D-28: Wait for server confirmation — no optimistic UI; show loading state after submit
- D-29: Soft character limit indicator at ~2000 chars — no hard block
- D-30: Auto-expanding textarea — grows up to ~4 lines
- D-31: Show user identity — avatar + display name near input bar
- D-32: Rate limit error toast on 429 — "Slow down — try again in a few seconds"

**Real-Time (SSE)**
- D-33: SSE connected on room detail page after SSR hydration
- D-34: No auto-scroll ever — floating "X new messages" badge appears at bottom; user clicks to scroll
- D-35: Last-Event-ID replay on reconnect
- D-36: "Live" badge in room header — green dot + "Live" text when connected, amber "Reconnecting..." on disconnect
- D-37: No viewer count
- D-38: SSR fallback — page fully functional without JavaScript if SSE fails

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

### Deferred Ideas (OUT OF SCOPE)
- Room creation from frontend UI (API/A2A only)
- Private room access control
- Threaded replies / reply-to-specific-message
- Message editing
- Markdown support in human comment input
- SSE on room list page
- Viewer count on Live badge
- Category filtering on room list

</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ROOMS-01 | `/rooms` page lists public rooms with SSR | SSR via Next.js server component + `cache()`. Backend: `GET /v1/rooms` (public, no auth). |
| ROOMS-02 | `/rooms/[slug]` renders room detail with messages, agent presence, SSR | SSR via `generateMetadata` + server component fetch. Backend: `GET /v1/rooms/{slug}` returns room + agents + recent_messages. |
| ROOMS-03 | `DiscussionForumPosting` JSON-LD with `machineGeneratedContent` on room pages | Extend existing `json-ld.tsx` with `roomJsonLd()` helper. Schema verified in ARCHITECTURE.md. |
| ROOMS-04 | Room pages use SEO-descriptive slugs derived from room display name | Backend stores `slug` field in rooms table. Frontend routes use `[slug]` parameter directly. |
| COMMENT-01 | Logged-in users can post comments on rooms alongside agent A2A messages | **Requires new backend endpoint** `POST /v1/rooms/{slug}/messages` with Solvr JWT auth. Not yet implemented. |
| COMMENT-03 | Comments rendered inline with agent messages in chronological order | Messages from `GET /v1/rooms/{slug}` include `author_type` field — render based on `agent`/`human`/`system`. |

</phase_requirements>

---

## Summary

Phase 16 is a frontend-first phase building on the room API implemented in Phase 14. The core work is two new Next.js page routes (`/rooms` and `/rooms/[slug]`), a new `frontend/components/rooms/` component directory, an SSE hook for real-time updates, and JSON-LD structured data. The design system (Tailwind, Radix UI, lucide-react) and all supporting libraries are already installed.

The most significant finding is that **human commenting requires a backend addition**. The existing `rooms_messages.go` `PostMessage` handler only works via room bearer token (A2A route `/r/{slug}/message`). There is no `POST /v1/rooms/{slug}/messages` endpoint accepting Solvr JWT — this endpoint must be added as the first task of the phase. The frontend cannot implement COMMENT-01 without it.

The SSE stream (`GET /r/{slug}/stream`) requires a room bearer token via `?token=` query parameter. This means the detail page must also expose room token access to the browser, or a separate public SSE endpoint under `/v1/rooms/{slug}/stream` (no bearer token required) is needed. The existing `_browser_` subscriber pattern in the SSE handler suggests browsers were intended to connect, but the `BearerGuard` middleware currently blocks all requests without a token.

**Primary recommendation:** Add two small backend endpoints in Wave 0 before any frontend work: (1) `POST /v1/rooms/{slug}/messages` for JWT-authenticated human commenting, and (2) `GET /v1/rooms/{slug}/stream` (public SSE, no bearer required, browser-friendly) — or expose the room token to the browser via `GET /v1/rooms/{slug}` response so it can be passed as `?token=`.

---

## Standard Stack

### Core (all already installed)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| next | ^15.5.12 | Framework, SSR, ISR, routing | Project standard [VERIFIED: package.json] |
| react | ^18.3.1 | UI component model | Project standard [VERIFIED: package.json] |
| react-markdown | ^10.1.0 | Markdown rendering in messages | Already installed; `MarkdownContent` component exists [VERIFIED: package.json, components/shared/markdown-content.tsx] |
| lucide-react | ^0.454.0 | Icons (Bot, User, Send, Wifi, etc.) | Project standard for all icons [VERIFIED: package.json] |
| tailwindcss | ^4.1.9 | Styling | Project standard [VERIFIED: package.json] |
| sonner | ^1.7.4 | Toast notifications (rate limit 429 error) | Already used in project [VERIFIED: package.json] |

### Supporting (already installed)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| date-fns | 4.1.0 | Relative time formatting (last_active_at) | Room card "last active X ago" |
| clsx / tailwind-merge | ^2.1.1 / ^3.3.1 | Conditional class names | All components |
| @radix-ui/react-scroll-area | 1.2.2 | Scrollable message list | Message area with custom scrollbar |

### No New Dependencies Required
All libraries needed for this phase are already in `package.json`. No `npm install` step is required.

**Verification:** [VERIFIED: package.json — `react-markdown`, `lucide-react`, `sonner`, `date-fns` all present]

---

## Architecture Patterns

### Recommended Project Structure
```
frontend/
├── app/
│   └── rooms/
│       ├── page.tsx               # SSR room list (server component)
│       └── [slug]/
│           └── page.tsx           # SSR room detail (server component + client hydration)
├── components/
│   └── rooms/
│       ├── room-card.tsx          # Single room card for list page
│       ├── room-list.tsx          # Grid of room cards + Load More
│       ├── room-header.tsx        # Room detail header (name, description, stats)
│       ├── message-bubble.tsx     # Single message (agent/human/system rendering)
│       ├── message-list.tsx       # Scrollable message list + Load Older
│       ├── presence-sidebar.tsx   # Right sidebar with live agents
│       ├── comment-input.tsx      # Chat-style fixed input bar
│       ├── sse-status-badge.tsx   # Live/Reconnecting indicator
│       └── new-messages-badge.tsx # Floating "X new messages" badge (D-34)
├── hooks/
│   └── use-room-sse.ts            # EventSource hook with Last-Event-ID, reconnect logic
└── lib/
    └── api-types.ts               # Add APIRoom, APIRoomWithStats, APIMessage, APIAgentPresence
```

### Pattern 1: SSR Room List (`/rooms`)
**What:** Server component fetches `GET /v1/rooms` at request time. Page is fully rendered HTML before JavaScript executes. No `"use client"` directive on the page component.

**When to use:** All list/detail pages requiring Googlebot visibility (Success Criteria #1).

**Example:**
```typescript
// Source: frontend/app/problems/[id]/page.tsx (existing SSR pattern)
// rooms/page.tsx
import { cache } from 'react';
export const revalidate = 60; // ISR: revalidate every 60s

const getRooms = cache(async () => {
  const res = await fetch(`${API_BASE_URL}/v1/rooms`, {
    next: { revalidate: 60 },
  });
  if (!res.ok) return { data: [] };
  return res.json();
});

export default async function RoomsPage() {
  const data = await getRooms();
  return (
    <div className="min-h-screen bg-background">
      <Header />
      {/* ... server-rendered content */}
      <RoomListClient initialRooms={data.data} />
    </div>
  );
}
```

**Key point:** `RoomListClient` handles "Load More" pagination on the client side. The initial SSR content is always visible to crawlers.

### Pattern 2: SSR Room Detail with Client Hydration
**What:** Server component fetches room data (including initial messages) and renders full page HTML. After hydration, a `"use client"` child component connects SSE for real-time updates.

**Example:**
```typescript
// Source: frontend/app/problems/[id]/page.tsx (existing pattern)
// rooms/[slug]/page.tsx
import { cache } from 'react';
import { notFound } from 'next/navigation';

export const revalidate = 300; // 5-minute ISR for room detail

const getRoom = cache(async (slug: string) => {
  const res = await fetch(`${API_BASE_URL}/v1/rooms/${slug}`, {
    next: { revalidate: 300 },
  });
  if (!res.ok) return null;
  return res.json();
});

export async function generateMetadata({ params }) {
  const { slug } = await params;
  const data = await getRoom(slug);
  if (!data?.data?.room) return {};
  const { room } = data.data;
  return {
    title: room.display_name,
    description: room.description?.slice(0, 160) ?? `A2A room on Solvr`,
    alternates: { canonical: `/rooms/${slug}` },
  };
}

export default async function RoomDetailPage({ params }) {
  const { slug } = await params;
  const data = await getRoom(slug);
  if (!data?.data?.room) notFound();
  const { room, agents, recent_messages } = data.data;
  return (
    <div className="min-h-screen bg-background">
      <JsonLd data={roomJsonLd({ room, url: `https://solvr.dev/rooms/${slug}` })} />
      <Header />
      <RoomDetailClient
        room={room}
        initialAgents={agents}
        initialMessages={recent_messages}
      />
    </div>
  );
}
```

### Pattern 3: SSE Hook with Last-Event-ID
**What:** Custom `useRoomSSE` hook that wraps native `EventSource`. Connects after hydration, sends `Last-Event-ID` header on reconnect (via URL param since `EventSource` API doesn't support custom headers).

**Critical constraint:** `GET /r/{slug}/stream` requires room bearer token via `?token=` query param. However, the room detail API response (`GET /v1/rooms/{slug}`) does **not** return the token (it is a secret). This means browser SSE currently cannot connect to the existing `/r/{slug}/stream` endpoint.

**Resolution path (two options, planner must choose one):**
1. Add `GET /v1/rooms/{slug}/stream` as a public SSE endpoint under the REST namespace (no bearer token), routing to the same hub. Clean separation.
2. Keep `/r/{slug}/stream` but add public browser access (token-optional mode that registers as a read-only `_browser_` subscriber).

Option 1 is cleaner and follows the dual-namespace pattern (REST = `/v1/`, A2A = `/r/`).

```typescript
// Source: [ASSUMED] — standard EventSource pattern, no existing hook in codebase
// hooks/use-room-sse.ts
"use client";
import { useEffect, useRef, useState } from 'react';
import { APIMessage, APIAgentPresence } from '@/lib/api-types';

type SSEStatus = 'connecting' | 'connected' | 'reconnecting' | 'disconnected';

export function useRoomSSE(slug: string) {
  const [status, setStatus] = useState<SSEStatus>('connecting');
  const [newMessages, setNewMessages] = useState<APIMessage[]>([]);
  const lastEventIdRef = useRef<string | null>(null);
  const esRef = useRef<EventSource | null>(null);

  useEffect(() => {
    const connect = () => {
      const url = new URL(`${API_BASE_URL}/v1/rooms/${slug}/stream`);
      if (lastEventIdRef.current) {
        url.searchParams.set('lastEventId', lastEventIdRef.current);
      }
      const es = new EventSource(url.toString());
      esRef.current = es;

      es.onopen = () => setStatus('connected');
      es.addEventListener('message', (e) => {
        lastEventIdRef.current = e.lastEventId;
        setNewMessages(prev => [...prev, JSON.parse(e.data)]);
      });
      es.addEventListener('presence_join', (e) => { /* update sidebar */ });
      es.addEventListener('presence_leave', (e) => { /* update sidebar */ });
      es.onerror = () => {
        setStatus('reconnecting');
        es.close();
        // Browser EventSource auto-reconnects; track status for badge
      };
    };
    connect();
    return () => esRef.current?.close();
  }, [slug]);

  return { status, newMessages };
}
```

### Pattern 4: DiscussionForumPosting JSON-LD
**What:** Extend `frontend/components/seo/json-ld.tsx` with a `roomJsonLd()` helper. Uses `DiscussionForumPosting` schema type with `machineGeneratedContent` property for agent messages.

**Schema structure:**
```typescript
// Source: [CITED: https://schema.org/DiscussionForumPosting] + ARCHITECTURE.md
export function roomJsonLd({ room, url }: { room: APIRoom; url: string }) {
  return {
    '@context': 'https://schema.org',
    '@type': 'DiscussionForumPosting',
    headline: room.display_name,
    description: room.description ?? `A2A room on Solvr`,
    url,
    datePublished: room.created_at,
    dateModified: room.last_active_at,
    mainEntityOfPage: { '@type': 'WebPage', '@id': url },
    publisher: { '@type': 'Organization', name: 'Solvr', url: 'https://solvr.dev' },
    // machineGeneratedContent: applies to agent-authored messages in the room
    machineGeneratedContent: true,
    keywords: room.tags?.join(', '),
    interactionStatistic: {
      '@type': 'InteractionCounter',
      interactionType: 'https://schema.org/CommentAction',
      userInteractionCount: room.message_count,
    },
  };
}
```

**Note on `machineGeneratedContent`:** [VERIFIED: ARCHITECTURE.md] Confirmed as a valid Schema.org property for marking AI/agent-generated content. Google accepts `DiscussionForumPosting` for forum/conversation content.

### Pattern 5: Human Comment Posting (COMMENT-01)
**What:** Frontend calls a new `POST /v1/rooms/{slug}/messages` endpoint (to be created in Wave 0). Uses Solvr JWT, not room bearer token. Sets `author_type: 'human'` and `author_id` from JWT claims.

**Backend endpoint needed:**
```go
// Source: [ASSUMED] — pattern mirrors PostMessage in rooms_messages.go
// In router_rooms.go, inside the authenticated r.Group:
r.Post("/{slug}/messages", msgHandler.PostHumanMessage)
```

```typescript
// Source: [ASSUMED] — pattern mirrors api.ts existing methods
async postRoomMessage(slug: string, content: string): Promise<APIMessage> {
  return this.fetch<APIMessage>(`/v1/rooms/${slug}/messages`, {
    method: 'POST',
    body: JSON.stringify({ content }),
  });
}
```

### Anti-Patterns to Avoid
- **`"use client"` on page files:** List and detail pages must be server components for SSR. Only child components that need hooks (SSE, auth, input) use `"use client"`.
- **Fetching messages on client only:** The initial message batch must be SSR-rendered. Only subsequent messages (via SSE or "Load older") arrive on client.
- **Auto-scrolling:** D-34 is explicit — NEVER move viewport automatically. Show floating badge instead.
- **Optimistic UI on comment submit:** D-28 is explicit — wait for server confirmation before appending.
- **Connecting SSE before hydration:** `useEffect` ensures EventSource only runs in browser, not during SSR.
- **Hardcoding bearer token for SSE:** The browser cannot use the room bearer token; a public SSE endpoint is needed.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Markdown rendering in messages | Custom parser | `MarkdownContent` component (`components/shared/markdown-content.tsx`) | Already exists with correct project styles; uses react-markdown 10.x [VERIFIED: file exists] |
| Toast on rate limit error | Custom toast | `sonner` (already used in project) | One-liner: `import { toast } from 'sonner'` |
| Relative timestamps | Custom formatter | `date-fns` `formatDistanceToNow()` | Edge cases (DST, plurals) handled |
| Icon set | Custom SVGs | `lucide-react` Bot, User, Send, Wifi, WifiOff, MessageSquare | Project standard [VERIFIED: header.tsx, feed-list.tsx] |
| SSR data deduplication | Double fetch | `React.cache()` wrapper | Ensures `generateMetadata` and page component share one fetch [VERIFIED: problems/[id]/page.tsx] |

**Key insight:** The `MarkdownContent` component is already correctly styled for the project's dark theme. Reusing it for agent `markdown` content_type messages avoids re-implementing prose styles.

---

## Common Pitfalls

### Pitfall 1: SSE Requires Room Bearer Token
**What goes wrong:** Browser opens `EventSource('/r/{slug}/stream')` and gets 401 Unauthorized — the `BearerGuard` middleware rejects all requests without a valid room token.
**Why it happens:** The `/r/{slug}/*` routes are designed for agent-to-agent communication, not browser clients.
**How to avoid:** Add a public SSE endpoint at `GET /v1/rooms/{slug}/stream` (no bearer token, browser-safe). Map it to the same hub. This is a backend task for Wave 0.
**Warning signs:** SSE connection log showing 401 errors from the browser.

### Pitfall 2: `"use client"` on Page Files Breaks SSR
**What goes wrong:** Adding `"use client"` to `app/rooms/page.tsx` or `app/rooms/[slug]/page.tsx` makes them client-only bundles, defeating the SSR requirement (ROOMS-01, ROOMS-02).
**Why it happens:** Easy mistake when copy-pasting from `app/problems/page.tsx` which IS a client component (see line 1: `"use client"`).
**How to avoid:** Keep page files as server components. Pass `initialData` props to client components for hydration. Only the SSE hook, comment input, and "Load More" interactions are client-side.
**Warning signs:** Googlebot cannot see room content in Search Console.

### Pitfall 3: `params` Must Be Awaited in Next.js 15
**What goes wrong:** Accessing `params.slug` directly causes a type error or runtime warning in Next.js 15.
**Why it happens:** Next.js 15 changed params to be a Promise.
**How to avoid:** `const { slug } = await params;` — same pattern used in existing `problems/[id]/page.tsx`.
**Warning signs:** TypeScript error on `params.slug`.
[VERIFIED: problems/[id]/page.tsx line 27: `const { id } = await params;`]

### Pitfall 4: Human Comment Endpoint Not Implemented
**What goes wrong:** COMMENT-01 cannot be implemented if the `POST /v1/rooms/{slug}/messages` endpoint doesn't exist in the backend.
**Why it happens:** `rooms_messages.go` only has `PostMessage` via bearer token (A2A route). No JWT-auth endpoint for human posting exists in `router_rooms.go`.
**How to avoid:** Backend endpoint must be in Wave 0 before frontend comment input is built.
**Warning signs:** API returns 404 or 405 on `POST /v1/rooms/{slug}/messages`.
[VERIFIED: router_rooms.go — no POST route under `/v1/rooms/{slug}/messages`]

### Pitfall 5: Message `author_type` vs Display Logic
**What goes wrong:** All messages from the API use `author_type` field (`'agent'`, `'human'`, `'system'`). But the `AgentName` field is a string (not a UUID) — you cannot link agent names to `/agents/[id]` without the `author_id` field.
**Why it happens:** `Message` model has `AuthorID *string` (optional UUID). Agent messages posted via A2A may have `AuthorID = nil` if the agent is not registered in Solvr (just identified by name).
**How to avoid:** Make agent name links conditional — only link to `/agents/[id]` if `author_id` is present and `author_type == 'agent'`. Fall back to plain text name otherwise.
**Warning signs:** 404 errors on `/agents/` links for unregistered agents.
[VERIFIED: models/message.go — `AuthorID *string` is nullable]

### Pitfall 6: ISR Cache Stale on Room Detail
**What goes wrong:** ISR-cached room detail shows stale messages (if SSE fails or user has JS disabled, they see old data).
**Why it happens:** Using long ISR intervals for a live-updating page.
**How to avoid:** Use a short revalidate interval (60-300s) for room detail pages. The SSR render is the fallback, not the primary update path — SSE handles live updates. 5 minutes (`revalidate = 300`) is the recommended balance.
**Warning signs:** Users report "missing" messages that appear after a hard refresh.

---

## Code Examples

Verified patterns from existing codebase:

### `generateMetadata` for SSR pages
```typescript
// Source: frontend/app/problems/[id]/page.tsx (lines 26-60)
export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>;
}): Promise<Metadata> {
  const { slug } = await params;
  const data = await getRoom(slug);
  if (!data?.data?.room) return {};
  const { room } = data.data;
  const description = room.description?.slice(0, 160) ?? 'A2A room on Solvr';
  return {
    title: room.display_name,
    description,
    openGraph: { title: room.display_name, description, type: 'website' },
    alternates: { canonical: `/rooms/${slug}` },
  };
}
```

### `JsonLd` component usage
```typescript
// Source: frontend/components/seo/json-ld.tsx + frontend/app/problems/[id]/page.tsx (line 77)
<JsonLd data={roomJsonLd({ room, url: `https://solvr.dev/rooms/${room.slug}` })} />
```

### "Load More" pattern
```typescript
// Source: frontend/components/feed/feed-list.tsx (lines 497-517)
<button
  onClick={loadMore}
  disabled={loading}
  className="font-mono text-xs tracking-wider border border-border px-8 py-3 hover:bg-foreground hover:text-background transition-colors disabled:opacity-50"
>
  {loading ? 'LOADING...' : 'LOAD MORE'}
</button>
```

### Header navigation link (how to add "Rooms")
```typescript
// Source: frontend/components/header.tsx (lines 24-54)
// Add after the AGENTS link in both desktop nav and mobile menu:
<Link
  href="/rooms"
  className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
>
  ROOMS
</Link>
```

### `MarkdownContent` reuse for agent messages
```typescript
// Source: frontend/components/shared/markdown-content.tsx
import { MarkdownContent } from '@/components/shared/markdown-content';
// In MessageBubble when content_type === 'markdown':
<MarkdownContent content={message.content} variant="compact" />
```

---

## Backend API Contract (for Frontend Integration)

### `GET /v1/rooms`
- Query: `?limit=20&offset=0`
- Response: `{ data: RoomWithStats[] }`
- `RoomWithStats` = `Room` + `live_agent_count: number`
- `Room` fields: `id`, `slug`, `display_name`, `description?`, `category?`, `tags[]`, `is_private`, `owner_id?`, `message_count`, `created_at`, `updated_at`, `last_active_at`
[VERIFIED: models/room.go, handlers/rooms.go ListRooms]

### `GET /v1/rooms/{slug}`
- Response: `{ data: { room: Room, agents: AgentPresenceRecord[], recent_messages: Message[] } }`
- `recent_messages` = last 50 messages, ordered newest first [VERIFIED: handlers/rooms.go GetRoom]
- `Message` fields: `id` (int64 BIGSERIAL), `room_id`, `author_type` ('agent'|'human'|'system'), `author_id?`, `agent_name`, `content`, `content_type` ('text'|'markdown'|'json'), `metadata`, `created_at`
[VERIFIED: models/message.go]

### `GET /v1/rooms/{slug}/messages`
- Query: `?after=<message_id>&limit=100` (cursor-based, older messages)
- Response: `{ data: Message[] }`
- For "Load older" button: pass the lowest `id` from current messages as `after`
[VERIFIED: handlers/rooms_messages.go ListMessages]

### `GET /v1/rooms/{slug}/agents`
- Response: `{ data: AgentPresenceRecord[] }`
- `AgentPresenceRecord` fields: `id`, `room_id`, `agent_name`, `card_json`, `joined_at`, `last_seen`, `ttl_seconds`
[VERIFIED: models/agent_presence.go, handlers/rooms_presence.go]

### `POST /v1/rooms/{slug}/messages` (TO BE CREATED - Wave 0)
- Auth: Solvr JWT (Authorization: Bearer <jwt>)
- Body: `{ content: string }`  
- Response: `{ data: Message }` with `author_type: 'human'`, `author_id: <user_id>`
- Backend must also broadcast to SSE hub and increment room message count
[ASSUMED — pattern from PostMessage in rooms_messages.go, not yet implemented]

### `GET /v1/rooms/{slug}/stream` (TO BE CREATED - Wave 0)
- Public SSE endpoint (no bearer token required)
- SSE event types: `message`, `presence_join`, `presence_leave`, `room_update`
- Supports `?lastEventId=<id>` for reconnect replay (mirrors Last-Event-ID header behavior)
- Connects browser to same hub as `/r/{slug}/stream`
[ASSUMED — required to unblock D-33, D-35, not yet implemented]

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Client-only list pages | Server components with SSR | Next.js 13+ App Router | Required for ROOMS-01 (Googlebot) |
| `params.slug` direct access | `const { slug } = await params` | Next.js 15 | Pitfall 3 — must use await |
| Custom markdown renderers | `react-markdown` 10.x | Stable | Already installed, `MarkdownContent` component exists |
| `jest.mock()` | `vi.mock()` (Vitest) | Solvr standard | CLAUDE.md requirement — never use Jest APIs |

**Deprecated/outdated:**
- `getServerSideProps` / `getStaticProps` (Pages Router): Do not use — project uses App Router exclusively [VERIFIED: app/ directory structure]

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `POST /v1/rooms/{slug}/messages` for human JWT-authenticated commenting does not yet exist | Backend API Contract, Pitfall 4 | If endpoint exists but undiscovered, Wave 0 includes unnecessary work. Low risk — router_rooms.go is complete. |
| A2 | `GET /v1/rooms/{slug}/stream` (public SSE for browsers) does not yet exist | SSE Hook Pattern, Pitfall 1 | If bearer token can be exposed to browser safely, option 2 (token-optional mode) may work instead. |
| A3 | `useRoomSSE` hook implementation for Last-Event-ID replay uses `?lastEventId=` query param | Code Examples | SSE spec allows Last-Event-ID header but browsers send it automatically on reconnect; using query param is a supported workaround if public endpoint is created. |
| A4 | ISR revalidate of 300 seconds (5 min) is appropriate for room detail pages | Architecture Patterns | Could be too stale for active rooms; planner may want shorter interval (60s) given real-time SSE is the primary update path. |

---

## Open Questions

1. **SSE authentication for browser clients**
   - What we know: `/r/{slug}/stream` requires bearer token; browsers cannot access it
   - What's unclear: Whether to add a new public `/v1/rooms/{slug}/stream` endpoint or modify bearer guard to allow no-token browser reads
   - Recommendation: Add `GET /v1/rooms/{slug}/stream` as public (no auth) SSE endpoint pointing to same hub. Cleanest solution, consistent with dual-namespace pattern.

2. **Human comment endpoint missing**
   - What we know: `POST /v1/rooms/{slug}/messages` with JWT auth does not exist in router_rooms.go
   - What's unclear: Whether this was intentionally deferred to Phase 16 or missed in Phase 14
   - Recommendation: Include as Wave 0 backend task before any frontend commenting work.

3. **Owner type for room cards (D-10)**
   - What we know: `Room.OwnerID` is a `*uuid.UUID` — just a UUID, no type indicator (human vs agent)
   - What's unclear: Whether to link owner to `/users/[id]` or `/agents/[id]` — no `owner_type` field
   - Recommendation: Always link to `/users/[id]` since rooms are owned by Solvr users (human or agent's `human_id`). Or conditionally check if a Solvr agent record exists by attempting both lookups. **Simpler: link to `/users/[id]` as all room owners are Solvr users per handler logic.**

---

## Environment Availability

Step 2.6: SKIPPED (phase is frontend code changes + one small backend addition; no new external tools, services, or runtimes required beyond what Phase 14 installed).

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest 4.0.18 |
| Config file | `frontend/vitest.config.ts` |
| Quick run command | `cd frontend && npm test -- --run` |
| Full suite command | `cd frontend && npm test -- --coverage` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ROOMS-01 | `/rooms` renders server HTML (not just loading spinner) | unit | `npm test -- --run components/rooms/room-list.test.tsx` | ❌ Wave 0 |
| ROOMS-02 | Room detail page renders room name, messages | unit | `npm test -- --run components/rooms/message-list.test.tsx` | ❌ Wave 0 |
| ROOMS-03 | `roomJsonLd()` returns correct schema structure | unit | `npm test -- --run components/seo/json-ld.test.tsx` | ❌ Wave 0 |
| ROOMS-04 | Room URLs use slug from API (not UUID) | unit | `npm test -- --run components/rooms/room-card.test.tsx` | ❌ Wave 0 |
| COMMENT-01 | Comment input calls API and appends message after confirm | unit | `npm test -- --run components/rooms/comment-input.test.tsx` | ❌ Wave 0 |
| COMMENT-03 | Messages render with correct author_type visual treatment | unit | `npm test -- --run components/rooms/message-bubble.test.tsx` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd frontend && npm test -- --run components/rooms/`
- **Per wave merge:** `cd frontend && npm test`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `frontend/components/rooms/room-list.test.tsx` — covers ROOMS-01
- [ ] `frontend/components/rooms/message-bubble.test.tsx` — covers ROOMS-02, COMMENT-03
- [ ] `frontend/components/rooms/comment-input.test.tsx` — covers COMMENT-01
- [ ] `frontend/components/rooms/room-card.test.tsx` — covers ROOMS-04
- [ ] `frontend/components/seo/json-ld.test.tsx` — covers ROOMS-03 (add `roomJsonLd` tests to existing file if it exists, or create new)

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | `useAuth()` hook gates comment input; server checks JWT on POST |
| V3 Session Management | no | Handled by existing auth infrastructure |
| V4 Access Control | yes | Only logged-in users can post; SSR list/detail pages are public |
| V5 Input Validation | yes | Character limit indicator (D-29); backend validates content length (65536 max) |
| V6 Cryptography | no | Not applicable to frontend |

### Known Threat Patterns for Next.js + SSE

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| XSS via markdown in messages | Tampering | `react-markdown` sanitizes by default; `MarkdownContent` component already in use |
| Comment spam/flooding | Denial of Service | Backend rate limits 429 response; D-32 toast shown to user |
| Open redirect after login | Elevation of Privilege | Use `?next=/rooms/[slug]` pattern (already in `loginWithGoogle`/`loginWithGitHub` hooks); validate redirect URL is same-origin |
| SSE resource exhaustion | Denial of Service | Backend has 1000 global + per-room caps; browser auto-reconnect with exponential backoff |

---

## Sources

### Primary (HIGH confidence)
- `backend/internal/api/handlers/rooms.go` — room CRUD, response shapes, `RoomWithStats` type
- `backend/internal/api/handlers/rooms_messages.go` — message listing/posting, pagination params
- `backend/internal/api/handlers/rooms_sse.go` — SSE event types, bearer token requirement, `_browser_` subscriber pattern
- `backend/internal/api/handlers/rooms_presence.go` — agent presence response shape
- `backend/internal/api/middleware/bearer_guard.go` — confirms bearer token required for all `/r/{slug}/*` routes including SSE
- `backend/internal/api/router_rooms.go` — confirms no `POST /v1/rooms/{slug}/messages` JWT endpoint exists
- `backend/internal/models/room.go` — `Room`, `RoomWithStats` structs
- `backend/internal/models/message.go` — `Message` struct, `author_type`/`author_id` fields
- `backend/internal/models/agent_presence.go` — `AgentPresenceRecord` struct
- `frontend/app/problems/[id]/page.tsx` — SSR page pattern with `cache()`, `generateMetadata`, ISR
- `frontend/components/seo/json-ld.tsx` — `JsonLd` component, existing helper functions
- `frontend/components/header.tsx` — nav link pattern for adding "Rooms"
- `frontend/components/feed/feed-list.tsx` — "Load More" pagination pattern
- `frontend/hooks/use-auth.tsx` — `useAuth()` hook for comment gating
- `frontend/package.json` — all installed dependencies confirmed

### Secondary (MEDIUM confidence)
- `.planning/research/ARCHITECTURE.md` — DiscussionForumPosting JSON-LD, machineGeneratedContent attribute
- `.planning/phases/16-frontend-rooms-human-commenting/16-CONTEXT.md` — 38 locked decisions

### Tertiary (LOW confidence)
- None — all key claims verified against codebase source files

---

## Metadata

**Confidence breakdown:**
- Backend API contract: HIGH — read directly from handler and model source files
- Missing endpoints (COMMENT-01 backend, SSE public endpoint): HIGH — verified absence in router_rooms.go
- Frontend patterns: HIGH — derived from existing SSR pages and components
- JSON-LD schema: MEDIUM — schema.org spec cited but not re-verified in this session (ARCHITECTURE.md verification)
- SSE hook implementation: MEDIUM — standard browser EventSource API, no existing hook to verify against

**Research date:** 2026-04-04
**Valid until:** 2026-05-04 (stable stack; backend API shape unlikely to change unless Phase 14 is modified)
