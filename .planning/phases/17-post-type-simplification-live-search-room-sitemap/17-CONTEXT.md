# Phase 17: Post Type Simplification + Live Search + Room Sitemap - Context

**Gathered:** 2026-04-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Questions are invisible from all creation and discovery surfaces (while the 9 existing question pages remain accessible), the `/data` page shows live agent search activity with jaw-dropping visualizations, and room URLs are included in Solvr's sitemap index. Three independent workstreams: simplify post types, build public analytics page, extend sitemap.

</domain>

<decisions>
## Implementation Decisions

### Data Page Presentation (`/data`)
- **D-01:** Dashboard cards layout — stat cards at top (total searches, agent %, human %, zero-result rate), trending queries table below, category breakdown chart at bottom. Similar to Vercel Analytics / Plausible
- **D-02:** Client-side rendered with 60s polling — no SSR, no SEO needed. Must feel gorgeous, live, jaw-dropping to humans
- **D-03:** "Live Search Activity" as page header
- **D-04:** Full chart library for visualizations — not lightweight/CSS-only. Jaw-dropping visuals following Solvr's design language. Charts for category breakdown and agent/human split
- **D-05:** Show full query text publicly in trending — no anonymization. Queries are about code topics, not PII
- **D-06:** Subtle pulse + fade transitions — green pulse dot in header ("Live"), smooth fade-in on data refresh, counter animations on stat cards
- **D-07:** "Data" added to main header navigation — nav order: Problems, Ideas, Rooms, Agents, Data
- **D-08:** Top 10 trending queries displayed by default
- **D-09:** Selectable time range: 1h / 24h / 7d — user can toggle between windows
- **D-10:** Stack cards + simplify charts on mobile — stat cards in 2x2 grid, charts switch to simplified bar view, trending table full-width
- **D-11:** Static meta tags for social sharing — og:title="Solvr — Live Search Activity", og:description about real-time developer/agent search activity

### Data Page Category Clusters
- **D-12:** Combined approach — type_filter (problem/idea) as primary category grouping, using existing search_queries.type_filter column. No tags in search_queries table, so tag-based topic clusters deferred
- **D-13:** type_filter only for now — fast aggregation, no JOINs needed

### Data Page Endpoints
- **D-14:** New public route: `GET /v1/data/*` — dedicated public endpoints separate from admin: `/v1/data/trending`, `/v1/data/breakdown`, `/v1/data/categories`. Accept `window` query param (1h/24h/7d)
- **D-15:** Both server-side cache (60s TTL) + rate limiting per IP
- **D-16:** Filtered view by default (exclude known bot/cron searches), with toggle to show all activity including automated

### Question Type Removal
- **D-17:** Keep question type in DB, hide from creation — leave DB constraint as-is. Backend still accepts 'question' for existing 9 posts. Remove only from frontend creation flows and navigation. No migration needed
- **D-18:** Remove sitemap-questions.xml entirely — delete the route file. Sitemap index stops referencing it
- **D-19:** Hide /questions listing page, keep /questions/[id] individual pages working (HTTP 200) — SIMPLIFY-02 satisfied
- **D-20:** Still return questions in search results — if a query matches an existing question, show it. Just prevent creating new ones
- **D-21:** Remove 'Questions' from all feed/filter UIs — type filters, sidebar navigation, any post type dropdown. Keep Problem + Idea only
- **D-22:** Keep 'question' in TypeScript API response types — backend returns it for existing 9 posts. Remove from CreatePostData type only
- **D-23:** No redirects for legacy question URLs — direct URLs return HTTP 200, that's sufficient

### Room Sitemap
- **D-24:** Standard sitemap pattern — add 'rooms' type to backend sitemap handler, create `sitemap-rooms.xml` frontend route, add to sitemap index. Follow exact same pattern as `sitemap-problems.xml`
- **D-25:** Exclude private rooms (is_private=true), include all public rooms regardless of expiry status — rooms have unique deep content (many messages per room), more SEO-valuable than similar static posts
- **D-26:** Use last_active_at for lastmod timestamps — rooms with recent activity get higher crawl priority
- **D-27:** Delete sitemap-questions.xml route entirely — remove from sitemap index

### Claude's Discretion
- Exact chart library choice (recharts, chart.js, nivo, etc.) — must be visually impressive
- Chart color palette and animation details
- Exact Tailwind tokens for stat cards and dashboard layout
- Rate limit thresholds (requests per minute per IP)
- Bot/cron detection heuristic for the filtered view toggle
- Loading skeleton design for /data page
- ISR or caching strategy for room sitemap generation
- Exact responsive breakpoints for dashboard mobile adaptation

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Search Analytics (existing backend)
- `backend/internal/api/handlers/search_analytics.go` — Existing admin trending/summary endpoints to reference for public /v1/data/* implementation
- `backend/internal/models/search_query.go` — SearchQuery, SearchAnalytics, TrendingSearch structs
- `backend/migrations/000067_create_search_queries.up.sql` — search_queries table schema (query, type_filter, searcher_type, results_count, duration_ms)

### Post Type System
- `backend/internal/models/post.go` — PostType enum (PostTypeProblem, PostTypeQuestion, PostTypeIdea), ValidPostTypes(), IsValidPostType()
- `frontend/components/new-post/new-post-form.tsx` — POST_TYPES array with hardcoded question/problem/idea
- `frontend/lib/api-types.ts` — TypeScript union types including 'question'
- `backend/migrations/000003_create_posts.up.sql` — DB constraint: CHECK (type IN ('problem', 'question', 'idea'))

### Sitemap System
- `frontend/app/sitemap.xml/route.ts` — Sitemap index referencing 7 sub-sitemaps
- `frontend/app/sitemap-problems.xml/route.ts` — Pattern to follow for sitemap-rooms.xml
- `frontend/app/sitemap-questions.xml/route.ts` — Route to delete
- `frontend/app/sitemap-core.xml/route.ts` — References /questions in core sitemap (remove)
- `backend/internal/api/handlers/sitemap.go` — SitemapHandler (add rooms type)
- `backend/internal/models/sitemap.go` — SitemapURLs struct (add Rooms field)

### Frontend Patterns
- `frontend/components/header.tsx` — Main navigation header (add "Data" link, reorder nav)
- `frontend/app/rooms/page.tsx` — Room list page (rooms already built, Phase 16)
- `backend/internal/models/room.go` — Room model with slug field for sitemap URLs

### Requirements
- `.planning/REQUIREMENTS.md` — SIMPLIFY-01 through SIMPLIFY-03, SEARCH-01 through SEARCH-04, SITEMAP-01 through SITEMAP-03

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `SearchAnalyticsHandler` (`handlers/search_analytics.go`) — existing admin endpoints for trending/summary can be referenced for public endpoint logic
- `SearchQuery` model (`models/search_query.go`) — existing structs for query analytics
- `SitemapHandler` (`handlers/sitemap.go`) — existing handler with `GetSitemapURLs()` and `GetSitemapCounts()` to extend with rooms
- `Header` component (`components/header.tsx`) — add "Data" nav link and reorder
- Sitemap route pattern (`app/sitemap-problems.xml/route.ts`) — copy for rooms

### Established Patterns
- Admin analytics endpoints use `X-Admin-API-Key` — new public `/v1/data/*` endpoints must NOT require admin auth
- Sitemap routes fetch from `/v1/sitemap/urls?type=X` — need to add rooms type support
- SSR detail pages use `cache()` + `generateMetadata()` + ISR — but /data page is CSR, different pattern
- Feed pagination uses "Load More" button — not relevant to /data

### Integration Points
- `frontend/app/data/page.tsx` — new CSR analytics page (route does not exist)
- `frontend/app/sitemap-rooms.xml/route.ts` — new sitemap route (does not exist)
- `backend/internal/api/handlers/` — new data handler for public analytics endpoints
- `frontend/components/header.tsx` — add "Data" to nav, reorder items
- `frontend/components/new-post/new-post-form.tsx` — remove question type option
- `frontend/app/sitemap-core.xml/route.ts` — remove /questions reference

</code_context>

<specifics>
## Specific Ideas

- The /data page must be "jaw-dropping" and "gorgeous" — this is a visual showcase page, not a utilitarian admin panel. User explicitly wants impressive chart visualizations
- Rooms are strategically more valuable for SEO than existing 1000+ similar posts: rooms have unique deep content (many messages per room), harder to reach via navigation clicks, different from the shallow similar posts that Google isn't indexing well
- User is thinking about pivoting the problem type to be tied to rooms/A2A in the future — "tie the problem to the new agent room kind of thing." This is deferred but informs why we're only removing questions (dead type) and keeping problems alive
- Both humans and agents are first-class searchers — the agent/human breakdown is the core value proposition of the /data page

</specifics>

<deferred>
## Deferred Ideas

- **Post similarity detection/spam prevention** — User flagged duplicate/similar posts clogging the feed (e.g., same agent posting very similar problems). Needs investigation and a plan. Not Phase 17 scope — new capability
- **Problem-Room integration** — Tying problems to rooms/A2A for agent collaboration on solutions. Future phase, significant new capability
- **Tag-based topic clusters on /data** — Would require adding tags to search_queries table or JOINing with posts. Deferred until type_filter-only clusters prove insufficient
- **SSE live stream on /data** — Real-time search feed as searches happen. More dramatic but needs new SSE endpoint
- **Dynamic OG meta** — og:description with current top trending query (requires SSR for meta at minimum)

None — discussion stayed within phase scope

</deferred>

---

*Phase: 17-post-type-simplification-live-search-room-sitemap*
*Context gathered: 2026-04-04*
