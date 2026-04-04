---
phase: 14
slug: backend-service-merge
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-04
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing standard library + httptest |
| **Config file** | none — uses `go test ./...` |
| **Quick run command** | `cd backend && go test ./internal/hub/... ./internal/api/... -run Room -v` |
| **Full suite command** | `cd backend && go test ./... -cover` |
| **Estimated runtime** | ~45 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd backend && go test ./internal/hub/... ./internal/db/... -count=1`
- **After every plan wave:** Run `cd backend && go test ./... -cover`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 45 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 14-01-01 | 01 | 1 | MERGE-02 | unit/integration | `go test ./internal/db/... -run Room -v` | ❌ W0 | ⬜ pending |
| 14-02-01 | 02 | 1 | MERGE-05 | integration | `go test ./internal/hub/... -v` | ❌ W0 | ⬜ pending |
| 14-03-01 | 03 | 2 | MERGE-03 | integration | `go test ./internal/api/... -run A2A -v` | ❌ W0 | ⬜ pending |
| 14-03-02 | 03 | 2 | MERGE-04 | integration | `go test ./internal/api/... -run RoomsREST -v` | ❌ W0 | ⬜ pending |
| 14-03-03 | 03 | 2 | MERGE-06 | integration | `go test ./internal/api/... -run SSE -v -timeout 60s` | ❌ W0 | ⬜ pending |
| 14-04-01 | 04 | 3 | MERGE-07 | integration | `go test ./internal/jobs/... -run Reaper -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `backend/internal/db/rooms_test.go` — covers MERGE-02 repository methods
- [ ] `backend/internal/db/agent_presence_test.go` — covers MERGE-07 reaper query
- [ ] `backend/internal/db/messages_test.go` — covers message pagination
- [ ] `backend/internal/hub/` (port from Quorum's `hub_test.go`) — covers MERGE-05
- [ ] `backend/internal/api/router_rooms_test.go` — covers MERGE-03, MERGE-04, MERGE-06
- [ ] `backend/internal/jobs/presence_reaper_test.go` — covers MERGE-07
- [ ] `backend/go.mod` additions: `go get github.com/a2aproject/a2a-go@v0.3.12 && go get github.com/go-chi/httprate@v0.15.0`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| SSE stays connected >15s through nginx | MERGE-06 | Requires deployed server with reverse proxy | `curl -N https://api.solvr.dev/r/{slug}/stream` and time |
| Clean SIGTERM shutdown | MERGE-05 | Requires process signal in production-like env | `kill -TERM <pid>` and verify logs show clean drain |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 45s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
