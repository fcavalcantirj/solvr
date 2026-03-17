# Architecture Research

**Domain:** Admin email integration for Go backend
**Researched:** 2026-03-17
**Confidence:** HIGH

---

## Standard Architecture

### System Overview

```
Admin (Claude Code)
        |
        v
solvr-admin.sh (bash skill)
        |
        | POST /admin/email/broadcast
        | X-Admin-API-Key header
        v
AdminEmailHandler.BroadcastEmail()
        |
        | checkAdminAuth()  (existing helper, reused)
        |
        +---> UserRepository.ListActiveEmails()
        |           |
        |           v
        |       PostgreSQL (users WHERE deleted_at IS NULL)
        |
        +---> for each user email:
        |       EmailService.SendEmail()
        |           |
        |           v
        |       DefaultSMTPClient.Send()
        |           |
        |           v
        |       Mailgun SMTP (smtp.mailgun.org:587)
        |           |
        |           v
        |       User inbox
        |
        +---> EmailBroadcastRepository.CreateLog()
                    |
                    v
                PostgreSQL (email_broadcast_logs table)
```

### Component Responsibilities

| Component | Responsibility | Status |
|-----------|---------------|--------|
| `solvr-admin.sh` | CLI entry point, wraps curl with X-Admin-API-Key auth | NEW |
| `AdminEmailHandler` | HTTP handler for broadcast endpoint, reuses `checkAdminAuth()` | NEW (extend admin.go) |
| `EmailService` | Sends individual email messages, retry logic | EXISTS (dead code, wire it) |
| `DefaultSMTPClient` | SMTP transport, TLS/STARTTLS, Mailgun-compatible | EXISTS (dead code, wire it) |
| `UserRepository.ListActiveEmails()` | Query non-deleted users for email+display_name | NEW method on existing repo |
| `EmailBroadcastRepository` | Insert and query email_broadcast_logs | NEW file: db/email_broadcast.go |
| `email_broadcast_logs` migration | New table for audit log | NEW migration (000069) |
| `config.go` | Already loads SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, FROM_EMAIL | EXISTS (no change) |
| `main.go` | Wire EmailService + SMTPClient + inject into router | MODIFY |
| `router.go` | Register POST /admin/email/broadcast, inject EmailService | MODIFY |

---

## Data Flow

### Email Broadcast Send Flow

```
1. Admin runs:
   solvr-admin.sh broadcast-email \
     --subject "Solvr announcement" \
     --body "Hello users..."

2. Script calls:
   POST https://api.solvr.dev/admin/email/broadcast
   Headers: X-Admin-API-Key: $ADMIN_API_KEY
   Body: {"subject": "...", "body_text": "...", "body_html": "..."}

3. AdminEmailHandler.BroadcastEmail():
   a. checkAdminAuth(w, r)          — reuse existing helper
   b. Parse request JSON
   c. Validate subject + body not empty
   d. userRepo.ListActiveEmails(ctx) — SELECT id, email, display_name FROM users WHERE deleted_at IS NULL
   e. Create broadcast log record    — INSERT INTO email_broadcast_logs (subject, body_text, body_html, total_recipients, status='sending')
   f. Loop over recipients:
      - emailSvc.SendEmail(ctx, &EmailMessage{To: email, Subject: subject, ...})
      - Track sent_count, failed_count
      - On failure: log error, continue (don't abort broadcast)
   g. Update broadcast log record    — UPDATE email_broadcast_logs SET status='completed', sent_count=N, failed_count=M
   h. Return JSON summary:
      {"broadcast_id": "uuid", "total": N, "sent": N, "failed": 0, "duration_ms": 1234}
```

### Audit Log Flow

```
email_broadcast_logs table:
  id            UUID PK
  subject       TEXT NOT NULL
  body_text     TEXT NOT NULL
  body_html     TEXT (optional)
  total_recipients  INT NOT NULL
  sent_count    INT NOT NULL DEFAULT 0
  failed_count  INT NOT NULL DEFAULT 0
  status        VARCHAR(20) NOT NULL DEFAULT 'sending'  -- 'sending', 'completed', 'failed'
  started_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
  completed_at  TIMESTAMPTZ
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()

GET /admin/email/broadcasts endpoint (optional, for history):
  AdminEmailHandler.ListBroadcasts()
  -> emailBroadcastRepo.List(ctx, page, perPage)
  -> returns audit log entries
```

---

## New Components Needed

### New Files

| File | Purpose |
|------|---------|
| `backend/internal/db/email_broadcast.go` | EmailBroadcastRepository: CreateLog, UpdateLog, List |
| `backend/migrations/000069_create_email_broadcast_logs.up.sql` | Create email_broadcast_logs table |
| `backend/migrations/000069_create_email_broadcast_logs.down.sql` | Drop email_broadcast_logs table |
| `cli/solvr-admin.sh` (or `.claude/skills/solvr/scripts/solvr-admin.sh`) | Admin skill script |

### New Methods on Existing Files

| File | New Method | Purpose |
|------|-----------|---------|
| `backend/internal/db/users.go` | `ListActiveEmails(ctx) ([]EmailRecipient, error)` | Fetch active user emails for broadcast |
| `backend/internal/api/handlers/admin.go` | `BroadcastEmail(w, r)` + `ListBroadcasts(w, r)` | New handler methods on existing AdminHandler |

---

## Modified Components

### `backend/internal/api/handlers/admin.go`

- Add `emailService` field to `AdminHandler` struct
- Add `emailBroadcastRepo` field to `AdminHandler` struct
- Add `SetEmailService(svc)` setter (same pattern as `SetTranslationJobRunner`)
- Add `BroadcastEmail(w, r)` handler
- Add `ListBroadcasts(w, r)` handler (optional, for history)

```go
// AdminHandler current
type AdminHandler struct {
    pool                 *db.Pool
    translationJobRunner TranslationJobRunner
}

// AdminHandler after
type AdminHandler struct {
    pool                 *db.Pool
    translationJobRunner TranslationJobRunner
    emailService         EmailSender          // NEW interface
    emailBroadcastRepo   EmailBroadcastRepo   // NEW interface
    userEmailRepo        UserEmailRepo        // NEW interface
}
```

**Use interfaces** (same pattern as `TranslationJobRunner`) to avoid import cycles and enable test mocking.

### `backend/internal/api/router.go`

In the admin section (after line ~130), add:

```go
// Wire email broadcast if SMTP is configured
if smtpHost := os.Getenv("SMTP_HOST"); smtpHost != "" && pool != nil {
    smtpPort, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
    if smtpPort == 0 { smtpPort = 587 }
    emailCfg := &services.EmailConfig{
        SMTPHost:  smtpHost,
        SMTPPort:  smtpPort,
        SMTPUser:  os.Getenv("SMTP_USER"),
        SMTPPass:  os.Getenv("SMTP_PASS"),
        FromEmail: os.Getenv("FROM_EMAIL"),
    }
    smtpClient, err := services.NewDefaultSMTPClient(emailCfg)
    if err == nil {
        emailSvc := services.NewEmailService(smtpClient, emailCfg.FromEmail)
        emailBroadcastRepo := db.NewEmailBroadcastRepository(pool)
        adminHandler.SetEmailService(emailSvc)
        adminHandler.SetEmailBroadcastRepo(emailBroadcastRepo)
    }
}
r.Post("/admin/email/broadcast", adminHandler.BroadcastEmail)
r.Get("/admin/email/broadcasts", adminHandler.ListBroadcasts)
```

### `backend/cmd/api/main.go`

No change needed. The email service wiring happens entirely inside `router.go` (matching the existing translation job runner pattern, which is also wired in `router.go` not `main.go`).

---

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|-------------------|-------|
| Mailgun SMTP | SMTP relay via DefaultSMTPClient (STARTTLS port 587) | Env vars: SMTP_HOST=smtp.mailgun.org, SMTP_USER=postmaster@solvr.dev, SMTP_PASS=mailgun-key |
| PostgreSQL | pgx/v5 pool queries, same as all other repos | email_broadcast_logs table, migration 000069 |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|--------------|-------|
| handler → service | Direct method call via interface `EmailSender` | Injected via `SetEmailService()` setter |
| handler → db (broadcast log) | Direct method call via interface `EmailBroadcastRepo` | Injected via `SetEmailBroadcastRepo()` |
| handler → db (user emails) | Direct method call via interface `UserEmailRepo` OR reuse existing `*db.UserRepository` | Can add method directly to UserRepository |
| admin.go → users.go | admin.go calls `userRepo.ListActiveEmails()` | New method on UserRepository, passed via interface |
| router.go → handlers | Setter injection after `NewAdminHandler()` | Same pattern as `SetTranslationJobRunner()` |

---

## Build Order

Dependencies require this strict order:

```
1. DNS + Mailgun setup (external)
   - Add solvr.dev MX/SPF/DKIM DNS records
   - Create Mailgun account, get SMTP credentials
   - Set SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, FROM_EMAIL

2. Database migration (000069)
   - Create email_broadcast_logs table
   - Run: migrate -path migrations -database "$DATABASE_URL" up

3. db/email_broadcast.go  (depends on migration)
   - EmailBroadcastRepository: CreateLog(), UpdateStatusAndCounts(), List()

4. db/users.go - add ListActiveEmails()  (independent, can parallel with #3)
   - Returns []EmailRecipient{ID, Email, DisplayName}

5. handlers/admin.go - extend AdminHandler  (depends on #3 and #4 interfaces)
   - Add EmailSender, EmailBroadcastRepo, UserEmailRepo interfaces
   - Add SetEmailService(), SetEmailBroadcastRepo() setters
   - Add BroadcastEmail(), ListBroadcasts() handlers

6. router.go - wire EmailService  (depends on #5)
   - Construct SMTPClient + EmailService if SMTP_HOST set
   - Call adminHandler.SetEmailService() + SetEmailBroadcastRepo()
   - Register POST /admin/email/broadcast + GET /admin/email/broadcasts

7. Tests  (depends on #3, #4, #5)
   - handlers/admin_test.go: BroadcastEmail with mock EmailSender
   - db/email_broadcast_test.go: integration tests for audit log
   - db/users_test.go: ListActiveEmails

8. solvr-admin.sh  (depends on #6 being deployed)
   - broadcast-email subcommand
   - Loads ADMIN_API_KEY from env or ~/.config/solvr/admin-credentials.json
   - Calls POST /admin/email/broadcast
   - Shows progress and summary
```

---

## Key Design Decisions

### Extend AdminHandler vs New Handler

**Decision: Extend existing `AdminHandler`.**

Rationale:
- Consistent with how `RunTranslationJob` was added — same struct, new method
- `checkAdminAuth()` is already on `AdminHandler`, reuse it without duplication
- Admin routes are cohesive: query, delete, email all belong together
- File size: `admin.go` is currently 386 lines. Adding ~100 lines for email stays well under 900-line limit.

### Broadcast Strategy: Synchronous One-by-One

**Decision: Iterate users, send one email per user synchronously.**

Rationale:
- No queue/worker infrastructure needed (per PROJECT.md constraint: "Sync send is fine for admin broadcasts")
- SMTP connections are per-send (not batched) in `DefaultSMTPClient.Send()`
- Admin broadcasts are infrequent (announcements only)
- HTTP timeout is 15s — for large user bases, handler needs to set its own longer deadline or use context with 5min timeout
- Failed sends are logged + counted, do NOT abort the broadcast

**Risk:** At scale (10k+ users), a 15s HTTP server write timeout will kill the connection before the loop completes. Mitigation: set a longer timeout in the handler context (5 minutes), or move to a fire-and-start-goroutine pattern that returns a broadcast_id immediately (poll for status). For current user count (~100 based on sitemap), synchronous is fine.

### Audit Log

**Decision: email_broadcast_logs table, not just application logs.**

Rationale:
- Provides queryable history for GET /admin/email/broadcasts
- Matches existing pattern (service_checks table for health checks, search_queries for analytics)
- Two-phase write: INSERT with status='sending' → UPDATE with final counts

---

## Anti-Patterns to Avoid

| Anti-Pattern | Problem | Correct Approach |
|-------------|---------|-----------------|
| Wiring EmailService in `main.go` | Inconsistent with how translation job is wired (router.go handles optional deps) | Wire in `router.go` behind `if smtpHost != ""` guard |
| Sending email in a goroutine without tracking | Broadcast fires-and-forgets, no audit trail | Always write to email_broadcast_logs before+after |
| Hard-coding `DefaultSMTPClient` in `AdminHandler` | Untestable, breaks unit tests | Use `EmailSender` interface, inject via setter |
| Querying all users with `SELECT *` | Loads unnecessary columns | `SELECT id, email, display_name FROM users WHERE deleted_at IS NULL` only |
| Aborting broadcast on first email failure | One bad address kills entire broadcast | Continue on individual failure, track failed_count |
| Reusing existing `sendFunc` closure pattern | `EmailService.sendFunc` is an internal field for testing override, not for dependency injection | Use the public `SendEmail()` method |
| Sending to agents | Agents have optional email; admin broadcast is user-only | `WHERE deleted_at IS NULL AND email IS NOT NULL` on users table only |

---

## Sources

- `/Users/fcavalcanti/dev/solvr/backend/internal/api/handlers/admin.go` — AdminHandler struct, checkAdminAuth(), SetTranslationJobRunner() pattern
- `/Users/fcavalcanti/dev/solvr/backend/internal/api/router.go` — Admin route registration, conditional wiring pattern (lines 107-148)
- `/Users/fcavalcanti/dev/solvr/backend/cmd/api/main.go` — main.go wiring pattern (background jobs, not email)
- `/Users/fcavalcanti/dev/solvr/backend/internal/services/email.go` — EmailService, EmailConfig, EmailMessage, SMTPClient interface
- `/Users/fcavalcanti/dev/solvr/backend/internal/services/smtp.go` — DefaultSMTPClient, STARTTLS/TLS send
- `/Users/fcavalcanti/dev/solvr/backend/internal/config/env.go` — Config struct, SMTP env vars already loaded
- `/Users/fcavalcanti/dev/solvr/backend/internal/db/users.go` — UserRepository, existing methods
- `/Users/fcavalcanti/dev/solvr/.planning/PROJECT.md` — milestone scope, constraints (sync send, no queue)
- `.claude/skills/solvr/scripts/solvr.sh` — skill script pattern (auth, api_call, subcommands)

---

*Architecture research for: admin email integration*
*Researched: 2026-03-17*
