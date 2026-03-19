---
phase: 12-api-docs-accuracy-audit-fix-all-discrepancies-between-api-docs-frontend-and-backend
plan: "12-02"
subsystem: ui
tags: [api-docs, content, typescript]

requires: []
provides:
  - Accurate content endpoint docs (Posts, Problems, Questions, Ideas) matching backend handlers
  - Generic CRUD endpoints removed from Posts group
  - Missing endpoints added: GET /posts/{id}/my-vote, GET /problems/{id}/approaches/{aid}/history
  - All response shapes verified against actual Go handler code
affects: [api-docs-frontend]

tech-stack:
  added: []
  patterns:
    - "API docs verified directly from handler code, not assumed from spec"
    - "All example IDs use UUID format matching backend PK type"

key-files:
  created: []
  modified:
    - frontend/components/api/api-endpoint-data-content.ts

key-decisions:
  - "Removed generic POST/PATCH/DELETE /posts — backend uses type-specific routes (POST /problems, POST /questions, POST /ideas)"
  - "GET /posts/{id} kept — valid backend route for reading any post type by ID"
  - "Evolve endpoint response is 'idea evolution linked' (links existing post, not creates new idea)"
  - "answer vote response is {message: 'vote recorded'} not {vote_score} — verified from VoteOnAnswer handler"
  - "DELETE /answers returns 204 No Content — verified from DeleteAnswer handler (w.WriteHeader(http.StatusNoContent))"
  - "Response types for ideas are: build, critique, expand, question, support (not support, concern, extension, question as plan guessed)"

patterns-established:
  - "Content endpoint docs: always read Go handler to get exact response fields before writing docs"

requirements-completed: []

duration: 15min
completed: 2026-03-19
---

# Phase 12 Plan 02: Content Endpoints — Remove Generic CRUD, Fix Accuracy, Add Missing Content Endpoints Summary

**Content endpoint docs rewritten: generic CRUD removed, 3 wrong response shapes fixed, 2 missing endpoints added, all IDs converted to UUID format — all verified against Go handler source**

## Performance

- **Duration:** 15 min
- **Started:** 2026-03-19T16:30:00Z
- **Completed:** 2026-03-19T16:45:00Z
- **Tasks:** 11
- **Files modified:** 1

## Accomplishments
- Removed 4 inaccurate endpoints from Posts group (generic GET list, POST, PATCH, DELETE) that don't exist as generic routes in the backend
- Added GET /posts/{id}/my-vote and GET /problems/{id}/approaches/{aid}/history — both verified in router.go
- Fixed 5 wrong response shapes by reading actual Go handlers: vote response (+user_vote), evolve response (linked not created), answers vote (message not vote_score), accept answer (message + UUID), DELETE answers (204 No Content)
- Fixed POST /ideas/{id}/evolve to use correct param (evolved_post_id) and description
- Fixed POST /ideas/{id}/responses to include required response_type param with correct enum values
- Replaced all fake prefixed IDs (p_abc123, q_abc123, i_abc123, apr_xyz, ans_xyz) with UUID format

## Task Commits

Tasks 1-11 committed as a single atomic file rewrite:

1. **Tasks 1-11: Full file rewrite** — `18ecc76` (fix)

## Files Created/Modified
- `frontend/components/api/api-endpoint-data-content.ts` — Rewritten: 477 lines, all 11 tasks applied

## Decisions Made
- Committed all 11 tasks in a single commit since all changes are to one file and form a coherent unit
- response_type enum for ideas: build, critique, expand, question, support (plan guessed: support, concern, extension, question — handler had the correct values)
- GetMyVote handler returns `{"data": {"vote": "<direction>"}}` — the field is "vote" not "direction" as plan guessed; null when no vote exists

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] response_type enum values differ from plan's guess**
- **Found during:** Task 5 (Fix POST /ideas/{id}/responses — add required response_type param)
- **Issue:** Plan suggested enum "support, concern, extension, question" but handler validates against: build, critique, expand, question, support
- **Fix:** Used actual enum values from IsValidResponseType() check and models.ResponseType constants
- **Files modified:** frontend/components/api/api-endpoint-data-content.ts
- **Verification:** grep "build, critique, expand, question, support" in file — matches handler error message exactly
- **Committed in:** 18ecc76

**2. [Rule 1 - Bug] GetMyVote response field is "vote" not "direction"**
- **Found during:** Task 3 (Add GET /posts/{id}/my-vote endpoint)
- **Issue:** Plan guessed response shape as `{direction, created_at}` but handler returns `{"data": {"vote": <pointer>}}`
- **Fix:** Used actual response shape from GetMyVote handler code: `{"data": {"vote": "up"}}`
- **Files modified:** frontend/components/api/api-endpoint-data-content.ts
- **Verification:** grep '"vote"' in my-vote endpoint response
- **Committed in:** 18ecc76

---

**Total deviations:** 2 auto-fixed (2 bugs — wrong guessed values corrected by reading handler source)
**Impact on plan:** Both fixes improve accuracy — the whole point of this plan. No scope creep.

## Issues Encountered
None — all handler reads were successful, all acceptance criteria verified before commit.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Content endpoint docs are accurate and complete
- Ready for Phase 12-03 (next plan in phase)

---
*Phase: 12-api-docs-accuracy-audit-fix-all-discrepancies-between-api-docs-frontend-and-backend*
*Completed: 2026-03-19*
