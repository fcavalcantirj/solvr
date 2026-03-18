---
plan: "02-03"
status: complete
started: 2026-03-17
completed: 2026-03-17
---

# Plan 02-03 Summary: ListActiveEmails + Service Wiring

## What Was Built

Added `UserRepository.ListActiveEmails()` to return only non-deleted users' email info, following TDD (RED→GREEN). Wired the `ResendClient` into the API server via `router.go` with conditional initialization on `RESEND_API_KEY`, and placed the `EmailSender` interface + `SetEmailSender` setter in `admin.go` following the existing `TranslationJobRunner` pattern.

## Key Files

### Modified
- `backend/internal/db/users.go` — Added `ListActiveEmails(ctx) ([]models.EmailRecipient, error)` method (569 lines)
- `backend/internal/db/users_test.go` — Added `TestUserRepository_ListActiveEmails` integration test
- `backend/internal/api/handlers/admin.go` — Added `EmailSender` interface, `emailSender` field, `SetEmailSender` setter (396 lines)
- `backend/internal/api/router.go` — Added Resend wiring block after GROQ wiring (pre-existing 1070+ lines, acknowledged)

## Deviations

None. Followed the plan exactly. Task 1 and 2 committed together (test + implementation as one atomic unit per TDD flow). Task 3 committed separately (wiring). Both commits are clean.

## Self-Check

PASSED

- `go vet ./internal/db/...` exits 0
- `go vet ./internal/api/...` exits 0
- `go build ./cmd/api` exits 0
- `go test ./... -count=1` — all 15 packages pass
- All new files under 900 lines (users.go: 569, admin.go: 396, email_broadcast.go: 113, resend.go: 61)
- router.go pre-existing violation (1078 lines after +8) — acknowledged, out of scope

## Test Results

```
=== DB integration tests (no DATABASE_URL in env — skip, compile OK) ===
--- SKIP: TestEmailBroadcastRepository_CreateLog (0.00s)
--- SKIP: TestEmailBroadcastRepository_UpdateStatusAndCounts (0.00s)
--- SKIP: TestEmailBroadcastRepository_List (0.00s)
--- SKIP: TestEmailBroadcastRepository_List_Empty (0.00s)
--- SKIP: TestUserRepository_ListActiveEmails (0.00s)
PASS
ok  	github.com/fcavalcantirj/solvr/internal/db	0.467s

=== ResendClient unit tests (httptest mock — always run) ===
--- PASS: TestResendClient_Send_Success (0.00s)
--- PASS: TestResendClient_Send_APIError (0.00s)
--- PASS: TestResendClient_Send_EmptyTextBody (0.00s)
--- PASS: TestResendClient_Send_CustomFromEmail (0.00s)
PASS
ok  	github.com/fcavalcantirj/solvr/internal/services	0.322s

=== Full test suite ===
ok  github.com/fcavalcantirj/solvr/cmd/api           0.502s
ok  github.com/fcavalcantirj/solvr/cmd/backfill-embeddings  0.866s
ok  github.com/fcavalcantirj/solvr/cmd/moderate-existing    18.230s
ok  github.com/fcavalcantirj/solvr/internal/api      1.426s
ok  github.com/fcavalcantirj/solvr/internal/api/handlers    7.922s
ok  github.com/fcavalcantirj/solvr/internal/api/middleware  0.440s
ok  github.com/fcavalcantirj/solvr/internal/api/response    1.149s
ok  github.com/fcavalcantirj/solvr/internal/auth     4.144s
ok  github.com/fcavalcantirj/solvr/internal/config   1.827s
ok  github.com/fcavalcantirj/solvr/internal/db       2.417s
ok  github.com/fcavalcantirj/solvr/internal/jobs     2.377s
ok  github.com/fcavalcantirj/solvr/internal/models   2.214s
ok  github.com/fcavalcantirj/solvr/internal/reputation     1.914s
ok  github.com/fcavalcantirj/solvr/internal/services 7.626s
```
