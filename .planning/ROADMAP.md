# Roadmap: Solvr Guides Redesign

**Milestone:** v1.2
**Created:** 2026-03-19
**Phases:** 2 (phases 10–11)
**Requirements:** 12

---

## Phase 10: Prompt-First Content + New Guide Sections

**Goal:** Transform all guide content from code-centric to prompt-first, replace Etiquette with OpenClaw guide, and update the Solvr skill integration guide.
**Requirements:** CONT-01, CONT-02, CONT-03, CLAW-01, CLAW-02, CLAW-03, CLAW-04, SKILL-01, SKILL-02, SKILL-03
**Status:** ○ NOT STARTED

### Success Criteria

1. The guides page shows 4 guide cards: "Give Before You Take", "Getting Started with AI Agents", "Search Before You Solve", and "OpenClaw: 4-Layer Architecture" — "Solvr Etiquette" no longer appears.
2. Code blocks throughout the page show natural language prompts (e.g., "Ask your agent to...") instead of curl commands or pseudocode — API references remain accessible as secondary content.
3. The OpenClaw section (id `#openclaw`) explains proactive-amcp, IPFS architecture, the 4-layer gotcha pattern (gateway override, OAuth override), and includes at least one complete real-world example prompt showing the search Solvr → find gotcha → restart gateway → verify OAuth flow.
4. The "Search Before You Solve" section shows the Solvr skill workflow (install, search, briefing) instead of the old pseudocode pattern, including a fresh agent onboarding example and at least one complete search → find → act cycle prompt.
5. The page layout, typography, Tailwind classes, 12-column grid, monospace labels, dark code blocks, bordered cards, and difficulty badges remain visually unchanged.

### Notes

- All 10 content requirements are in a single phase because they touch the same file (`frontend/app/docs/guides/page.tsx`, 472 lines) and are deeply intertwined — splitting them across phases would create merge conflicts and wasted rework.
- Replace the `guides` array entry for "Solvr Etiquette" with the OpenClaw guide (CLAW-01). Update `href` to `#openclaw` and icon to `Layers` or similar.
- For CONT-01/CONT-02: rewrite code blocks to show prompt examples like "Tell your agent:" followed by a natural language instruction, with an expandable or secondary block for the raw API call.
- For SKILL-01/SKILL-02/SKILL-03: reference `solvr.dev/skill.md` commands (search, briefing, post, approach) and show the install-and-use flow from zero.
- The OpenClaw guide section replaces the Etiquette section entirely — no need to relocate etiquette content (confirmed out of scope in REQUIREMENTS.md).
- Keep file under ~900 lines per project convention.

---

## Phase 11: Test Suite Update

**Goal:** Update the test suite to verify the new guide structure, prompt-first content, and OpenClaw section.
**Requirements:** TEST-01, TEST-02
**Status:** ○ NOT STARTED

### Success Criteria

1. Tests verify exactly 4 guide cards render with the updated titles including "OpenClaw" (not "Solvr Etiquette"), and all anchor links point to the correct section IDs (`#core-principle`, `#agent-quickstart`, `#search-pattern`, `#openclaw`).
2. Tests verify that prompt-style content is present (e.g., "Tell your agent" or "Ask your agent") and that no raw curl commands appear in the primary code blocks of the guides.
3. All existing layout and structural tests that remain relevant continue to pass (page heading, "ALL GUIDES" label, section numbering).
4. `npm test` passes with zero failures in `frontend/app/docs/guides/page.test.tsx`.

### Notes

- Update the existing test file (`frontend/app/docs/guides/page.test.tsx`, 151 lines, 20 tests) rather than creating a new one.
- Tests asserting "Solvr Etiquette" should be replaced with assertions for "OpenClaw" content.
- Tests asserting `PATCH /v1/agents/me` curl blocks should be updated to match the new prompt-first content.
- Section numbering test (`03 — COMMUNITY`) should change to match the new OpenClaw section label.
- Phase 10 must be complete before this phase — tests verify the content changes.

---

## Phase Summary

| Phase | Name | Layer | Requirements | Count |
|-------|------|-------|--------------|-------|
| 10 | 1/1 | Complete    | 2026-03-19 | 10 |
| 11 | Test Suite Update | Complete    | 2026-03-19 | 2 |

**Total:** 12 requirements across 2 phases

---

## Coverage

All 12 v1.2 requirements mapped:

| Requirement | Phase |
|-------------|-------|
| CONT-01 | Phase 10 |
| CONT-02 | Phase 10 |
| CONT-03 | Phase 10 |
| CLAW-01 | Phase 10 |
| CLAW-02 | Phase 10 |
| CLAW-03 | Phase 10 |
| CLAW-04 | Phase 10 |
| SKILL-01 | Phase 10 |
| SKILL-02 | Phase 10 |
| SKILL-03 | Phase 10 |
| TEST-01 | Phase 11 |
| TEST-02 | Phase 11 |

Coverage: **12/12 (100%)**

---

## Dependency Order

- Phase 10 must precede Phase 11 — tests verify the content changes from Phase 10

### Phase 12: API Docs Accuracy Audit — fix all discrepancies between /api-docs frontend and backend

**Goal:** [To be planned]
**Requirements**: TBD
**Depends on:** Phase 11
**Plans:** 1/4 plans executed

Plans:
- [ ] TBD (run /gsd:plan-phase 12 to break down)

---
*Roadmap created: 2026-03-19*
*Milestone: v1.2 Guides Redesign*
