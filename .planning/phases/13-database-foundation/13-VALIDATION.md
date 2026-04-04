---
phase: 13
slug: database-foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-03
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing stdlib |
| **Config file** | none — `go test` runs directly |
| **Quick run command** | `cd backend && go test ./internal/db/ -run TestMigrations_Rooms -v` |
| **Full suite command** | `cd backend && go test ./internal/db/ -run TestMigrations -v` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd backend && go test ./internal/db/ -run TestMigrations_Rooms -v`
- **After every plan wave:** Run `cd backend && go test ./internal/db/ -run TestMigrations -v`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 13-01-01 | 01 | 1 | MERGE-01 | integration | `go test ./internal/db/ -run TestMigrations_RoomsTable` | ❌ W0 | ⬜ pending |
| 13-01-02 | 01 | 1 | MERGE-01 | integration | `go test ./internal/db/ -run TestMigrations_AgentPresenceTable` | ❌ W0 | ⬜ pending |
| 13-01-03 | 01 | 1 | MERGE-01 | integration | `go test ./internal/db/ -run TestMigrations_MessagesTable` | ❌ W0 | ⬜ pending |
| 13-01-04 | 01 | 1 | MERGE-01 | integration | `go test ./internal/db/ -run TestMigrations_AllTablesExist` | ✅ (modify) | ⬜ pending |
| 13-01-05 | 01 | 1 | COMMENT-02 | integration | `go test ./internal/db/ -run TestMigrations_MessagesAuthorTypeConstraint` | ❌ W0 | ⬜ pending |
| 13-01-06 | 01 | 1 | MERGE-01 | integration | `go test ./internal/db/ -run TestMigrations_RoomsSlugUnique` | ❌ W0 | ⬜ pending |
| 13-01-07 | 01 | 1 | MERGE-01 | integration | `go test ./internal/db/ -run TestMigrations_RoomsTagsConstraint` | ❌ W0 | ⬜ pending |
| 13-01-08 | 01 | 1 | MERGE-01 | integration | `go test ./internal/db/ -run TestMigrations_AgentPresenceUnique` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `backend/internal/db/migrations_rooms_test.go` — new file with stubs for rooms, agent_presence, messages table tests and constraint violation tests (migrations_test.go already at 834 lines, must split per 900-line limit)
- [ ] Update `TestMigrations_AllTablesExist` in `backend/internal/db/migrations_test.go` to include `rooms`, `agent_presence`, `messages`
- [ ] No framework install needed — Go stdlib testing already in use

*Existing infrastructure covers framework requirements. New test file needed only for file-size compliance.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Production migration via admin query route | MERGE-01 | Prod has no schema_migrations table, manual apply | Run each migration SQL via `POST /admin/query` and verify tables exist |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
