---
phase: 15
slug: data-migration
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-04
---

# Phase 15 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) |
| **Config file** | none — standard `go test` |
| **Quick run command** | `cd backend && go test ./cmd/migrate-quorum/... -v` |
| **Full suite command** | `cd backend && go test ./cmd/migrate-quorum/... -v -count=1` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd backend && go test ./cmd/migrate-quorum/... -v`
- **After every plan wave:** Run `cd backend && go test ./cmd/migrate-quorum/... -v -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 15-01-01 | 01 | 1 | DATA-01 | unit (mock) | `go test ./cmd/migrate-quorum/... -run TestMigration_RoomOwnerMapping` | no — Wave 0 | ⬜ pending |
| 15-01-02 | 01 | 1 | DATA-01 | unit (mock) | `go test ./cmd/migrate-quorum/... -run TestMigration_MessageContentType` | no — Wave 0 | ⬜ pending |
| 15-01-03 | 01 | 1 | DATA-01 | unit (mock) | `go test ./cmd/migrate-quorum/... -run TestMigration_SkippedRooms` | no — Wave 0 | ⬜ pending |
| 15-01-04 | 01 | 1 | DATA-01 | unit (mock) | `go test ./cmd/migrate-quorum/... -run TestMigration_Idempotent` | no — Wave 0 | ⬜ pending |
| 15-01-05 | 01 | 1 | DATA-02 | unit (mock) | `go test ./cmd/migrate-quorum/... -run TestMigration_AgentPresenceSkipped` | no — Wave 0 | ⬜ pending |
| 15-01-06 | 01 | 1 | DATA-03 | unit (mock) | `go test ./cmd/migrate-quorum/... -run TestMigration_SequenceNumbers` | no — Wave 0 | ⬜ pending |
| 15-01-07 | 01 | 1 | DATA-01 | unit (mock) | `go test ./cmd/migrate-quorum/... -run TestMigration_TxRollback` | no — Wave 0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `backend/cmd/migrate-quorum/main.go` — the migration script itself
- [ ] `backend/cmd/migrate-quorum/migrate_test.go` — unit tests with mock migrationDB interface

*Both created as part of the implementation — no pre-existing test infrastructure needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Dry-run against prod Quorum DB shows expected counts (5 rooms, 215 msgs) | DATA-01 | Requires prod DB access via SSH tunnel | Run `QUORUM_DB_URL="..." go run ./cmd/migrate-quorum/ --dry-run` and verify output |
| GET /v1/rooms shows migrated rooms after cutover | DATA-01 | Requires prod API access | `curl https://api.solvr.dev/v1/rooms` after migration |
| Quorum process and Docker container stopped | DATA-02 | Operational step | `docker ps \| grep quorum` returns empty |
| Room metadata enrichment by Claude Code post-migration | DATA-01 | LLM step outside script | Claude Code reads first 25 msgs/room, updates via admin query |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
