---
phase: 4
plan: "01"
title: "Email Audit History Endpoint"
status: complete
completed: 2026-03-17
---

# Phase 4 Plan 01 — SUMMARY

## What Was Done

Exposed broadcast history via `GET /admin/email/history` by making three minimal changes:

### Task 1 — Extended `EmailBroadcastRepo` interface
- Added `List(ctx context.Context) ([]models.EmailBroadcast, error)` to the interface in `admin.go`
- `db.EmailBroadcastRepository` already implemented the method — zero changes to the DB layer
- Build passed immediately

### Task 2 — Wrote failing tests (RED)
- Added `listResult []models.EmailBroadcast` and `listErr error` fields to `mockEmailBroadcastRepo`
- Added `List()` stub method to satisfy the expanded interface
- Appended `TestListBroadcasts_Unauthorized` (asserts 401 without key) and `TestListBroadcasts_ReturnsBroadcasts` (asserts 200, `broadcasts` array with 2 items, correct `broadcast_id` and all required keys) to `admin_broadcast_test.go`
- Tests failed RED with `handler.ListBroadcasts undefined` as expected

### Task 3 — Implemented `ListBroadcasts` handler (GREEN)
- Added `listBroadcastItem` response type (omits large `body_html`/`body_text` fields)
- Added `ListBroadcasts` method after `BroadcastEmail` in `admin.go`
- Both new tests pass GREEN; all 7 broadcast handler tests pass; no regressions
- `admin.go` is 589 lines (under 600 limit)

### Task 4 — Registered route
- Added `r.Get("/admin/email/history", adminHandler.ListBroadcasts)` to `router.go` immediately after the broadcast POST route
- `go build ./cmd/api` and full `go test ./...` pass with no regressions

## Files Changed

| File | Change |
|------|--------|
| `backend/internal/api/handlers/admin.go` | +1 method to interface; +`listBroadcastItem` type; +`ListBroadcasts` handler |
| `backend/internal/api/handlers/admin_broadcast_test.go` | +`listResult`/`listErr` fields; +`List()` stub; +2 test functions |
| `backend/internal/api/router.go` | +1 route line |

## Acceptance Criteria

- [x] `go build ./...` passes
- [x] `go test ./internal/api/handlers/... -run TestListBroadcasts` — both tests PASS
- [x] `go test ./...` — full suite passes, no regressions
- [x] `admin.go` < 600 lines (589)
- [x] Route registered: `GET /admin/email/history`

## Commits

1. `375cae0` — extend interface + RED tests
2. `7dfcff3` — implement handler (GREEN)
3. `d44d56a` — register route
