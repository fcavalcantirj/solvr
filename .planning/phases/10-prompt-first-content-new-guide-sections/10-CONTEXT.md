# Phase 10: Prompt-First Content + New Guide Sections - Context

**Gathered:** 2026-03-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Redesign all guide content on `/docs/guides` from code-centric (curl commands, pseudocode) to prompt-first (natural language directives humans paste into AI agents). Replace the Solvr Etiquette guide with an OpenClaw 4-Layer Gotcha guide. Update "Search Before You Solve" with Solvr skill workflow. Preserve the existing look & feel completely.

</domain>

<decisions>
## Implementation Decisions

### Prompt Voice & Format
- Directive style with emphasis — ALL CAPS for critical instructions (e.g., "ONLY START DOING WORK AFTER FINDING THE POST")
- Raw prompts in dark code blocks — no framing labels like "Tell your agent:", readers understand it's what you paste
- All English, no mixed language
- All code examples (curl, pseudocode, API calls) replaced with natural language prompts — zero code in primary blocks
- If someone wants API details, they go to `/api-docs`

### OpenClaw Guide (replaces Etiquette)
- Practical gotcha guide only — no architecture recap (about page covers that)
- Content: one core example prompt showing the full Solvr search → find → act workflow for the 4-layer auth override gotcha
- Reference Solvr post: `44781b98` — "OpenClaw Auth Override Model: The 4-Layer Stack (complete reference)" by jack_openclaw (3 votes, 19 views)
- Link to the full Solvr post at the end of the section (heading style at Claude's discretion)
- The 4 layers: Shell env vars (Layer 0) → sessions.json override (Layer 1) → auth-profiles.json (Layer 2) → openclaw.json global (Layer 3)

### Solvr Skill Presentation ("Search Before You Solve")
- Show `solvr.dev/skill.md` URL as the onboarding entry point — "add this to your agent's instructions"
- Solvr is reactive, not proactive: you hit a wall/bug → search Solvr → if found, use it; if not found, create problem → post approach → mark succeeded/failed
- Drop the old pseudocode (`async function solveProblem...`) entirely
- Prompt structure: Claude's discretion on whether to show one long prompt or separate per-step prompts — optimize for what reads best

### Look & Feel
- **LOCKED — NO CHANGES**: Layout, typography, Tailwind classes, 12-column grid, monospace labels, dark code blocks, bordered cards, difficulty badges all stay exactly as-is
- Design system is gorgeous and must not change

### Claude's Discretion
- Guide card identity for OpenClaw (icon, title, difficulty, description)
- How to structure the Solvr skill prompts (one long vs separate)
- Heading for the Solvr post link at end of OpenClaw section
- Whether "Give Before You Take" and "Getting Started" guides need prompt rewrites or if their current content already fits the prompt-first philosophy (they currently mix code + explanatory text)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing code
- `frontend/app/docs/guides/page.tsx` — Current guides page (472 lines, all content inline, 4 guides)
- `frontend/app/docs/guides/page.test.tsx` — Current test suite (151 lines, 20 tests)
- `frontend/app/about/page.tsx` lines 323-375 — Existing OpenClaw section (3 architecture layers, avoid duplicating)

### External references
- `https://solvr.dev/skill.md` — Solvr skill file, the onboarding entry point for fresh agents
- Solvr post `44781b98` — "OpenClaw Auth Override Model: The 4-Layer Stack" — the reference content for the OpenClaw guide example prompt

### Design system
- Existing page IS the design reference — copy its patterns exactly (Tailwind classes, grid layout, dark blocks, monospace labels)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `guides` array (lines 11-40): metadata for guide cards — modify entries, don't restructure
- Section layout pattern: 12-col grid with 4-col sidebar (label + title + description) and 8-col content area — reuse for OpenClaw section
- Dark code block pattern: `bg-foreground text-background p-4` — use for prompt blocks
- Difficulty badges: emerald (BEGINNER), amber (INTERMEDIATE), red (ADVANCED) — already implemented
- Icons from lucide-react: Heart, Bot, Search, Layers, Cpu, Users already imported

### Established Patterns
- Each guide section has: section label (`00 — THE CORE PRINCIPLE`), h2 title, description paragraph, then bordered content cards
- Code blocks use `<pre><code>` inside dark foreground div
- Content is all inline JSX — no external data files, no markdown rendering

### Integration Points
- `guides` array drives the card grid (lines 83-122)
- Each card links to an anchor (`href: "#openclaw"`)
- Section IDs must match card hrefs

</code_context>

<specifics>
## Specific Ideas

- The OpenClaw example prompt should be close to the user's real prompt: "search on solvr for gateway override, oauth override, the 4 layers of gotcha. ONLY START DOING WORK AFTER FINDING THE POST. after that restart openclaw gateway and make sure all is good. make sure all layers have the correct OAUTH token"
- The Solvr workflow is: hit a wall → search → if found, use it; if not, create problem + approach + mark outcome — this is the narrative for "Search Before You Solve"
- Post `44781b98` is the showcase — the guide demonstrates that Solvr actually has the answer, proving the "search first" philosophy works

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 10-prompt-first-content-new-guide-sections*
*Context gathered: 2026-03-19*
