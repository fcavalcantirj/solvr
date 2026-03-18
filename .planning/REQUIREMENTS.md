# Requirements: Solvr Growth Engine

**Defined:** 2026-03-17
**Core Value:** Developers and AI agents can find solutions to programming problems faster than searching the web

## v1.1 Requirements

Requirements for growth hack email campaign infrastructure. Each maps to roadmap phases.

### Email Personalization

- [ ] **EML-01**: Admin can send broadcast with `{name}` template var replaced per-recipient with display_name
- [ ] **EML-02**: Admin can send broadcast with `{referral_code}` template var replaced per-recipient
- [ ] **EML-03**: Template vars work in both `body_html` and `body_text` fields
- [ ] **EML-04**: Admin can send broadcast with `{referral_link}` template var (full URL: `solvr.dev/join?ref=CODE`)

### Referral System

- [ ] **REF-01**: Each user has a unique referral code (8-char alphanumeric, stored in users table)
- [ ] **REF-02**: Referral codes are auto-generated for all existing users via migration/backfill
- [ ] **REF-03**: New users signing up via `/join?ref=CODE` are tracked in a `referrals` table
- [ ] **REF-04**: API endpoint `GET /v1/users/me/referral` returns user's code and referral count
- [ ] **REF-05**: Authenticated user can see their referral code on the frontend

### Landing Pages

- [ ] **PAGE-01**: `/join?ref=CODE` page passes ref param to registration API and attributes signup
- [ ] **PAGE-02**: `/zh/promote` page shows Chinese-language promotion guide with share links
- [ ] **PAGE-03**: Frontend generates pre-filled tweet link with user's referral URL

### Referral Dashboard

- [ ] **DASH-01**: Authenticated user can view their referral dashboard at `/referrals`
- [ ] **DASH-02**: Dashboard shows referral code with copy button
- [ ] **DASH-03**: Dashboard shows count of successful referrals
- [ ] **DASH-04**: Dashboard shows share links (X/Twitter, copy URL)

## v2 Requirements

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
| SolvrClaw product | Separate project, promised as future reward |
| Reward fulfillment automation | Track referrals now, fulfill manually when 1K hit |
| Referral leaderboard page | Existing leaderboard infra exists, extend later |
| Email unsubscribe/preferences | Admin-only broadcasts, not needed yet |
| Referral link analytics (click tracking) | Simple attribution is enough for v1.1 |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| EML-01 | — | Pending |
| EML-02 | — | Pending |
| EML-03 | — | Pending |
| EML-04 | — | Pending |
| REF-01 | — | Pending |
| REF-02 | — | Pending |
| REF-03 | — | Pending |
| REF-04 | — | Pending |
| REF-05 | — | Pending |
| PAGE-01 | — | Pending |
| PAGE-02 | — | Pending |
| PAGE-03 | — | Pending |
| DASH-01 | — | Pending |
| DASH-02 | — | Pending |
| DASH-03 | — | Pending |
| DASH-04 | — | Pending |

**Coverage:**
- v1.1 requirements: 16 total
- Mapped to phases: 0
- Unmapped: 16

---
*Requirements defined: 2026-03-17*
*Last updated: 2026-03-17 after initial definition*
