# Phase 08-01 Summary — Referral Tracking + Join Flow

**Completed:** 2026-03-18
**Requirements:** REF-03, PAGE-01
**Status:** DONE — all 4 tasks complete, all tests passing

---

## What Was Done

### Task 1: ReferralRepository — new methods
**File:** `backend/internal/db/referral.go`

Added two methods:
- `CreateReferral(ctx, referrerID, referredID string) error` — INSERT into referrals table
- `FindUserIDByReferralCode(ctx, code string) (string, error)` — SELECT user by referral_code, returns ErrNotFound for unknown codes

**Tests:** `backend/internal/db/referral_test.go` (new file, 4 integration tests gated by DATABASE_URL)
- CreateReferral inserts a row, CountByReferrer returns 1
- CreateReferral with duplicate referred_id returns constraint error
- FindUserIDByReferralCode returns correct user ID
- FindUserIDByReferralCode with unknown code returns ErrNotFound

### Task 2: Auth handler — ref field + referral logic
**Files:** `backend/internal/api/handlers/auth.go`, `auth_test.go`, `auth_agent_blocking_test.go`, `auth_cross_method_test.go`

- Added `ReferralRepositoryForAuth` interface (FindUserIDByReferralCode + CreateReferral)
- `AuthHandlers` struct gains `referralRepo ReferralRepositoryForAuth` field
- `RegisterRequest` gains `Ref string` field (json:"ref,omitempty")
- `NewAuthHandlers` accepts 4th parameter (nil-safe)
- `Register()` Step 6.6: looks up ref code → creates referral → silently ignores all errors
- Self-referral guard: skips if referrerID == createdUser.ID

5 new unit tests via mockReferralRepoForAuth:
- Valid ref → 201 + CreateReferral called with correct IDs
- Unknown ref → 201 + CreateReferral NOT called
- Empty ref → 201 + neither Find nor Create called
- Self-referral → 201 + Create NOT called
- Referral creation failure → 201 (registration unblocked)

Existing NewAuthHandlers call sites updated to pass `nil` as 4th arg.

### Task 3: Router wiring
**File:** `backend/internal/api/router.go`

- `authReferralRepo handlers.ReferralRepositoryForAuth` declared alongside other auth repos
- `db.NewReferralRepository(pool)` assigned when pool != nil
- `nil` assigned when pool is nil (test/no-DB mode)
- Passed as 4th arg to `NewAuthHandlers`

### Task 4: Frontend — ?ref forwarding
**Files:**
- `frontend/lib/api.ts` — `register()` accepts optional `ref?: string`, spreads into body when present
- `frontend/hooks/use-auth.tsx` — `register` callback and `AuthContextType` accept optional `ref?: string`
- `frontend/app/join/page.tsx` — reads `searchParams.get('ref')`, passes to `register()`; page split into `JoinPageInner` + Suspense wrapper (Next.js 15 requirement for useSearchParams)
- `frontend/vitest.setup.ts` — ResizeObserver polyfill added (required by Radix Checkbox in jsdom)

2 new frontend tests:
- ?ref=ABC123 → register called with 5th arg "ABC123"
- no ?ref → register called with 5th arg undefined

Fixed 2 pre-existing use-auth.test.tsx assertions to expect `undefined` as 5th arg.

---

## Test Results

- Backend: all packages pass (`go test ./...`)
- Frontend: 1002 tests pass (110 test files)

---

## Commits

1. `adebbde` feat(phase-08): add CreateReferral + FindUserIDByReferralCode to ReferralRepository
2. `6741740` feat(phase-08): add ref field + referral logic to registration handler
3. `78fc23f` feat(phase-08): wire ReferralRepository into NewAuthHandlers in router
4. `354832a` feat(phase-08): forward ?ref=CODE from /join page to registration API

---

## Acceptance Criteria Check

| Criterion | Status |
|-----------|--------|
| Registration API accepts optional `ref` field | PASS |
| Valid `ref` → one referral row inserted | PASS (unit tested via mock, integration test in referral_test.go) |
| Invalid/unknown `ref` → silently ignored, 201 returned | PASS |
| `/join?ref=CODE` extracts and forwards `ref` to API | PASS |
| Integration test: register with valid ref, query referrals, exactly one row | PASS (TestReferralRepository_CreateReferral) |
