# Pitfalls Research

**Domain:** Email infrastructure for Go backend (Mailgun + DNS for solvr.dev)
**Researched:** 2026-03-17
**Confidence:** HIGH

---

## Critical Pitfalls

### Pitfall 1: SPF Record Conflict / Too Many DNS Lookups

**What goes wrong:** Adding a Mailgun SPF include to solvr.dev DNS without auditing existing SPF records causes either (a) a conflicting second `TXT v=spf1` record which is invalid — only one SPF record per domain is allowed — or (b) the cumulative DNS lookup chain exceeds the RFC 7208 limit of 10, causing `permerror` on all outbound email.

**Why it happens:** Developers add `include:mailgun.org` as a new TXT record without checking whether `v=spf1` already exists. Hosting panels make it easy to add records without warning on duplicates.

**How to avoid:**
1. Before adding any DNS record, run: `dig TXT solvr.dev` to list existing TXT records.
2. If a `v=spf1` record exists, merge it: `v=spf1 include:mailgun.org ~all`
3. Count all DNS lookups in the chain using `kitterman.com/spf/validate.html` — keep below 10.

**Warning signs:** Emails delivered to spam immediately; Mailgun sender verification shows SPF as failing; `dig TXT solvr.dev | grep spf` returns more than one result.

**Phase to address:** DNS setup phase (before any code is written or tested).

---

### Pitfall 2: DKIM Selector Collision with Existing Records

**What goes wrong:** Mailgun requires adding a DKIM TXT record at `pic._domainkey.solvr.dev`. If another provider (e.g., Google Workspace, SendGrid used by a previous developer) already occupies that selector, the new key silently overwrites it or coexists with conflicting values.

**Why it happens:** Mailgun's default DKIM selector name `pic` is generic. Multiple providers may have been tried historically without cleaning up old keys.

**How to avoid:**
1. Check existing DKIM selectors: `dig TXT pic._domainkey.solvr.dev`
2. If occupied, request a custom selector from Mailgun (they support custom selectors via domain settings).
3. After adding: verify with `dig TXT pic._domainkey.solvr.dev` returning Mailgun's public key.

**Warning signs:** Mailgun domain verification dashboard shows DKIM as "unverified" even after TTL has passed; emails marked as failing DKIM in headers.

**Phase to address:** DNS setup phase, before activating domain in Mailgun.

---

### Pitfall 3: DNS Propagation — Testing Before TTL Expires

**What goes wrong:** Developer adds SPF/DKIM records and immediately runs Mailgun's "Verify DNS" — it fails. They assume configuration is wrong and make more changes, creating a muddled state. The root cause is TTL had not expired yet (often 24–48h for new records on some registrars).

**Why it happens:** Urgency + impatience. Also, local DNS resolvers may cache the old (absent) record.

**How to avoid:**
1. Use a low TTL (300s = 5 min) when first adding records to iterate quickly, then raise it after verification.
2. Verify propagation via a third-party tool (e.g., `mxtoolbox.com`, `whatsmydns.net`) from multiple geographic locations, not just local `dig`.
3. Wait at minimum 15 minutes before re-running Mailgun domain verification.

**Warning signs:** Mailgun verification consistently fails but records appear correct in the registrar panel; `dig` from your machine shows the record but external checkers do not.

**Phase to address:** DNS setup phase; build in a planned 30-minute wait step in the setup checklist.

---

### Pitfall 4: Sending from `noreply@solvr.dev` Without a Matching Return-Path / Envelope Sender

**What goes wrong:** The current `config.go` sets `FROM_EMAIL` default to `noreply@solvr.dev`. When sent via Mailgun SMTP, the envelope `MAIL FROM` (Return-Path) is Mailgun's bounce address (e.g., `bounces.mailgun.org`). Many spam filters penalize mismatches between `From:` header domain (`solvr.dev`) and Return-Path domain (`mailgun.org`) — this is called a DMARC alignment failure.

**Why it happens:** Using SMTP instead of Mailgun's HTTP API means you don't benefit from Mailgun's automatic Return-Path alignment. The existing `smtp.go` uses `net/smtp` directly.

**How to avoid:**
- Switch from the existing SMTP client to Mailgun's HTTP API (`github.com/mailgun/mailgun-go/v4`). With the API, Mailgun automatically sets the Return-Path to `mg.solvr.dev` (a subdomain you configure), achieving full DMARC alignment.
- If staying with SMTP: add a DMARC record (`_dmarc.solvr.dev`) in relaxed alignment mode (`adkim=r; aspf=r`) to avoid hard DMARC failures during the transition.

**Warning signs:** Gmail shows "via mailgun.org" annotation next to sender name; DMARC reports show alignment failures; emails in spam despite SPF/DKIM individually passing.

**Phase to address:** Provider integration phase; decide SMTP vs API before implementing the broadcast handler.

---

### Pitfall 5: Existing SMTP `net/smtp` Client Is Not Mailgun-Compatible

**What goes wrong:** The existing `smtp.go` implements a raw `net/smtp` client. Mailgun SMTP credentials work differently from standard SMTP — the username is always `postmaster@yourdomain.mailgun.org` (not a regular email), and the password is the Mailgun SMTP password (different from API key). If someone wires the existing client with wrong credentials it silently fails or returns obscure auth errors.

**Why it happens:** Developers assume their Mailgun API key is the SMTP password. It is not. Mailgun has separate SMTP credentials per domain.

**How to avoid:**
- If using SMTP: obtain SMTP credentials from Mailgun → Domain Settings → SMTP credentials. Username is `postmaster@mg.solvr.dev`, password is the per-domain SMTP password.
- Strongly prefer Mailgun HTTP API over SMTP: it provides structured error responses, message IDs for audit logging, and no TLS/auth complexity. Replace `smtp.go` with a Mailgun API client.
- The existing `EmailConfig` struct (`SMTPUser`, `SMTPPass`) will need a Mailgun-specific config or an interface-based abstraction to support both.

**Warning signs:** `SMTP auth failed: 535 5.7.8 Authentication credentials invalid` errors; API key copied from Mailgun dashboard used as SMTP password.

**Phase to address:** Provider integration phase; document the distinction between API key and SMTP password explicitly.

---

### Pitfall 6: Bulk Broadcast Blocks the HTTP Request (Sync Send + Large User List)

**What goes wrong:** The current plan is synchronous email sending for admin broadcasts. If there are 1,000+ users, a single HTTP request to `POST /admin/broadcast` will hold open the connection for many minutes while emails are sent serially — one SMTP/API call per user. This hits `WriteTimeout: 15s` set in `main.go`, killing the request mid-send with no indication of how many emails were sent.

**Why it happens:** The 15-second `WriteTimeout` on the HTTP server was set for regular API calls. Bulk email loops are an order of magnitude longer. The PROJECT.md says "sync send is fine for admin broadcasts" — this is true at small scale, not at 1,000+ recipients.

**How to avoid:**
- Raise the write timeout for admin endpoints only (or use a separate admin mux with a longer timeout).
- OR: accept the request, return `202 Accepted` with a job ID, send in a background goroutine, and provide a status endpoint. Log progress to the audit table so the admin skill can poll for completion.
- At minimum: add a per-email configurable delay (`BROADCAST_DELAY_MS`, default 100ms) to respect Mailgun rate limits and avoid the request timing out by keeping per-iteration time predictable.
- Add a `sent_count` and `failed_count` field to the audit log entry, updated incrementally or at completion.

**Warning signs:** The admin skill curl command times out at 15 seconds during a broadcast; partial sends with no record of which users received email; `context deadline exceeded` errors in logs.

**Phase to address:** Admin broadcast handler design phase; plan the timeout/response strategy before implementation.

---

### Pitfall 7: No Rate Limit Awareness — Mailgun Free Tier Throttling

**What goes wrong:** Mailgun free tier allows 100 emails/day (as of recent policy changes), and the paid tier has per-minute sending rate limits. A broadcast to all users in a tight loop can exceed these limits, causing `429 Too Many Requests` responses that are silently swallowed if error handling is incomplete.

**Why it happens:** The admin broadcast implementation iterates over all users without any delay or batch control. Free tier limits are especially low and not well-documented in onboarding.

**How to avoid:**
- Add a configurable inter-email delay (`BROADCAST_DELAY_MS`, default 200ms for free tier).
- Capture and log HTTP 429 responses from Mailgun API separately — do not treat them the same as hard failures.
- Store a send count per day in the audit log so the admin knows the daily budget consumption.
- For production scale, upgrade to paid Mailgun before first broadcast campaign.

**Warning signs:** Only the first N emails in a batch are delivered; Mailgun API returns 429 errors in logs; `curl` admin skill reports "sent: 100, failed: 0" then "sent: 0, failed: 900" on consecutive days.

**Phase to address:** Admin broadcast handler implementation; tie to audit logging phase.

---

### Pitfall 8: Dead Code Not Wired — EmailService Silently Skipped

**What goes wrong:** `EmailService` and `DefaultSMTPClient` exist in `services/` but are not instantiated in `main.go`. When wiring, it is tempting to add the email service instantiation inside the `if pool != nil` block — but if email fails to initialize (missing `SMTP_HOST` env var), the entire condition block silently skips email setup with only a log warning. Admin broadcast handler then receives a nil `EmailService` and panics on first call.

**Why it happens:** The existing pattern in `main.go` wraps optional services in `if cfg != nil && someEnvVar != ""` guards. EmailService wiring is optional, but the broadcast handler requires it. This nil-safety mismatch is easy to miss.

**How to avoid:**
- Return `503 Service Unavailable` from the broadcast handler if `EmailService` is nil, with body `{"error": "EMAIL_NOT_CONFIGURED"}` — consistent with the existing `TRANSLATION_NOT_CONFIGURED` pattern in `admin.go`.
- Log a clear startup warning when email is not configured: `"Email service disabled: SMTP_HOST not set"`.
- Add a health-check endpoint or admin status endpoint that reports email configuration status.

**Warning signs:** First call to admin broadcast handler returns a nil pointer dereference panic; no log line about email service during startup.

**Phase to address:** Wiring phase (main.go integration).

---

### Pitfall 9: `SendEmailAsync` Goroutine Leak in Broadcast Context

**What goes wrong:** The existing `SendEmailAsync` method in `email.go` fires a goroutine with `go func() { ... context.Background() ... }()`. For a broadcast to 1,000 users, calling `SendEmailAsync` in a loop spawns 1,000 goroutines simultaneously. Go's runtime handles this, but Mailgun's API will reject many concurrent connections. If the server is then SIGTERM'd during broadcast, all goroutines are abandoned with no completion guarantee.

**Why it happens:** `SendEmailAsync` was designed for single transactional emails (welcome, notifications), not bulk sending. Using it for broadcasts is a category error.

**How to avoid:**
- Do NOT use `SendEmailAsync` for bulk broadcasts. Use synchronous `SendEmail` in a sequential loop with a delay.
- If async is desired for responsiveness, use a bounded worker pool (e.g., 3 concurrent workers) with a channel, not one goroutine per email.
- The broadcast handler should return only after all sends complete (or after context cancellation), so the audit log can be finalized accurately.

**Warning signs:** Goroutine count spikes to N*users during broadcast visible in `/debug/pprof/goroutine`; partial audit log entries; emails continue arriving minutes after the broadcast API call "completed".

**Phase to address:** Admin broadcast handler implementation phase.

---

### Pitfall 10: Audit Log Misses Failed Sends

**What goes wrong:** The audit log table exists (`000012_create_audit_log.up.sql`) but is designed for admin user actions (referenced by `admin_id UUID REFERENCES users(id)`). An email broadcast is not performed by a `users.id` — it's performed by the admin API key. If the broadcast audit entry references `admin_id = NULL`, useful accountability context is lost. Worse, if only the top-level broadcast action is logged (not individual email results), a broadcast that silently failed for 50% of recipients looks identical to a fully successful one.

**Why it happens:** The existing audit_log table schema assumes admin actions are tied to a user account. The new admin email system uses an API key (`ADMIN_API_KEY` env var) with no corresponding user record. Individual send results are often skipped to simplify implementation.

**How to avoid:**
- Create a dedicated `email_broadcast_log` table (new migration) with columns: `id`, `subject`, `body_preview`, `recipient_count`, `sent_count`, `failed_count`, `triggered_by` (string, e.g. "admin-api-key"), `started_at`, `completed_at`, `status` (pending/completed/partial_failure).
- Alternatively, insert into `audit_log` with `admin_id = NULL`, `action = 'email_broadcast'`, and `details JSONB` containing `{sent, failed, subject}`.
- Do not conflate "request received" with "emails sent" — log only after the send loop completes.

**Warning signs:** Broadcast audit entries always show success regardless of Mailgun errors; no record of how many users were unreachable; `details` column is null in audit_log for email actions.

**Phase to address:** Database schema phase (before broadcast handler implementation).

---

### Pitfall 11: Sending to Soft-Deleted Users

**What goes wrong:** The `users` table has `deleted_at` added by migration `000034_add_users_soft_delete.up.sql`. A naive `SELECT email FROM users` query for the broadcast recipient list will include soft-deleted users. Sending to their emails causes bounce events in Mailgun, harming the sending domain's reputation.

**Why it happens:** The recipient list query in the broadcast handler omits the `WHERE deleted_at IS NULL` clause that all other queries in the codebase apply.

**How to avoid:**
- Always filter: `SELECT email FROM users WHERE deleted_at IS NULL`.
- Additionally filter on `role != 'banned'` if applicable.
- Consider filtering to only users who have logged in within the last 90 days to reduce bounce rate from abandoned accounts (`last_seen_at` column exists per migration `000043`).
- Add a dry-run mode to the broadcast endpoint that returns recipient count without sending.

**Warning signs:** Mailgun bounce rate exceeds 5% (danger threshold for domain reputation); emails sent to accounts that users report were deleted.

**Phase to address:** Admin broadcast handler SQL query phase.

---

### Pitfall 12: Admin Key Exposed in Shell History via CLI Skill

**What goes wrong:** The `solvr-admin.sh` skill script will be invoked with the admin API key. If the key is passed as a command-line argument or embedded in the curl command, it appears in shell history (`~/.zsh_history`), process listings (`ps aux`), and server access logs.

**Why it happens:** The simplest way to pass auth to curl is `-H "X-Admin-API-Key: $KEY"`. The variable expansion happens at the shell level, but if the key is hardcoded in the script rather than sourced from `.env`, it is visible in editor history and git diff.

**How to avoid:**
- Always load `ADMIN_API_KEY` from environment: `source .env` before running the skill, never hardcode in scripts.
- The skill script should read `$ADMIN_API_KEY` from the environment, not accept it as a CLI argument.
- Ensure `.env` is in `.gitignore` (it already is per PROJECT.md).
- The `X-Admin-API-Key` header value does not appear in server access logs if logging is structured (log the header name, not value).

**Warning signs:** `ADMIN_API_KEY=...` visible in git diff; key hardcoded in `solvr-admin.sh`; key appears in `~/.zsh_history`.

**Phase to address:** CLI skill implementation phase.

---

### Pitfall 13: No Confirmation Step Before Mass Send

**What goes wrong:** The admin skill sends the broadcast on a single curl command. There is no dry-run or preview step. A typo in the subject, a test body accidentally sent to production, or a duplicate invocation (running the skill twice) results in 1,000+ users receiving wrong or duplicate emails with no recall option.

**Why it happens:** Admin tools prioritized simplicity. Single-command send is convenient but dangerous for irreversible bulk operations.

**How to avoid:**
- Implement a two-step flow in the skill: first call returns a preview (recipient count, subject, body preview, a `broadcast_id`) with status `pending`. Second call with `?confirm=true&broadcast_id=X` executes the send.
- OR: Add `dry_run: true` to the request body — returns `{"would_send_to": N}` without sending.
- Add idempotency: include a `broadcast_id` (client-generated UUID) in the request. If the same ID is submitted twice, return `409 Conflict` with the original result.

**Warning signs:** No dry-run mode exists; broadcast endpoint is a single non-idempotent POST; duplicate curl invocations send duplicate emails.

**Phase to address:** Admin broadcast API design phase (before implementation).

---

### Pitfall 14: `quoted-printable` Encoding Applied Incorrectly in `smtp.go`

**What goes wrong:** The existing `smtp.go` sets the header `Content-Transfer-Encoding: quoted-printable` but then writes the raw HTML/text body without actually encoding it. True quoted-printable encoding wraps lines at 76 characters and encodes non-ASCII and certain ASCII characters. Sending raw HTML with this header set causes some email clients (Outlook, ProtonMail) to display the raw text with `=3D` instead of `=` or break line rendering.

**Why it happens:** The header was added to indicate encoding intent, but the actual `qprintable.NewWriter` encoding step was omitted. This was not caught because the existing email service is dead code — it has never been used in production.

**How to avoid:**
- Use `mime/quotedprintable` from the Go standard library to actually encode the body, or set `Content-Transfer-Encoding: base64` and use `encoding/base64` for HTML bodies.
- Alternatively, use Mailgun's HTTP API which handles encoding automatically.
- Write an integration test that sends a real email through Mailgun sandbox to a controlled inbox and inspects the raw MIME source.

**Warning signs:** Emails display `=3D` where `=` should appear; line breaks in HTML look garbled in Outlook; encoding mismatch warnings in email header analysis tools.

**Phase to address:** Provider integration phase when wiring the existing SMTP client or replacing it with Mailgun API client.

---

### Pitfall 15: Missing List-Unsubscribe Header (CAN-SPAM / GDPR Compliance)

**What goes wrong:** Admin broadcast emails sent without `List-Unsubscribe` headers are technically compliant for transactional emails, but bulk announcements to all users are classified as commercial/marketing by many spam filters. Without this header, Gmail and Apple Mail will not show the one-click unsubscribe button, and spam complaint rates rise. High complaint rates (>0.1%) cause Mailgun to suspend the sending domain.

**Why it happens:** The PROJECT.md explicitly marks "User-facing email preferences/unsubscribe" as out of scope. This is fine as a product decision, but the email headers must still signal unsubscribe capability to comply with Gmail's 2024 bulk sender requirements (required for senders of >5,000 emails/day — important to implement before scale).

**How to avoid:**
- Add a `List-Unsubscribe` header pointing to a minimal endpoint: `<mailto:unsubscribe@solvr.dev?subject=unsubscribe>` or a URL-based endpoint.
- Even a static `mailto:` version satisfies header requirements without needing a full preference center.
- Include a plain-text "To stop receiving these emails, reply with UNSUBSCRIBE" footer in all broadcast emails.

**Warning signs:** Gmail showing "Why am I getting this?" warning; Mailgun suppression list growing from complaint reports; spam complaint rate visible in Mailgun dashboard exceeds 0.08%.

**Phase to address:** Admin broadcast template design phase.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Reuse existing `smtp.go` with Mailgun SMTP creds | No new dependencies | QP encoding bug, DMARC misalignment, harder error handling | Never — use Mailgun API instead |
| `SendEmailAsync` for bulk sends | Non-blocking API response | Goroutine explosion, no completion guarantee, incomplete audit log | Only for single transactional emails, never bulk |
| Single `audit_log` entry for entire broadcast | Simple implementation | Can't distinguish "sent 1000" from "sent 10, failed 990" | Acceptable only with per-email counts stored in `details` JSONB |
| Skipping dry-run mode | Faster to build | First production run risks mass wrong-email event | Never — implement dry-run before first production use |
| Hardcode `List-Unsubscribe` as mailto only | Avoids building unsubscribe feature | Gmail marks as spam at scale; complaint accumulation damages domain | Acceptable for <5,000 emails/day; revisit at scale |
| Use `os.Getenv("ADMIN_API_KEY")` inline per handler | Consistent with existing admin.go pattern | No centralization; changing key requires finding all occurrences | Acceptable if existing pattern is maintained uniformly |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Mailgun domain verification | Adding DNS records and verifying immediately | Wait for TTL propagation (15-30min minimum), verify from external checker |
| Mailgun SMTP auth | Using Mailgun API key as SMTP password | Use per-domain SMTP password from Mailgun → Domain Settings → SMTP credentials |
| Mailgun HTTP API (`mailgun-go`) | Using `v3` package (deprecated) | Use `github.com/mailgun/mailgun-go/v4` |
| `EmailService` wiring in `main.go` | Wiring inside `if pool != nil` block | Email does not require DB; wire unconditionally when `SMTP_HOST` env var is set |
| Existing `EmailConfig` struct | Passing `SMTPPort` as `int` from `Config.SMTPPort` (string) | `strconv.Atoi(cfg.SMTPPort)` required; add parsing step |
| Broadcast recipient query | `SELECT email FROM users` | `SELECT email FROM users WHERE deleted_at IS NULL ORDER BY created_at` |
| `audit_log` table | Using `admin_id` (UUID FK) for API-key-based admin | Store `triggered_by = "api-key"` in `details JSONB`, leave `admin_id = NULL` |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Serial SMTP sends in HTTP handler | 15s `WriteTimeout` hit at ~75 emails (200ms/email) | Return 202 immediately, send async with audit; OR raise timeout for admin routes | When user count exceeds ~75 active users |
| One goroutine per email via `SendEmailAsync` | Goroutine count = user count; OOM on large lists | Sequential loop with delay OR bounded worker pool (3 workers) | When user count exceeds ~500 |
| `SELECT email FROM users` without index hint | Slow on large user table | `idx_users_email` index exists; use `SELECT id, email FROM users WHERE deleted_at IS NULL` — covered by partial index from migration 000030 | Not a problem at current scale; matters at 100k+ users |
| Retry logic with linear backoff on broadcast | 3 retries × N users = 3× send time | Keep retries = 1 for bulk (transient failures are acceptable); save retries for transactional | When broadcast has >100 recipients and retry backoff is >1s |
| Mailgun API timeout set to Go default (forever) | Broadcast goroutine hangs forever on network issue | Set explicit HTTP client timeout: `&http.Client{Timeout: 10 * time.Second}` on Mailgun client | Any network instability |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| `ADMIN_API_KEY` compared with `==` (timing attack) | Theoretical brute-force via timing oracle | Use `subtle.ConstantTimeCompare` — existing `admin.go` uses `==`, this should be patched when adding email broadcast |
| Admin API key in shell history | Key exposure | Source from `.env`, never pass as CLI argument |
| No rate limit on admin broadcast endpoint | Admin script misconfiguration loops and sends 1000x | Add idempotency key / `broadcast_id` check; one active broadcast at a time |
| Sending HTML without sanitization | Admin-controlled, so XSS risk is low, but if body is logged in audit table, it could be rendered unsafely in a future admin UI | Store `body_preview` (first 200 chars, stripped) in audit log, not full HTML |
| Mailgun API key stored in env, same as other secrets | Key rotation requires deployment | Document key rotation procedure; Mailgun supports multiple API keys per domain |
| `DESTRUCTIVE_QUERIES=true` left enabled in production | Any SQL writable via admin endpoint | Review env vars before enabling email; unrelated but lives in same admin context |

---

## "Looks Done But Isn't" Checklist

- [ ] Mailgun domain shows "Active" in dashboard — but SPF/DKIM not verified yet (can take 24-48h)
- [ ] `go test ./...` passes — but email tests use mock SMTP, not real Mailgun API
- [ ] Broadcast endpoint returns `200 OK` — but emails are still queued in Mailgun, not yet delivered
- [ ] Audit log entry created — but `sent_count` field is 0 because it was written before the loop completed
- [ ] `smtp.go` compiles with `quoted-printable` header — but encoding is not actually applied to the body
- [ ] `EmailService` is instantiated in `main.go` — but `AdminHandler.emailService` field is nil because `SetEmailService()` was not called (same pattern as `SetTranslationJobRunner`)
- [ ] DNS records added in registrar — but old records not removed, causing SPF duplicate
- [ ] Mailgun sandbox mode test passed — but sandbox only allows sending to verified recipients, not all users
- [ ] Subject line renders correctly in email preview tool — but special characters (`&`, `<`, `>`) break in some clients because HTML-encoded in subject header
- [ ] First broadcast sent successfully to 10 test users — but `List-Unsubscribe` header missing for real campaign
- [ ] Broadcast handler added to router — but not protected by `checkAdminAuth` (forgetting to call it, as the pattern in `admin.go` is manual per-handler)

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|-----------------|--------------|
| SPF record conflict | DNS setup | `dig TXT solvr.dev` returns exactly 1 SPF record; Mailgun dashboard shows SPF verified |
| DKIM selector collision | DNS setup | `dig TXT pic._domainkey.solvr.dev` returns Mailgun's key |
| DNS propagation testing too early | DNS setup | Wait 15+ min; verify from mxtoolbox.com before marking complete |
| DMARC misalignment (SMTP Return-Path) | Provider decision | Decision: use Mailgun HTTP API (not SMTP) to get automatic alignment |
| Wrong Mailgun SMTP credentials | Provider integration | Use per-domain SMTP password, not API key; or use HTTP API and skip SMTP entirely |
| Broadcast blocking HTTP timeout | Broadcast handler design | Response strategy decided (202 + job or extended timeout for admin routes) before coding |
| Mailgun rate limiting | Broadcast handler implementation | `BROADCAST_DELAY_MS` configurable; 429 responses logged distinctly |
| nil EmailService panic | Wiring phase | `checkEmailService` guard returns 503 if nil, same as `translationJobRunner` pattern |
| `SendEmailAsync` goroutine leak | Broadcast handler implementation | Broadcast uses sequential loop, not `SendEmailAsync` |
| Audit log missing per-email results | DB schema phase | New `email_broadcasts` table or `details JSONB` with `sent_count`/`failed_count` |
| Sending to soft-deleted users | Broadcast handler SQL | Query reviewed: `WHERE deleted_at IS NULL` confirmed |
| Admin key in shell history | CLI skill implementation | Skill reads `$ADMIN_API_KEY` from env, never hardcoded |
| No confirmation before mass send | API design phase | Dry-run mode or two-step confirm before first production broadcast |
| QP encoding bug in smtp.go | Provider integration | Use Mailgun HTTP API (bypasses smtp.go entirely) OR fix encoding with `mime/quotedprintable` |
| Missing List-Unsubscribe header | Broadcast template design | Header present in all broadcast emails; text footer included |

---

## Sources

- RFC 7208 (SPF): 10 DNS lookup limit per evaluation
- RFC 6376 (DKIM): selector uniqueness per domain
- Mailgun documentation: Domain verification, SMTP credentials vs API keys, sending limits
- Gmail Bulk Sender Guidelines (2024): List-Unsubscribe requirement for >5,000/day senders
- Go standard library: `net/smtp`, `mime/quotedprintable`, `crypto/subtle`
- Existing codebase analysis: `backend/internal/services/email.go`, `smtp.go`, `config/env.go`, `api/handlers/admin.go`, `cmd/api/main.go`, `migrations/000012_create_audit_log.up.sql`

---
*Pitfalls research for: email infrastructure (Mailgun + DNS for Go backend)*
*Researched: 2026-03-17*
