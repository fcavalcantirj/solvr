---
phase: 6
plan: "01"
title: "Migration + Code Generation + Auto-Assign"
status: complete
completed_at: 2026-03-17
---

# Plan 06-01 Summary: Migration + Code Generation + Auto-Assign

## What Was Done

Implemented the database foundation for referral codes across 5 TDD tasks:

### Task 1 — Migration 000070 (committed atomically)
- `backend/migrations/000070_add_referral_codes.up.sql`: adds `referral_code VARCHAR(8)` to `users`, backfills all existing users with unique 8-char alphanumeric codes via a PL/pgSQL `DO $$` block, then adds `NOT NULL` + `UNIQUE` constraints
- `backend/migrations/000070_add_referral_codes.down.sql`: drops `referrals` table and `referral_code` column
- Created `referrals` tracking table with `referrer_id`, `referred_id` (unique FK), and `idx_referrals_referrer_id` index

### Task 2 — TDD RED: codegen_test.go (committed atomically)
- Created `backend/internal/referral/codegen_test.go` with 4 tests
- Verified RED state: `go test ./internal/referral/...` failed with `undefined: GenerateCode`

### Task 3 — TDD GREEN: codegen.go (committed atomically)
- Created `backend/internal/referral/codegen.go` — `GenerateCode()` uses `crypto/rand.Int` for cryptographically secure 8-char A-Z0-9 codes
- All 4 tests pass: Length, Charset, Uniqueness, UsesCryptoRand

### Task 4 — User model + scan functions (committed atomically)
- Added `ReferralCode string` field to `models.User` struct
- Updated `UserRepository.Create`: auto-generates code via `referral.GenerateCode()` if not provided; INSERT includes `referral_code` ($10); RETURNING includes it (13 cols)
- Updated `scanUser` to scan 13 columns (added `referralCode sql.NullString`)
- Updated `FindByID`, `FindByEmail`, `FindByUsername` SELECT lists
- Updated `FindByAuthProvider` SELECT list (`u.referral_code`)
- Updated `Update` RETURNING clause
- Updated `ListDeleted` SELECT + manual scan

### Task 5 — Full verification
- `go vet ./...` — clean
- `go build ./cmd/api` — passes
- `go test ./internal/referral/... -count=1 -v` — 4/4 PASS
- `go test ./... -count=1 -short` — all 16 packages pass, no regressions

## Files Changed

| File | Change |
|------|--------|
| `backend/migrations/000070_add_referral_codes.up.sql` | NEW |
| `backend/migrations/000070_add_referral_codes.down.sql` | NEW |
| `backend/internal/referral/codegen.go` | NEW — GenerateCode() |
| `backend/internal/referral/codegen_test.go` | NEW — 4 tests |
| `backend/internal/models/user.go` | Add ReferralCode field |
| `backend/internal/db/users.go` | Add referral_code to all queries + scans |

## File Sizes (all within 900-line limit)

| File | Lines |
|------|-------|
| `codegen.go` | 28 |
| `codegen_test.go` | 56 |
| `users.go` | 591 |
| `user.go` | 127 |

## Commits (4 atomic)

1. `feat(phase-06): add migration 000070 for referral_code column and referrals table`
2. `test(phase-06): add TDD RED tests for referral code generation`
3. `feat(phase-06): implement GenerateCode using crypto/rand (TDD GREEN)`
4. `feat(phase-06): add ReferralCode to User model + update all scan/query functions`

## Requirements Addressed

- **REF-01**: `referral_code` column added to `users` with backfill and UNIQUE constraint
- **REF-02**: `referrals` table created with `referrer_id`, `referred_id`, unique constraint; `GenerateCode()` auto-assigns on `Create`
