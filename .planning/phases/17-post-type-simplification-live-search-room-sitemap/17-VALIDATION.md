---
phase: 17
slug: post-type-simplification-live-search-room-sitemap
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-04
---

# Phase 17 -- Validation Strategy

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
| 17-01-01 | 01 | 1 | SIMPLIFY-01, SIMPLIFY-02, SIMPLIFY-03 | -- | N/A | integration | `cd frontend && npx vitest run --reporter=verbose 2>&1 \| tail -30` | tdd=true | green |
| 17-02-01 | 02 | 1 | SITEMAP-01, SITEMAP-02 | -- | N/A | unit | `cd backend && go test ./internal/api/handlers/... -run "Sitemap" -v -count=1 2>&1 \| tail -40` | tdd=true | green |
| 17-02-02 | 02 | 1 | SITEMAP-03 | -- | N/A | integration | `cd frontend && test -f app/sitemap-rooms.xml/route.ts && echo "rooms route exists" && ! test -f app/sitemap-questions.xml/route.ts && echo "questions route deleted" && grep -q "sitemap-rooms.xml" app/sitemap.xml/route.ts && echo "rooms in index"` | auto | green |
| 17-03-01 | 03 | 1 | SEARCH-01, SEARCH-02 | -- | N/A | unit | `cd backend && go test ./internal/db/... -run "TestDataAnalytics\|TestWindowTo" -v -count=1 2>&1 \| tail -30` | tdd=true | green |
| 17-03-02 | 03 | 1 | SEARCH-02, SEARCH-03 | T-17-09 | Cache 60s TTL | unit | `cd backend && go test ./internal/api/handlers/... -run "TestDataHandler" -v -count=1 2>&1 \| tail -40` | tdd=true | green |
| 17-04-01 | 04 | 2 | SEARCH-04 | T-17-11 | XSS via JSX auto-escape | unit | `cd frontend && npx vitest run app/data/ --reporter=verbose 2>&1 \| tail -30 && npx next build 2>&1 \| tail -20` | tdd=true | green |
| 17-04-02 | 04 | 2 | -- | -- | N/A | manual | `cd frontend && npx vitest run --reporter=verbose 2>&1 \| tail -10 && cd backend && go test ./... -count=1 2>&1 \| tail -10` | checkpoint | green |

*Status: pending -- ready -- green -- red -- flaky*

---

## Wave 0 Requirements

All tasks now have `tdd="true"` or are checkpoints with automated suite runs. No separate Wave 0 test scaffolding needed -- TDD tasks create their own tests as part of the RED phase.

- [x] Backend data handler tests created by Plan 03 Task 1 and Task 2 (tdd=true)
- [x] Backend sitemap room tests created by Plan 02 Task 1 (tdd=true)
- [x] Frontend simplification tests created by Plan 01 Task 1 (tdd=true)
- [x] Frontend /data page tests created by Plan 04 Task 1 (tdd=true, page.test.tsx)

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

- [x] All tasks have `<automated>` verify or are TDD tasks creating tests inline
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covered by tdd=true tasks creating own tests
- [x] No watch-mode flags
- [x] Feedback latency < 50s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** ready
