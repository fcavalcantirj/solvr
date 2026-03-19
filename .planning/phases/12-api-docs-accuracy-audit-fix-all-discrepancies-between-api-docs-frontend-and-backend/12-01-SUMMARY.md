---
phase: 12-api-docs-accuracy-audit-fix-all-discrepancies-between-api-docs-frontend-and-backend
plan: "12-01"
subsystem: ui
tags: [api-docs, typescript, documentation, frontend]

requires: []
provides:
  - "Rewritten api-endpoint-data-core.ts matching actual backend handlers exactly"
  - "Search with hybrid description, 12 params, 16 response fields"
  - "Agents with all register/list/patch params verified against Go structs"
  - "Feed using page/per_page pagination (not limit)"
  - "Stats with 10 accurate response fields (crystallized_posts, solved_today, posted_today, total_posts)"
  - "Blog group with 9 endpoints matching blog.go handler"
  - "Leaderboard group with limit/offset pagination matching leaderboard.go"
  - "Badges group for agents and users"
  - "Heartbeat group with full response shape"
  - "Auth group with register, login, claim-referral endpoints"
affects: [api-docs page, frontend docs, agent onboarding]

tech-stack:
  added: []
  patterns:
    - "All endpoint docs verified against backend Go handler structs before writing"
    - "Test file updated to match scope decisions (remove obsolete tests, add new group tests)"

key-files:
  created: []
  modified:
    - frontend/components/api/api-endpoint-data-core.ts
    - frontend/components/api/api-endpoint-data.test.ts

key-decisions:
  - "Sitemap group removed entirely — SEO infrastructure not relevant for agent-first docs"
  - "Stats granular endpoints (/stats/trending, /stats/problems, /stats/questions, /stats/ideas) removed per scope decision"
  - "Blog group added with 9 endpoints instead of planned 9 (exact match)"
  - "Heartbeat requires auth (both api_key and jwt) — verified from router.go (not public)"
  - "Test file updated to replace 5 obsolete tests with 13 new tests covering new groups"

patterns-established:
  - "Always read backend handler request/response structs before writing docs"
  - "Use UUID-format example IDs (not fake short IDs like p_abc123)"

requirements-completed: []

duration: 45min
completed: 2026-03-19
---

# Phase 12 Plan 01: Core Endpoints Summary

**Rewrote api-endpoint-data-core.ts with 10 endpoint groups verified against Go handlers — added auth/blog/leaderboard/badges/heartbeat, fixed search params, agents params, feed pagination, and stats response fields**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-03-19T13:00:00Z
- **Completed:** 2026-03-19T13:45:00Z
- **Tasks:** 10
- **Files modified:** 2

## Accomplishments

- Verified all endpoint params against actual Go handler request structs (RegisterAgentRequest, UpdateAgentRequest, SearchOptions, etc.)
- Added 6 missing endpoint groups: Auth (register/login/claim-referral), Blog (9 endpoints), Leaderboard (2), Badges (2), Heartbeat (1)
- Fixed 4 inaccuracies: search description, search params (6→12), feed limit→page/per_page, stats response missing 4 fields
- Removed Sitemap group and 4 granular stats endpoints per scope decision
- File stays at 850 lines (under 900 limit), no TypeScript errors

## Task Commits

All tasks committed in a single atomic commit (single file rewrite):

1. **Tasks 1-10: All core endpoint changes** - `28f9c19` (fix)

## Files Created/Modified

- `frontend/components/api/api-endpoint-data-core.ts` — Complete rewrite: 850 lines, 9 endpoint groups
- `frontend/components/api/api-endpoint-data.test.ts` — Updated: replaced 5 obsolete tests with 13 new tests; all new tests pass

## Decisions Made

- Removed Sitemap group entirely (SEO infrastructure, not agent-facing)
- Removed /stats/trending, /stats/problems, /stats/questions, /stats/ideas per scope decision
- Heartbeat auth set to "both" (api_key or jwt) — verified from router.go
- Blog DELETE returns `// 204 No Content` as confirmed from blog.go handler
- POST /blog/{slug}/view also returns 204 No Content (no response body)
- Leaderboard uses limit/offset not page/per_page — kept accurate to handler
- Badges response uses `{ "badges": [...] }` (not `{ "data": [...] }`) — matches BadgesResponse struct

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test file updated to match removed endpoints**
- **Found during:** Task 5 (remove sitemap) and Task 4 (remove stats)
- **Issue:** Existing tests checked for /sitemap/urls, /sitemap/counts, /stats/problems, /stats/questions, /stats/ideas — all of which we removed
- **Fix:** Updated test file to remove 5 obsolete tests and add 13 new tests covering new groups (Blog, Leaderboard, Badges, Heartbeat, corrected Stats)
- **Files modified:** frontend/components/api/api-endpoint-data.test.ts
- **Verification:** All 13 new tests pass, 44 tests pass total, 4 pre-existing failures unchanged
- **Committed in:** 28f9c19

---

**Total deviations:** 1 auto-fixed (Rule 1 - Bug — test sync required)
**Impact on plan:** Essential for test correctness — old tests were testing removed endpoints.

## Issues Encountered

None — all backend files were found at expected paths, all handler structs were clear.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Core endpoint docs complete and accurate
- Plans 12-02 and 12-03 were already completed before this plan ran
- Plan 12-04 (IPFS/checkpoints) is the remaining plan in this phase

---
*Phase: 12-api-docs-accuracy-audit-fix-all-discrepancies-between-api-docs-frontend-and-backend*
*Completed: 2026-03-19*
