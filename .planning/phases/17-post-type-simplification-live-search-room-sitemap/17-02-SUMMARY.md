---
phase: 17-post-type-simplification-live-search-room-sitemap
plan: 02
subsystem: api, sitemap
tags: [go, sitemap, seo, nextjs, typescript, rooms]

# Dependency graph
requires:
  - phase: 13-database-foundation
    provides: rooms table with slug, is_private, last_active_at, deleted_at columns
provides:
  - SitemapRoom struct in backend/internal/models/sitemap.go
  - Rooms field on SitemapURLs and SitemapCounts models
  - /v1/sitemap/urls?type=rooms endpoint serving public room slugs + last_active_at
  - /v1/sitemap/counts rooms count
  - sitemap-rooms.xml frontend route generating XML with /rooms/{slug} URLs
  - sitemap index updated: rooms added, questions removed
affects: [16-frontend-rooms-human-commenting, sitemap, seo]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Sitemap extension pattern: add struct to models, query in db, validTypes + response in handler, frontend route"
    - "TDD RED/GREEN/REFACTOR across model + handler + db layers for sitemap type addition"

key-files:
  created:
    - backend/internal/api/handlers/sitemap_rooms_test.go
    - frontend/app/sitemap-rooms.xml/route.ts
  modified:
    - backend/internal/models/sitemap.go
    - backend/internal/db/sitemap.go
    - backend/internal/api/handlers/sitemap.go
    - backend/internal/api/handlers/sitemap_test.go
    - frontend/app/sitemap.xml/route.ts
  deleted:
    - frontend/app/sitemap-questions.xml/route.ts

key-decisions:
  - "changefreq=daily for rooms (update more frequently than static posts), priority=0.8"
  - "Split sitemap_rooms_test.go from sitemap_test.go to comply with 900-line CLAUDE.md constraint"
  - "sitemap-questions.xml deleted entirely (9 questions, dead post type)"

patterns-established:
  - "New sitemap type = 4 changes: model struct + SitemapURLs/SitemapCounts fields, db query in all 3 repo methods, handler validTypes + response maps, frontend route"

requirements-completed: [SITEMAP-01, SITEMAP-02, SITEMAP-03]

# Metrics
duration: 20min
completed: 2026-04-05
---

# Phase 17 Plan 02: Room Sitemap + Questions Sitemap Removal Summary

**SitemapRoom model, /v1/sitemap/urls?type=rooms backend endpoint, sitemap-rooms.xml Next.js route, and sitemap index update removing dead questions sitemap**

## Performance

- **Duration:** ~20 min
- **Started:** 2026-04-05T00:13:00Z
- **Completed:** 2026-04-05T00:34:14Z
- **Tasks:** 2
- **Files modified:** 6 (+ 1 created, 1 deleted)

## Accomplishments
- Backend sitemap system extended with rooms type: SitemapRoom struct, Rooms fields on SitemapURLs + SitemapCounts, room queries in all 3 DB methods (GetSitemapURLs, GetSitemapCounts, GetPaginatedSitemapURLs)
- New /v1/sitemap/urls?type=rooms endpoint returns public (is_private=false, deleted_at IS NULL) room slugs with last_active_at timestamps
- sitemap-rooms.xml Next.js route created following exact sitemap-problems.xml pattern, using /rooms/{slug} URL format and last_active_at for lastmod
- Sitemap index updated: sitemap-rooms.xml added, sitemap-questions.xml removed (9 dead posts)
- TDD: RED tests committed first, GREEN implementation followed, REFACTOR split test file for 900-line compliance

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: Add failing tests for rooms sitemap support** - `bf653cb` (test)
2. **Task 1 GREEN: Implement rooms support in backend sitemap system** - `0741561` (feat)
3. **Task 1 REFACTOR: Split sitemap_rooms_test.go** - `a75a005` (refactor)
4. **Task 2: Create sitemap-rooms.xml route, update index, delete questions sitemap** - `e69e187` (feat)

**Plan metadata:** (docs commit pending)

_Note: TDD tasks have multiple commits (test → feat → refactor)_

## Files Created/Modified
- `backend/internal/models/sitemap.go` - Added SitemapRoom struct, Rooms field on SitemapURLs and SitemapCounts
- `backend/internal/db/sitemap.go` - Added room queries to all 3 sitemap methods; initialized Rooms slice; added case "rooms" to switch
- `backend/internal/api/handlers/sitemap.go` - Added "rooms" to validTypes, updated error message, added rooms to both response maps
- `backend/internal/api/handlers/sitemap_test.go` - Updated MockSitemapRepository with Rooms field; split out room tests (780 lines)
- `backend/internal/api/handlers/sitemap_rooms_test.go` - New file: room-specific tests (206 lines) — split for line-limit compliance
- `frontend/app/sitemap-rooms.xml/route.ts` - New route: fetches /v1/sitemap/urls?type=rooms, generates /rooms/{slug} XML
- `frontend/app/sitemap.xml/route.ts` - Removed sitemap-questions.xml, added sitemap-rooms.xml (7 sub-sitemaps total)
- `frontend/app/sitemap-questions.xml/route.ts` - DELETED (dead post type, 9 questions)

## Decisions Made
- changefreq=daily for rooms (vs weekly for posts) — rooms receive new messages frequently
- priority=0.8 for rooms (high, below problems at 0.9, reflecting deep content value)
- Test file split: sitemap_rooms_test.go extracted to keep sitemap_test.go under 900 lines per CLAUDE.md
- sitemap-questions.xml fully deleted (not redirected) — questions type is being removed in this phase

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - File Size Constraint] Split sitemap_rooms_test.go to comply with 900-line limit**
- **Found during:** Task 1 REFACTOR
- **Issue:** After adding room tests, sitemap_test.go grew to 975 lines, exceeding the ~900 line CLAUDE.md constraint
- **Fix:** Moved room-specific tests (TestSitemapHandler_GetSitemapURLs_Rooms, TestSitemapHandler_GetSitemapURLs_RoomsPaginated, TestGetSitemapCounts_IncludesRooms) to new sitemap_rooms_test.go
- **Files modified:** backend/internal/api/handlers/sitemap_test.go (780 lines), backend/internal/api/handlers/sitemap_rooms_test.go (206 lines new)
- **Verification:** go build ./internal/api/handlers/ passes; both files under 900 lines
- **Committed in:** a75a005 (refactor commit)

---

**Total deviations:** 1 auto-fixed (1 CLAUDE.md constraint compliance)
**Impact on plan:** File reorganization only. No behavior change. Both test files in same package, tests run identically.

## Issues Encountered
- `data_handler_test.go` in the handlers package (added by parallel agent 17-03) caused package-level build failure when running `go test ./internal/api/handlers/...`. This is a pre-existing cross-agent wave issue — the 17-03 agent committed a TDD RED test without the corresponding implementation. Our sitemap code compiles cleanly (`go build ./internal/api/handlers/` passes). This will resolve when the 17-03 agent commits the GREEN implementation. Logged to deferred-items.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Backend: /v1/sitemap/urls?type=rooms is ready to serve room data once rooms exist in the DB (Phase 13 migration applied)
- Frontend: sitemap-rooms.xml route will generate room URLs once backend returns data
- Sitemap index is clean: 7 sitemaps (core, problems, ideas, agents, users, blog, rooms), no dead questions references
- No blockers for remaining Phase 17 plans

## Known Stubs
None - all sitemap data flows from real DB queries.

## Threat Flags
None - rooms sitemap only exposes slug and last_active_at for public rooms, consistent with threat register T-17-03 (accepted, non-sensitive data).

## Self-Check: PASSED

All files confirmed present:
- backend/internal/models/sitemap.go — FOUND
- backend/internal/db/sitemap.go — FOUND
- backend/internal/api/handlers/sitemap.go — FOUND
- backend/internal/api/handlers/sitemap_rooms_test.go — FOUND
- frontend/app/sitemap-rooms.xml/route.ts — FOUND
- frontend/app/sitemap.xml/route.ts — FOUND
- frontend/app/sitemap-questions.xml/route.ts — CONFIRMED DELETED

All commits confirmed:
- bf653cb (test RED) — FOUND
- 0741561 (feat GREEN) — FOUND
- a75a005 (refactor file split) — FOUND
- e69e187 (feat Task 2) — FOUND

---
*Phase: 17-post-type-simplification-live-search-room-sitemap*
*Completed: 2026-04-05*
