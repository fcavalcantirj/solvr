---
phase: 12
name: api-docs-accuracy-audit
status: gaps_found
verified_at: 2026-03-19
verifier: gsd-verifier
score: 4/5 must-haves verified
---

# Phase 12 Verification: API Docs Accuracy Audit

## Summary

4 plans completed, 4 data files rewritten. All 4 files compile with zero TypeScript errors. Spot-checked 5 endpoints across all 4 data files; 4 of 5 are accurate, 1 gap found.

## What Was Verified

### 1. POST /auth/register (core.ts vs auth.go) — PASS

Docs document all 5 fields of `RegisterRequest` struct (email, password, username, display_name, ref). Response shape matches `RegisterResponse` struct exactly (access_token, refresh_token, user with id/username/display_name/email/role). Auth: none. Status: 201. Accurate.

### 2. POST /problems (content.ts vs problems.go) — PASS

Docs document all 5 fields of `CreateProblemRequest` struct (title, description, tags, success_criteria, weight). Response shape `{"data": createdPost}` matches handler. Auth: both. Status: 201. Accurate.

### 3. GET /me/auth-methods (user.ts vs me.go) — PASS

Docs show `{"data": {"auth_methods": [...]}}`. Handler calls `writeMeJSON(w, 200, response)` where `writeMeJSON` wraps in `{"data": data}` and response is `AuthMethodsListResponse{AuthMethods: [...]}` with json tag `auth_methods`. Full shape matches. Fields provider, linked_at, last_used_at all match `AuthMethodResponse` struct. Auth: jwt. Accurate.

### 4. GET /agents/{id}/storage (ipfs.ts vs storage.go) — PASS

Docs show `{"data": {"used": ..., "quota": ..., "percentage": ...}}`. Handler encodes `map[string]interface{}{"data": resp}` where `StorageResponse` has fields `used`, `quota`, `percentage`. Auth: both (agent API key or claiming human JWT). Accurate.

### 5. POST /ideas/{id}/evolve (content.ts vs ideas.go) — GAP FOUND

**Discrepancy:** The docs show the response wrapped in a `"data"` key:
```json
{
  "data": {
    "message": "idea evolution linked",
    "idea_id": "...",
    "evolved_post_id": "..."
  }
}
```
The actual handler (`ideas.go` line 511) calls `writeIdeasJSON(w, http.StatusOK, map[string]interface{}{"message": ..., "idea_id": ..., "evolved_post_id": ...})`. The `writeIdeasJSON` helper does NOT add a `"data"` wrapper — it encodes the map directly. The actual response is:
```json
{
  "message": "idea evolution linked",
  "idea_id": "...",
  "evolved_post_id": "..."
}
```
The `"data"` wrapper in the docs is incorrect.

### TypeScript Compilation — PASS

`npx tsc --noEmit` produces zero errors in all 4 api-endpoint-data-*.ts files:
- `api-endpoint-data-core.ts` (850 lines)
- `api-endpoint-data-content.ts` (477 lines)
- `api-endpoint-data-user.ts` (447 lines)
- `api-endpoint-data-ipfs.ts`

Other TS errors exist project-wide but all are in test files and predate this phase.

## Gaps Found

| # | File | Endpoint | Issue | Severity |
|---|------|----------|-------|----------|
| 1 | api-endpoint-data-content.ts | POST /ideas/{id}/evolve | Response docs show `{"data": {...}}` wrapper but handler returns bare `{"message": ..., "idea_id": ..., "evolved_post_id": ...}` | Minor |

## Recommendation

The gap is minor and doc-only. It does not affect runtime behavior. The fix is to remove the `"data"` wrapper from the evolve endpoint's response example in `api-endpoint-data-content.ts`. This can be fixed in a follow-up or as a quick patch — it does not block phase completion.

Overall, the phase achieved its goal: all 25+ missing endpoints were added across 4 files, all known accuracy discrepancies were fixed, and the docs are now agent-first. The one remaining gap is a response wrapper inconsistency on a single endpoint.
