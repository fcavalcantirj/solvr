# Phase 17: Post Type Simplification + Live Search + Room Sitemap — Research

**Researched:** 2026-04-04
**Domain:** Frontend UI cleanup, public analytics API, Next.js sitemap routes, Go handler extension
**Confidence:** HIGH — all claims verified against the live codebase

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Data Page Presentation (`/data`)**
- D-01: Dashboard cards layout — stat cards at top (total searches, agent %, human %, zero-result rate), trending queries table below, category breakdown chart at bottom
- D-02: Client-side rendered with 60s polling — no SSR, no SEO needed. Must feel gorgeous, live, jaw-dropping
- D-03: "Live Search Activity" as page header
- D-04: Full chart library — jaw-dropping visuals following Solvr's design language. Charts for category breakdown and agent/human split
- D-05: Show full query text publicly — no anonymization
- D-06: Subtle pulse + fade transitions — green pulse dot in header ("Live"), smooth fade-in on data refresh, counter animations on stat cards
- D-07: "Data" added to main header navigation — nav order: Problems, Ideas, Rooms, Agents, Data
- D-08: Top 10 trending queries displayed by default
- D-09: Selectable time range: 1h / 24h / 7d
- D-10: Stack cards + simplify charts on mobile — stat cards in 2x2 grid, charts switch to simplified bar view, trending table full-width
- D-11: Static meta tags for social sharing — og:title="Solvr — Live Search Activity"

**Data Page Category Clusters**
- D-12: type_filter (problem/idea) as primary category grouping using existing search_queries.type_filter column
- D-13: type_filter only for now — fast aggregation, no JOINs needed

**Data Page Endpoints**
- D-14: New public route: `GET /v1/data/*` — `/v1/data/trending`, `/v1/data/breakdown`, `/v1/data/categories`. Accept `window` query param (1h/24h/7d)
- D-15: Both server-side cache (60s TTL) + rate limiting per IP
- D-16: Filtered view by default (exclude known bot/cron searches), with toggle to show all activity including automated

**Question Type Removal**
- D-17: Keep question type in DB, hide from creation — no migration needed
- D-18: Remove sitemap-questions.xml entirely — delete the route file
- D-19: Hide /questions listing page, keep /questions/[id] individual pages working (HTTP 200)
- D-20: Still return questions in search results
- D-21: Remove 'Questions' from all feed/filter UIs
- D-22: Keep 'question' in TypeScript API response types — remove from CreatePostData type only
- D-23: No redirects for legacy question URLs — direct URLs return HTTP 200

**Room Sitemap**
- D-24: Standard sitemap pattern — add 'rooms' type to backend sitemap handler, create sitemap-rooms.xml frontend route, add to sitemap index
- D-25: Exclude private rooms (is_private=true), include all public rooms regardless of expiry status
- D-26: Use last_active_at for lastmod timestamps
- D-27: Delete sitemap-questions.xml route entirely — remove from sitemap index

### Claude's Discretion
- Exact chart library choice (recharts, chart.js, nivo, etc.) — must be visually impressive
- Chart color palette and animation details
- Exact Tailwind tokens for stat cards and dashboard layout
- Rate limit thresholds (requests per minute per IP)
- Bot/cron detection heuristic for the filtered view toggle
- Loading skeleton design for /data page
- ISR or caching strategy for room sitemap generation
- Exact responsive breakpoints for dashboard mobile adaptation

### Deferred Ideas (OUT OF SCOPE)
- Post similarity detection/spam prevention
- Problem-Room integration
- Tag-based topic clusters on /data
- SSE live stream on /data
- Dynamic OG meta
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SIMPLIFY-01 | Question type removed from post creation flows (frontend + API validation) | D-17: keep in DB, remove from new-post-form.tsx POST_TYPES array and CreatePostData type |
| SIMPLIFY-02 | Existing 9 questions remain accessible via direct URL (no 404s) | D-23: no redirects needed; /questions/[id] already serves HTTP 200 via questionsHandler.Get |
| SIMPLIFY-03 | Question type removed from navigation, new-post selector, and sitemap generation | Header.tsx has two QUESTIONS links; sitemap-core.xml has /questions; sitemap-questions.xml to delete |
| SEARCH-01 | `/data` page shows trending search queries (rolling 24h) | GetTrending() in db/search_analytics.go returns query_normalized + count; new /v1/data/trending endpoint |
| SEARCH-02 | `/data` page shows agent vs human search breakdown | GetSummary().BySearcherType map already aggregates by searcher_type; expose via /v1/data/breakdown |
| SEARCH-03 | `/data` page shows search category clusters | type_filter column in search_queries; simple GROUP BY query; new /v1/data/categories endpoint |
| SEARCH-04 | `/data` page refreshes data via polling (60s interval) | CSR page with useEffect + setInterval(60000); recharts already installed |
| SITEMAP-01 | sitemap-rooms.xml generated with all public rooms | Add GetSitemapRooms() to SitemapRepository; new /v1/sitemap/urls?type=rooms endpoint |
| SITEMAP-02 | Room sitemap added to sitemap index | Add 'sitemap-rooms.xml' to SUB_SITEMAPS in sitemap.xml/route.ts |
| SITEMAP-03 | Room URLs use SEO-descriptive slugs matching /rooms/[slug] routes | Room model has slug field; use slug in loc URL construction |
</phase_requirements>

---

## Summary

Phase 17 is three independent workstreams touching frontend UI, backend API, and sitemap infrastructure. All three are additive or subtractive with no cross-workstream dependencies — they can be planned and executed in parallel.

**Workstream A — Post Type Simplification** is entirely frontend-only. The DB constraint stays unchanged (problem/question/idea). The backend validation stays unchanged (all three types accepted). Only the frontend surfaces need changes: remove question from the POST_TYPES array in new-post-form.tsx, remove QUESTIONS links from Header.tsx (both desktop and mobile), delete sitemap-questions.xml route, remove /questions from sitemap-core.xml, and hide the /questions listing page. The 9 existing question detail pages (/questions/[id]) already return HTTP 200 via questionsHandler.Get — no backend changes needed.

**Workstream B — Live Search /data page** requires one new backend handler (data_handler.go) with three public GET endpoints under /v1/data/*, and one new frontend page (app/data/page.tsx). The DB queries are simple extensions of the already-working SearchAnalyticsRepository. Recharts is already installed at 2.15.4. Server-side caching uses sync.Map with TTL; rate limiting uses the project's existing apimiddleware.RateLimiter pattern.

**Workstream C — Room Sitemap** follows the exact pattern of sitemap-problems.xml. Extend SitemapRepository with a GetSitemapRooms() method, add 'rooms' to the type whitelist in SitemapHandler.GetSitemapURLs, add a Rooms field to SitemapURLs/SitemapCounts models, create the frontend route app/sitemap-rooms.xml/route.ts, and add it to the SUB_SITEMAPS list while removing sitemap-questions.xml.

**Primary recommendation:** Execute all three workstreams as independent plans in any order. No shared state, no merge conflicts between workstreams.

---

## Standard Stack

### Core (already installed — no new dependencies needed)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| recharts | 2.15.4 (in package.json) | Charts for /data page | Already installed; composable React charts with excellent animation support |
| lucide-react | ^0.454.0 (in package.json) | Icons (pulse dot, indicators) | Already installed project-wide |
| Tailwind CSS | ^4.1.9 | Styling stat cards, animations | Already installed project-wide |
| go-chi/chi | v5 (in router.go) | Router for new /v1/data/* endpoints | Already the project router |
| sync.Map (stdlib) | Go stdlib | In-process 60s response cache | No new dep; matches existing in-memory rate limit store pattern |

[VERIFIED: codebase grep] — recharts 2.15.4 confirmed in frontend/package.json line 63.
[VERIFIED: codebase grep] — go-chi/chi confirmed in router.go imports.

### No New Dependencies Required

The entire phase can be implemented with zero new npm or Go module dependencies. Recharts is already available for charts. The project's existing rate limiter (`apimiddleware.NewRateLimiter`) can be reused for /v1/data/* endpoints.

**Installation:** Nothing to install.

---

## Architecture Patterns

### Recommended Project Structure (new files only)

```
backend/internal/api/handlers/
└── data_handler.go              # New: GET /v1/data/trending, /breakdown, /categories

backend/internal/db/
└── data_analytics.go            # New: DataAnalyticsRepository (or extend search_analytics.go)

backend/internal/models/
└── sitemap.go                   # Extend: add SitemapRoom struct, Rooms field to SitemapURLs/SitemapCounts

frontend/app/
├── data/
│   └── page.tsx                 # New: CSR analytics page
└── sitemap-rooms.xml/
    └── route.ts                 # New: Room sitemap route

# Files to delete:
frontend/app/sitemap-questions.xml/route.ts
```

### Pattern 1: Public Data Handler (Workstream B)

**What:** New DataHandler with three public endpoints, 60s server-side cache, per-IP rate limiting, `window` query param (1h/24h/7d mapped to integer days/hours).
**When to use:** Public read-only aggregate data, no auth required.

```go
// Source: backend/internal/api/handlers/search_analytics.go (existing pattern)
// New file: backend/internal/api/handlers/data_handler.go

type DataHandler struct {
    repo   DataAnalyticsReaderInterface
    cache  sync.Map // key: "trending:24h", value: cachedResponse{data, expiresAt}
}

// GET /v1/data/trending?window=24h
// GET /v1/data/breakdown?window=24h
// GET /v1/data/categories?window=24h
```

The `window` param maps to: `1h` = 0.042 days, `24h` = 1 day, `7d` = 7 days. Use hours directly in the SQL query (`NOW() - INTERVAL '1 hour'` etc.) rather than converting to fractional days.

**Rate limiting:** Add a dedicated rate limit group for /v1/data/* using the existing `apimiddleware.RateLimiter`. Recommended: 60 req/min per IP (generous, it's analytics). The existing RateLimiter reads config from DB via `loadRateLimitConfig(pool)` — add a new config key `data_api` or use the global default.

**Server-side cache pattern:**
```go
// Source: [ASSUMED] — standard Go sync.Map TTL cache pattern
type cachedEntry struct {
    data      any
    expiresAt time.Time
}

func (h *DataHandler) getCached(key string, ttl time.Duration, fetch func() (any, error)) (any, error) {
    if v, ok := h.cache.Load(key); ok {
        entry := v.(cachedEntry)
        if time.Now().Before(entry.expiresAt) {
            return entry.data, nil
        }
    }
    data, err := fetch()
    if err != nil {
        return nil, err
    }
    h.cache.Store(key, cachedEntry{data: data, expiresAt: time.Now().Add(ttl)})
    return data, nil
}
```

### Pattern 2: Bot/Cron Filter (Workstream B — D-16)

**What:** Exclude known automated searcher_ids from default trending queries. Toggle in query param: `?include_bots=true`.
**Detection heuristic (Claude's discretion):** From STATE.md analysis, two known bot searcher_ids: `e48fb1b2` (449 searches, cron), `agent_NaoParis` (362 searches). Maintain a small hardcoded exclusion list as a starting point. The filterable view is the default; `?include_bots=true` shows raw data.

SQL filter to add to GetTrending-style queries:
```sql
-- Default (filtered): exclude known cron/bot searchers
AND (searcher_id IS NULL OR searcher_id NOT IN ($known_bots...))
-- OR use a simpler heuristic: exclude searcher_ids with >100 searches of same 2-3 queries
```

For phase 17, use the simple hardcoded exclusion list approach. Tag-based clustering is deferred.

### Pattern 3: Categories Query (Workstream B — SEARCH-03)

**What:** Aggregate search_queries by type_filter column.
**SQL (simple, no JOIN needed):**

```sql
-- Source: backend/migrations/000067_create_search_queries.up.sql (type_filter column confirmed)
SELECT
    COALESCE(type_filter, 'unfiltered') AS category,
    COUNT(*) AS search_count
FROM search_queries
WHERE searched_at >= NOW() - INTERVAL '24 hours'
GROUP BY type_filter
ORDER BY search_count DESC
```

[VERIFIED: codebase grep] — `type_filter VARCHAR(20)` confirmed in migration 000067.

### Pattern 4: Room Sitemap Repository Extension (Workstream C)

**What:** Add GetSitemapRooms() to SitemapRepository. Add 'rooms' to valid types in SitemapHandler.
**Pattern:** Exact copy of the 'posts' case in GetPaginatedSitemapURLs.

```go
// Source: backend/internal/db/sitemap.go — GetPaginatedSitemapURLs (existing pattern)
// Add to SitemapRepository:
func (r *SitemapRepository) GetSitemapRooms(ctx context.Context) ([]models.SitemapRoom, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT slug, last_active_at
        FROM rooms
        WHERE is_private = false
        AND deleted_at IS NULL
        ORDER BY last_active_at DESC
    `)
    // scan slug + last_active_at
}
```

New model struct (add to sitemap.go):
```go
// Source: backend/internal/models/sitemap.go (existing file, extend)
type SitemapRoom struct {
    Slug         string    `json:"slug"`
    LastActiveAt time.Time `json:"last_active_at"`
}
```

[VERIFIED: codebase read] — Room model has `slug string` and `last_active_at time.Time` fields.
[VERIFIED: codebase read] — Room model has `is_private bool` and `deleted_at *time.Time` fields.

### Pattern 5: Frontend sitemap-rooms.xml Route (Workstream C)

**What:** Exact copy of sitemap-problems.xml with rooms substitution.

```typescript
// Source: frontend/app/sitemap-problems.xml/route.ts (existing pattern)
// New file: frontend/app/sitemap-rooms.xml/route.ts
import { buildSitemapXml, BASE_URL, API_URL } from '@/lib/sitemap-utils';

export async function GET() {
  try {
    const res = await fetch(`${API_URL}/v1/sitemap/urls?type=rooms`, {
      next: { revalidate: 21600 },
    });
    const json = await res.json();
    const rooms = json.data?.rooms || [];

    const entries = rooms.map((r: { slug: string; last_active_at: string }) => ({
      loc: `${BASE_URL}/rooms/${r.slug}`,
      lastmod: r.last_active_at,
      changefreq: 'daily' as const,
      priority: 0.8,
    }));

    return buildSitemapXml(entries);
  } catch {
    return buildSitemapXml([]);
  }
}
```

[VERIFIED: codebase read] — buildSitemapXml, BASE_URL, API_URL are all exported from frontend/lib/sitemap-utils.ts.

### Pattern 6: CSR /data Page Structure (Workstream B)

**What:** Client-side rendered page using recharts, 60s polling via useEffect + setInterval.
**Pattern:** Based on existing CSR pages in the codebase that use `"use client"` + hooks.

```typescript
// Source: [ASSUMED] — standard React CSR polling pattern
// New file: frontend/app/data/page.tsx
"use client";

import { useState, useEffect } from 'react';
// recharts is already installed — use BarChart, PieChart, ResponsiveContainer
import { BarChart, Bar, PieChart, Pie, Cell, ResponsiveContainer, Tooltip } from 'recharts';

const POLL_INTERVAL_MS = 60_000;

export default function DataPage() {
  const [window, setWindow] = useState<'1h' | '24h' | '7d'>('24h');
  const [trending, setTrending] = useState(null);
  const [breakdown, setBreakdown] = useState(null);
  const [categories, setCategories] = useState(null);
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null);

  const fetchAll = async () => {
    // fetch /v1/data/trending?window=24h etc.
    setLastRefresh(new Date());
  };

  useEffect(() => {
    fetchAll();
    const id = setInterval(fetchAll, POLL_INTERVAL_MS);
    return () => clearInterval(id);
  }, [window]);
  // ...
}
```

### Anti-Patterns to Avoid

- **Don't add 'question' to CreatePostData type guard**: D-22 says keep 'question' in APIPost.type (for existing 9 posts) but remove from CreatePostData. These are different types in api-types.ts.
- **Don't add SSR to /data page**: D-02 mandates CSR. Static meta tags (D-11) use Next.js `export const metadata = {}` at module level, which works even in CSR pages via layout.tsx.
- **Don't filter out questions from /v1/questions GET**: D-20 says questions still appear in search results. The questions listing endpoint stays functional; only the frontend UI hides the navigation link.
- **Don't paginate the room sitemap via frontend**: Use the same non-paginated fetch pattern as sitemap-problems.xml. Room count is small (not thousands); no pagination needed yet.
- **Don't create a separate cache layer**: Use sync.Map in-process cache. No Redis, no external cache.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Charts for /data page | Custom SVG charts | recharts (already installed, v2.15.4) | BarChart, PieChart, ResponsiveContainer, animations, tooltips all included |
| XML sitemap generation | Custom XML string | buildSitemapXml() from lib/sitemap-utils.ts | Already handles all XML escaping, Content-Type header, Cache-Control |
| Rate limiting for /v1/data | Custom IP counter | apimiddleware.NewRateLimiter | Already in use for all /v1 routes; just add data config |
| Counter animation on stat cards | JS animation library | CSS transition + React state | Tailwind transition classes + count-up via useEffect, no new dep needed |
| API response caching | Redis or external cache | sync.Map with TTL in handler | 60s TTL, in-process, zero deps, matches project's existing in-memory store pattern |

---

## Common Pitfalls

### Pitfall 1: Deleting sitemap-questions.xml breaks the sitemap index

**What goes wrong:** The SUB_SITEMAPS array in sitemap.xml/route.ts still references 'sitemap-questions.xml' after the route file is deleted, causing a 404 for that sub-sitemap entry when Google crawls the index.
**Why it happens:** Two separate changes must happen atomically: delete the file AND remove the entry from the index array.
**How to avoid:** Delete route file and update sitemap.xml/route.ts SUB_SITEMAPS in the same commit/task.
**Warning signs:** `curl https://solvr.dev/sitemap-questions.xml` returns 404.

### Pitfall 2: questions listing page hidden but navigation still linked

**What goes wrong:** Remove /questions from sitemap-core.xml but forget to remove QUESTIONS links from header.tsx (desktop AND mobile nav).
**Why it happens:** Header.tsx has two separate QUESTIONS links — desktop nav (line 37-44) and mobile nav (line 138-140).
**How to avoid:** Search header.tsx for all `/questions` href occurrences. Both must be removed.
**Warning signs:** Mobile menu still shows QUESTIONS link.

### Pitfall 3: new-post-form.tsx defaultType prop accepts 'question'

**What goes wrong:** The `NewPostFormProps` interface has `defaultType?: 'problem' | 'question' | 'idea'`. If 'question' is removed from POST_TYPES but not from the prop type, TypeScript still allows passing `defaultType="question"` which would silently render no type selected.
**Why it happens:** Two places to update: the POST_TYPES array AND the defaultType type union.
**How to avoid:** Update both simultaneously. Also check if any page passes `defaultType="question"`.
**Warning signs:** TypeScript error or empty type selection on new-post form.

### Pitfall 4: /v1/data/* endpoints missing from router.go

**What goes wrong:** DataHandler is implemented but never registered in router.go, so all /v1/data/* requests return 404.
**Why it happens:** Router registration is separate from handler creation in this codebase (see router.go lines 640-646 for sitemap pattern).
**How to avoid:** Add the route registrations inside the `r.Route("/v1", ...)` block in router.go, after creating the handler.
**Warning signs:** `curl https://api.solvr.dev/v1/data/trending` returns 404.

### Pitfall 5: SitemapURLs response shape for rooms

**What goes wrong:** Frontend sitemap-rooms.xml fetches `/v1/sitemap/urls?type=rooms` and reads `json.data?.rooms` but the handler's `writeSitemapURLsResponse` only writes posts/agents/users/blog_posts fields.
**Why it happens:** writeSitemapURLsResponse is hardcoded to specific fields (router.go lines 116-128). Adding rooms requires updating BOTH the model and the response writer.
**How to avoid:** Update `writeSitemapURLsResponse` to include rooms, AND add Rooms to SitemapURLs struct, AND add rooms to the valid types map in getSitemapURLsPaginated.
**Warning signs:** `json.data?.rooms` is undefined; site generates empty rooms sitemap.

### Pitfall 6: 1h window produces zero results

**What goes wrong:** The 1h time window returns empty data because most search activity is sparse over 1 hour, making the page look broken.
**Why it happens:** Real traffic is thin on an early-stage platform; 1h often has <5 searches.
**How to avoid:** Ensure the UI handles empty/sparse data gracefully — show "No activity in the last 1h" placeholder rather than broken charts.
**Warning signs:** Recharts renders empty axes with no data.

---

## Code Examples

### Existing: Search Analytics DB Query

```go
// Source: backend/internal/db/search_analytics.go — GetSummary() (existing)
// BySearcherType already aggregates agent vs human:
summary.BySearcherType = make(map[string]int)
// rows from: SELECT searcher_type, COUNT(*) FROM search_queries WHERE ... GROUP BY searcher_type
```

### Existing: Sitemap Handler Type Dispatch

```go
// Source: backend/internal/api/handlers/sitemap.go — getSitemapURLsPaginated()
// Current valid types:
validTypes := map[string]bool{"posts": true, "agents": true, "users": true, "blog_posts": true}
// Phase 17: add "rooms": true
```

### Existing: Sitemap Response Writer (needs rooms field)

```go
// Source: backend/internal/api/handlers/sitemap.go — writeSitemapURLsResponse()
// Current (lines 116-128):
response := map[string]interface{}{
    "data": map[string]interface{}{
        "posts":      urls.Posts,
        "agents":     urls.Agents,
        "users":      urls.Users,
        "blog_posts": urls.BlogPosts,
        // Phase 17: add "rooms": urls.Rooms
    },
}
```

### Existing: Header nav links to remove

```typescript
// Source: frontend/components/header.tsx lines 37-44 (desktop) and 138-140 (mobile)
// DELETE both occurrences:
<Link href="/questions" className="...">QUESTIONS</Link>
// Replace with nothing (or reorder per D-07: Problems, Ideas, Rooms, Agents, Data)
```

### Existing: new-post-form.tsx POST_TYPES to trim

```typescript
// Source: frontend/components/new-post/new-post-form.tsx lines 13-17
// CURRENT:
const POST_TYPES = [
  { value: 'question', label: 'Question', description: '...' },
  { value: 'problem', label: 'Problem', description: '...' },
  { value: 'idea', label: 'Idea', description: '...' },
] as const;
// PHASE 17 — remove question entry. Also change grid from grid-cols-3 to grid-cols-2
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| recharts v2 composable API | recharts v2 composable API | Still current | BarChart, PieChart, ResponsiveContainer are stable; no v3 breaking changes yet |
| Next.js route handlers for sitemaps | Next.js route handlers (`route.ts`) | Since Next.js 13.4 | Full control over XML output, works with `output: 'standalone'` |

**Note:** The project explicitly forbids `generateSitemaps()` (CLAUDE.md deployment constraints). All sitemaps must use route handlers — this is already what the existing sitemaps use. The rooms sitemap follows the same pattern.

---

## Runtime State Inventory

> Step 2.5 check: This phase is not a rename/rebrand/migration phase. The only "removal" is hiding UI surfaces; no DB records are moved, renamed, or deleted. Sitemap route deletion is a file delete, not a data migration.

**Nothing found in any category** — verified by reviewing all 5 categories:
- Stored data: No DB records change. search_queries table is read-only for this phase.
- Live service config: No n8n workflows, no Datadog, no Tailscale changes.
- OS-registered state: No cron tasks, systemd units, or pm2 processes reference 'questions'.
- Secrets/env vars: No new environment variables required. /v1/data/* endpoints are public (no API key).
- Build artifacts: No installed packages renamed; no egg-info directories affected.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Node.js | Frontend build/test | Yes | v22.22.0 | — |
| Go | Backend build/test | Yes | 1.25.6 | — |
| npm | Frontend package install | Yes | 10.9.4 | — |
| recharts | /data page charts | Yes (in package.json) | 2.15.4 | — |
| PostgreSQL | search_queries table | Assumed running (project standard) | — | Tests skip if DATABASE_URL unset |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** None.

[VERIFIED: bash] — node v22.22.0, go 1.25.6, npm 10.9.4 confirmed.
[VERIFIED: codebase read] — recharts 2.15.4 in package.json dependencies.

---

## Validation Architecture

> workflow.nyquist_validation is not explicitly set to false in .planning/config.json — treating as enabled.

### Test Framework

| Property | Value |
|----------|-------|
| Backend framework | Go testing package (standard) |
| Frontend framework | Vitest 4.0.18 |
| Backend config | No config file; run from backend/ |
| Frontend config | vitest config inferred via package.json scripts |
| Backend quick run | `cd backend && go test ./internal/api/handlers/... -run TestData -v` |
| Backend full suite | `cd backend && go test ./...` |
| Frontend quick run | `cd frontend && npm test -- --run` |
| Frontend full suite | `cd frontend && npm test -- --coverage` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SIMPLIFY-01 | POST_TYPES has no 'question' entry | unit | `cd frontend && npm test -- --run` | ❌ Wave 0 |
| SIMPLIFY-02 | /questions/[id] returns HTTP 200 | manual smoke | manual: `curl https://solvr.dev/questions/{id}` | — |
| SIMPLIFY-03 | Header has no QUESTIONS link; sitemap-questions.xml deleted | unit | `cd frontend && npm test -- --run` | ❌ Wave 0 |
| SEARCH-01 | /v1/data/trending returns trending queries | unit (handler) | `cd backend && go test ./internal/api/handlers/... -run TestDataHandler_Trending` | ❌ Wave 0 |
| SEARCH-02 | /v1/data/breakdown returns agent/human counts | unit (handler) | `cd backend && go test ./internal/api/handlers/... -run TestDataHandler_Breakdown` | ❌ Wave 0 |
| SEARCH-03 | /v1/data/categories returns type_filter groups | unit (handler) | `cd backend && go test ./internal/api/handlers/... -run TestDataHandler_Categories` | ❌ Wave 0 |
| SEARCH-04 | /data page polls every 60s | unit (component) | `cd frontend && npm test -- --run` | ❌ Wave 0 |
| SITEMAP-01 | sitemap-rooms.xml contains public room slugs | unit (route handler) | `cd frontend && npm test -- --run` | ❌ Wave 0 |
| SITEMAP-02 | sitemap.xml/route.ts references sitemap-rooms.xml | unit | `cd frontend && npm test -- --run` | ❌ Wave 0 |
| SITEMAP-03 | Room URLs use slug format /rooms/[slug] | unit | `cd frontend && npm test -- --run` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd backend && go test ./internal/api/handlers/... && cd ../frontend && npm test -- --run`
- **Per wave merge:** `cd backend && go test ./... && cd ../frontend && npm test -- --coverage`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `backend/internal/api/handlers/data_handler_test.go` — covers SEARCH-01, SEARCH-02, SEARCH-03
- [ ] `frontend/components/new-post/__tests__/new-post-form.test.tsx` — covers SIMPLIFY-01
- [ ] `frontend/components/__tests__/header.test.tsx` — covers SIMPLIFY-03
- [ ] `frontend/app/data/__tests__/page.test.tsx` — covers SEARCH-04
- [ ] `frontend/app/sitemap-rooms.xml/__tests__/route.test.ts` — covers SITEMAP-01, SITEMAP-03

**Note:** Backend handler tests should use mock repos (implement the DataAnalyticsReaderInterface with a mock struct) — same pattern as existing handler tests. Frontend component tests use Vitest + @testing-library/react with vi.mock() for fetch.

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | /v1/data/* is public, no auth required |
| V3 Session Management | No | No session created |
| V4 Access Control | Yes | /v1/data/* must NOT require admin key — verify no checkSearchAnalyticsAuth() is called |
| V5 Input Validation | Yes | `window` query param validated to enum (1h/24h/7d), reject others with 400 |
| V6 Cryptography | No | No crypto operations |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Unvalidated `window` query param (SQL injection via interval) | Tampering | Whitelist: only accept "1h", "24h", "7d" mapped to constants; never interpolate raw param into SQL |
| DoS via polling — client hammers /v1/data/* every second | DoS | Server-side 60s cache + per-IP rate limit |
| Scraping full query history via trending endpoint | Information Disclosure | Already mitigated: endpoint returns aggregates only (count + normalized query), not raw queries or searcher_ids |

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | sync.Map TTL cache is sufficient for 60s data freshness at current traffic | Architecture Patterns — Pattern 1 | Low risk: if traffic spikes, the worst outcome is cache misses, not data corruption. Upgrade to Redis if needed. |
| A2 | /questions listing page can be hidden by removing nav links only (no redirect needed) | Pitfall 2 | Low risk: D-19 explicitly says HTTP 200 is sufficient. Google will naturally stop crawling once removed from sitemap and nav. |
| A3 | Hardcoded bot exclusion list (2 known searcher_ids) is sufficient for D-16 | Architecture Patterns — Pattern 2 | Medium risk: new bot IDs may appear. The toggle (`?include_bots=true`) provides an escape hatch. |

**All other claims verified directly from codebase source files.**

---

## Open Questions

1. **Does the /questions listing page need to return 404 or stay as HTTP 200?**
   - What we know: D-19 says "hide /questions listing page, keep /questions/[id] individual pages working (HTTP 200)". D-23 says "no redirects for legacy question URLs".
   - What's unclear: "hide" could mean (a) remove from nav only, page still renders, or (b) make the route return 404 or redirect.
   - Recommendation: Interpret as (a) — remove from nav, sitemap-core.xml, and new-post selector, but leave the page rendering. This satisfies SIMPLIFY-02 (no 404s for individual questions) and is the minimal-risk approach.

2. **Static metadata (`export const metadata`) on a CSR page**
   - What we know: D-02 mandates CSR ("use client"). D-11 requires static og:meta. Next.js 15 does not allow `export const metadata` in a `"use client"` file.
   - What's unclear: whether to put metadata in a layout.tsx wrapper or use a server component wrapper around the CSR component.
   - Recommendation: Create `app/data/layout.tsx` (server component) that exports the metadata, with `app/data/page.tsx` being the CSR component. This is the standard Next.js pattern for mixing SSR metadata with CSR pages.

---

## Sources

### Primary (HIGH confidence — verified from codebase)
- `backend/internal/db/search_analytics.go` — GetTrending, GetSummary, GetZeroResults implementations verified
- `backend/internal/api/handlers/search_analytics.go` — existing admin + public stats patterns
- `backend/migrations/000067_create_search_queries.up.sql` — table schema with type_filter column confirmed
- `backend/internal/models/search_query.go` — TrendingSearch, SearchAnalytics structs confirmed
- `backend/internal/models/post.go` — PostType constants (problem/question/idea) confirmed
- `backend/internal/models/sitemap.go` — SitemapURLs struct (no Rooms field currently) confirmed
- `backend/internal/models/room.go` — Room.Slug, Room.LastActiveAt, Room.IsPrivate, Room.DeletedAt confirmed
- `backend/internal/db/sitemap.go` — GetPaginatedSitemapURLs type switch pattern confirmed
- `backend/internal/api/handlers/sitemap.go` — validTypes map, writeSitemapURLsResponse confirmed
- `backend/internal/api/router.go` — /v1/stats/search, /v1/sitemap routes, rate limiter setup confirmed
- `frontend/app/sitemap.xml/route.ts` — SUB_SITEMAPS array with sitemap-questions.xml confirmed
- `frontend/app/sitemap-problems.xml/route.ts` — exact pattern for rooms sitemap
- `frontend/app/sitemap-questions.xml/route.ts` — file to delete confirmed
- `frontend/app/sitemap-core.xml/route.ts` — /questions static entry to remove confirmed
- `frontend/components/header.tsx` — QUESTIONS links in desktop (lines 37-44) and mobile (138-140) confirmed
- `frontend/components/new-post/new-post-form.tsx` — POST_TYPES array with question entry confirmed
- `frontend/lib/api-types.ts` — APIPost.type includes 'question' confirmed
- `frontend/lib/sitemap-utils.ts` — buildSitemapXml, BASE_URL, API_URL exports confirmed
- `frontend/package.json` — recharts 2.15.4 confirmed

### Secondary (MEDIUM confidence)
- npm registry: recharts latest is 3.8.1; project pins 2.15.4 which is stable and sufficient

### Tertiary (LOW confidence)
- None — all claims verified from codebase

---

## Metadata

**Confidence breakdown:**
- Post type simplification: HIGH — all touch points found and confirmed in codebase
- Search analytics /data endpoints: HIGH — existing DB layer fully verified; new handler is a thin wrapper
- Recharts integration: HIGH — library already installed, API is stable
- Room sitemap: HIGH — exact pattern to copy exists in sitemap-problems.xml
- Bot/cron filter heuristic: MEDIUM — known bot IDs from STATE.md analysis, but new bots may appear

**Research date:** 2026-04-04
**Valid until:** 2026-05-04 (stable codebase, no fast-moving dependencies)
