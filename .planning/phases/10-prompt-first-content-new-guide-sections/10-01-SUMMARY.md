---
phase: 10
plan: "10-01"
status: complete
---

# Summary: 10-01

## What was built

All guide content in `frontend/app/docs/guides/page.tsx` was rewritten from code-centric (curl commands, pseudocode) to prompt-first (natural language directives in dark code blocks). The "Solvr Etiquette" guide card and section were replaced with the "OpenClaw: 4-Layer Auth Gotcha" guide documenting the 4-layer auth override pattern with a real Solvr search prompt and a link to post `44781b98`. The "Search Before You Solve" section was expanded from a single pseudocode card to three cards covering skill installation, the reactive workflow, and a real PostgreSQL pool exhaustion example. Unused `Cpu` and `Users` lucide imports were removed. The file shrank from 472 to 428 lines and the build passes cleanly.

## Tasks completed

| # | Task | Status |
|---|------|--------|
| 1 | Replace "Solvr Etiquette" card with OpenClaw card in guides array | ✓ |
| 2 | Rewrite "Give Before You Take" section to prompt-first | ✓ |
| 3 | Rewrite "Getting Started with AI Agents" section to prompt-first | ✓ |
| 4 | Rewrite "Search Before You Solve" section with Solvr skill workflow | ✓ |
| 5 | Replace "Solvr Etiquette" section with "OpenClaw: 4-Layer Auth Gotcha" section | ✓ |
| 6 | Verify file size, build, and visual consistency | ✓ |

## Key files

### Created
- (none)

### Modified
- frontend/app/docs/guides/page.tsx

## Self-Check

PASSED

- File is 428 lines (under 900 limit)
- `npm run build` exits 0, no TypeScript errors
- All 19 positive acceptance criteria grep patterns confirmed present
- All 9 negative acceptance criteria grep patterns confirmed absent (0 matches)
- Guides array has exactly 4 entries: Give Before You Take, Getting Started with AI Agents, Search Before You Solve, OpenClaw: 4-Layer Auth Gotcha
- No curl commands anywhere in the file
- No pseudocode (async function, solvr.search, solvr.contribute) anywhere
- Section IDs match card hrefs: #core-principle, #agent-quickstart, #search-pattern, #openclaw
- Cpu and Users imports removed

## Issues

Note: The existing test file (`frontend/app/docs/guides/page.test.tsx`) has 6 tests that now fail because they reference the replaced "Solvr Etiquette" content ("Solvr Etiquette", "03 — COMMUNITY", "HOW TO THRIVE", "FOR AI AGENTS", "FOR HUMANS", "KNOWLEDGE COMPOUNDING", "Help others first.", "PATCH https://api.solvr.dev/v1/agents/me"). These test updates are the responsibility of Phase 11 per the plan dependency structure.
