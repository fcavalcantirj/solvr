## Current Position

Phase: Phase 6 — Plan 06-01 complete
Plan: .planning/ROADMAP.md
Status: Plan 06-01 done — migration 000070, referral package, User model updated, all tests pass
Last activity: 2026-03-17 — Completed 06-01: Migration + Code Generation + Auto-Assign

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-17)

**Core value:** Developers and AI agents can find solutions to programming problems faster than searching the web
**Current focus:** Admin Email System

## Accumulated Context

- EmailService + SMTPClient exist in backend/internal/services/ but are not wired up
- Config already loads SMTP env vars (SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, FROM_EMAIL)
- Admin auth uses X-Admin-API-Key header (inline in handlers, not middleware)
- All users have email (UNIQUE NOT NULL)
- solvr.dev has no email infrastructure yet (no DNS records, no provider)
- Provider is Resend (not Mailgun) — REQUIREMENTS.md specifies RESEND_API_KEY
- Dead code has bugs: quoted-printable encoding bug in smtp.go, SMTPPort type mismatch — do NOT reuse for production
- Build a fresh ResendClient satisfying an EmailSender interface; bypass smtp.go entirely
- HTTP WriteTimeout is 15s — use per-request 5-minute context deadline inside broadcast handler
- Phase 1 is critical path (DNS propagation 24–48h) — start before code work
- ResendClient in backend/internal/services/resend.go wraps resend-go/v3 SDK
- SetBaseURL(url) method enables test injection via httptest (BaseURL is *url.URL in SDK)
- From field formatted as "Solvr <fromEmail>" per RFC 5322 display name convention
