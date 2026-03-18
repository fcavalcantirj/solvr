# Phase 6: Referral Codes + DB Foundation - Context

**Gathered:** 2026-03-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Add referral infrastructure to the database: `referral_code` column on users, `referrals` tracking table, backfill existing users with unique codes, and expose `GET /v1/users/me/referral` API endpoint.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — pure infrastructure phase.

Key constraints from ROADMAP:
- 8-char alphanumeric codes using `crypto/rand` with charset `[A-Z0-9]`
- Backfill in migration itself (DO $$...$$) for atomicity
- `referrals` table: id UUID PK, referrer_id FK, referred_id UNIQUE FK, created_at
- `GET /v1/users/me/referral` counts referrals WHERE referrer_id = caller
- New user registration must auto-generate referral_code

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `backend/internal/db/users.go` — UserRepository with Create, FindByID, ListActiveEmails patterns
- `backend/internal/api/handlers/admin.go` — handler patterns, admin auth
- `backend/internal/api/router.go` — route registration patterns
- `backend/internal/auth/middleware.go` — JWT auth middleware for /v1/ routes
- Migration pattern: `backend/migrations/000069_create_email_broadcast_logs.{up,down}.sql`

### Established Patterns
- Repositories: struct with pool, constructor NewXxxRepository(pool)
- Handlers follow handler → service → repository pattern
- Auth middleware extracts user ID from JWT claims
- TDD required: tests first, then implementation

### Integration Points
- New migration: 000070 (add referral_code to users + create referrals table)
- New repo: db/referral.go (or extend users.go)
- New handler: handlers/referral.go for GET /v1/users/me/referral
- Route wiring in router.go under authenticated /v1/ routes

</code_context>

<specifics>
## Specific Ideas

No specific requirements — infrastructure phase.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---
*Phase: 06-referral-codes-db-foundation*
*Context gathered: 2026-03-17 via Smart Discuss (infrastructure skip)*
