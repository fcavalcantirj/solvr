# Phase 17: Post Type Simplification + Live Search + Room Sitemap - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-04
**Phase:** 17-Post Type Simplification + Live Search + Room Sitemap
**Areas discussed:** Data page presentation, Question type removal, Room sitemap wiring, Data page endpoints, Data page mobile experience, Data page SEO metadata, Question removal edge cases, Header nav order

---

## Data Page Presentation

| Option | Description | Selected |
|--------|-------------|----------|
| Dashboard cards | Stats cards at top, trending table below, category chart. Like Vercel Analytics | ✓ |
| Single-page table view | One big table, minimal, data-dense | |
| Marketing-style metrics | Hero numbers, animated counters, colorful charts | |

**User's choice:** Dashboard cards
**Notes:** None

---

## Category Clusters

| Option | Description | Selected |
|--------|-------------|----------|
| Use existing type_filter | Group by type_filter field, no new logic | |
| Tag-based clustering | Analyze tags on search results for topic clustering | |
| Both combined | type_filter as primary, tags as secondary | ✓ |

**User's choice:** Both combined, but noted post type simplification means type_filter is just problem/idea
**Notes:** User reminded we're simplifying types. Final decision: type_filter only for now (tags not in search_queries table)

---

## Rendering Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Client-side with polling | CSR, fetch on mount, poll 60s | ✓ |
| SSR + client hydration | Initial SSR for SEO, then client polling | |
| SSR only with ISR | Server-rendered with ISR revalidation | |

**User's choice:** Client-side with polling
**Notes:** "I don't care for SEO in this page, it must be awesome, gorgeous, live to humans"

---

## Page Header

| Option | Description | Selected |
|--------|-------------|----------|
| "Live Search Activity" | Emphasizes real-time activity. Clean, factual | ✓ |
| "What developers are searching for" | Marketing-oriented, frames as demand signal | |
| "Solvr Intelligence" | Data product positioning | |

**User's choice:** "Live Search Activity"

---

## Chart Visualizations

| Option | Description | Selected |
|--------|-------------|----------|
| CSS-only bars | Pure CSS horizontal bars, zero deps | |
| Lightweight chart library | ~40KB, more polished | |
| Full chart library | Jaw-dropping visuals | ✓ |

**User's choice:** Full chart library
**Notes:** "Dude, make it gorgeous. Use a chart library, not necessarily a lightweight. Follow the gorgeous-looking view of Solvr, but make this page jaw-dropping."

---

## Query Text Privacy

| Option | Description | Selected |
|--------|-------------|----------|
| Show full query text | Display exact searches publicly | ✓ |
| Anonymize/aggregate | Show categories instead of exact queries | |

**User's choice:** Show full query text publicly

---

## Live Animations

| Option | Description | Selected |
|--------|-------------|----------|
| Subtle pulse + fade transitions | Green pulse dot, smooth fade-in, counter animations | ✓ |
| Real-time SSE stream | Live terminal-style feed | |
| You decide | Claude picks | |

**User's choice:** Subtle pulse + fade transitions

---

## Navigation

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, in main nav | Add "Data" alongside other nav items | ✓ |
| Footer link only | Less prominent | |
| Both nav + landing CTA | Maximum visibility | |

**User's choice:** Yes, in main nav

---

## Trending Query Count

| Option | Description | Selected |
|--------|-------------|----------|
| Top 10 | Fits above fold | ✓ |
| Top 20 with "Show more" | Expandable list | |
| Top 5 | Minimal | |

**User's choice:** Top 10

---

## Time Range Selector

| Option | Description | Selected |
|--------|-------------|----------|
| Fixed 24h | Simple, per SEARCH-01 | |
| Selectable: 1h / 24h / 7d | User toggles between windows | ✓ |
| 24h default + 7d toggle | Two options only | |

**User's choice:** Selectable: 1h / 24h / 7d

---

## Backend Question Handling

| Option | Description | Selected |
|--------|-------------|----------|
| Keep in DB, hide from creation | No migration, just frontend removal | ✓ |
| Soft-deprecate with new migration | Add deprecated flag | |
| Remove entirely + redirect | Delete type, convert to idea | |

**User's choice:** Keep in DB, hide from creation

---

## Sitemap Questions XML

| Option | Description | Selected |
|--------|-------------|----------|
| Remove entirely | Delete route file | ✓ |
| Keep for 9 existing URLs | Belt-and-suspenders | |

**User's choice:** Remove entirely

---

## Question Routes

| Option | Description | Selected |
|--------|-------------|----------|
| Hide listing, keep individual pages | Remove /questions listing, keep /questions/[id] | ✓ |
| Keep both routes | Leave listing, remove from nav | |
| Redirect /questions to /problems | 301 redirect listing | |

**User's choice:** Hide listing, keep individual pages
**Notes:** User is thinking about pivoting problems to be tied to rooms/A2A in the future. Deferred idea captured.

---

## Search Results with Questions

| Option | Description | Selected |
|--------|-------------|----------|
| Still return in search results | Show matching questions in search | ✓ |
| Filter out from search | Completely hide from discovery | |
| Return but hide type badge | Show in results, suppress type indicator | |

**User's choice:** Still return in search results

---

## Data Page Endpoints Structure

| Option | Description | Selected |
|--------|-------------|----------|
| New public route: GET /v1/data/* | Dedicated public endpoints with window param | ✓ |
| Extend /v1/stats/search | Overload existing endpoint | |
| Single endpoint: GET /v1/data | One endpoint returns everything | |

**User's choice:** New public route: GET /v1/data/*

---

## Data Quality (Bot Filtering)

| Option | Description | Selected |
|--------|-------------|----------|
| Filter known bots | Exclude cron searchers | |
| Show raw data | All searches including automated | |
| Both with toggle | Filtered default, toggle to show all | ✓ |

**User's choice:** Both with toggle
**Notes:** User also flagged a spam/duplicate post problem (deferred to separate investigation)

---

## Rate Limiting

| Option | Description | Selected |
|--------|-------------|----------|
| Server-side cache (60s TTL) | Cache results, match poll interval | ✓ |
| Rate limit per IP | Defensive middleware | ✓ |
| You decide | | |

**User's choice:** Both — server-side cache AND rate limit per IP

---

## Mobile Experience

| Option | Description | Selected |
|--------|-------------|----------|
| Stack cards + simplify charts | 2x2 grid, simplified bar view, full-width table | ✓ |
| Full desktop experience, narrower | Same layout squeezed | |
| You decide | | |

**User's choice:** Stack cards + simplify charts

---

## SEO/OG Meta Tags

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, static meta tags | og:title, og:description for social sharing | ✓ |
| No meta tags | Utility page, not for sharing | |
| Dynamic meta with top query | Requires SSR for meta | |

**User's choice:** Yes, static meta tags

---

## Nav Order

| Option | Description | Selected |
|--------|-------------|----------|
| Problems, Ideas, Rooms, Agents, Data | Content types first, then participants, then analytics | ✓ |
| Problems, Ideas, Agents, Rooms, Data | Keep current order, append new | |
| Data, Problems, Ideas, Rooms, Agents | Lead with Data | |

**User's choice:** Problems, Ideas, Rooms, Agents, Data

---

## TypeScript Types

| Option | Description | Selected |
|--------|-------------|----------|
| Keep 'question' in API types | Backend returns it for 9 existing posts. Remove from CreatePostData only | ✓ |
| Remove everywhere + cast | Clean types but lossy for existing questions | |

**User's choice:** Keep 'question' in API types

---

## Claude's Discretion

- Chart library choice (must be visually impressive)
- Chart color palette and animation details
- Exact Tailwind tokens for stat cards
- Rate limit thresholds
- Bot/cron detection heuristic
- Loading skeleton design for /data
- Room sitemap caching strategy
- Responsive breakpoints

## Deferred Ideas

- Post similarity detection/spam prevention (user flagged duplicate posts clogging feed)
- Problem-Room integration (tying problems to rooms/A2A)
- Tag-based topic clusters on /data page
- SSE live stream on /data page
- Dynamic OG meta for /data page
