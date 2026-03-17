# Roadmap: Solvr Admin Email System

**Milestone:** v1.0
**Created:** 2026-03-17
**Phases:** 5
**Requirements:** 15

---

## Phase 1: DNS + Resend Infrastructure

**Goal:** Establish verified email sending infrastructure for solvr.dev with zero code changes.
**Requirements:** INFRA-01, INFRA-02

### Success Criteria

1. Resend account is created and solvr.dev domain is added with SPF and DKIM DNS records live and validated by Resend's dashboard.
2. A test email sent from the Resend dashboard reaches a real inbox without landing in spam, confirming end-to-end deliverability.
3. `RESEND_API_KEY` is available in the production environment (`.env` or secrets manager) and staging environment.

### Notes

- DNS propagation takes up to 48 hours — start this phase before any code work begins.
- Run `dig TXT solvr.dev` before touching DNS to check for existing SPF records; merge rather than add a second TXT record (RFC 7208 prohibits multiple SPF records).
- Run `dig TXT pic._domainkey.solvr.dev` to check for DKIM selector conflicts before adding Mailgun's DKIM.
- Resend DNS records required: SPF (`v=spf1 include:amazonses.com ~all` per Resend docs), DKIM (Resend-generated TXT on `resend._domainkey.solvr.dev`), and optionally DMARC (`_dmarc.solvr.dev` — start with `p=none`).
- INFRA-01 and INFRA-02 are purely manual admin actions; no Go code is written in this phase.

---

## Phase 2: Backend Foundation — DB Migration + Resend Client + ListActiveEmails

**Goal:** Build the complete Go backend foundation: database table, Resend HTTP client, and user email query.
**Requirements:** INFRA-03, EMAIL-04, EMAIL-05, AUDIT-01

### Success Criteria

1. Running `go test ./internal/db/...` passes, including an integration test that inserts a row into `email_broadcast_logs` and reads it back.
2. Running `go test ./internal/services/...` passes, including a unit test where a mock Resend HTTP server receives a well-formed API request with the correct `Authorization: Bearer` header and `from` field matching `FROM_EMAIL`.
3. `UserRepository.ListActiveEmails()` returns only non-deleted users (those with `deleted_at IS NULL`) — verified by a unit test that seeds one active and one deleted user, then asserts only the active user is returned.
4. Migration `000069_create_email_broadcast_logs` runs `up` and `down` without error against a local PostgreSQL instance.
5. The API server starts without error when `RESEND_API_KEY` is set, and the `resend-go/v2` package is compiled into the binary (verified by `go build ./cmd/api`).

### Notes

- Use `resend-go/v2` SDK (`github.com/resend/resend-go/v2`) or direct HTTP to `api.resend.com` — SDK preferred for error handling.
- Existing `EmailService`/`SMTPClient` are dead code with bugs (quoted-printable encoding bug, SMTPPort type mismatch). Do NOT reuse `smtp.go` for production. Build a fresh `ResendClient` struct that satisfies a new `EmailSender` interface.
- `EmailSender` interface should be minimal: `Send(ctx, to, subject, htmlBody, textBody string) error`.
- `email_broadcast_logs` schema: `id UUID PK`, `subject TEXT NOT NULL`, `body_html TEXT NOT NULL`, `body_text TEXT`, `total_recipients INT NOT NULL`, `sent_count INT NOT NULL DEFAULT 0`, `failed_count INT NOT NULL DEFAULT 0`, `status VARCHAR(20) NOT NULL DEFAULT 'sending'`, `started_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `completed_at TIMESTAMPTZ`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`.
- `EmailBroadcastRepository` needs: `CreateLog()`, `UpdateStatusAndCounts()`, `List()` — all in a new file `backend/internal/db/email_broadcast.go`.
- Sender address `noreply@solvr.dev` is controlled by `FROM_EMAIL` env var (already loaded by `config/env.go`).
- `ListActiveEmails` must query only the `users` table (not agents), selecting only `id`, `email`, `display_name` — no `SELECT *`.
- AUDIT-01 is included here because the `email_broadcast_logs` table and `CreateLog()`/`UpdateStatusAndCounts()` methods are the audit mechanism — they must exist before the handler can use them.

---

## Phase 3: Admin Broadcast Handler + Dry-Run

**Goal:** Implement the protected HTTP handler for broadcast sends, including dry-run preview mode, wired into the router.
**Requirements:** EMAIL-01, EMAIL-02, EMAIL-03, EMAIL-06

### Success Criteria

1. `POST /admin/email/broadcast` with a valid `X-Admin-API-Key` header returns `200` with `{ "broadcast_id": "...", "sent": N, "failed": 0, "total": N, "duration_ms": ... }`.
2. `POST /admin/email/broadcast` without or with an invalid `X-Admin-API-Key` header returns `401` — verified by handler unit test.
3. `POST /admin/email/broadcast` with `"dry_run": true` returns `{ "would_send": N, "recipients": [...] }` and does NOT call the email sender — verified by a mock `EmailSender` that asserts zero calls.
4. `POST /admin/email/broadcast` with missing `subject` or `body_html` returns `400` with a descriptive error — verified by handler unit test.
5. When `emailService` is nil (email not configured), the endpoint returns `503` with `{ "error": "EMAIL_NOT_CONFIGURED" }`.

### Notes

- Extend existing `AdminHandler` struct in `backend/internal/api/handlers/admin.go` (currently ~386 lines; adding ~100 lines stays under 900-line limit).
- Add `emailService EmailSender`, `emailBroadcastRepo EmailBroadcastRepo`, and `userEmailRepo UserEmailRepo` fields (interfaces) to `AdminHandler`.
- Add `SetEmailService()` and `SetEmailBroadcastRepo()` setters following the existing `SetTranslationJobRunner()` pattern.
- Broadcast loop must be synchronous and sequential — do NOT use `SendEmailAsync` (causes goroutine leak + Resend 429s). Failed individual sends are logged and counted; broadcast continues.
- HTTP `WriteTimeout` is 15s. At ~100 users, synchronous is safe today. Use a per-request context with a 5-minute deadline inside the handler to avoid connection drop during broadcast.
- Add `List-Unsubscribe` header (at minimum a `mailto:` value) to all broadcast emails to satisfy Gmail 2024 bulk sender requirements and prevent Resend domain suspension.
- Wire in `router.go` behind `if resendKey := os.Getenv("RESEND_API_KEY"); resendKey != ""` guard — consistent with how optional services are wired.
- Dry-run response must include a recipient list so the admin can verify targeting before live send.

---

## Phase 4: Email Audit History Endpoint

**Goal:** Expose past broadcast records via a queryable history endpoint so the admin can review what was sent.
**Requirements:** AUDIT-02

### Success Criteria

1. `GET /admin/email/history` with a valid `X-Admin-API-Key` header returns a JSON array of past broadcasts, each including `broadcast_id`, `subject`, `sent_count`, `failed_count`, `status`, `started_at`, and `completed_at`.
2. `GET /admin/email/history` without a valid `X-Admin-API-Key` returns `401`.
3. After a broadcast completes, its record appears in `GET /admin/email/history` with the correct `sent_count`, `failed_count`, and `status: "completed"` — verified by an integration test or handler test that seeds a log row.

### Notes

- `List()` method on `EmailBroadcastRepository` is already built in Phase 2 — this phase only adds the handler method and route registration.
- Add `ListBroadcasts(w, r)` to `AdminHandler` in `admin.go`.
- Register `GET /admin/email/history` in `router.go` alongside the broadcast route.
- Default to descending sort by `started_at` — most recent first.
- No pagination required for v1 (broadcasts are infrequent admin-only actions).

---

## Phase 5: Admin CLI Skill (solvr-admin.sh)

**Goal:** Provide a Claude Code-callable bash skill for email broadcasts, wrapping the HTTP API with proper credential handling.
**Requirements:** TOOL-01, TOOL-02, TOOL-03, TOOL-04

### Success Criteria

1. Running `solvr-admin email send --subject "Test" --body "Hello"` calls `POST /admin/email/broadcast` and prints a human-readable summary (`Sent: N, Failed: 0`).
2. Running `solvr-admin email dry-run --subject "Test" --body "Hello"` calls the endpoint with `"dry_run": true` and prints the recipient count and email list without sending.
3. Running `solvr-admin email history` calls `GET /admin/email/history` and prints a formatted table of past broadcasts.
4. The skill reads `ADMIN_API_KEY` from the environment variable `ADMIN_API_KEY` and, if not set, falls back to `~/.config/solvr/admin-credentials.json` — the key is never printed, logged, or passed as a CLI positional argument.
5. If `ADMIN_API_KEY` is not found in either location, the script exits with a clear error message and non-zero exit code.

### Notes

- Place at `.claude/skills/solvr/scripts/solvr-admin.sh` (follows the existing `solvr.sh` skill location).
- Follow the existing `solvr.sh` pattern: `api_call()` wrapper function, subcommand dispatch via `case`, `jq` for JSON parsing and formatting.
- The `--body` flag accepts plain text; the script sends it as `body_text` and also wraps it in `<p>` tags for `body_html` (or admin can pass `--body-html` for rich HTML).
- Never expose `ADMIN_API_KEY` in shell history — pass via header, not as a URL parameter.
- Phase 5 depends on Phase 3's endpoint being deployed to `api.solvr.dev`. Test against production after deployment.

---

## Phase Summary

| # | Phase | Goal | Requirements | Criteria |
|---|-------|------|--------------|----------|
| 1 | DNS + Resend Infrastructure | Verified email sending for solvr.dev | INFRA-01, INFRA-02 | 3 |
| 2 | Backend Foundation | DB migration + Resend client + user query | INFRA-03, EMAIL-04, EMAIL-05, AUDIT-01 | 5 |
| 3 | Admin Broadcast Handler + Dry-Run | Protected HTTP broadcast endpoint | EMAIL-01, EMAIL-02, EMAIL-03, EMAIL-06 | 5 |
| 4 | Email Audit History Endpoint | Past broadcast history via HTTP | AUDIT-02 | 3 |
| 5 | Admin CLI Skill | solvr-admin.sh for Claude Code | TOOL-01, TOOL-02, TOOL-03, TOOL-04 | 5 |

## Coverage

All 15 requirements mapped. No gaps.

| Requirement | Phase |
|-------------|-------|
| INFRA-01 | 1 |
| INFRA-02 | 1 |
| INFRA-03 | 2 |
| EMAIL-01 | 3 |
| EMAIL-02 | 3 |
| EMAIL-03 | 3 |
| EMAIL-04 | 2 |
| EMAIL-05 | 2 |
| EMAIL-06 | 3 |
| AUDIT-01 | 2 |
| AUDIT-02 | 4 |
| TOOL-01 | 5 |
| TOOL-02 | 5 |
| TOOL-03 | 5 |
| TOOL-04 | 5 |
