---
phase: 12-api-docs-accuracy-audit-fix-all-discrepancies-between-api-docs-frontend-and-backend
plan: "12-03"
subsystem: ui
tags: [api-docs, typescript, frontend, endpoints]

requires: []
provides:
  - Accurate user endpoint docs with correct PATCH /me params (avatar_url, no username)
  - Social/follows group (POST /follow, DELETE /follow, GET /following, GET /followers)
  - GET /me/auth-methods endpoint documented
  - DELETE /me and DELETE /agents/me endpoints documented
  - Removed excluded groups: API Keys, Bookmarks, Views & Reports
  - DELETE /comments/{id} correctly shows 204 No Content
  - GET /users sort uses "reputation" not "karma"
  - All fake IDs replaced with UUID format
affects: []

tech-stack:
  added: []
  patterns:
    - "Agent-first docs: exclude human-only UI features (API Keys, Bookmarks, Views/Reports)"
    - "DELETE /me returns 200 OK with message body (not 204), DELETE /agents/me same"
    - "Follows endpoints use limit/offset pagination (not page/per_page)"

key-files:
  created: []
  modified:
    - frontend/components/api/api-endpoint-data-user.ts

key-decisions:
  - "DELETE /me returns 200 OK (not 204) — verified from handler: w.WriteHeader(http.StatusOK) with JSON body"
  - "DELETE /agents/me returns 200 OK (not 204) — verified from handler: w.WriteHeader(http.StatusOK) with JSON body"
  - "GET /users uses limit/offset params (backend UsersListResponse struct has Limit/Offset fields)"
  - "GET /me/auth-methods response has last_used_at field in addition to linked_at"
  - "Follows use limit/offset pagination (parseFollowsPagination reads limit/offset query params)"
  - "DELETE /follow returns {status: 'unfollowed'} not 204 — verified from handler: response.WriteJSON(w, http.StatusOK, ...)"

requirements-completed: []

duration: 15min
completed: 2026-03-19
---

# Phase 12 Plan 03: User Endpoints — Fix Accuracy, Remove Excluded Sections, Add Follows and /me Endpoints Summary

**Rewrote api-endpoint-data-user.ts to remove 3 excluded groups, fix 4 accuracy bugs, and add 6 missing endpoints — all verified against actual Go handler source**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-19T16:15:00Z
- **Completed:** 2026-03-19T16:30:00Z
- **Tasks:** 11
- **Files modified:** 1

## Accomplishments
- Removed API Keys, Bookmarks, and Views & Reports groups (human-only UI features excluded from agent-first docs)
- Fixed PATCH /me: removed username param, added avatar_url param; fixed response example
- Fixed GET /users: sort description now says "reputation" not "karma"; response uses "reputation" field
- Fixed DELETE /comments/{id}: correctly shows 204 No Content (verified handler returns StatusNoContent)
- Added Social group with 4 follow endpoints (POST /follow, DELETE /follow, GET /following, GET /followers)
- Added GET /me/auth-methods with correct response shape (auth_methods array with provider, linked_at, last_used_at)
- Added DELETE /me and DELETE /agents/me with correct 200 OK responses (verified handler code)
- Replaced all fake IDs (user_abc, cmt_xyz, key_abc, bm_xyz) with UUID format
- File went from 424 lines to 447 lines (under 900 limit), TypeScript compiles clean

## Task Commits

All tasks executed as a single atomic commit (plan tasks 1-11 are all changes to the same file):

1. **Tasks 1-11: Complete rewrite of api-endpoint-data-user.ts** - `693ac5d` (fix)

## Files Created/Modified
- `frontend/components/api/api-endpoint-data-user.ts` - Rewrote to remove excluded groups, fix accuracy, add missing endpoints

## Decisions Made
- DELETE /me returns 200 OK (not 204): `me.go` handler calls `w.WriteHeader(http.StatusOK)` and encodes JSON body
- DELETE /agents/me returns 200 OK (not 204): `agents.go` handler calls `w.WriteHeader(http.StatusOK)` and encodes JSON body
- Follows use limit/offset pagination: `parseFollowsPagination()` reads `limit` and `offset` query params
- DELETE /follow returns `{status: "unfollowed"}` (200 OK): handler calls `response.WriteJSON(w, http.StatusOK, ...)` with that body
- GET /me/auth-methods includes `last_used_at` field: `AuthMethodResponse` struct has `LastUsedAt string` field

## Deviations from Plan

None - plan executed exactly as written.

Note: Pre-existing test failures exist for `PATCH /posts/{id}` and `DELETE /posts/{id}` (expected in content file, left for another plan).

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan 12-03 complete; user endpoint docs are accurate and agent-first
- Pre-existing test failures for PATCH/DELETE /posts/{id} remain in api-endpoint-data.test.ts (to be fixed by a future plan adding those endpoints to content file)

---
*Phase: 12-api-docs-accuracy-audit-fix-all-discrepancies-between-api-docs-frontend-and-backend*
*Completed: 2026-03-19*
