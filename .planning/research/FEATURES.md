# Feature Research

**Domain:** Admin email broadcast system
**Researched:** 2026-03-17
**Confidence:** HIGH

## Feature Landscape

### Table Stakes (Admin Expects These)

| Feature | Why Expected | Complexity | Notes |
|---------|-------------|------------|-------|
| Send to all active users | Core use case — "broadcast" means everyone | Low | `SELECT email FROM users WHERE deleted_at IS NULL` already works; `UserRepository.List()` exists but does not return emails — needs `ListActiveEmails()` |
| Subject + body inputs | Every email tool has these | Low | Admin provides inline (no template UI per PROJECT.md) |
| Plain text + HTML body | Deliverability and readability | Low | `EmailMessage` already supports both; `SMTPClient` already sends `multipart/alternative` |
| Admin-only access | Security baseline | Low | `checkAdminAuth()` already exists in `admin.go` — same `X-Admin-API-Key` pattern |
| Send confirmation response | Admin needs to know it worked | Low | Return `{ sent: N, failed: N }` in HTTP response |
| Audit log entry | Accountability for bulk sends | Low | `audit_log` table exists (migration 000012); already has `action`, `details JSONB`, `created_at` — just needs an INSERT per broadcast |
| "From" uses solvr.dev domain | Sender credibility, SPF/DKIM alignment | Medium | Requires Mailgun DNS setup — this is infrastructure, not code |

### Differentiators (Nice to Have)

| Feature | Value Proposition | Complexity | Notes |
|---------|------------------|------------|-------|
| Dry-run mode (`dry_run: true`) | Admin can preview recipient count before sending | Low | Add `dry_run` flag to request body; return count without sending |
| Preview first recipient | Sanity-check template rendering | Low | Send to admin email first, then proceed to all users |
| Per-user unsubscribe token in footer | Legal compliance (CAN-SPAM, GDPR) | Medium | Requires generating tokens per recipient and a `GET /unsubscribe?token=X` endpoint; out of scope per PROJECT.md but worth noting |
| Subject preview text (preheader) | Email open rate improvement | Low | Inject `<span style="display:none">` snippet before HTML body |
| Rate limiting sends per day | Prevent accidental spam | Low | Simple check: 1 broadcast per 24h enforced in handler or checked against audit_log |
| Paginated send (batch by 50) | Avoid SMTP connection timeouts on large lists | Low | Loop with configurable batch size; already using sync send so this is a `for` loop |
| Filter by signup date or activity | Target only recently active users | Medium | Requires query parameters and DB filter; overkill for v1 with ~100 users |

### Anti-Features (Avoid)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|--------------|-----------------|-------------|
| Email template builder UI | Looks professional | Adds frontend surface area, auth complexity, and DB tables — disproportionate to a CLI-driven admin tool | Admin provides HTML body inline via CLI skill |
| Per-user targeting / segmentation | "Send to users who did X" | Scope creep; requires complex query builder; 100 users makes targeting pointless today | Broadcast only in v1; add filtering later if list grows |
| Email queue / worker system | "What if SMTP is slow?" | Async job infrastructure for infrequent admin sends is over-engineering; sync with error reporting is fine | Sync send with `{ sent, failed, errors[] }` response |
| Scheduled sends | "Send this email tomorrow at 9am" | Needs a cron table, job runner, and state management; complexity outweighs value for manual admin tool | Admin triggers manually via skill |
| Unsubscribe preferences UI | User empowerment | PROJECT.md explicitly out of scope for v1; admin-only broadcasts are not marketing email at this scale | CAN-SPAM footer text with admin email for opt-out requests |
| Open/click tracking pixel | Marketing analytics | Requires image hosting or redirect endpoints; privacy concerns; out of scale for this platform | Audit log is sufficient visibility |

---

## Feature Dependencies

```
[Mailgun DNS setup]
    └── SPF record (TXT on solvr.dev)
    └── DKIM record (TXT on mail.solvr.dev)
    └── MX record (optional — for bounces only)
          │
          ▼
[Wire EmailService into main.go]  ←── Already coded: email.go + smtp.go exist as dead code
    └── NewDefaultSMTPClient(EmailConfig{...})
    └── NewEmailService(client, fromEmail)
          │
          ▼
[UserRepository.ListActiveEmails()]  ←── New method needed; List() already exists but omits email
    └── SELECT id, email, display_name FROM users WHERE deleted_at IS NULL
          │
          ▼
[POST /admin/email/broadcast]  ←── New handler in admin.go (or new admin_email.go)
    └── Validates request (subject, body required)
    └── Calls ListActiveEmails()
    └── Loops, calls EmailService.SendEmail() per recipient
    └── Records to audit_log (action="email_broadcast", details={subject, sent, failed})
    └── Returns { sent: N, failed: N, errors: [...] }
          │
          ▼
[Admin email skill (solvr-admin.sh)]  ←── New CLI script wrapping the HTTP endpoint
    └── broadcast subcommand
    └── dry-run flag
    └── Reads ADMIN_API_KEY from env
```

**Existing code already covers:**
- `EmailMessage` struct with HTML + Text fields (`email.go:42`)
- `SMTPClient` interface + `DefaultSMTPClient` implementation (`smtp.go`)
- `EmailService.SendEmail()` with retry support (`email.go:107`)
- `checkAdminAuth()` in `handlers/admin.go:218` — reusable as-is
- `audit_log` table with `details JSONB` field (migration 000012)
- `writeAdminJSON()` / `writeAdminError()` helper functions in `admin.go:177`

**New code required:**
- `UserRepository.ListActiveEmails()` — simple SELECT, ~15 lines
- `POST /admin/email/broadcast` handler — ~60 lines
- Migration for `email_broadcasts` table OR reuse `audit_log` (audit_log is sufficient)
- Route registration in router
- CLI skill script

---

## MVP Definition

### Launch With (v1)

- Mailgun account + DNS records for solvr.dev (SPF, DKIM)
- Wire `EmailService` into `main.go` with SMTP env vars
- `UserRepository.ListActiveEmails()` — returns `[]{ id, email, display_name }`
- `POST /admin/email/broadcast` — subject + body (HTML + optional text), sends to all active users, logs to audit_log, returns `{ sent, failed }`
- `dry_run` flag — returns `{ would_send: N }` without sending
- Admin email skill (`solvr-admin.sh broadcast`) wrapping the HTTP endpoint

### Add After Validation (v1.x)

- Batch sending (50 per iteration) to avoid timeout on large user lists
- Preview send (send to admin email first)
- Rate-limit guard: reject if broadcast sent within last 24h
- `GET /admin/email/broadcasts` — list past broadcasts from audit_log

### Future Consideration (v2+)

- Per-user unsubscribe tokens and `GET /unsubscribe?token=X`
- Filtering by user cohort (last_seen, signup date, reputation range)
- Bounce handling via Mailgun webhook
- HTML template library (stored in DB, selected by slug)

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|-----------|---------------------|----------|
| Mailgun DNS setup | Critical — nothing works without it | Low (config, no code) | P0 |
| Wire EmailService into main.go | Critical — dead code activation | Low (~20 lines) | P0 |
| `ListActiveEmails()` repo method | Critical — need recipient list | Low (~15 lines) | P0 |
| `POST /admin/email/broadcast` endpoint | Core feature | Low (~60 lines) | P0 |
| Audit log on send | Required for accountability | Low (~5 lines, table exists) | P0 |
| `dry_run` mode | High — prevents accidents | Very Low (flag in handler) | P1 |
| Admin CLI skill | High — primary UX for this admin | Low (~50 lines shell) | P1 |
| Batch sending | Medium — only matters at scale | Low (loop refactor) | P2 |
| Preview send to self | Medium — QA assist | Very Low | P2 |
| Per-user unsubscribe | Low today, required at scale | Medium | P3 |
| Bounce handling | Low today | Medium | P3 |
| HTML template library | Low — inline body is fine | High | P4 |
| Email open/click tracking | Low — privacy concerns too | High | Avoid |

---

## Sources

- `/Users/fcavalcanti/dev/solvr/.planning/PROJECT.md` — milestone scope, constraints, explicit out-of-scope items
- `/Users/fcavalcanti/dev/solvr/.planning/STATE.md` — accumulated context on current state
- `/Users/fcavalcanti/dev/solvr/backend/internal/services/email.go` — existing EmailService and template functions
- `/Users/fcavalcanti/dev/solvr/backend/internal/services/smtp.go` — DefaultSMTPClient with TLS/STARTTLS support
- `/Users/fcavalcanti/dev/solvr/backend/internal/api/handlers/admin.go` — existing admin auth pattern, helpers
- `/Users/fcavalcanti/dev/solvr/backend/internal/db/users.go` — UserRepository.List() — confirmed email field absent from public list queries
- `/Users/fcavalcanti/dev/solvr/backend/migrations/000012_create_audit_log.up.sql` — audit_log schema
- `/Users/fcavalcanti/dev/solvr/backend/migrations/000058_add_admin_type_to_audit_log.up.sql` — audit_log admin_type extension
- Industry conventions: CAN-SPAM Act (2003), GDPR Art. 6, Mailgun/SendGrid admin broadcast patterns

---
*Feature research for: admin email broadcast*
*Researched: 2026-03-17*
