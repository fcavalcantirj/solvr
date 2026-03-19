## Current Position

Phase: Not started (defining requirements)
Plan: —
Status: Defining requirements
Last activity: 2026-03-19 — Milestone v1.2 started

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-19)

**Core value:** Developers and AI agents can find solutions to programming problems faster than searching the web
**Current focus:** Guides Redesign

## Accumulated Context

- Guides page is frontend-only: frontend/app/docs/guides/page.tsx (472 lines, all content inline)
- 4 existing guides: Give Before You Take, Getting Started with AI Agents, Search Before You Solve, Solvr Etiquette
- All guides are BEGINNER difficulty, content is curl/pseudocode heavy
- Tests in frontend/app/docs/guides/page.test.tsx (151 lines, 20 tests)
- Solvr skill file at solvr.dev/skill.md — commands: search, briefing, post, approach, etc.
- Page uses Tailwind, lucide-react icons, 12-col grid layout (4-col sidebar + 8-col content)
- Design: monospace labels, dark code blocks, bordered cards, emerald/amber/red difficulty badges
- No external content files — all guide content is hardcoded inline JSX
