# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.2 — Guides Redesign

**Shipped:** 2026-03-19
**Phases:** 3 | **Plans:** 6

### What Was Built
- Prompt-first guides page — 4 guides with natural language prompts replacing all curl/pseudocode
- OpenClaw 4-layer auth gotcha guide with real Solvr search example
- Solvr skill integration guide (install, reactive workflow, search→find→act)
- Full API docs accuracy audit — 4 data files rewritten, 25+ endpoints verified against Go handlers
- Test suite for guides page (23 tests covering structure, content, prompt-first assertions)

### What Worked
- Parallel agent execution for Phase 12 (4 agents, 4 plans, all completed in one wave)
- Agent-first docs approach: reading actual Go handler source to verify response shapes caught real bugs (evolve response wrapper, auth levels)
- Single-file content phases (Phase 10) avoided merge conflicts

### What Was Inefficient
- Phase 11 executed outside GSD tracking — no SUMMARY.md or VERIFICATION.md created, needed manual verification during audit
- Audit found a response wrapper bug that should have been caught during execution (verifier spot-checks caught it)

### Patterns Established
- Verify API docs against actual handler source code, not just route definitions
- Agent-first doc writing: params, response shapes, auth levels all match backend structs

### Key Lessons
1. When executing phases outside GSD tracking, at minimum create SUMMARY.md — it's the artifact everything else depends on
2. Parallel execution of independent docs-only plans is highly efficient — 4 agents finished in ~7 minutes each

### Cost Observations
- Model mix: orchestrator opus, executors/verifier sonnet
- Phase 12 execution: 4 parallel sonnet agents (~76k-118k tokens each)
- Notable: Parallel execution of 4 independent plans saved significant wall-clock time

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Phases | Plans | Key Change |
|-----------|--------|-------|------------|
| v1.2 | 3 | 6 | Parallel agent execution, agent-first docs approach |

### Cumulative Quality

| Milestone | Tests | Key Metric |
|-----------|-------|------------|
| v1.2 | 1022 | 12/12 requirements, 5/5 endpoint spot-checks |

### Top Lessons (Verified Across Milestones)

1. Always create tracking artifacts even for quick phases — audit depends on them
2. Verify docs against source code, not assumptions
