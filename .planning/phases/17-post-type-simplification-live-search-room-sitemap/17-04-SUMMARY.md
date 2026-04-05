---
phase: 17-post-type-simplification-live-search-room-sitemap
plan: 04
subsystem: ui
tags: [nextjs, recharts, vitest, csr, analytics, polling, shadcn]

requires:
  - phase: 17-03
    provides: "Backend /v1/data/trending, /v1/data/breakdown, /v1/data/categories endpoints"

provides:
  - "/data page: CSR analytics dashboard with stat cards, trending queries, PieChart, BarChart, 60s polling"
  - "Static OG metadata layout for /data route"
  - "Vitest test suite: 8 tests covering all 8 behavior cases"

affects: [sitemap, header-nav, phase-18]

tech-stack:
  added: []
  patterns:
    - "CSR page with useCallback + useEffect polling pattern (60s interval)"
    - "Tabs mock pattern for Radix UI in Vitest: global callback registry avoids React.Children prop-drilling"
    - "act(async) + setTimeout(150ms) pattern for testing components with internal setTimeout state updates"

key-files:
  created:
    - frontend/app/data/layout.tsx
    - frontend/app/data/page.tsx
    - frontend/app/data/page.test.tsx

key-decisions:
  - "Mock Radix Tabs in Vitest using global callback registry — React.Children.map/cloneElement approach failed (React not in scope in vi.mock factory)"
  - "Remove vi.useFakeTimers() from beforeEach — page.tsx uses setTimeout(100ms) inside fetchAll for fade transition, which blocked all async tests with fake timers"
  - "Use act(async) + real setTimeout(150ms) to let fetchAll's internal 100ms delay complete before assertions"

patterns-established:
  - "Radix UI tab mock: store onValueChange in module-level registry keyed by counter, TabsTrigger onClick reads from registry"
  - "CSR dashboard polling: useCallback deps=[window, includeBots] + useEffect deps=[fetchAll] ensures correct re-fetch on state changes"

requirements-completed: [SEARCH-04]

duration: 8min
completed: 2026-04-05
---

# Phase 17 Plan 04: /data Live Search Analytics Page Summary

**CSR analytics dashboard at /data with recharts pie+bar charts, 60s polling, time range toggle, bot filter, and 8 Vitest tests — wires to Phase 17-03 backend endpoints**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-05T00:39:12Z
- **Completed:** 2026-04-05T00:47:30Z
- **Tasks:** 1 (+ 1 checkpoint)
- **Files modified:** 3

## Accomplishments
- `/data` page renders 4 stat cards (Total Searches, Agent %, Human %, Zero Results %), trending queries table (top 10), PieChart (searcher breakdown), BarChart (by content type)
- 60-second auto-polling via `setInterval` with fade transition on data refresh
- Time range toggle (1h/24h/7d) and bot filter switch both trigger immediate re-fetch
- Loading skeletons, empty state, and error state with retry button all implemented
- Static OG metadata in `layout.tsx` for social sharing
- Page is 431 lines (under 800 limit), builds to 115 kB
- 8/8 Vitest tests pass; 1078/1078 total frontend tests pass; Next.js build succeeds

## Task Commits

1. **Task 1: /data page (TDD)** - `d36c24a` (feat)

## Files Created/Modified
- `frontend/app/data/layout.tsx` - Static metadata: `title: "Solvr -- Live Search Activity"`, OG tags
- `frontend/app/data/page.tsx` - CSR dashboard (431 lines): stat cards, trending table, PieChart, BarChart, polling, toggles, loading/empty/error states
- `frontend/app/data/page.test.tsx` - 8 Vitest tests covering all behavior cases

## Decisions Made
- Removed `vi.useFakeTimers()` from test `beforeEach` — the `setTimeout(100ms)` inside `fetchAll` for the fade transition blocked all async `waitFor` assertions with fake timers
- Used `act(async) + setTimeout(150ms)` pattern to allow the internal delay to complete before asserting rendered state
- Mocked Radix Tabs using a module-level callback registry instead of React.Children.map — the factory function for `vi.mock` doesn't have React in scope, so cloneElement failed

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed Radix Tabs mock breaking test rendering**
- **Found during:** Task 1 (GREEN phase)
- **Issue:** Initial Tabs mock used `React.Children.map` and `React.cloneElement` which require `React` to be in scope inside `vi.mock()` factory — this is not available, causing all 8 tests to fail
- **Fix:** Rewrote Tabs mock to use a module-level callback registry (`tabsCallbacks` map) — `Tabs` stores its `onValueChange`, `TabsTrigger` reads all stored callbacks on click
- **Files modified:** `frontend/app/data/page.test.tsx`
- **Verification:** All 8 tests pass
- **Committed in:** `d36c24a`

**2. [Rule 1 - Bug] Fixed test timeout caused by fake timers**
- **Found during:** Task 1 (GREEN phase, first test run)
- **Issue:** `vi.useFakeTimers()` in `beforeEach` blocked Promise resolution for async tests using `waitFor` — `fetchAll` contains a `setTimeout(100ms)` for the fade transition, which never fired under fake timers
- **Fix:** Removed `vi.useFakeTimers()`, added `afterEach(() => vi.useRealTimers())`, used `act(async) + setTimeout(150ms)` to wait for the internal delay
- **Files modified:** `frontend/app/data/page.test.tsx`
- **Verification:** 7/8 tests passed after this fix (then fixed Tabs mock for the 8th)
- **Committed in:** `d36c24a`

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both fixes were test infrastructure issues, not production code changes. The page implementation required no deviations from the plan.

## Issues Encountered
- Radix UI Tabs `onValueChange` does not fire with `fireEvent.click` alone — Radix uses pointer events internally. Solved by mocking the Tabs component entirely in tests.

## Known Stubs
None — all data is fetched from real API endpoints wired to the `/v1/data/*` backend built in Plan 03.

## Threat Flags
None — no new network endpoints, auth paths, or schema changes introduced. This plan is frontend-only CSR, reading from existing Plan 03 endpoints.

## Next Phase Readiness
- `/data` page complete and building — ready for human verification in Task 2 checkpoint
- Phase 17 human verification requires running dev servers and checking all 15 verification items across plans 01-04
- No blockers

## Self-Check: PASSED

- FOUND: `frontend/app/data/layout.tsx`
- FOUND: `frontend/app/data/page.tsx`
- FOUND: `frontend/app/data/page.test.tsx`
- FOUND: `17-04-SUMMARY.md`
- FOUND commit: `d36c24a`

---
*Phase: 17-post-type-simplification-live-search-room-sitemap*
*Completed: 2026-04-05*
