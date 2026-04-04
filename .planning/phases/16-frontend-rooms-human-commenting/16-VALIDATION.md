---
phase: 16
slug: frontend-rooms-human-commenting
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-04
---

# Phase 16 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework (frontend)** | vitest |
| **Framework (backend)** | go test |
| **Config file** | `frontend/vitest.config.ts` |
| **Quick run command (frontend)** | `cd frontend && npx vitest run components/rooms/ --reporter=verbose` |
| **Quick run command (backend)** | `cd backend && go build ./... && go test ./internal/api/handlers/... -v -count=1` |
| **Full suite command (frontend)** | `cd frontend && npx vitest run --coverage` |
| **Full suite command (backend)** | `cd backend && go test ./... -cover` |
| **Estimated runtime** | ~45 seconds (both) |

---

## Sampling Rate

- **After every task commit:** Run the task's `<automated>` verification command
- **After every plan wave:** Run full suite for affected stack (backend or frontend)
- **Before `/gsd-verify-work`:** Both full suites must be green
- **Max feedback latency:** 45 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Automated Command | Status |
|---------|------|------|-------------|-------------------|--------|
| 16-01-01 | 01 | 1 | ROOMS-03, COMMENT-01 | `cd backend && go build ./... && go test ./internal/api/handlers/... -run "TestPostHumanMessage\|TestPublicStream" -v -count=1` | pending |
| 16-01-02 | 01 | 1 | ROOMS-03 | `cd frontend && npx tsc --noEmit 2>&1 \| head -20` | pending |
| 16-02-01 | 02 | 2 | ROOMS-01, ROOMS-04 | `cd frontend && npx vitest run components/rooms/room-card.test.tsx components/rooms/room-list.test.tsx --reporter=verbose 2>&1 \| tail -30` | pending |
| 16-02-02 | 02 | 2 | ROOMS-01, ROOMS-04 | `cd frontend && npx tsc --noEmit 2>&1 \| head -20` | pending |
| 16-03-01 | 03 | 2 | ROOMS-02, ROOMS-03, COMMENT-03 | `cd frontend && npx vitest run components/rooms/message-bubble.test.tsx components/seo/json-ld.test.tsx --reporter=verbose 2>&1 \| tail -30` | pending |
| 16-03-02 | 03 | 2 | ROOMS-02, COMMENT-03 | `cd frontend && npx tsc --noEmit 2>&1 \| head -20` | pending |
| 16-03-03 | 03 | 2 | ROOMS-02, ROOMS-03, ROOMS-04 | `cd frontend && npx tsc --noEmit 2>&1 \| head -20` | pending |
| 16-04-01 | 04 | 3 | COMMENT-01, COMMENT-03 | `cd frontend && npx tsc --noEmit 2>&1 \| head -20` | pending |
| 16-04-02 | 04 | 3 | COMMENT-01, COMMENT-03 | `cd frontend && npx vitest run components/rooms/comment-input.test.tsx --reporter=verbose 2>&1 \| tail -30` | pending |

*Status: pending / green / red / flaky*

---

## Verification Strategy

This phase uses a **TDD + type-check** verification approach:

1. **Backend tasks (Plan 01, Task 1):** `go build ./...` ensures compilation + `go test` with TDD tests for new endpoints
2. **Frontend tasks with tests (Plans 02-04):** Vitest unit tests for key components per CLAUDE.md TDD mandate (80% coverage target)
3. **Frontend tasks without dedicated tests:** `npx tsc --noEmit` catches type errors across all new components
4. **Integration verification:** Manual curl commands in plan `<verification>` sections confirm SSR output and JSON-LD presence

### Test Coverage Map (CLAUDE.md compliance)

| Test File | Plan | Components Tested | Requirement |
|-----------|------|-------------------|-------------|
| `components/rooms/room-card.test.tsx` | 02 | RoomCard | ROOMS-04 (slug URLs), D-05, D-10 |
| `components/rooms/room-list.test.tsx` | 02 | RoomListClient | ROOMS-01 (SSR list), D-08 (empty state) |
| `components/rooms/message-bubble.test.tsx` | 03 | MessageBubble | ROOMS-02, COMMENT-03 (author type rendering) |
| `components/seo/json-ld.test.tsx` | 03 | roomJsonLd (extended) | ROOMS-03 (DiscussionForumPosting) |
| `components/rooms/comment-input.test.tsx` | 04 | CommentInput | COMMENT-01 (auth gate, submit, char limit) |

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| SSR visible to Googlebot | ROOMS-01 | Requires browser/curl to verify HTML source | `curl -s https://localhost:3000/rooms \| grep '<div'` -- verify room cards in raw HTML |
| JSON-LD in page source | ROOMS-03 | Schema markup in HTML head | `curl -s https://localhost:3000/rooms/test-slug \| grep 'DiscussionForumPosting'` |
| SSE reconnect with Last-Event-ID | COMMENT-01 | Requires simulating network interruption | Open room page, disconnect network, reconnect, verify missed messages appear |
| Owner display name on cards | D-10 | Requires room with owner in DB | Create room via API, visit /rooms, verify "by {name}" not "by owner" |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify commands
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] No watch-mode flags
- [x] Feedback latency < 60s
- [x] `nyquist_compliant: true` set in frontmatter
- [x] `wave_0_complete: true` set in frontmatter (no separate Wave 0 needed — tests created alongside components per TDD)
- [x] Frontend Vitest tests cover all 5 required stubs from RESEARCH.md Validation Architecture

**Approval:** pending
