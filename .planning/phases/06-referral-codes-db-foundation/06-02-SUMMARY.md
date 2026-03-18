---
phase: 6
plan: "02"
title: "Referral API Endpoint"
status: complete
completed: 2026-03-17
---

# Summary: Plan 06-02 — Referral API Endpoint

## What Was Done

Implemented `GET /v1/users/me/referral` — an authenticated endpoint returning the caller's referral code and the number of users they have referred.

### Task 1 — ReferralRepository (`backend/internal/db/referral.go`)
- `NewReferralRepository(pool)` constructor
- `GetReferralCode(ctx, userID)` — SELECT referral_code FROM users WHERE id = $1 AND deleted_at IS NULL
- `CountByReferrer(ctx, referrerID)` — SELECT COUNT(*) FROM referrals WHERE referrer_id = $1
- `go vet ./internal/db/...` passes

### Task 2 — TDD RED (`backend/internal/api/handlers/referral_test.go`)
- 5 tests written before implementation:
  - `TestGetMyReferral_Unauthenticated` — expects 401 when no JWT context
  - `TestGetMyReferral_Success_ZeroReferrals` — expects 200 + code + count=0
  - `TestGetMyReferral_Success_WithReferrals` — expects 200 + code + count=5
  - `TestGetMyReferral_CodeLookupError` — expects 500 on DB error for code
  - `TestGetMyReferral_CountError` — expects 500 on DB error for count
- Tests confirmed RED (undefined: NewReferralHandler)

### Task 3 — TDD GREEN (`backend/internal/api/handlers/referral.go`)
- `ReferralRepositoryInterface` — GetReferralCode + CountByReferrer
- `ReferralResponse` struct with `json:"referral_code"` and `json:"referral_count"` tags
- `ReferralHandler` struct + `NewReferralHandler` constructor
- `GetMyReferral` — uses `auth.ClaimsFromContext`, returns 401/500/200 as required
- All 5 tests pass (GREEN)
- 86 lines (< 100 limit)

### Task 4 — Route wiring (`backend/internal/api/router.go`)
- Added `referralRepo := db.NewReferralRepository(pool)` after other repo instantiations (line 228)
- Added route inside authenticated group after bookmarks block:
  ```
  referralHandler := handlers.NewReferralHandler(referralRepo)
  r.Get("/users/me/referral", referralHandler.GetMyReferral)
  ```
- `go build ./cmd/api` exits 0

### Task 5 — Full verification
- All 5 referral handler tests pass
- All 4 referral codegen tests pass (from 06-01)
- Full suite `go test ./... -count=1 -short` — 16 packages, all OK, zero regressions
- File sizes: referral.go=86, referral_test.go=135, db/referral.go=43 (all within limits)
- router.go=1095 (pre-existing violation, only 6 lines added)

## Files Changed

| File | Change |
|------|--------|
| `backend/internal/db/referral.go` | NEW (43 lines) |
| `backend/internal/api/handlers/referral_test.go` | NEW (135 lines, 5 tests) |
| `backend/internal/api/handlers/referral.go` | NEW (86 lines) |
| `backend/internal/api/router.go` | +6 lines (repo + route wiring) |

## Response Shape

```json
GET /v1/users/me/referral  →  200
{"referral_code": "ABCD1234", "referral_count": 5}

GET /v1/users/me/referral (unauthenticated)  →  401
{"error": {"code": "UNAUTHORIZED", "message": "authentication required"}}

GET /v1/users/me/referral (DB error)  →  500
{"error": {"code": "INTERNAL_ERROR", "message": "failed to get referral code"}}
```

## Commits

1. `feat(phase-06): add ReferralRepository with CountByReferrer and GetReferralCode`
2. `feat(phase-06): TDD RED - write 5 handler tests for GET /v1/users/me/referral`
3. `feat(phase-06): TDD GREEN - implement ReferralHandler (5 tests pass)`
4. `feat(phase-06): wire GET /v1/users/me/referral route in router.go`

## Requirement Status

- REF-04: ✅ PASSES — GET /v1/users/me/referral implemented, tested, wired
