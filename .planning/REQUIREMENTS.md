# Requirements: Solvr Admin Email System

**Defined:** 2026-03-17
**Core Value:** Developers and AI agents can find solutions to programming problems faster than searching the web

## v1 Requirements

Requirements for admin email broadcast capability. Each maps to roadmap phases.

### Infrastructure

- [ ] **INFRA-01**: Admin can create Resend account and verify solvr.dev domain (DNS: SPF, DKIM)
- [ ] **INFRA-02**: Admin can send a test email from Resend dashboard to verify deliverability
- [ ] **INFRA-03**: API loads Resend API key from `RESEND_API_KEY` env var and initializes email client

### Email Sending

- [ ] **EMAIL-01**: Admin can broadcast email to all active users via `POST /admin/email/send`
- [ ] **EMAIL-02**: Broadcast endpoint requires `X-Admin-API-Key` header (same as other admin routes)
- [ ] **EMAIL-03**: Broadcast accepts subject, HTML body, and optional plain text body
- [ ] **EMAIL-04**: Broadcast sends from `noreply@solvr.dev` (configurable via `FROM_EMAIL` env var)
- [ ] **EMAIL-05**: Broadcast skips soft-deleted users (`WHERE deleted_at IS NULL`)
- [ ] **EMAIL-06**: Admin can preview broadcast via dry-run mode (returns recipient count + list, sends nothing)

### Audit

- [ ] **AUDIT-01**: Each broadcast creates an `email_broadcasts` record (subject, body, recipient count, status, sent_at)
- [ ] **AUDIT-02**: Admin can list past broadcasts via `GET /admin/email/history`

### Tooling

- [ ] **TOOL-01**: Admin can send broadcast email via `solvr-admin email send` CLI command
- [ ] **TOOL-02**: Admin can preview broadcast via `solvr-admin email dry-run` CLI command
- [ ] **TOOL-03**: Admin can view past broadcasts via `solvr-admin email history` CLI command
- [ ] **TOOL-04**: CLI authenticates via `ADMIN_API_KEY` env var or `~/.config/solvr/admin-credentials.json`

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### User Preferences

- **PREF-01**: User can opt out of broadcast emails via unsubscribe link
- **PREF-02**: User can configure email notification preferences (never/weekly/immediate)

### Targeted Sending

- **TARG-01**: Admin can send email to filtered user subset (by role, join date, activity)
- **TARG-02**: Admin can send email to specific user(s) by email or ID

### Analytics

- **ANAL-01**: Admin can view email open rates and bounce rates
- **ANAL-02**: Admin can view delivery statistics over time

## Out of Scope

| Feature | Reason |
|---------|--------|
| Email template builder UI | Over-engineering for admin-only CLI tool |
| Async email queue/worker | ~100 users, synchronous send is fine |
| Open/click tracking pixels | Privacy concern, unnecessary for announcements |
| POP3/IMAP inbox for solvr.dev | Send-only domain, no need to receive email |
| User-facing email management | Admin-only for v1, user preferences in v2 |
| Per-email Resend webhooks | Audit log is sufficient for v1 |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| INFRA-01 | 1 | Pending |
| INFRA-02 | 1 | Pending |
| INFRA-03 | 2 | Complete |
| EMAIL-01 | 3 | Pending |
| EMAIL-02 | 3 | Pending |
| EMAIL-03 | 3 | Pending |
| EMAIL-04 | 2 | Complete |
| EMAIL-05 | 2 | Complete |
| EMAIL-06 | 3 | Pending |
| AUDIT-01 | 2 | Complete |
| AUDIT-02 | 4 | Pending |
| TOOL-01 | 5 | Pending |
| TOOL-02 | 5 | Pending |
| TOOL-03 | 5 | Pending |
| TOOL-04 | 5 | Pending |

**Coverage:**
- v1 requirements: 15 total
- Mapped to phases: 15
- Unmapped: 0

---
*Requirements defined: 2026-03-17*
*Last updated: 2026-03-17 after roadmap creation (5 phases)*
