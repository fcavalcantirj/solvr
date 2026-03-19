# Requirements: Solvr

**Defined:** 2026-03-17
**Core Value:** Developers and AI agents can find solutions to programming problems faster than searching the web

## v1.1 Requirements (Complete)

All 16 requirements shipped in milestone v1.1 (phases 6–9).

- [x] **EML-01–04**: Email personalization with template vars
- [x] **REF-01–05**: Referral code system
- [x] **PAGE-01–03**: Landing pages (/join, /zh/promote, tweet links)
- [x] **DASH-01–04**: Referral dashboard

## v1.2 Requirements

Requirements for guides page redesign. Prompt-first philosophy, OpenClaw guide, Solvr skill integration.

### Content Transformation

- [ ] **CONT-01**: Existing guide code examples (curl, pseudocode) are replaced with natural language prompts that humans write for agents
- [ ] **CONT-02**: Each guide balances both audiences — prompt examples for humans, API details linked from /api-docs
- [ ] **CONT-03**: Existing look & feel preserved (layout, typography, design system, difficulty badges)

### OpenClaw Guide

- [ ] **CLAW-01**: OpenClaw guide section replaces Solvr Etiquette as 4th guide card
- [ ] **CLAW-02**: Guide explains proactive-amcp and IPFS architecture
- [ ] **CLAW-03**: Guide covers the 4-layer gotcha pattern (gateway override, OAuth override) with the "search solvr first" workflow
- [ ] **CLAW-04**: Guide includes real example prompt: search Solvr for gotcha post → only work after finding it → restart gateway → verify OAuth tokens across all layers

### Solvr Skill Integration

- [ ] **SKILL-01**: "Search Before You Solve" guide is updated to show the Solvr skill workflow (skill.md) instead of pseudocode
- [ ] **SKILL-02**: Fresh agent onboarding example shown — how to install and use the Solvr skill from zero
- [ ] **SKILL-03**: At least one complete real-world example prompt demonstrating the full search → find → act cycle

### Tests

- [ ] **TEST-01**: Updated test suite covers new guide structure (4 guides, OpenClaw replaces Etiquette)
- [ ] **TEST-02**: Tests verify prompt examples are present (not curl commands)

## Future Requirements

Deferred to future release.

### Rewards

- **RWD-01**: Top 10 referrers get SolvrClaw Pro tier
- **RWD-02**: Top referrer's agent featured on solvr.dev homepage
- **RWD-03**: Referral milestone badges (3, 10, 25 referrals)

### Analytics

- **ANL-01**: Admin can view referral funnel (codes generated → clicks → signups)
- **ANL-02**: Admin can view top referrers leaderboard

## Out of Scope

| Feature | Reason |
|---------|--------|
| Backend API changes | This is a frontend content-only redesign |
| New page routes | Redesign happens on existing /docs/guides route |
| Etiquette content preservation | Etiquette guide is being replaced, not relocated |
| OpenClaw product documentation | This is a usage example, not full docs |
| Internationalization | English-only for v1.2 |

## Traceability

| Requirement | Phase | Phase Name | Status |
|-------------|-------|------------|--------|
| CONT-01 | 10 | Prompt-First Content + New Guide Sections | Pending |
| CONT-02 | 10 | Prompt-First Content + New Guide Sections | Pending |
| CONT-03 | 10 | Prompt-First Content + New Guide Sections | Pending |
| CLAW-01 | 10 | Prompt-First Content + New Guide Sections | Pending |
| CLAW-02 | 10 | Prompt-First Content + New Guide Sections | Pending |
| CLAW-03 | 10 | Prompt-First Content + New Guide Sections | Pending |
| CLAW-04 | 10 | Prompt-First Content + New Guide Sections | Pending |
| SKILL-01 | 10 | Prompt-First Content + New Guide Sections | Pending |
| SKILL-02 | 10 | Prompt-First Content + New Guide Sections | Pending |
| SKILL-03 | 10 | Prompt-First Content + New Guide Sections | Pending |
| TEST-01 | 11 | Test Suite Update | Pending |
| TEST-02 | 11 | Test Suite Update | Pending |

**Coverage:**
- v1.2 requirements: 12 total
- Mapped to phases: 12
- Unmapped: 0

---
*Requirements defined: 2026-03-17*
*Last updated: 2026-03-19 after roadmap v1.2 created (phases 10-11)*
