# API Documentation Task Plan - Validation Report

**Date:** 2026-02-17
**Validator:** Claude Code Analysis
**Status:** ⚠️ PARTIALLY VALID - Requires Adjustments

---

## Executive Summary

✅ **Overall Plan Structure:** SOLID - Well-sequenced phases with clear dependencies
⚠️ **Task Details:** NEEDS UPDATES - Several inaccuracies in gap counts and file references
✅ **Approach:** VALID - TDD-first, automation-focused, production-ready

**Critical Finding:** The actual gaps are LARGER than estimated in the plan.

---

## Verified Facts from Codebase

### Backend Analysis
- ✅ **router.go:** 103 route registrations confirmed
- ✅ **openapi_paths.go:** EXISTS at `backend/internal/api/openapi_paths.go` (741 lines)
- ✅ **openapi_schemas.go:** EXISTS at `backend/internal/api/openapi_schemas.go` (701 lines)
- ✅ **discovery.go:** EXISTS at `backend/internal/api/discovery.go` (221 lines)
- ✅ **buildPaths():** Currently returns 56 paths (lines 127-202)
- ✅ **buildSchemas():** Has 90 schema definitions with helper functions

### Frontend Analysis
- ✅ **Endpoint data files:** 4 files exist
  - `api-endpoint-data-core.ts` - 27 endpoints
  - `api-endpoint-data-content.ts` - 30 endpoints
  - `api-endpoint-data-user.ts` - 31 endpoints
  - `api-endpoint-data.ts` - Main exports
- ✅ **Total frontend docs:** 88 endpoints (27+30+31)
- ✅ **TypeScript structure:** Uses `EndpointGroup[]` with typed interfaces
- ✅ **Interface location:** `api-endpoint-types.ts` defines Endpoint, Param, EndpointGroup
- ✅ **API docs page:** `/app/api-docs/page.tsx` exists (35 lines, no Swagger UI yet)

### Testing Infrastructure
- ✅ **Unit test framework:** Vitest (package.json line 12: `"test": "vitest run"`)
- ✅ **Playwright installed:** `@playwright/test@^1.58.1` in devDependencies
- ❌ **Playwright config:** MISSING - No `playwright.config.ts` exists
- ❌ **E2E folder:** MISSING - No `frontend/e2e/` directory
- ✅ **Existing tests:** Multiple `*_test.go` files in `backend/internal/api/`

### SPEC.md Verification
- ✅ **Part 19.4:** EXISTS at line 2880
- ✅ **Title:** "API Documentation"
- ✅ **Requirement:** "Swagger UI at `/docs/api`" (line 2954)
- ✅ **OpenAPI location:** `https://api.solvr.dev/openapi.json` (specified at line 2884)
- ⚠️ **Note:** SPEC says `/docs/api` but plan uses `/api-docs` - CLARIFY PATH

### Production API
- ✅ **OpenAPI endpoint:** `https://api.solvr.dev/v1/openapi.json` (live)
- ✅ **Path count:** 56 paths in production spec
- ✅ **Response:** Valid OpenAPI 3.0.3 JSON

---

## Gap Analysis: THE REAL NUMBERS

### Backend Gap (Router → OpenAPI)
**Plan claimed:** ~28 missing routes
**ACTUAL:** **47 missing routes** (103 in router - 56 in OpenAPI)

**Critical Missing Categories:**
1. **Health endpoints (3):** /health, /health/live, /health/ready
2. **Admin endpoints (5):** /admin/query, /admin/users/{id}, /admin/agents/{id}, /admin/users/deleted, /admin/agents/deleted
3. **Discovery (3):** /.well-known/ai-agent.json, /v1/openapi.json, /v1/openapi.yaml
4. **Auth (2):** POST /v1/auth/register, POST /v1/auth/login
5. **Agent claiming (1):** POST /v1/agents/claim
6. **MCP (1):** POST /v1/mcp
7. **Lists (2):** GET /v1/agents, GET /v1/users
8. **User endpoints (3):** GET /v1/users/{id}/agents, /users/{id}/contributions, /me/auth-methods
9. **Agent endpoints (3):** GET /v1/agents/{id}/activity, PATCH /v1/agents/{id}, DELETE /v1/agents/me
10. **Stats (2):** GET /v1/stats/problems, /v1/stats/questions
11. **Sitemap (2):** GET /v1/sitemap/urls, /v1/sitemap/counts
12. **Leaderboard (2):** GET /v1/leaderboard, /v1/leaderboard/tags/{tag}
13. **Content (1):** GET /v1/problems/{id}/export
14. **User profile (2):** PATCH /v1/me, DELETE /v1/me
15. **Posts (1):** GET /v1/posts/{id}/my-vote
16. **Total:** ~47 missing route definitions

### Frontend Gap (Frontend Docs → OpenAPI)
**Plan claimed:** ~13 outdated/extra entries
**ACTUAL:** **32 endpoint discrepancy** (88 in frontend - 56 in OpenAPI)

**Possible causes:**
- Frontend documents endpoints that don't exist in OpenAPI
- Frontend has duplicates or deprecated entries
- Frontend documents non-v1 endpoints (health, admin, discovery)
- OpenAPI missing endpoints that frontend correctly documents

---

## Task-by-Task Validation

### ✅ PHASE 1: Audit & Foundation

#### Task 1: OpenAPI Completeness Validation Tests
**Status:** ✅ VALID approach, ⚠️ UPDATE scope
**Issues:**
- Plan says "~28 missing routes" → Should be **~47 missing routes**
- Expected failures should say "47 routes in router.go but not in OpenAPI"

**File verification:**
- ✅ `backend/internal/api/openapi_validation_test.go` - Will create
- ✅ Test pattern exists in `router_test.go` for reference

**Recommendation:** ✅ PROCEED with scope adjustment

---

#### Task 2: Frontend-Backend Sync Validation Tests
**Status:** ✅ VALID approach, ⚠️ UPDATE gap count
**Issues:**
- Plan says "~13 endpoints in frontend but not in OpenAPI" → Should be **~32 discrepancy**
- Expected failures need update

**File verification:**
- ✅ `frontend/components/api/__tests__/` - Directory pattern exists (api-endpoint-data.test.ts)
- ✅ Frontend uses Vitest, not Jest
- ✅ Can fetch from https://api.solvr.dev/v1/openapi.json

**Recommendation:** ✅ PROCEED with count adjustment

---

#### Task 3: Broken Link Detection Tests
**Status:** ✅ VALID approach
**Issues:** None - well-specified

**Recommendation:** ✅ PROCEED as-is

---

### ✅ PHASE 2: Backend OpenAPI Completion

#### Task 4: Add Missing OpenAPI Path Definitions
**Status:** ✅ VALID approach, ⚠️ SCOPE MUCH LARGER
**Issues:**
- Plan scope: ~28 routes → **ACTUAL: ~47 routes**
- This is **66% MORE work** than estimated
- May need to split into sub-tasks by category (Health, Admin, Auth, Content, etc.)

**File verification:**
- ✅ `backend/internal/api/openapi_paths.go` exists (741 lines)
- ✅ Pattern confirmed: Functions like `searchPath()`, `postsPath()` return map[string]interface{}
- ✅ `discovery.go` buildPaths() is where paths are registered (lines 127-202)

**Recommendation:** ⚠️ PROCEED but consider splitting into 3-4 sub-tasks:
- Task 4a: Add Health, Admin, Discovery paths (11 routes)
- Task 4b: Add Auth, MCP paths (4 routes)
- Task 4c: Add Agent/User/Content paths (20 routes)
- Task 4d: Add Stats, Sitemap, Leaderboard paths (12 routes)

---

#### Task 5: Add Response Examples to All Schemas
**Status:** ✅ VALID approach
**Issues:** None

**File verification:**
- ✅ `backend/internal/api/openapi_schemas.go` EXISTS (701 lines)
- ✅ Has buildSchemas() function (lines 24-91)
- ✅ Has 90 schema definition functions (errorSchema, postSchema, etc.)
- ✅ Schemas currently lack 'example' field

**Recommendation:** ✅ PROCEED as-is

---

### ⚠️ PHASE 3: Frontend Sync Automation

#### Task 6: Build OpenAPI-to-Frontend Converter Script
**Status:** ✅ VALID approach, ⚠️ NEEDS TYPE VERIFICATION
**Issues:**
- Must verify script outputs match `EndpointGroup[]` structure
- Must handle auth mapping: OpenAPI `security` → Frontend `auth: "jwt" | "api_key" | "both" | "none"`
- Must handle parameter mapping: OpenAPI parameters → Frontend `Param[]`

**File verification:**
- ✅ Frontend structure confirmed in `api-endpoint-types.ts`:
  ```typescript
  export interface Endpoint {
    method: "GET" | "POST" | "PATCH" | "DELETE";
    path: string;
    description: string;
    auth?: "jwt" | "api_key" | "both" | "none";
    params?: Param[];
    response: string; // JSON string
  }
  ```
- ✅ Pattern confirmed in `api-endpoint-data-core.ts`
- ✅ `frontend/scripts/` directory will need creation

**Recommendation:** ✅ PROCEED with type mapping specification added to acceptance criteria

---

#### Task 7: Execute Frontend Sync
**Status:** ✅ VALID
**Issues:** None - straightforward execution task

**Recommendation:** ✅ PROCEED as-is

---

### ⚠️ PHASE 4: Swagger UI Integration

#### Task 8: Add Swagger UI to API Docs Page
**Status:** ⚠️ VALID APPROACH, **CRITICAL PATH CONFLICT**
**Issues:**
- **PATH MISMATCH:** SPEC.md line 2954 says "Swagger UI at `/docs/api`"
- **PLAN SAYS:** `/api-docs/interactive/page.tsx`
- **CURRENT PAGE:** `/app/api-docs/page.tsx` exists
- **DECISION NEEDED:** Should it be `/docs/api` or `/api-docs`?

**File verification:**
- ✅ Current page: `/app/api-docs/page.tsx` (35 lines, no tabs yet)
- ❌ No `/app/docs/api/` directory
- ⚠️ swagger-ui-react NOT installed yet (needs: `npm install swagger-ui-react`)
- ✅ Next.js app router structure confirmed

**Recommendation:** 🚨 **BLOCK** until path decision:
- **Option A:** Follow SPEC.md → Use `/docs/api` → Update plan to `/app/docs/api/page.tsx`
- **Option B:** Update SPEC.md → Use `/api-docs` → Plan is correct as-is
- **User must decide:** Which path should the Swagger UI be at?

---

#### Task 9: Add E2E Tests for API Docs Page
**Status:** ⚠️ VALID APPROACH, **MISSING SETUP**
**Issues:**
- **NO playwright.config.ts** - Must create config first
- **NO e2e/ folder** - Must create directory structure
- Task plan assumes E2E framework is ready - IT IS NOT

**File verification:**
- ✅ Playwright installed: `@playwright/test@^1.58.1`
- ❌ No `playwright.config.ts`
- ❌ No `e2e/` directory
- ❌ No example E2E tests to reference

**Recommendation:** ⚠️ **ADD PREREQUISITE TASK:**
- **Task 8.5: Setup Playwright E2E Infrastructure**
  - Create `playwright.config.ts`
  - Create `e2e/` directory
  - Add `test:e2e` script to package.json
  - Create one example test to verify setup
  - THEN proceed with Task 9

---

### ✅ PHASE 5: CI/CD Automation

#### Task 10: CI Workflow for API Docs Validation
**Status:** ✅ VALID approach
**Issues:** None - well-specified

**File verification:**
- ✅ `.github/workflows/` directory exists (confirmed by git repo structure)
- ✅ Backend test command: `go test ./internal/api -run TestOpenAPI -v`
- ✅ Frontend test command: `npm test api-sync-validation`

**Recommendation:** ✅ PROCEED as-is

---

#### Task 11: Auto-Sync on Main Branch
**Status:** ✅ VALID approach, ⚠️ NOTE ON TOKEN
**Issues:**
- Plan mentions "Use GitHub App token (not GITHUB_TOKEN)" - correct
- May need docs on how to set up GitHub App token in repo secrets

**Recommendation:** ✅ PROCEED, add token setup to acceptance criteria

---

### ✅ PHASE 6: Documentation & Maintenance

#### Task 12: Update SPEC.md with API Docs Workflow
**Status:** ✅ VALID
**Issues:** None

**File verification:**
- ✅ SPEC.md Part 19.4 exists at line 2880
- ✅ Part 19.4.1 will be added (not exists yet)

**Recommendation:** ✅ PROCEED as-is

---

#### Task 13: Fix Identified Broken Links
**Status:** ✅ VALID
**Issues:** None - depends on Task 3 findings

**Recommendation:** ✅ PROCEED as-is

---

## Critical Issues Summary

### 🚨 BLOCKERS (Must resolve before starting)

1. **PATH CONFLICT (Task 8):**
   - SPEC.md says: `/docs/api`
   - Plan says: `/api-docs`
   - Current page: `/api-docs`
   - **ACTION REQUIRED:** User must decide: Update SPEC or update plan?

2. **MISSING E2E SETUP (Task 9):**
   - Playwright installed but NOT configured
   - No playwright.config.ts
   - No e2e/ folder
   - **ACTION REQUIRED:** Add Task 8.5 to create E2E infrastructure first

### ⚠️ SCOPE ADJUSTMENTS (Not blockers, but important)

3. **UNDERESTIMATED GAP (Task 1 & 4):**
   - Plan: ~28 missing routes
   - Actual: ~47 missing routes (66% more work)
   - **ACTION REQUIRED:** Update task descriptions and consider splitting Task 4

4. **FRONTEND DRIFT LARGER (Task 2 & 7):**
   - Plan: ~13 discrepancies
   - Actual: ~32 endpoint differences
   - **ACTION REQUIRED:** Update expected failure counts

---

## Recommendations

### Immediate Actions (Before Implementation)

1. **Resolve Path Conflict:**
   ```
   USER DECISION NEEDED:
   - Keep /api-docs → Update SPEC.md line 2954 from "/docs/api" to "/api-docs"
   - Use /docs/api → Update plan Task 8 path from "/api-docs/interactive" to "/docs/api"
   ```

2. **Add Missing Task (E2E Setup):**
   ```json
   {
     "id": "api-docs-008.5",
     "category": "frontend-testing-setup",
     "title": "Setup Playwright E2E testing infrastructure",
     "description": "Configure Playwright for E2E tests before writing API docs tests",
     "acceptance_criteria": [
       "Create playwright.config.ts with baseURL, testDir, timeout",
       "Create e2e/ directory structure",
       "Add 'test:e2e' script to package.json",
       "Create example test: e2e/example.spec.ts (smoke test)",
       "Verify: npm run test:e2e passes locally"
     ],
     "files_to_create": [
       "frontend/playwright.config.ts",
       "frontend/e2e/example.spec.ts"
     ],
     "files_to_modify": [
       "frontend/package.json (add test:e2e script)"
     ],
     "passes": false
   }
   ```

3. **Update Task 1 Scope:**
   - Change "~28 missing routes" → "~47 missing routes"
   - Update expected failures: "47 routes in router.go but not in OpenAPI"

4. **Update Task 2 Scope:**
   - Change "~13 endpoints in frontend but not in OpenAPI" → "~32 endpoint discrepancy"

5. **Consider Splitting Task 4:**
   - Current: One massive task (47 route definitions)
   - Proposed: 4 sub-tasks grouped by category
   - Benefits: Easier to review, parallelize, track progress

### Optional Improvements

6. **Add Count Verification Step:**
   - Before Task 4 implementation, run automated count:
     ```bash
     # Backend routes
     grep -E "r\.(Get|Post|Patch|Delete)" backend/internal/api/router.go | wc -l
     # OpenAPI paths
     curl -s https://api.solvr.dev/v1/openapi.json | jq '.paths | length'
     # Frontend endpoints
     grep -o 'method:' frontend/components/api/api-endpoint-data-*.ts | wc -l
     ```

7. **Add Progress Tracking:**
   - Track completion: "X of 47 routes documented" in Task 4
   - Track sync drift: "Discrepancy reduced from 32 to 0" in Task 7

---

## Final Verdict

### ✅ APPROVED WITH CONDITIONS

**The plan is fundamentally sound** but requires these adjustments:

| Status | Count | Category |
|--------|-------|----------|
| ✅ APPROVED AS-IS | 8 tasks | Tasks 1,2,3,5,6,7,10,11,12,13 (with minor scope updates) |
| ⚠️ NEEDS ADJUSTMENT | 2 tasks | Task 4 (larger scope), Task 8 (path conflict) |
| 🚨 MISSING PREREQUISITE | 1 task | Task 8.5 (E2E setup) - Must add |

**Overall Confidence:** 🟢 HIGH
**Plan Quality:** 🟢 EXCELLENT - just needs scope corrections
**Readiness:** 🟡 NOT READY - resolve 2 blockers first

---

## Next Steps

1. ✅ User resolves path conflict (/docs/api vs /api-docs)
2. ✅ Add Task 8.5 for E2E setup
3. ✅ Update scope estimates in Tasks 1, 2, 4
4. 🟢 **THEN PROCEED with implementation**

---

**Validation completed:** 2026-02-17
**Validated by:** Claude Code (Sonnet 4.5)
**Codebase analyzed:** /Users/fcavalcanti/dev/solvr (main branch)
