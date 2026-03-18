# Phase 2 Research: Backend Foundation

**Researched:** 2026-03-17
**Confidence:** HIGH

---

## Existing Patterns

### UserRepository Pattern

File: `backend/internal/db/users.go` (~535 lines, well under limit)

Key observations for `ListActiveEmails`:

- All existing list methods use `WHERE deleted_at IS NULL` as the soft-delete filter
- Queries select specific columns, never `SELECT *`
- The existing `List()` method shows the pagination+count pattern; `ListActiveEmails` needs no pagination (small dataset, ~100 users)
- Nullable fields use `sql.NullString` with conversion after scan: `user.AuthProvider = authProvider.String`
- For `ListActiveEmails`, the target columns (`id`, `email`, `display_name`) are all NOT NULL in the schema — no NullString needed
- `scanUser()` private helper pattern: used for single-row queries. Multi-row uses inline scan in `rows.Next()` loop
- Error handling: `errors.Is(err, pgx.ErrNoRows)` → return `ErrNotFound`; `LogQueryError(ctx, ...)` for other errors

**Pattern for ListActiveEmails:**
```go
func (r *UserRepository) ListActiveEmails(ctx context.Context) ([]EmailRecipient, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT id, email, display_name
        FROM users
        WHERE deleted_at IS NULL
        ORDER BY created_at ASC
    `)
    if err != nil {
        LogQueryError(ctx, "ListActiveEmails", "users", err)
        return nil, err
    }
    defer rows.Close()

    var recipients []EmailRecipient
    for rows.Next() {
        var r EmailRecipient
        if err := rows.Scan(&r.ID, &r.Email, &r.DisplayName); err != nil {
            LogQueryError(ctx, "ListActiveEmails.Scan", "users", err)
            return nil, err
        }
        recipients = append(recipients, r)
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }
    if recipients == nil {
        recipients = []EmailRecipient{}
    }
    return recipients, nil
}
```

`EmailRecipient` is a new small struct — define it in `backend/internal/models/` or inline in the db package. Given it's used across db and handlers, `models/email.go` is cleaner.

### EmailBroadcastRepository Pattern

Model: `backend/internal/db/service_checks.go` (~140 lines)

- Constructor: `NewXxxRepository(pool *Pool) *XxxRepository`
- `pool *Pool` embedded in struct
- All methods take `ctx context.Context` as first argument
- Simple INSERT uses `pool.Exec()`; queries returning rows use `pool.Query()` or `pool.QueryRow()`
- Error wrapping: `fmt.Errorf("insert email broadcast: %w", err)`
- Nullable columns use `*string`, `*time.Time` (pointer semantics, not sql.NullXxx) — see `ServiceCheck.ResponseTimeMs *int`, `ErrorMessage *string`

For `email_broadcast_logs`, the `completed_at TIMESTAMPTZ` is nullable → use `*time.Time` in model.

### Service Wiring Pattern

File: `backend/internal/api/router.go` lines 117–129

The exact pattern for optional services:
```go
if groqKey := os.Getenv("GROQ_API_KEY"); groqKey != "" && pool != nil {
    // create repos and service
    adminHandler.SetTranslationJobRunner(translationJob)
}
r.Post("/admin/jobs/translation/run", adminHandler.RunTranslationJob)
```

Key points:
- Route registration happens OUTSIDE the `if` block — the route is always registered
- Service/repo creation happens INSIDE the `if` block — conditional on env var
- Setter method on handler (`SetTranslationJobRunner`) injects dependency after handler creation
- Handler checks for nil and returns 503 with error code (see `RunTranslationJob`: `if h.translationJobRunner == nil { writeAdminError(w, 503, "TRANSLATION_NOT_CONFIGURED", ...) }`)

For Phase 2 we follow this exactly:
```go
if resendKey := os.Getenv("RESEND_API_KEY"); resendKey != "" && pool != nil {
    resendClient := services.NewResendClient(resendKey, cfg.FromEmail)
    broadcastRepo := db.NewEmailBroadcastRepository(pool)
    adminHandler.SetResendClient(resendClient)
    adminHandler.SetEmailBroadcastRepo(broadcastRepo)
}
// Routes registered in Phase 3
```

Phase 2 only wires; Phase 3 adds the routes and handler methods.

### AdminHandler Pattern

File: `backend/internal/api/handlers/admin.go` (386 lines — well under limit)

- `AdminHandler` struct holds `pool *db.Pool` + optional injected dependencies (interfaces)
- `TranslationJobRunner` is a local interface in `handlers/admin.go` to avoid import cycles
- `SetTranslationJobRunner(runner TranslationJobRunner)` is the setter
- `checkAdminAuth(w, r)` is a private helper returning bool — reuse for all new admin endpoints
- `writeAdminJSON(w, status, data)` and `writeAdminError(w, status, code, message)` are private helpers — reuse these

**New fields to add to AdminHandler in Phase 3** (not Phase 2 — Phase 2 only creates the services/repos):
- `emailSender EmailSender` (interface defined in handlers package)
- `emailBroadcastRepo EmailBroadcastRepository` (interface)
- `userEmailRepo UserEmailRepository` (interface)

### Config Pattern

File: `backend/internal/config/env.go`

- `Config` struct holds all env vars as strings
- `FROM_EMAIL` already loaded: `cfg.FromEmail = getEnvOrDefault("FROM_EMAIL", "noreply@solvr.dev")`
- SMTP vars already loaded (dead code uses them)
- `RESEND_API_KEY` is NOT yet in Config struct — needs to be added as an optional field
- Pattern for adding: `cfg.ResendAPIKey = os.Getenv("RESEND_API_KEY")` (no default, optional)
- Config is passed into `NewRouter()` indirectly — router reads env vars directly with `os.Getenv()` (see line 117: `if groqKey := os.Getenv("GROQ_API_KEY")`)
- For `FROM_EMAIL`, config already has it but router.go reads it via `os.Getenv` directly too. Best to read `os.Getenv("FROM_EMAIL")` in router.go for consistency, with fallback to `"noreply@solvr.dev"`

**Decision:** Do NOT add `RESEND_API_KEY` to `Config` struct. Router.go reads it directly with `os.Getenv("RESEND_API_KEY")` — consistent with how `GROQ_API_KEY` is handled. This keeps main.go untouched.

### Migration Pattern

Latest migration: `000068_bump_translation_max_attempts` (confirmed by directory listing)

Next migration number: **000069**

Naming convention: `{6-digit-number}_{snake_case_description}.{up|down}.sql`

New migration files:
- `backend/migrations/000069_create_email_broadcast_logs.up.sql`
- `backend/migrations/000069_create_email_broadcast_logs.down.sql`

### Test Patterns

Two patterns coexist in this codebase:

**Pattern A — `getTestPool` helper (users_test.go):**
```go
pool := getTestPool(t)
if pool == nil {
    t.Skip("Skipping test: no database connection available")
}
defer cleanupTestDB(t, pool)
```
`getTestPool` calls `os.Getenv("DATABASE_URL")` internally. Used for most tests in the db package.

**Pattern B — Inline DATABASE_URL check (service_checks_test.go, users_test.go larger tests):**
```go
databaseURL := os.Getenv("DATABASE_URL")
if databaseURL == "" {
    t.Skip("DATABASE_URL not set, skipping integration test")
}
pool, err := NewPool(ctx, databaseURL)
```
Used for tests that need explicit control over pool lifecycle.

**Both patterns skip when `DATABASE_URL` is not set** — tests are silent in CI if DB not available, which is consistent.

**Setup helper pattern:**
```go
func setupEmailBroadcastTest(t *testing.T) (*Pool, *EmailBroadcastRepository) {
    t.Helper()
    databaseURL := os.Getenv("DATABASE_URL")
    if databaseURL == "" {
        t.Skip("DATABASE_URL not set, skipping integration test")
    }
    pool, err := NewPool(context.Background(), databaseURL)
    if err != nil {
        t.Fatalf("failed to connect: %v", err)
    }
    _, _ = pool.Exec(context.Background(), "DELETE FROM email_broadcast_logs WHERE subject LIKE 'test_%'")
    return pool, NewEmailBroadcastRepository(pool)
}
```

**Unit tests for services** (no DB) — mock the HTTP client or use `httptest.NewServer`. See services test folder.

---

## Resend Go SDK v3

Package: `github.com/resend/resend-go/v2`

**IMPORTANT NOTE:** The ROADMAP.md says "resend-go/v2" in one place and "resend-go/v3" in another. The user confirmed v3. Check the actual package name:
- `github.com/resend/resend-go/v2` → this is the v2 import path (stable, documented)
- The Roadmap Success Criterion says `resend-go/v2` compiled into binary

**Resolution:** Use `github.com/resend/resend-go/v2` — this is the current stable SDK that the ROADMAP explicitly calls out. The "v3" in the prompt is likely a mistake; the ROADMAP note says `/v2`.

**SDK API surface (resend-go/v2):**
```go
import "github.com/resend/resend-go/v2"

client := resend.NewClient("RESEND_API_KEY")

params := &resend.SendEmailRequest{
    From:    "Solvr <noreply@solvr.dev>",
    To:      []string{"user@example.com"},
    Subject: "Hello",
    Html:    "<p>Hello</p>",
    Text:    "Hello",  // optional
    // Headers can be set for List-Unsubscribe
}

sent, err := client.Emails.Send(params)
if err != nil {
    // err contains Resend API error details
    return fmt.Errorf("resend send failed: %w", err)
}
// sent.Id is the Resend message ID
```

**Error handling:** The SDK returns structured errors. Wrap with `fmt.Errorf` for caller visibility.

**Testing the ResendClient without hitting Resend API:**
Use `net/http/httptest.NewServer` to mock the Resend API endpoint. The ResendClient should accept a configurable base URL (or use the default `https://api.resend.com`). For unit tests, override the base URL to point at a test server.

Alternative: define `EmailSender` interface and mock it entirely — the ResendClient unit test verifies the HTTP request shape, not the business logic.

---

## Implementation Decisions

### EmailSender Interface

Define in `backend/internal/api/handlers/admin.go` (like `TranslationJobRunner`) to avoid import cycles. Handlers → services is fine; services → handlers would be a cycle.

```go
// EmailSender is the interface for sending a single email.
// Minimal surface: one method, context-aware, no implementation details exposed.
type EmailSender interface {
    Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}
```

`from` address is baked into the implementation (loaded once from `FROM_EMAIL` env var at construction time), not passed per-call. This matches how `EmailService.fromEmail` works in the dead code.

### ResendClient

New file: `backend/internal/services/resend.go`

```go
type ResendClient struct {
    client    *resend.Client
    fromEmail string
}

func NewResendClient(apiKey, fromEmail string) *ResendClient { ... }

func (c *ResendClient) Send(ctx context.Context, to, subject, htmlBody, textBody string) error { ... }
```

`ResendClient` satisfies `EmailSender` interface (defined in handlers, but ResendClient doesn't import handlers — it just structurally satisfies the interface). Go interfaces are implicit.

**List-Unsubscribe header**: Add at minimum `List-Unsubscribe: <mailto:unsubscribe@solvr.dev>` via `resend.SendEmailRequest.Headers` map. Required for Gmail bulk sender compliance.

### EmailBroadcastRepository

New file: `backend/internal/db/email_broadcast.go`

Model: `backend/internal/models/email_broadcast.go`

```go
type EmailBroadcast struct {
    ID               string     `json:"id"`
    Subject          string     `json:"subject"`
    BodyHTML         string     `json:"body_html"`
    BodyText         string     `json:"body_text,omitempty"`
    TotalRecipients  int        `json:"total_recipients"`
    SentCount        int        `json:"sent_count"`
    FailedCount      int        `json:"failed_count"`
    Status           string     `json:"status"`
    StartedAt        time.Time  `json:"started_at"`
    CompletedAt      *time.Time `json:"completed_at,omitempty"`
    CreatedAt        time.Time  `json:"created_at"`
}
```

Repository methods:
- `CreateLog(ctx, *EmailBroadcast) (*EmailBroadcast, error)` — INSERT, RETURNING id + timestamps
- `UpdateStatusAndCounts(ctx, id, status string, sentCount, failedCount int, completedAt *time.Time) error` — UPDATE
- `List(ctx) ([]EmailBroadcast, error)` — SELECT ORDER BY started_at DESC (no pagination for v1)

### ListActiveEmails

Added to `UserRepository` in `backend/internal/db/users.go`.

Returns `[]models.EmailRecipient` — a new minimal struct:
```go
// EmailRecipient holds the minimum user data needed for email broadcasts.
type EmailRecipient struct {
    ID          string `json:"id"`
    Email       string `json:"email"`
    DisplayName string `json:"display_name"`
}
```

Query: `SELECT id, email, display_name FROM users WHERE deleted_at IS NULL ORDER BY created_at ASC`

This is intentionally minimal — no `SELECT *`, no joins, no subqueries.

---

## File Plan

### New Files (Phase 2)

| File | Purpose | ~Lines |
|------|---------|--------|
| `backend/migrations/000069_create_email_broadcast_logs.up.sql` | Table DDL | ~20 |
| `backend/migrations/000069_create_email_broadcast_logs.down.sql` | DROP TABLE | ~3 |
| `backend/internal/models/email.go` | EmailRecipient + EmailBroadcast models | ~30 |
| `backend/internal/db/email_broadcast.go` | EmailBroadcastRepository | ~80 |
| `backend/internal/db/email_broadcast_test.go` | Integration tests for repo | ~100 |
| `backend/internal/services/resend.go` | ResendClient implementing EmailSender | ~60 |
| `backend/internal/services/resend_test.go` | Unit tests with httptest mock server | ~80 |

### Modified Files (Phase 2)

| File | Change | Current Lines |
|------|--------|---------------|
| `backend/internal/db/users.go` | Add `ListActiveEmails()` method | 535 → ~560 |
| `backend/internal/db/users_test.go` | Add `TestUserRepository_ListActiveEmails` | 1325 → ~1370 |

**No changes to:** `main.go`, `router.go`, `admin.go`, `env.go`

Router wiring and `EmailSender` interface definition are Phase 3 concerns.

**Exception:** `router.go` conditionally wires the ResendClient + repo into adminHandler — this could be Phase 2 (wiring without routes) or Phase 3 (routes + wiring together). Roadmap places it in Phase 3 to keep Phase 2 focused on infrastructure. Confirm this before planning.

---

## Risks

1. **resend-go/v2 vs v3 confusion:** ROADMAP.md says `/v2` in the success criteria, but the prompt says "user confirmed v3". Verify by checking the Resend GitHub releases. If v3 is `github.com/resend/resend-go/v3`, the import path changes. Do NOT assume — check `go get github.com/resend/resend-go/v2` vs `/v3` before coding.

2. **users.go file size:** Currently 535 lines. Adding `ListActiveEmails` (~25 lines) brings it to ~560 — still under the 900-line limit. No split needed.

3. **email_broadcast_logs not in cleanupTestDB:** The `cleanupTestDB` helper in `users_test.go` cleans specific tables. If `email_broadcast_log` tests use the shared helper, add `DELETE FROM email_broadcast_logs` to it — OR use isolated setup/teardown in the new test file (recommended to avoid coupling).

4. **Migration 000069 conflict:** If another migration 000069 was added between the directory listing and implementation, the migration tool will fail with a duplicate. Always run `ls migrations/ | tail -3` immediately before creating the new migration file.

5. **FROM_EMAIL env var:** Already loaded by `config/env.go` with default `noreply@solvr.dev`. The ResendClient constructor receives it via parameter — no need to re-read from env inside the service. Wire it in router.go: `fromEmail := os.Getenv("FROM_EMAIL"); if fromEmail == "" { fromEmail = "noreply@solvr.dev" }`.

6. **`go.mod` needs updating:** `resend-go/v2` is not in `go.mod`. Must run `go get github.com/resend/resend-go/v2` (or v3) and commit the updated `go.mod` + `go.sum`.

---

## Validation Architecture

The Phase 2 success criteria define exactly what to verify:

### Criterion 1: DB Integration Test
`go test ./internal/db/... -run TestEmailBroadcastRepository` must pass with `DATABASE_URL` set.
- Seeds: INSERT into `email_broadcast_logs` via `CreateLog()`
- Reads: SELECT back via `List()` and verify fields match
- Updates: call `UpdateStatusAndCounts()`, verify status changed
- Cleanup: DELETE test rows by subject LIKE 'test_%'

### Criterion 2: ResendClient Unit Test
`go test ./internal/services/... -run TestResendClient` must pass without network.
- Setup: `httptest.NewServer` that validates request shape
- Assert: Authorization header = `Bearer <api_key>`, body contains `from` field matching `FROM_EMAIL`
- Mock server returns Resend-shaped JSON `{"id":"test123"}`
- Assert: `Send()` returns nil error

### Criterion 3: ListActiveEmails Unit Test
`go test ./internal/db/... -run TestUserRepository_ListActiveEmails` must pass with `DATABASE_URL`.
- Setup: create 1 active user + 1 soft-deleted user
- Assert: `ListActiveEmails()` returns exactly 1 recipient (the active one)
- Assert: the deleted user's email is NOT in the result
- Cleanup: hard-delete test users

### Criterion 4: Migration Up/Down
```bash
migrate -path backend/migrations -database "$DATABASE_URL" up
migrate -path backend/migrations -database "$DATABASE_URL" down 1
```
Both must succeed without error. The down migration must be `DROP TABLE IF EXISTS email_broadcast_logs`.

### Criterion 5: Build Compiles
`cd backend && go build ./cmd/api` must succeed with the resend-go package imported.
Verifies: import path is correct, all interfaces satisfied, no compile errors.

### Nyquist Validation Notes (for Phase 3 planner)

Phase 3 depends on these Phase 2 contracts:

1. **`EmailSender` interface location:** Will be defined in `handlers/admin.go` (Phase 3) — Phase 2 `ResendClient` satisfies it structurally (no explicit declaration needed until Phase 3).

2. **`EmailBroadcastRepository` interface:** Phase 3 will define a minimal interface in `handlers/admin.go` matching the concrete `EmailBroadcastRepository` methods. The methods must be: `CreateLog(ctx, *models.EmailBroadcast) (*models.EmailBroadcast, error)` and `UpdateStatusAndCounts(ctx, id, status string, sentCount, failedCount int, completedAt *time.Time) error`.

3. **`UserEmailRepository` interface:** Phase 3 needs `ListActiveEmails(ctx) ([]models.EmailRecipient, error)` — this method signature must be stable from Phase 2.

4. **Model field names:** Phase 3 handler will access `EmailBroadcast.ID`, `.TotalRecipients`, `.SentCount`, `.FailedCount`, `.Status` — these must match what Phase 2 implements.

---

*Phase 2 research completed: 2026-03-17*
