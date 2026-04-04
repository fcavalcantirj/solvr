---
phase: 17
slug: post-type-simplification-live-search-room-sitemap
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-04
---

# Phase 17 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (backend) / vitest (frontend) |
| **Config file** | backend: go.test flags / frontend: vitest.config.ts |
| **Quick run command** | `cd backend && go test ./internal/api/... -count=1 -run "Data\|Sitemap\|Question"` / `cd frontend && npx vitest run --reporter=verbose src/` |
| **Full suite command** | `cd backend && go test ./... -count=1` / `cd frontend && npx vitest run` |
| **Estimated runtime** | ~30 seconds (backend) / ~20 seconds (frontend) |

---

## Sampling Rate

- **After every task commit:** Run quick run command for affected area
- **After every plan wave:** Run full suite command
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 50 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 17-01-01 | 01 | 1 | SIMPLIFY-01 | — | N/A | integration | `cd frontend && npx vitest run src/ -t "post type"` | ❌ W0 | ⬜ pending |
| 17-01-02 | 01 | 1 | SIMPLIFY-02 | — | N/A | integration | `cd frontend && npx vitest run src/ -t "navigation"` | ❌ W0 | ⬜ pending |
| 17-01-03 | 01 | 1 | SIMPLIFY-03 | — | N/A | integration | `cd frontend && npx vitest run src/ -t "sitemap"` | ❌ W0 | ⬜ pending |
| 17-02-01 | 02 | 1 | SEARCH-01 | — | N/A | unit | `cd backend && go test ./internal/api/... -run "Data"` | ❌ W0 | ⬜ pending |
| 17-02-02 | 02 | 1 | SEARCH-02 | — | N/A | unit | `cd backend && go test ./internal/api/... -run "Data"` | ❌ W0 | ⬜ pending |
| 17-02-03 | 02 | 2 | SEARCH-03 | — | N/A | integration | `cd frontend && npx vitest run src/ -t "data page"` | ❌ W0 | ⬜ pending |
| 17-02-04 | 02 | 2 | SEARCH-04 | — | N/A | integration | `cd frontend && npx vitest run src/ -t "auto refresh"` | ❌ W0 | ⬜ pending |
| 17-03-01 | 03 | 1 | SITEMAP-01 | — | N/A | unit | `cd backend && go test ./internal/api/... -run "Sitemap"` | ❌ W0 | ⬜ pending |
| 17-03-02 | 03 | 1 | SITEMAP-02 | — | N/A | integration | `curl -s localhost:8080/v1/sitemap/urls?type=rooms` | ❌ W0 | ⬜ pending |
| 17-03-03 | 03 | 1 | SITEMAP-03 | — | N/A | integration | `cd frontend && npx vitest run src/ -t "sitemap rooms"` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Backend data handler test stubs for SEARCH-01, SEARCH-02
- [ ] Backend sitemap room test stubs for SITEMAP-01, SITEMAP-02
- [ ] Frontend test stubs for SIMPLIFY-01, SIMPLIFY-02, SIMPLIFY-03
- [ ] Frontend test stubs for SEARCH-03, SEARCH-04
- [ ] Frontend test stubs for SITEMAP-03

*Existing test infrastructure (go test, vitest) covers framework needs.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Existing question pages return HTTP 200 | SIMPLIFY-02 | Requires live server with actual question data | Visit any of the 9 existing question URLs, verify 200 response |
| /data page auto-refresh visual | SEARCH-04 | 60s polling requires visual observation | Open /data, wait 60s, verify data updates without page reload |
| sitemap-rooms.xml in Google Search Console | SITEMAP-03 | External service validation | Submit sitemap index to GSC, verify rooms sitemap accepted |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 50s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
