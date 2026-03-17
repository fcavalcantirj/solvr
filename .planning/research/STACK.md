# Stack Research

**Domain:** Email infrastructure for Go backend
**Researched:** 2026-03-17
**Confidence:** HIGH

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Mailgun (email provider) | — | Transactional email delivery for solvr.dev | Free tier: 1,000 emails/month (formerly 5k, now 1k on Flex plan). Cheapest for low-volume. Simple HTTP API. First-class SPF/DKIM setup wizard. EU and US regions. Excellent Go SDK. |
| `github.com/mailgun/mailgun-go/v5` | v5.14.0 | Go client for Mailgun HTTP API | Latest stable v5 line. Uses context-aware API. Clean interface. Wraps all Mailgun REST endpoints. Preferred over SMTP relay because it provides delivery status, bounce info, and avoids firewall/port issues. |
| Existing `DefaultSMTPClient` | (already exists) | SMTP fallback / local dev | Keep as-is. Wire for dev/test via Mailgun SMTP credentials. No code change needed for SMTP path. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `net/smtp` (stdlib) | Go 1.23 stdlib | SMTP client (already used in smtp.go) | Already in use. No additional dependency. Sufficient for Mailgun SMTP relay if using SMTP instead of API. |
| Standard `database/sql` / pgx (already present) | pgx v5.7.2 | Audit log writes to PostgreSQL | No new DB library needed. Audit log writes go through existing pgx pool. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| Mailgun dashboard | DNS record generation, domain verification, send logs | Free. Navigate to Sending > Domains to get SPF/DKIM values. |
| `dig` / `nslookup` | Verify DNS propagation after adding TXT records | Built into macOS/Linux. Run `dig TXT solvr.dev` to confirm SPF. |
| MXToolbox (https://mxtoolbox.com/SuperTool.aspx) | Validate SPF, DKIM, and DMARC records | Web-based. Useful before going live. |
| Mailgun test mode | Send without actual delivery during development | Set `mailgun.EnableTestMode()` in Go client or use sandbox domain. |

## Installation

```bash
# Add Mailgun Go SDK to backend module
cd backend
go get github.com/mailgun/mailgun-go/v5@v5.14.0

# Verify
go mod tidy
```

No frontend dependencies needed — email is backend-only.

## Provider Comparison

| Provider | Free Tier | Paid Pricing | SMTP Support | API | DNS Setup | Verdict |
|----------|-----------|-------------|-------------|-----|-----------|---------|
| **Mailgun** | 1,000 emails/month (Flex) | $15/mo for 50k | Yes (port 587) | REST API + Go SDK | Guided wizard in dashboard | **Recommended** — best free tier for low-volume, battle-tested Go SDK |
| Resend | 3,000 emails/month, 100/day | $20/mo for 50k | No (API only) | REST API + Go SDK (`resend-go/v2`) | Standard TXT records | Strong alternative if SMTP is abandoned; cleaner modern API; Go SDK at v2.28.0 |
| SendGrid | 100 emails/day free forever | $19.95/mo for 50k | Yes | REST API | Standard TXT records | Free tier too restrictive (100/day cap); Go SDK uses `+incompatible` versioning (messy) |
| AWS SES | 62,000 emails/month (if sending from EC2) | $0.10 per 1k | Yes | REST API | Requires SES-specific DNS verification | Overkill; requires AWS account; complex setup; no Go v2 `ses` SDK is clean |
| Postmark | None | $15/mo for 10k | Yes | REST API | Standard TXT records | No free tier; best deliverability for transactional; overkill for this scale |

**Budget recommendation:** Mailgun Flex (1k/month free) is sufficient for Solvr's current scale. At ~100 active users, even weekly broadcasts stay well within 1k/month. Upgrade to $15/mo if sends exceed 1k.

**Second choice:** Resend — if Mailgun free tier shrinks further or SMTP flexibility is not needed.

## DNS Requirements for solvr.dev

All records are added via your domain registrar's DNS management panel (or Cloudflare if proxied).

### Required Records

| Record Type | Host / Name | Value | Purpose |
|-------------|-------------|-------|---------|
| TXT | `solvr.dev` (or `@`) | `v=spf1 include:mailgun.org ~all` | SPF — authorizes Mailgun to send as solvr.dev. Exact value provided by Mailgun dashboard. |
| TXT | `pic._domainkey.solvr.dev` | (long DKIM public key from Mailgun) | DKIM — cryptographic sender verification. Mailgun dashboard provides the exact subdomain and key. |
| CNAME | `email.solvr.dev` | `mailgun.org` | Mailgun tracking domain (optional but recommended for delivery). |

### Optional Records

| Record Type | Host / Name | Value | Purpose |
|-------------|-------------|-------|---------|
| MX | `solvr.dev` | `mxa.mailgun.org` / `mxb.mailgun.org` | Only needed if solvr.dev should receive email (e.g., bounce replies). PROJECT.md says receive is out of scope — skip for now. |
| TXT | `solvr.dev` | `v=DMARC1; p=none; rua=mailto:admin@solvr.dev` | DMARC policy — improves deliverability. Start with `p=none` (monitor only). Add after SPF+DKIM are verified. |

### DNS Propagation
- Changes take 15 minutes to 48 hours to propagate globally.
- Verify with: `dig TXT solvr.dev` (SPF) and `dig TXT pic._domainkey.solvr.dev` (DKIM).
- Mailgun dashboard has a built-in "Verify DNS" button that confirms records are live.

## Integration with Existing EmailService

The existing `EmailService` uses the `SMTPClient` interface:

```go
type SMTPClient interface {
    Send(msg *EmailMessage) error
}
```

**Two integration options:**

### Option A: Use Mailgun SMTP relay (no new library)
Configure Mailgun SMTP credentials in env vars — the existing `DefaultSMTPClient` already handles TLS/STARTTLS on port 587. No code change in `smtp.go` or `email.go`. Just set:
```
SMTP_HOST=smtp.mailgun.org
SMTP_PORT=587
SMTP_USER=postmaster@solvr.dev
SMTP_PASS=<mailgun-smtp-password>
FROM_EMAIL=noreply@solvr.dev
```
Config already loads these. `DefaultSMTPClient` already works. Zero new code to wire up.

### Option B: Add MailgunClient implementing SMTPClient interface (new library)
Create a `MailgunClient` struct implementing `SMTPClient.Send()` using `mailgun-go/v5`. Wire into `EmailService` via the existing interface. Benefits: delivery status, bounce events, no SMTP port dependency.

**Recommendation: Start with Option A (SMTP relay).** The existing code handles it fully. No new library needed initially. If delivery analytics or bounce handling become important, add Option B later. The `SMTPClient` interface already makes this a clean swap.

## Email Audit Logging

The existing `audit_log` table (migration 000012) is sufficient — no new migration needed.

### What to store per broadcast

```json
{
  "action": "email_broadcast",
  "resource_type": "email",
  "details": {
    "subject": "...",
    "recipient_count": 42,
    "sent": 41,
    "failed": 1,
    "failed_emails": ["user@example.com"],
    "dry_run": false,
    "from": "noreply@solvr.dev"
  }
}
```

The `audit_log` table already has `action VARCHAR`, `details JSONB`, `created_at TIMESTAMP`, and `admin_type VARCHAR` (added in migration 000058). An INSERT with `admin_type='email_broadcast'` covers the full audit trail.

**Do not create a separate `email_broadcasts` table** — the existing audit_log with JSONB details is sufficient for the admin-only use case.

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| Full email queue system (Redis/RabbitMQ/Bull) | Massive infrastructure overhead for infrequent admin sends; PROJECT.md explicitly says "sync send is fine" | Synchronous `for` loop in handler with `{ sent, failed }` response |
| `gomail` / `jordan-wright/email` (pure SMTP libraries) | Redundant — `DefaultSMTPClient` already implements SMTP with TLS. Adding another SMTP library solves nothing | Keep existing `smtp.go` |
| SendGrid Go SDK | Uses `+incompatible` module versioning (v3.16.x); Go module hygiene issue; free tier too restrictive (100/day) | Mailgun or Resend |
| AWS SES | Requires AWS SDK v2 dependency tree (~20 transitive deps); overkill; complex IAM setup | Mailgun |
| Separate `email_broadcasts` DB table | Extra migration, extra repository, extra model — all for data already capturable in `audit_log.details JSONB` | INSERT into existing `audit_log` |
| Per-user email tracking pixels | Privacy concerns; requires image hosting endpoint; disproportionate for admin broadcasts | Audit log is sufficient |
| DKIM signing in Go code | Mailgun handles DKIM at the MTA layer automatically once DNS is set; rolling your own is error-prone | Mailgun dashboard DNS setup |

## Environment Variables Required

The existing config already loads these — no changes to `env.go` needed. Just set values pointing to Mailgun:

```bash
SMTP_HOST=smtp.mailgun.org
SMTP_PORT=587
SMTP_USER=postmaster@solvr.dev      # from Mailgun dashboard
SMTP_PASS=<mailgun-smtp-password>   # from Mailgun dashboard
FROM_EMAIL=noreply@solvr.dev
```

If using the Mailgun API (Option B), add:
```bash
MAILGUN_API_KEY=<private-api-key>   # from Mailgun dashboard
MAILGUN_DOMAIN=solvr.dev
```

## Sources

- Mailgun pricing page (https://www.mailgun.com/pricing/) — Flex plan: 1,000 free emails/month
- Resend pricing (https://resend.com/pricing) — 3,000/month free, 100/day cap
- SendGrid pricing (https://sendgrid.com/pricing/) — 100/day free forever
- `github.com/mailgun/mailgun-go` releases — v5.14.0 is latest stable (verified via `go list -m -versions`)
- `github.com/resend/resend-go/v2` — v2.28.0 latest stable (verified via `go list -m -versions`)
- Mailgun SMTP relay docs (https://documentation.mailgun.com/docs/mailgun/user-manual/sending-messages/#smtp-relay)
- Mailgun DNS setup guide (https://documentation.mailgun.com/docs/mailgun/user-manual/verifying-your-domain/)
- SPF record specification: RFC 7208
- DKIM specification: RFC 6376
- `/Users/fcavalcanti/dev/solvr/backend/internal/services/smtp.go` — existing DefaultSMTPClient (TLS/STARTTLS, port 465/587)
- `/Users/fcavalcanti/dev/solvr/backend/internal/services/email.go` — existing EmailService and SMTPClient interface
- `/Users/fcavalcanti/dev/solvr/backend/internal/config/env.go` — SMTP vars already loaded
- `/Users/fcavalcanti/dev/solvr/.planning/PROJECT.md` — budget constraints, Mailgun preference, sync-send constraint
- `/Users/fcavalcanti/dev/solvr/.planning/research/FEATURES.md` — audit_log reuse decision, feature dependencies

---
*Stack research for: email infrastructure*
*Researched: 2026-03-17*
