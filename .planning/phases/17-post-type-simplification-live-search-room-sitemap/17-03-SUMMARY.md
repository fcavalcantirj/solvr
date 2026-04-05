---
phase: 17-post-type-simplification-live-search-room-sitemap
plan: "03"
subsystem: backend-data-analytics
tags: [backend, analytics, public-api, caching, tdd]
dependency_graph:
  requires: []
  provides: [GET /v1/data/trending, GET /v1/data/breakdown, GET /v1/data/categories]
  affects: [backend/internal/api/router.go, backend/internal/models/search_query.go]
tech_stack:
  added: []
  patterns: [sync.Map TTL cache, window whitelist validation, bot exclusion list, interface-driven handler testing]
key_files:
  created:
    - backend/internal/db/data_analytics.go
    - backend/internal/db/data_analytics_test.go
    - backend/internal/api/handlers/data_handler.go
    - backend/internal/api/handlers/data_handler_test.go
  modified:
    - backend/internal/models/search_query.go
    - backend/internal/api/router.go
decisions:
  - "Use fmt.Sprintf interval interpolation only from windowToInterval whitelist output, never raw user input — safe SQL construction without parameterization for INTERVAL clause"
  - "sync.Map chosen over mutex+map for cache: zero-alloc reads for a hot public endpoint"
  - "Bot exclusion uses hardcoded string interpolation (safe: list is compile-time constant, not user input)"
  - "Strip avg_results/avg_duration_ms from public trending response per T-17-07 information disclosure accept"
metrics:
  duration: "~4 min"
  completed_date: "2026-04-05"
  tasks_completed: 2
  files_created: 4
  files_modified: 2
---

# Phase 17 Plan 03: Data Analytics Endpoints Summary

**One-liner:** Three public search analytics endpoints (/v1/data/trending, breakdown, categories) with 60s sync.Map cache, window whitelist validation, and bot exclusion via hardcoded ID list.

## What Was Built

### Task 1: DataAnalyticsRepository (commit e0fd7cd)

Created `backend/internal/db/data_analytics.go` with three SQL query methods:

- `GetTrendingPublic(ctx, window, limit, excludeBots)` — returns top queries ordered by count DESC
- `GetBreakdown(ctx, window, excludeBots)` — returns total_searches, zero_result_rate, by_searcher_type
- `GetCategories(ctx, window, excludeBots)` — returns type_filter counts with NULL coalesced to "unfiltered"

Key implementation details:
- `windowToInterval()` — whitelist mapper returning safe SQL interval strings ("1 hour", "24 hours", "7 days")
- `buildBotExclusionClause()` — generates SQL fragment from `KnownBotSearcherIDs` (compile-time constant, not user input)
- `KnownBotSearcherIDs = []string{"e48fb1b2", "agent_NaoParis"}` — derived from production analytics

Added `DataBreakdown` and `DataCategory` model structs to `backend/internal/models/search_query.go`.

### Task 2: DataHandler + Router Wiring (commit e74801e)

Created `backend/internal/api/handlers/data_handler.go`:

- `DataAnalyticsReaderInterface` — testable interface with three methods
- `DataHandler.getCached()` — sync.Map TTL cache helper (60s TTL, T-17-06 DoS mitigation)
- `parseWindowParam()` — validates window param against `validWindows` map, defaults to "24h"
- `parseIncludeBots()` — reads include_bots=true param
- Three HTTP handlers: `GetTrending`, `GetBreakdown`, `GetCategories`
- `publicTrending` struct strips avg_results/avg_duration_ms (T-17-07)

Wired into `backend/internal/api/router.go` after the sitemap block:
- `GET /v1/data/trending`
- `GET /v1/data/breakdown`
- `GET /v1/data/categories`

## Test Coverage

**Repository tests** (data_analytics_test.go) — 6 tests:
- `TestWindowToInterval_ValidWindows` — 3 subtests (1h, 24h, 7d)
- `TestWindowToInterval_InvalidWindow` — 9 subtests (injection attempt included)
- `TestNewDataAnalyticsRepository_NotNil`
- `TestKnownBotSearcherIDs_NotEmpty`
- `TestKnownBotSearcherIDs_ContainsExpectedBots`
- `TestBuildBotExclusionClause`

**Handler tests** (data_handler_test.go) — 9 tests:
- `TestDataHandler_Trending_Success`
- `TestDataHandler_Trending_DefaultWindow`
- `TestDataHandler_Trending_InvalidWindow`
- `TestDataHandler_Breakdown_Success`
- `TestDataHandler_Categories_Success`
- `TestDataHandler_Trending_NoAuth`
- `TestDataHandler_Trending_CacheHit`
- `TestDataHandler_Trending_IncludeBots`
- `TestDataHandler_Trending_StripsAvgFields`

## Deviations from Plan

None — plan executed exactly as written.

## Threat Mitigations Applied

| Threat ID | Mitigation |
|-----------|------------|
| T-17-05 | windowToInterval whitelist; invalid window returns 400 |
| T-17-06 | sync.Map 60s TTL cache in DataHandler.getCached() |
| T-17-07 | publicTrending struct strips avg_results/avg_duration_ms |
| T-17-08 | No checkSearchAnalyticsAuth() call in data_handler.go (verified: 0 matches) |

## Known Stubs

None.

## Self-Check: PASSED

- `backend/internal/db/data_analytics.go` — EXISTS
- `backend/internal/db/data_analytics_test.go` — EXISTS
- `backend/internal/api/handlers/data_handler.go` — EXISTS
- `backend/internal/api/handlers/data_handler_test.go` — EXISTS
- Commit e0fd7cd — EXISTS
- Commit e74801e — EXISTS
- Routes `/data/trending`, `/data/breakdown`, `/data/categories` wired in router.go — VERIFIED
- `checkSearchAnalyticsAuth` in data_handler.go — 0 matches (public endpoint confirmed)
