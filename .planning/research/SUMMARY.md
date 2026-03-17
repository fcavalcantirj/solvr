# Project Research Summary

**Project:** Solvr
**Domain:** Admin email broadcast infrastructure
**Researched:** 2026-03-17
**Confidence:** HIGH

---

## Executive Summary

The core discovery from this research is that Solvr already has most of the email infrastructure it needs — `EmailService`, `DefaultSMTPClient`, the `SMTPClient` interface, `EmailMessage` struct, five transactional email templates, and full SMTP config loading via env vars. All of this exists in `backend/internal/services/email.go` and `smtp.go` as dead code that was never wired into `main.go`. The milestone is primarily an activation effort, not a build-from-scratch effort. The net-new code is small: one repo method (`ListActiveEmails`), one HTTP handler (`BroadcastEmail`), one migration (`email_broadcast_logs`), one router registration, and one bash skill script.

However, the dead code has bugs. The `smtp.go` sets `Content-Transfer-Encoding: quoted-printable` in the MIME header but never applies the actual encoding to the body — emails sent through it will display garbled characters in Outlook and ProtonMail. There is also a type mismatch: `Config.SMTPPort` is a string in env.go but callers pass an int — `strconv.Atoi` is required. Neither bug was caught because the code has never run in production. These issues are best bypassed entirely by using Mailgun's HTTP API instead of the existing SMTP client, which also resolves the DMARC alignment concern.

The external dependency with the longest lead time is DNS propagation (up to 48h). This makes DNS setup the first critical path item: it must be started before any code work begins. The code changes themselves are small enough to complete in a single session once DNS is staged. The overall milestone risk is low — the architecture is familiar (same patterns as the existing translation job runner), the scope is tightly bounded by PROJECT.md, and the only genuine unknowns are DNS configuration state and Mailgun account setup.

---

## Key Findings

### Recommended Stack

**Primary:** Mailgun HTTP API via `github.com/mailgun/mailgun-go/v5` (v5.14.0)

- Free tier: 1,000 emails/month — sufficient for Solvr's current ~100 active users
- Preferred over the existing SMTP path because it: avoids the quoted-printable bug in `smtp.go`, achieves automatic DMARC Return-Path alignment, provides structured delivery status and bounce info, and eliminates SMTP port/auth complexity
- Implement as a `MailgunClient` struct satisfying the existing `SMTPClient` interface — the interface is already the correct abstraction point; no other code changes needed

**Fallback / SMTP path:** The existing `DefaultSMTPClient` works for dev/local testing when pointed at `smtp.mailgun.org:587`, but should not be used in production until the quoted-printable encoding bug is fixed.

**No new database library needed.** Existing pgx/v5 pool handles all audit log writes.

**DNS records required for solvr.dev:**
- SPF: `TXT @ v=spf1 include:mailgun.org ~all` (merge with any existing SPF record — do not add a second)
- DKIM: `TXT pic._domainkey.solvr.dev` (key from Mailgun dashboard)
- DMARC: `TXT _dmarc.solvr.dev v=DMARC1; p=none; rua=mailto:admin@solvr.dev` (start as monitor-only)
- Tracking CNAME: `email.solvr.dev → mailgun.org` (optional but improves deliverability)

### Expected Features

**Must Have (P0 — launch blockers):**
- Mailgun account + DNS records verified for solvr.dev (SPF + DKIM)
- Wire `EmailService` (or `MailgunClient`) into the router — currently dead code
- `UserRepository.ListActiveEmails()` — returns `[]EmailRecipient` with `id`, `email`, `display_name` for non-deleted users
- `POST /admin/email/broadcast` — subject + body (HTML + optional plain text), sends to all active users, returns `{ sent, failed }`
- `email_broadcast_logs` table (migration 000069) — two-phase write: INSERT status=sending → UPDATE with final counts
- Audit log entry written per broadcast (not per individual email)

**Should Have (P1 — high value, low cost):**
- `dry_run: true` flag on broadcast endpoint — returns `{ would_send: N }` without sending; prevents accidents; must exist before first production use
- Admin CLI skill (`solvr-admin.sh`) wrapping the HTTP endpoint with `ADMIN_API_KEY` sourced from environment

**Defer (P2+):**
- `GET /admin/email/broadcasts` — history list from `email_broadcast_logs`
- Batch sends (50/iteration) with configurable `BROADCAST_DELAY_MS`
- Preview send to admin email before full broadcast
- Per-user unsubscribe tokens and `GET /unsubscribe?token=X`
- Bounce handling via Mailgun webhook
- HTML template library stored in DB

**Explicitly out of scope (per PROJECT.md):**
- User-facing email preferences UI
- Per-user targeting or segmentation
- Email queue / worker system
- Open/click tracking pixels

### Architecture Approach

The system follows the existing `TranslationJobRunner` pattern throughout:

```
solvr-admin.sh (bash skill)
    → POST /admin/email/broadcast  (X-Admin-API-Key)
    → AdminEmailHandler.BroadcastEmail()
        → checkAdminAuth()               [reuse existing helper]
        → UserRepository.ListActiveEmails()
        → email_broadcast_logs INSERT    [status=sending]
        → for each user: EmailService.SendEmail() / MailgunClient
        → email_broadcast_logs UPDATE    [final counts]
        → return { broadcast_id, sent, failed, duration_ms }
```

**Key design decisions:**
- Extend existing `AdminHandler` struct (not a new handler file) — `admin.go` is 386 lines, adding ~100 lines stays well under the 900-line limit
- Use `EmailSender` interface injected via `SetEmailService()` setter — same pattern as `SetTranslationJobRunner()`, enables test mocking without import cycles
- Wire email in `router.go` behind `if smtpHost != ""` guard — consistent with how optional services are wired
- If `emailService` is nil when broadcast is called, return `503 {"error": "EMAIL_NOT_CONFIGURED"}` — consistent with existing `TRANSLATION_NOT_CONFIGURED` pattern
- New `email_broadcast_logs` table (migration 000069) — dedicated table preferred over reusing `audit_log` because broadcasts need a queryable two-phase status (`sending` → `completed`) and the `audit_log.admin_id` FK assumes a user account, not an API key
- `main.go` requires no changes — all wiring happens in `router.go`

### Critical Pitfalls

**Pitfall 1: Dead code has a quoted-printable encoding bug**
`smtp.go` sets `Content-Transfer-Encoding: quoted-printable` but never applies the encoding. This is a silent correctness bug that will cause garbled rendering in Outlook and ProtonMail. Mitigation: bypass `smtp.go` entirely by implementing a `MailgunClient` using the HTTP API, or fix with `mime/quotedprintable.NewWriter`.

**Pitfall 2: SMTPPort type mismatch**
`Config.SMTPPort` is loaded as a string from env. Any router wiring code that passes it directly as `int` will fail to compile. Always apply `strconv.Atoi(os.Getenv("SMTP_PORT"))` in the wiring code.

**Pitfall 3: DNS SPF duplicate / lookup overflow**
Only one `v=spf1` TXT record is allowed per domain (RFC 7208). Adding a second record invalidates both. Audit existing DNS before adding Mailgun's SPF include. Also verify the total lookup chain stays under 10. Run `dig TXT solvr.dev` before touching DNS.

**Pitfall 4: Broadcast blocks HTTP WriteTimeout (15s)**
The Go HTTP server has `WriteTimeout: 15s`. At ~200ms per email (SMTP), the handler times out at ~75 recipients with no indication of how many emails were sent. Mitigation options: raise the timeout for admin routes via a separate mux, return `202 Accepted` with a job ID and poll endpoint, or use the Mailgun batch send API. This must be decided before implementation.

**Pitfall 5: `SendEmailAsync` goroutine leak**
`EmailService.SendEmailAsync()` spawns one goroutine per call. Using it in a broadcast loop creates N concurrent goroutines and N simultaneous Mailgun connections — Mailgun will reject many with 429. Use synchronous `SendEmail()` in a sequential loop with delay instead. `SendEmailAsync` was designed for single transactional emails only.

**Additional pitfalls to track:**
- nil `EmailService` causes panic unless guarded — use the 503 pattern
- Sending to soft-deleted users raises Mailgun bounce rate (use `WHERE deleted_at IS NULL`)
- No `dry_run` mode before first production broadcast is a data loss risk
- `ADMIN_API_KEY` must be sourced from env in the skill, never hardcoded or passed as CLI arg
- DMARC Return-Path misalignment when using SMTP relay (use HTTP API to avoid)
- Missing `List-Unsubscribe` header; at minimum add a mailto version and plain-text footer

---

## Implications for Roadmap

### Phase 1: DNS + Mailgun Setup (External Infrastructure)
**Rationale:** DNS propagation takes up to 48 hours and blocks everything downstream. This is the critical path item with the longest lead time. No code can be tested against real email delivery until Mailgun domain verification passes. Start this first, in parallel with any code work.
**Delivers:** Verified Mailgun account for solvr.dev, SPF + DKIM records live and validated, SMTP credentials available, `SMTP_HOST` / `SMTP_PASS` env vars ready for staging and production.

### Phase 2: Database Migration + Audit Repository
**Rationale:** The `email_broadcast_logs` table must exist before the handler can write to it. This is a one-way dependency. The migration is simple (~15 lines SQL) and the repository is ~40 lines of Go.
**Delivers:** Migration 000069, `EmailBroadcastRepository` with `CreateLog()` / `UpdateStatusAndCounts()` / `List()` methods, integration tests for the repository.

### Phase 3: UserRepository.ListActiveEmails()
**Rationale:** Independent of Phase 2 and can run in parallel. The handler needs a recipient list, and the existing `UserRepository.List()` intentionally omits email from public queries. This is a new method on an existing file.
**Delivers:** `ListActiveEmails(ctx) ([]EmailRecipient, error)` returning `id`, `email`, `display_name` for `WHERE deleted_at IS NULL` users, with unit tests.

### Phase 4: EmailSender Interface + MailgunClient
**Rationale:** Replaces the buggy `DefaultSMTPClient` for production use with Mailgun HTTP API. Implements `SMTPClient` interface so the rest of the system is unaffected. Includes the `quoted-printable` fix decision (bypass via API rather than patching `smtp.go`).
**Delivers:** `MailgunClient` struct implementing `EmailSender` interface, wired via `mailgun-go/v5`, with dry-run/sandbox mode for tests. Unit tests with mock.

### Phase 5: Broadcast Handler + Router Wiring
**Rationale:** Depends on Phases 2, 3, and 4. This is the core feature: extending `AdminHandler` with `BroadcastEmail()`, deciding the sync vs. 202+poll strategy for timeouts, adding `dry_run` support, and registering routes. This is also where the nil-guard (503) and `checkAdminAuth` must be verified.
**Delivers:** `POST /admin/email/broadcast` endpoint with `dry_run` flag, audit log writes, `{ sent, failed, broadcast_id }` response, protected by `checkAdminAuth`. Handler tests with mock `EmailSender`. Route registration in `router.go`.

### Phase 6: Admin CLI Skill (solvr-admin.sh)
**Rationale:** Depends on Phase 5 being deployed. The skill wraps the HTTP endpoint with bash, reads `ADMIN_API_KEY` from environment, supports `broadcast` and `dry-run` subcommands, and provides human-readable output for the admin.
**Delivers:** `solvr-admin.sh` skill with `broadcast-email` subcommand, dry-run flag, loads credentials from env, handles error responses gracefully, does not expose key in shell history.

### Phase Ordering Rationale

Phase 1 (DNS) is first because it is the only phase with multi-hour external blocking time. All code phases (2–6) can be developed while DNS propagates. Phases 2 and 3 are independent and can run in parallel. Phase 4 (email client) is sequenced before Phase 5 (handler) because the handler depends on the interface. Phase 6 (skill) is last because it depends on the deployed endpoint.

The most dangerous ordering mistake would be implementing the handler (Phase 5) before deciding the HTTP timeout strategy — the 15s `WriteTimeout` issue must be resolved architecturally before writing a line of handler code.

### Research Flags

**Needs deeper research before Phase 1:**
- Current DNS state of solvr.dev (run `dig TXT solvr.dev` to check for existing SPF records before adding anything)
- Mailgun account availability: does an account for solvr.dev already exist? Are SMTP credentials already generated?

**Needs decision before Phase 5:**
- HTTP timeout strategy: raise admin route timeout vs. 202+poll. At ~100 users, sync with 30s timeout is safe today. Plan the migration path for when user count grows.

**Standard patterns (no additional research needed):**
- Phase 2: pgx repository pattern — identical to `db/service_checks.go`
- Phase 3: simple SELECT — no unknowns
- Phase 4: `mailgun-go/v5` API is well-documented
- Phase 5: follows `SetTranslationJobRunner` pattern exactly
- Phase 6: follows `solvr.sh` skill pattern exactly

---

## Confidence Assessment

| Area | Confidence | Basis |
|------|------------|-------|
| Existing code inventory (what's dead, what's live) | HIGH | Direct file inspection of `email.go`, `smtp.go`, `admin.go`, `env.go`, migrations |
| Required new code volume | HIGH | Small: ~15 lines repo method, ~60 lines handler, ~15 lines SQL migration, ~50 lines bash skill |
| Mailgun SMTP/API integration | HIGH | Verified via `mailgun-go/v5` docs + existing `SMTPClient` interface shape |
| DNS record requirements | HIGH | Standard SPF/DKIM/DMARC for Mailgun; RFC-backed |
| Current DNS state of solvr.dev | LOW | Not inspected — must run `dig TXT solvr.dev` before Phase 1 |
| HTTP timeout solution at scale | MEDIUM | Current user count (~100) makes sync safe; growth path needs a decision |
| Mailgun free tier limits | HIGH | 1,000 emails/month on Flex plan — researched against current pricing page |
| Bugs in dead code | HIGH | QP encoding bug and port type mismatch confirmed by direct code reading |
| Compliance (CAN-SPAM, GDPR) | MEDIUM | `List-Unsubscribe` header requirement and plain-text footer are necessary; full unsubscribe flow deferred |

### Gaps to Address

1. **DNS pre-flight:** Run `dig TXT solvr.dev` before touching any DNS. If an SPF record already exists, merge rather than add. Check `dig TXT pic._domainkey.solvr.dev` for DKIM selector conflicts.
2. **Mailgun account status:** Confirm whether a Mailgun account for solvr.dev already exists or must be created. This affects Phase 1 duration.
3. **Timeout decision:** Document the chosen strategy (sync with extended timeout OR 202+poll) before Phase 5 begins. At current user count, either is acceptable — pick one and stick to it.
4. **`List-Unsubscribe` header:** Add to broadcast emails at minimum as a `mailto:` value. This is a one-line addition in the handler that prevents Mailgun domain suspension from spam complaints.
5. **Mailgun sandbox vs. production domain:** The Mailgun sandbox domain only sends to verified recipients. Tests should use sandbox for unit/integration but the Phase 1 DNS setup is required before any end-to-end test reaches a real inbox.

---

## Sources

**From STACK.md:**
- Mailgun pricing page — Flex plan: 1,000 free emails/month
- `github.com/mailgun/mailgun-go` releases — v5.14.0 latest stable
- `backend/internal/services/smtp.go` — existing DefaultSMTPClient
- `backend/internal/services/email.go` — existing EmailService and SMTPClient interface
- `backend/internal/config/env.go` — SMTP vars already loaded

**From FEATURES.md:**
- `backend/internal/api/handlers/admin.go` — checkAdminAuth(), writeAdminJSON(), SetTranslationJobRunner() pattern
- `backend/internal/db/users.go` — UserRepository.List() (confirmed email field absent)
- `backend/migrations/000012_create_audit_log.up.sql` — audit_log schema
- `backend/migrations/000058_add_admin_type_to_audit_log.up.sql` — audit_log admin_type extension
- CAN-SPAM Act (2003), GDPR Art. 6, Mailgun/SendGrid admin broadcast patterns

**From ARCHITECTURE.md:**
- `backend/internal/api/router.go` — admin route registration, conditional wiring pattern
- `backend/cmd/api/main.go` — background job wiring pattern
- `.claude/skills/solvr/scripts/solvr.sh` — skill script pattern

**From PITFALLS.md:**
- RFC 7208 (SPF): 10 DNS lookup limit
- RFC 6376 (DKIM): selector uniqueness
- Mailgun documentation: SMTP credentials vs API keys, sending limits
- Gmail Bulk Sender Guidelines (2024): List-Unsubscribe requirements
- Go stdlib: `mime/quotedprintable`, `crypto/subtle`
- Direct analysis: QP encoding bug and SMTPPort type mismatch in dead code

**From PROJECT.md:**
- Milestone scope and out-of-scope boundaries
- Budget, security, simplicity constraints

---

*Research completed: 2026-03-17*
*Ready for roadmap: yes*
