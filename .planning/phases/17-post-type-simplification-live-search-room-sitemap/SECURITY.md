# SECURITY.md — Phase 17: Post Type Simplification, Live Search, Room Sitemap

**Phase:** 17 — post-type-simplification-live-search-room-sitemap
**Audited:** 2026-04-05
**ASVS Level:** 1
**Threats Closed:** 11/11

---

## Threat Verification

| Threat ID | Category | Disposition | Evidence |
|-----------|----------|-------------|----------|
| T-17-01 | Information Disclosure | accept | Accepted risk: removing /questions from sitemap-core.xml/route.ts reduces crawl surface. No sensitive data exposed. See accepted risks log below. |
| T-17-02 | Spoofing | accept | Accepted risk: /data is a public page with no auth. The DATA nav link in header.tsx is a standard navigation element. See accepted risks log below. |
| T-17-03 | Information Disclosure | accept | Accepted risk: /v1/sitemap/urls?type=rooms returns only slug and last_active_at for public rooms (is_private=false AND deleted_at IS NULL). See accepted risks log below. |
| T-17-04 | Denial of Service | accept | Accepted risk: sitemap-rooms.xml route uses ISR (revalidate: 21600 — 6h cache). No DB query per request. See accepted risks log below. |
| T-17-05 | Tampering | mitigate | CLOSED. `validWindows` map whitelist at data_handler.go:37-41. `parseWindowParam` rejects non-whitelisted values with 400 at data_handler.go:66-75. `windowToInterval` in data_analytics.go:29-40 provides an additional whitelist layer ensuring only safe strings reach SQL INTERVAL clause. No raw user input interpolated into SQL. |
| T-17-06 | Denial of Service | mitigate | CLOSED. `dataCacheTTL = 60 * time.Second` with `sync.Map` cache at data_handler.go:22-61. `getCached` called on all three handler methods (lines 101, 137, 168). DB is queried at most once per 60s per cache key. |
| T-17-07 | Information Disclosure | accept | Accepted risk: trending query text is public by design (D-05). `publicTrending` struct at data_handler.go:82-87 strips avg_results and avg_duration_ms from the public response. See accepted risks log below. |
| T-17-08 | Elevation of Privilege | mitigate | CLOSED. `checkSearchAnalyticsAuth` is absent from data_handler.go (0 matches confirmed). All three handlers (GetTrending, GetBreakdown, GetCategories) are public with no auth check, consistent with design requirement. |
| T-17-09 | Denial of Service | accept | Accepted risk: 60s client polling interval is reasonable. Server-side cache (T-17-06) prevents DB hammering even if client polls at a higher frequency. See accepted risks log below. |
| T-17-10 | Information Disclosure | accept | Accepted risk: /data page displays query text publicly per design decision D-05. Queries are code/developer topics, not PII. See accepted risks log below. |
| T-17-11 | Tampering | mitigate | CLOSED. No `dangerouslySetInnerHTML` present in frontend/app/data/page.tsx (0 matches confirmed). All query text rendered via standard JSX interpolation (`{item.query}` at page.tsx:341), which React auto-escapes. |

---

## Accepted Risks Log

| Threat ID | Category | Component | Risk Summary | Accepted By |
|-----------|----------|-----------|--------------|-------------|
| T-17-01 | Information Disclosure | sitemap-core.xml | Removing /questions from sitemap reduces crawl surface for a dead post type (9 posts). No sensitive data was ever exposed; this is a cleanup. | Plan 17-01 threat register |
| T-17-02 | Spoofing | header.tsx /data link | The DATA navigation link points to a fully public, unauthenticated page. No impersonation or trust boundary concern. Standard nav element. | Plan 17-01 threat register |
| T-17-03 | Information Disclosure | /v1/sitemap/urls?type=rooms | Room sitemap exposes slug and last_active_at for public rooms only (is_private=false, deleted_at IS NULL). Both fields are intentionally public. Private rooms are excluded at the DB layer. | Plan 17-02 threat register |
| T-17-04 | Denial of Service | sitemap-rooms.xml route | Route is protected by Next.js ISR with a 6-hour revalidation window (revalidate: 21600). No DB hit per request during the cache window. Acceptable for a sitemap endpoint. | Plan 17-02 threat register |
| T-17-07 | Information Disclosure | trending query text | Full search query text is intentionally public per design decision D-05. Queries are code and developer topics. avg_results and avg_duration_ms are stripped from the public trending response (publicTrending struct). | Plan 17-03 threat register |
| T-17-09 | Denial of Service | client polling /v1/data/* | Frontend polls at 60-second intervals. Combined with the 60s server-side cache, the DB impact of polling is bounded. Acceptable for a live analytics page. | Plan 17-04 threat register |
| T-17-10 | Information Disclosure | /data page query text | /data page displays search queries publicly, same as T-17-07. Design decision D-05 explicitly allows this. Queries are not PII. | Plan 17-04 threat register |

---

## Unregistered Threat Flags

None. All four SUMMARY.md files (17-01 through 17-04) report no threat flags.

---

## Audit Notes

- Plans 17-01 and 17-04 are purely subtractive or frontend-only changes with no new trust boundaries or auth paths.
- Plan 17-02 (room sitemap) introduces a new public API surface (`/v1/sitemap/urls?type=rooms`) which is correctly scoped to public rooms at the DB layer.
- Plan 17-03 (data analytics backend) introduces three new public endpoints. T-17-05 (window injection) and T-17-06 (DoS via polling) are both mitigated in code. T-17-08 (public access by design) is confirmed by absence of auth checks.
- The `buildBotExclusionClause` function in data_analytics.go:45-57 uses string interpolation of a compile-time constant list (`KnownBotSearcherIDs`), not user input. This is safe and consistent with the mitigation rationale for T-17-05.
