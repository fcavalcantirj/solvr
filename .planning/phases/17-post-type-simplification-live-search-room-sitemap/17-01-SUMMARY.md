---
phase: 17-post-type-simplification-live-search-room-sitemap
plan: "01"
subsystem: frontend
tags: [post-type-simplification, navigation, sitemap, ux]
dependency_graph:
  requires: []
  provides: [SIMPLIFY-01, SIMPLIFY-02, SIMPLIFY-03]
  affects: [frontend/components/header.tsx, frontend/components/new-post/new-post-form.tsx, frontend/components/feed/feed-filters.tsx, frontend/app/sitemap-core.xml/route.ts]
tech_stack:
  added: []
  patterns: [TDD red-green, subtractive change]
key_files:
  created: []
  modified:
    - frontend/components/header.tsx
    - frontend/components/header.test.tsx
    - frontend/components/new-post/new-post-form.tsx
    - frontend/components/feed/feed-filters.tsx
    - frontend/app/sitemap-core.xml/route.ts
    - frontend/app/new/page.tsx
    - frontend/lib/api-types.ts
decisions:
  - "Existing /questions/[id] pages left untouched — backwards compatibility by omission (SIMPLIFY-02)"
  - "QUESTIONS kept in negative test assertions (queryByRole) — verifies absence, not presence"
  - "api-types.ts APIPost.type retains 'question' union — backend still returns question for 9 existing posts"
  - "Nav order changed to: FEED, PROBLEMS, IDEAS, ROOMS, AGENTS, DATA, IPFS, LEADERBOARD, SKILL, GUIDES"
metrics:
  duration: "~5 minutes"
  completed: "2026-04-05"
  tasks_completed: 1
  files_modified: 7
---

# Phase 17 Plan 01: Post Type Simplification — Frontend Surfaces Summary

One-liner: Removed question post type from all frontend creation and discovery surfaces (nav, form, filters, sitemap, metadata) and added DATA navigation link, preserving existing question detail pages via backwards-compatible omission.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| RED | Failing tests for DATA nav / no QUESTIONS | 73143a7 | header.test.tsx |
| GREEN | Implement all frontend changes | 1744a85 | header.tsx, new-post-form.tsx, feed-filters.tsx, sitemap-core.xml/route.ts, new/page.tsx, api-types.ts |

## What Was Built

### Header Navigation (header.tsx)
- Removed QUESTIONS link from both desktop and mobile nav
- Added DATA link after AGENTS (before IPFS) in both desktop and mobile nav
- Final nav order: FEED, PROBLEMS, IDEAS, ROOMS, AGENTS, DATA, IPFS, LEADERBOARD, SKILL, GUIDES

### New Post Form (new-post-form.tsx)
- Reduced POST_TYPES from 3 entries to 2 (problem, idea only — question removed)
- Changed grid from `grid-cols-3` to `grid-cols-2` for 2-column layout
- Updated `NewPostFormProps.defaultType` union: `'problem' | 'idea'` (removed 'question')
- Simplified `handleSubmit` route logic: `form.type === 'problem' ? 'problems' : 'ideas'`

### Feed Filters (feed-filters.tsx)
- Removed `{ label: "Questions", value: "question" }` from `types` array
- Remaining filters: All, Problems, Ideas

### Sitemap Core (sitemap-core.xml/route.ts)
- Removed `/questions` entry
- Added `/rooms` entry (changefreq: hourly, priority: 0.8)
- Added `/data` entry (changefreq: hourly, priority: 0.7)

### New Post Page Metadata (new/page.tsx)
- Updated description from `'Create a new problem, question, or idea'` to `'Create a new problem or idea'`

### API Types (api-types.ts)
- Removed `'question'` from `CreatePostData.type` union (write side only)
- `APIPost.type` retains `'question'` — backend still returns question type for 9 existing posts

### Tests (header.test.tsx)
- Updated from asserting QUESTIONS presence to asserting DATA presence and QUESTIONS absence
- Added tests: no QUESTIONS in desktop nav, DATA link renders, DATA positioned between AGENTS and IPFS, no QUESTIONS in mobile nav, DATA in mobile menu
- 10 tests total, all passing

## Verification Results

All 1070 frontend tests pass across 117 test files.

```
Test Files  117 passed (117)
Tests       1070 passed (1070)
```

Acceptance criteria:
- `grep -c "question" frontend/components/new-post/new-post-form.tsx` → 0
- `grep -c "/questions" frontend/components/header.tsx` → 0
- `grep -c 'href="/data"' frontend/components/header.tsx` → 2
- `grep -c '"question"' frontend/components/feed/feed-filters.tsx` → 0
- `grep -c "question" frontend/app/new/page.tsx` → 0
- `grep -c "/questions" frontend/app/sitemap-core.xml/route.ts` → 0
- `grep -c "/rooms" frontend/app/sitemap-core.xml/route.ts` → 1
- `grep -c "/data" frontend/app/sitemap-core.xml/route.ts` → 1
- `grep -c "grid-cols-2" frontend/components/new-post/new-post-form.tsx` → 1
- `grep -c "'question'" frontend/lib/api-types.ts` → 7 (read types intact)

## Deviations from Plan

None — plan executed exactly as written. The nav order in the plan listed `FEED, PROBLEMS, IDEAS, ROOMS, AGENTS, DATA, IPFS, LEADERBOARD, SKILL, GUIDES` and the implementation matches this exactly. The QUESTIONS test references in header.test.tsx are negative assertions (checking absence), which is correct behavior.

## Known Stubs

None — all changes are real removals. No placeholder text introduced.

## Threat Flags

None — this plan is purely subtractive (removing dead content type from frontend surfaces). No new network endpoints, auth paths, or trust boundaries introduced.

## Self-Check: PASSED

Files exist:
- FOUND: frontend/components/header.tsx
- FOUND: frontend/components/header.test.tsx
- FOUND: frontend/components/new-post/new-post-form.tsx
- FOUND: frontend/components/feed/feed-filters.tsx
- FOUND: frontend/app/sitemap-core.xml/route.ts
- FOUND: frontend/app/new/page.tsx
- FOUND: frontend/lib/api-types.ts

Commits exist:
- FOUND: 73143a7 (test RED phase)
- FOUND: 1744a85 (feat GREEN phase)
