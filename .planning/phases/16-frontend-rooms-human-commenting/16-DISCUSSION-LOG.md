# Phase 16: Frontend Rooms + Human Commenting - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-04
**Phase:** 16-Frontend Rooms + Human Commenting
**Areas discussed:** Room list layout, Message presentation, Comment input UX, Real-time vs static

---

## Room List Layout

| Option | Description | Selected |
|--------|-------------|----------|
| Card grid | Responsive grid with room metadata, consistent with problems/ideas patterns | ✓ |
| Compact list | Single-column rows like GitHub issues | |
| You decide | Claude picks | |

**User's choice:** Card grid
**Notes:** Consistent with existing card-heavy design patterns in Solvr

| Option | Description | Selected |
|--------|-------------|----------|
| SSR | Server-rendered for SEO, Googlebot sees HTML | ✓ |
| Client-side | Client-rendered like problems list | |
| Hybrid | SSR + client-side hydration for interactivity | |

**User's choice:** SSR
**Notes:** Required by success criteria #1

| Option | Description | Selected |
|--------|-------------|----------|
| Last active | Most recently active rooms first | ✓ |
| Newest created | Most recently created rooms | |
| Message count | Most active by total messages | |

**User's choice:** Last active

| Option | Description | Selected |
|--------|-------------|----------|
| Simple message | "No rooms yet" text | |
| CTA to docs | "No rooms yet" + link to API docs | ✓ |
| You decide | Claude picks | |

**User's choice:** CTA to docs

| Option | Description | Selected |
|--------|-------------|----------|
| No filters | Too few rooms for filtering | ✓ |
| Category filter | Filter by room category | |

**User's choice:** No filters

| Option | Description | Selected |
|--------|-------------|----------|
| Historical count | Total unique agent participants | |
| Live presence | Currently online agents | |
| Both | Live pulsing dot + historical count | ✓ |

**User's choice:** Both — "always shows live, blinking, and shows historically how many agents conversed on that room"

| Option | Description | Selected |
|--------|-------------|----------|
| Category only | Badge on cards, tags on detail | ✓ |
| Category + tags | Badge plus tag chips on cards | |

**User's choice:** Category only

| Option | Description | Selected |
|--------|-------------|----------|
| Green dot pulse | CSS pulse animation, Discord/Slack style | ✓ |
| Animated icon | Robot icon pulses/glows | |
| You decide | Claude picks | |

**User's choice:** Green dot pulse

| Option | Description | Selected |
|--------|-------------|----------|
| Show all | Render all rooms | |
| Follow /feed pattern | Load More button pagination | ✓ |

**User's choice:** Follow /feed Load More button pattern

| Option | Description | Selected |
|--------|-------------|----------|
| Add to header nav | "Rooms" in main navigation | ✓ |
| No header link yet | Direct URL only | |

**User's choice:** Add to header nav

**Page header:** User wants A2A reference — "collective" and "a super fast way for agents to talk together"

| Option | Description | Selected |
|--------|-------------|----------|
| Metadata only | No message preview on cards | ✓ |
| Latest message preview | Show recent message snippet | |

**User's choice:** Metadata only

| Option | Description | Selected |
|--------|-------------|----------|
| No owner shown | Focus on room topic | |
| Show owner | Clickable profile link | ✓ |

**User's choice:** Show owner — "rooms are created by any logged user, human or agent. show owner, clickable to profile"

---

## Message Presentation

| Option | Description | Selected |
|--------|-------------|----------|
| Chat bubbles | Left/right aligned by author type | ✓ |
| Linear feed | Single-column, same alignment | |
| You decide | Claude picks | |

**User's choice:** Chat bubbles

| Option | Description | Selected |
|--------|-------------|----------|
| Color + icon | Agent blue/slate, human green, system muted | ✓ |
| Badge only | Same color, type badge by name | |

**User's choice:** Color + icon

| Option | Description | Selected |
|--------|-------------|----------|
| Render markdown | Code blocks, bold, links | ✓ |
| Plain text only | Raw text display | |

**User's choice:** Render markdown

| Option | Description | Selected |
|--------|-------------|----------|
| Sidebar | Right sidebar with active agents | ✓ |
| Inline badges | Top-of-page presence badges | |

**User's choice:** Sidebar, collapses on mobile

| Option | Description | Selected |
|--------|-------------|----------|
| Full header | Name, description, category, tags, owner, date, count | ✓ |
| Compact header | Name + category inline, minimal | |

**User's choice:** Full header

| Option | Description | Selected |
|--------|-------------|----------|
| Latest first + Load older | Most recent at bottom, load older at top | ✓ |
| Oldest first + Load more | Start from beginning | |
| All at once | Load all messages SSR | |

**User's choice:** Latest first + Load older

**Author linking:** "agent or human. both can interact. ONLY logged users can interact, agents or human. think about the human posting manually also, on front (not api)"

| Option | Description | Selected |
|--------|-------------|----------|
| Scroll to bottom | Auto-scroll on page load | ✓ |
| Start at top | Show header first | |

**User's choice:** Scroll to bottom on initial page load

| Option | Description | Selected |
|--------|-------------|----------|
| Show inline | Centered, muted system messages | ✓ |
| Hide system messages | Only agent/human messages | |

**User's choice:** Show inline

---

## Comment Input UX

| Option | Description | Selected |
|--------|-------------|----------|
| Chat input bar | Fixed at bottom, like Slack/Discord | ✓ |
| Form below messages | Scrollable form, like GitHub comments | |

**User's choice:** Chat input bar

| Option | Description | Selected |
|--------|-------------|----------|
| Login prompt | "Log in to join" with button | ✓ |
| Hidden entirely | No input area for logged-out | |
| Disabled input | Grayed out input with CTA | |

**User's choice:** Login prompt

| Option | Description | Selected |
|--------|-------------|----------|
| Plain text only | Simple text input | ✓ |
| Markdown support | Formatting toolbar/preview | |

**User's choice:** Plain text only

**Thread model:** Flat single thread — no reply-to-message. User confirmed: "right now, cannot reply to a message, it's a single thread"

| Option | Description | Selected |
|--------|-------------|----------|
| Enter sends | Enter submits, Shift+Enter newline | |
| Button only | Enter adds newline, must click send | |
| Both | Enter + button, Shift+Enter newline | ✓ |

**User's choice:** Both — Enter sends + button available, Shift+Enter for newlines

| Option | Description | Selected |
|--------|-------------|----------|
| Wait for server | Loading state, append after confirmation | ✓ |
| Optimistic append | Instant local append | |

**User's choice:** Wait for server

| Option | Description | Selected |
|--------|-------------|----------|
| Soft limit indicator | Counter at ~2000 chars, no hard block | ✓ |
| No limit shown | Rely on DB 65KB constraint | |

**User's choice:** Soft limit indicator

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-expand | Grows to ~4 lines, then scrolls | ✓ |
| Fixed height | Single-line, always same height | |

**User's choice:** Auto-expand

| Option | Description | Selected |
|--------|-------------|----------|
| Show identity | Avatar + name beside input | ✓ |
| No identity shown | Just the input bar | |

**User's choice:** Show identity

| Option | Description | Selected |
|--------|-------------|----------|
| Show error on rate limit | Inline toast on 429 | ✓ |
| You decide | Claude handles | |

**User's choice:** Show error on rate limit

---

## Real-Time vs Static

| Option | Description | Selected |
|--------|-------------|----------|
| SSE in this phase | Wire up EventSource, live messages + presence | ✓ |
| Static SSR only | Ship SSR, add SSE later | |
| Polling fallback | Poll API every 30-60s | |

**User's choice:** SSE in this phase

| Option | Description | Selected |
|--------|-------------|----------|
| No SSE on list page | Static SSR, cacheable | ✓ |
| SSE on list too | Live updating counts | |

**User's choice:** No SSE on list page

| Option | Description | Selected |
|--------|-------------|----------|
| Floating badge | "X new messages" badge, no scroll | ✓ |
| Smart scroll | Auto-scroll if at bottom | |
| Always auto-scroll | Always jump to bottom | |
| Header flash | Flash room header on new messages | |

**User's choice:** Floating badge — **explicit rejection of auto-scroll**: "auto scroll - usually is horrible. this Smart scroll can be buggy. just present the last messages in a totally different way, like blinking something, but don't scroll"

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, with replay | Last-Event-ID for gap-free reconnect | ✓ |
| Simple reconnect | Reconnect without replay | |

**User's choice:** Yes, with replay

| Option | Description | Selected |
|--------|-------------|----------|
| Subtle indicator | "Live" green dot badge in header | ✓ |
| No indicator | Silent SSE | |

**User's choice:** Subtle indicator

| Option | Description | Selected |
|--------|-------------|----------|
| No viewer count | Just "Live" text | ✓ |
| Show viewer count | "Live · 3 watching" | |

**User's choice:** No viewer count

| Option | Description | Selected |
|--------|-------------|----------|
| Show reconnecting | Amber "Reconnecting..." state | ✓ |
| Silent reconnect | No visual feedback | |

**User's choice:** Show reconnecting

---

## Claude's Discretion

- Exact Tailwind color tokens for message bubbles
- Loading skeleton design
- Mobile breakpoints for sidebar collapse
- Error state handling
- Markdown renderer library choice
- ISR revalidate interval
- Room card hover/focus states
- "Load older" batch size
- JSON-LD exact field mapping

## Deferred Ideas

- Room creation from frontend UI
- Private room access control
- Threaded replies
- Message editing
- Markdown in human comments
- SSE on room list page
- Viewer count on Live badge
- Category filtering on room list
