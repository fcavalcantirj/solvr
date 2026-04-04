---
phase: 16
slug: frontend-rooms-human-commenting
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-04
---

# Phase 16 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | vitest |
| **Config file** | `frontend/vitest.config.ts` |
| **Quick run command** | `cd frontend && npx vitest run --reporter=verbose` |
| **Full suite command** | `cd frontend && npx vitest run --coverage` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd frontend && npx vitest run --reporter=verbose`
- **After every plan wave:** Run `cd frontend && npx vitest run --coverage`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 16-01-01 | 01 | 1 | ROOMS-01 | — | N/A | unit | `cd frontend && npx vitest run --reporter=verbose rooms` | ❌ W0 | ⬜ pending |
| 16-01-02 | 01 | 1 | ROOMS-02 | — | N/A | unit | `cd frontend && npx vitest run --reporter=verbose rooms` | ❌ W0 | ⬜ pending |
| 16-02-01 | 02 | 1 | ROOMS-03 | — | N/A | unit | `cd frontend && npx vitest run --reporter=verbose rooms` | ❌ W0 | ⬜ pending |
| 16-02-02 | 02 | 1 | ROOMS-04 | — | N/A | unit | `cd frontend && npx vitest run --reporter=verbose rooms` | ❌ W0 | ⬜ pending |
| 16-03-01 | 03 | 2 | COMMENT-01 | — | Auth required for POST | integration | `cd frontend && npx vitest run --reporter=verbose comment` | ❌ W0 | ⬜ pending |
| 16-03-02 | 03 | 2 | COMMENT-03 | — | N/A | unit | `cd frontend && npx vitest run --reporter=verbose comment` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `frontend/__tests__/rooms/` — test directory for room components
- [ ] `frontend/__tests__/rooms/room-list.test.tsx` — stubs for ROOMS-01, ROOMS-02
- [ ] `frontend/__tests__/rooms/room-detail.test.tsx` — stubs for ROOMS-03, ROOMS-04
- [ ] `frontend/__tests__/rooms/comment-input.test.tsx` — stubs for COMMENT-01, COMMENT-03

*Existing vitest infrastructure covers framework needs — only test file stubs needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| SSR visible to Googlebot | ROOMS-01 | Requires browser/curl to verify HTML source | `curl -s https://localhost:3000/rooms \| grep '<div'` — verify room cards in raw HTML |
| JSON-LD in page source | ROOMS-03 | Schema markup in HTML head | `curl -s https://localhost:3000/rooms/test-slug \| grep 'DiscussionForumPosting'` |
| SSE reconnect with Last-Event-ID | ROOMS-04 | Requires simulating network interruption | Open room page, disconnect network, reconnect, verify missed messages appear |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
