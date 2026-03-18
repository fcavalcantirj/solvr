---
phase: 2
slug: backend-foundation
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-03-17
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — Go test infrastructure already exists |
| **Quick run command** | `cd backend && go test ./internal/db/... ./internal/services/...` |
| **Full suite command** | `cd backend && go test ./... -cover` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd backend && go test ./internal/db/... ./internal/services/...`
- **After every plan wave:** Run `cd backend && go test ./... -cover`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 02-01-01 | 01 | 1 | AUDIT-01 | integration | `go test ./internal/db/... -run TestEmailBroadcast` | ❌ W0 | ⬜ pending |
| 02-03-01 | 03 | 2 | EMAIL-05 | integration | `go test ./internal/db/... -run TestUserRepository_ListActiveEmails` | ❌ W0 | ⬜ pending |
| 02-02-01 | 02 | 1 | INFRA-03 | unit | `go test ./internal/services/... -run TestResendClient` | ❌ W0 | ⬜ pending |
| 02-03-02 | 03 | 2 | INFRA-03 | build | `cd backend && go build ./cmd/api` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `backend/internal/db/email_broadcast_test.go` — stubs for AUDIT-01 (CreateLog, UpdateStatusAndCounts, List)
- [ ] `backend/internal/db/users_test.go` — add ListActiveEmails test (file exists, new test function)
- [ ] `backend/internal/services/resend_test.go` — stubs for INFRA-03 (Send with mock HTTP)

*Existing Go test infrastructure covers all framework requirements.*

> **Test isolation note:** Integration tests use pre-run cleanup (`DELETE FROM ... WHERE subject LIKE 'test_%'`) plus deferred cleanup. This is sufficient for isolation — tests create uniquely-prefixed data and clean up after themselves. No additional test isolation infrastructure is needed.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Migration runs up/down | AUDIT-01 | Requires local PostgreSQL | `migrate -path migrations -database "$DATABASE_URL" up` then `down 1` |
| API starts with RESEND_API_KEY | INFRA-03 | Requires env var + running server | Set RESEND_API_KEY, run `go run ./cmd/api`, check no startup errors |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved (post-revision)
