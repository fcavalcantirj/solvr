# Stack Research

**Domain:** Quorum A2A relay merge into Go backend + live search
**Researched:** 2026-04-02
**Confidence:** HIGH

## Context: What Is Being Merged

Quorum is a fully operational Go relay server at `/Users/fcavalcanti/dev/quorum`.
Solvr is a Go API at `/Users/fcavalcanti/dev/solvr/backend`.
Both use Chi v5, pgx/v5, HS256 JWT, and PostgreSQL — the merge is code transplant, not architecture redesign.

The merge has five distinct problems to solve:

1. **DB layer conflict:** Quorum uses sqlc-generated code. Solvr uses raw pgx. Pick one pattern for the merged backend.
2. **SSE:** Quorum has a working SSE handler. Solvr has WriteTimeout=15s set, which kills SSE. Fix the server config.
3. **A2A middleware:** Quorum's `A2AVersionGuard` and `BearerTokenQueryStringGuard` need to land in Solvr unchanged.
4. **Bearer token auth per room:** Quorum's `token` package (SHA-256 hash, constant-time compare) is independent — port directly.
5. **Data migration:** Quorum has 3 migrations (5 tables). Solvr is at migration 000072. Add Quorum's tables as migration 000073+.

---

## Recommended Stack

### Core Technologies — No Changes Needed

| Technology | Current Version | Solvr vs Quorum | Verdict |
|------------|----------------|-----------------|---------|
| Go | 1.25.3 (local) / 1.23 (Solvr go.mod) / 1.25.3 (Quorum go.mod) | Solvr go.mod says `go 1.23.0`. Quorum says `go 1.25.3`. No conflict — Go is forwards-compatible. Solvr go.mod min version only. | Bump Solvr go.mod to `go 1.23` minimum; local toolchain is 1.25.3, no issue. |
| Chi v5 | v5.2.0 (Solvr) / v5.2.5 (Quorum) | Both Chi v5. Quorum is one minor ahead. | Upgrade Solvr's Chi to v5.2.5 to match during merge. |
| pgx/v5 | v5.7.2 (Solvr) / v5.9.1 (Quorum) | Both pgx/v5. Quorum is ahead on patch versions. | Keep Solvr's v5.7.2 or upgrade to v5.9.1. Both wire-compatible. Upgrade recommended. |
| go-chi/cors | v1.2.1 (Solvr) / v1.2.2 (Quorum) | Identical API, patch diff only. | Use v1.2.2. |
| golang-jwt/jwt/v5 | v5.2.1 (Solvr) / v5.3.1 (Quorum) | Both v5, same API surface. | Use v5.3.1 (Quorum's newer). |
| google/uuid | v1.6.0 | Identical in both. | No change. |

### New Dependencies to Add to Solvr

| Library | Version | Purpose | Rationale |
|---------|---------|---------|-----------|
| `github.com/a2aproject/a2a-go` | v0.3.12 | A2A protocol types (`AgentCard`, `AgentCapabilities`, `AgentSkill`) | Quorum's relay handler and discovery handler both use `a2a.AgentCard` from this package. Cannot port code without it. Add to Solvr go.mod. |
| `github.com/go-chi/jwtauth/v5` | v5.4.0 | Chi-native JWT verifier/authenticator middleware | Quorum uses `jwtauth.Verifier` + `jwtauth.Authenticator` chain for optional and required JWT. Solvr uses a hand-rolled `JWTMiddleware`. During the merge, Quorum's room routes can use jwtauth (keeping Quorum's exact pattern), while Solvr's existing routes keep their own JWT middleware. No refactoring of Solvr auth needed. |
| `github.com/go-chi/httprate` | v0.15.0 | Per-route rate limiting by IP for room creation | Quorum rate-limits anonymous room creation (2/hour) and authenticated creation (5/hour) via `httprate.Limit`. Solvr has its own rate limiter for API routes, but httprate gives per-route granularity that Solvr's global middleware cannot replicate cleanly. Add httprate alongside Solvr's existing rate limiter. |
| `github.com/caarlos0/env/v11` | v11.4.0 | Struct-tag-based config loading | Quorum uses `env.Parse(&cfg)`. Solvr uses manual `os.Getenv`. Do NOT add this to Solvr. Port Quorum's config fields (`MaxSSEPerRoom`, `AnonRoomLimitPerHour`, `AuthedRoomLimitPerHour`) into Solvr's existing `config.Load()` function using `os.Getenv`. |
| `github.com/pressly/goose/v3` | v3.27.0 | Migration runner for Quorum | Quorum runs goose at startup from embedded FS. Solvr uses `golang-migrate` via CLI. Do NOT add goose to Solvr. Convert Quorum's 3 goose migrations to golang-migrate format (add `-- migrate:up` / `-- migrate:down` comments) and add as migrations 000073, 000074, 000075 to Solvr's `migrations/` directory. |

### Libraries NOT Needed (Do Not Add)

| Library | Why Quorum Has It | Why Solvr Should Not Add It |
|---------|-------------------|-----------------------------|
| `golang.org/x/oauth2` | Quorum has standalone Google/GitHub OAuth | Solvr already has its own OAuth in `internal/auth/`. Rooms will use Solvr's JWT auth, not Quorum's parallel auth system. Do not add a second OAuth stack. |
| `github.com/caarlos0/env/v11` | Quorum's config loader | Solvr has working config. Adding env/v11 creates two config patterns in the same binary. Port the 4 new env vars into Solvr's existing config struct. |
| `github.com/pressly/goose/v3` | Quorum runs migrations on startup | Solvr uses golang-migrate CLI. Do not add goose. |
| `go.uber.org/goleak` | Quorum test goroutine leak detection | Already a dev/test concern, not production. Add to Solvr's test deps if desired, but not critical path for the merge. |
| sqlc toolchain | Quorum generated code with sqlc v1.27.0 | See "sqlc vs raw pgx" decision below. |

---

## Critical Decision: sqlc vs Raw pgx

**Decision: Port Quorum's DB queries as raw pgx — do not introduce sqlc into Solvr.**

**Rationale:**

Quorum's `db/` package is sqlc-generated (`// Code generated by sqlc. DO NOT EDIT.`), but sqlc is a code generator, not a runtime dependency. The generated output is plain Go + pgx. The generated pattern (`db.New(pool)` returning `*db.Queries`) is a thin wrapper over `pool.QueryRow`, `pool.Exec`, etc. — identical to what Solvr writes by hand.

Solvr has 70+ migrations and dozens of hand-written repository files. Introducing sqlc would:
1. Require a `sqlc.yaml` config for the entire merged schema.
2. Force regeneration of ALL queries from scratch, touching hundreds of working files.
3. Create a two-pattern codebase where new-code uses sqlc and old code uses raw pgx.

The right move: copy Quorum's 5 query files (`querier.go`, `query.sql.go`, `models.go`, `db.go`, `event.go`) into `backend/internal/rooms/` as a `rooms` package, replace the `db.Queries` struct with a hand-written equivalent that embeds `*pgxpool.Pool`, and rewrite the ~20 queries as raw pgx calls matching Solvr's existing pattern. Total work: ~200 lines of straightforward translation. No sqlc toolchain needed.

**Confidence: HIGH** — Quorum's DBTX interface (`Exec`, `Query`, `QueryRow`) is the same interface pgx exposes. The translation is mechanical.

---

## SSE: Server Configuration Fix Required

Solvr's `main.go` sets `WriteTimeout: 15 * time.Second`. SSE connections are long-lived; a 15-second write timeout kills them.

**Fix: Remove `WriteTimeout` from the http.Server for the SSE route group.**

Quorum handles this correctly — its `main.go` has no `WriteTimeout` set (it's zero, meaning unlimited). The comment in Quorum's code says: `// WriteTimeout disabled — SSE connections are long-lived.`

Options:
1. Remove `WriteTimeout` globally from Solvr's http.Server. Clean, but removes timeout for all routes.
2. Use a custom `http.ResponseController` with per-response deadline extension on the SSE route. More precise.
3. Mount rooms SSE on a separate port. Overkill for a single-VPS deployment.

**Recommendation: Option 1** — remove the global `WriteTimeout`. Solvr is a single-VPS deployment. The 15-second write timeout was never tested in production against SSE. Per-request read timeouts (via `r.Context()`) are still enforced. Use `ReadTimeout: 15 * time.Second` only, drop `WriteTimeout`.

The `X-Accel-Buffering: no` header must also be set on SSE responses. Quorum has this as `SSENoBuffering` middleware. Port it into Solvr's `internal/api/middleware/` as `ssebuffering.go`.

---

## A2A Middleware: Ports Cleanly

All four Quorum middleware files map directly into Solvr with zero conflicts:

| Quorum Middleware | File | Port To | Conflict? |
|-------------------|------|---------|-----------|
| `A2AVersionGuard` | `a2aversion.go` | `backend/internal/api/middleware/a2aversion.go` | None. Completely new functionality. |
| `BearerTokenQueryStringGuard` | `bearerguard.go` | `backend/internal/api/middleware/bearerguard.go` | None. Solvr does not have this. |
| `SSENoBuffering` | `ssebuffering.go` | `backend/internal/api/middleware/ssebuffering.go` | None. New. |
| `AnonSession` | `anonsession.go` | `backend/internal/api/middleware/anonsession.go` | Check for cookie name collision with Solvr's existing session cookies. |

CORS update required: add `"A2A-Version"` and `"X-Agent-Name"` to Solvr's `AllowedHeaders` in `router.go`. Current list is `["Accept", "Authorization", "Content-Type", "X-Request-ID", "X-Session-ID"]`.

---

## JWT Auth: Two Patterns, Keep Both

Quorum uses `go-chi/jwtauth/v5` (`jwtauth.Verifier` + `jwtauth.Authenticator`).
Solvr uses a hand-rolled `auth.JWTMiddleware(secret)`.

Both use HS256 with the same `JWT_SECRET` env var. They produce compatible tokens — a token issued by Solvr's `auth.GenerateToken` can be validated by jwtauth's `Verifier`, and vice versa.

**Strategy:** Keep both patterns in the merged binary. Room routes use jwtauth (imported from Quorum with zero changes). Solvr's existing `/v1/` routes keep their current JWT middleware. This avoids touching 150+ existing handlers.

However, one JWT claim field differs:
- Solvr JWT uses `user_id` claim for the user UUID.
- Quorum JWT uses `sub` claim for the user UUID.

These tokens are issued by different code paths. After the merge, rooms routes will accept Quorum-issued tokens (with `sub`). Solvr routes will accept Solvr-issued tokens (with `user_id`). If a unified auth is desired (one token works everywhere), normalize to `sub` at a later phase. Do not do it during the merge.

---

## Bearer Token Auth Per Room

Quorum's `internal/token/` package (SHA-256 hash + constant-time compare) is fully self-contained with no external dependencies. Port the entire `token.go` file into `backend/internal/rooms/token/token.go` unchanged. It has 3 functions and 38 lines.

---

## Data Migration Strategy

Quorum has 3 migrations with 5 tables:
- `users` — **conflict**: Solvr already has a `users` table with a different schema (adds `username`, `role`, `bio`; uses `auth_provider` instead of `provider`).
- `rooms` — new, no conflict.
- `refresh_tokens` — **conflict**: Solvr has a `refresh_tokens` table in its own schema.
- `agent_presence` — new, no conflict.
- `messages` — new, no conflict.

**Plan:**

Migration 000073 — Add rooms table (from Quorum 00001, rooms only):
```sql
CREATE TABLE rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT UNIQUE NOT NULL CHECK (...),
    display_name TEXT NOT NULL,
    description TEXT,
    tags TEXT[] NOT NULL DEFAULT '{}',
    is_private BOOLEAN NOT NULL DEFAULT FALSE,
    owner_id UUID REFERENCES users(id) ON DELETE SET NULL,
    anonymous_session_id TEXT,
    token_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);
```
Note: `owner_id` references Solvr's `users.id` directly — Solvr's users table has UUIDs, this works.

Migration 000074 — Add agent_presence table (from Quorum 00002, unchanged).

Migration 000075 — Add messages table (from Quorum 00003, unchanged).

Migration 000076 — Migrate existing Quorum data (INSERT INTO rooms SELECT ... FROM quorum_db.rooms) — only needed if running Quorum on the same PostgreSQL server and schema. If Quorum runs in a separate DB, use `pg_dump | psql` to copy data before decommissioning.

**Quorum's `users` and `refresh_tokens` tables: do not migrate.** Quorum users are a subset of Solvr users (same OAuth providers). After the merge, rooms reference Solvr's `users.id`. Existing Quorum rooms with `owner_id` will either get reassigned to the matching Solvr user (matched by email) in a one-time script, or set to NULL (anonymous ownership).

---

## Hub Infrastructure: Port as-is

The hub package (`hub.go`, `manager.go`, `registry.go`, `roomid.go`, `event.go`, `messages.go`) is a pure in-memory pub/sub system with no external dependencies beyond stdlib. Port the entire `internal/hub/` directory into `backend/internal/rooms/hub/`. No changes needed — it compiles cleanly with just pgx and the a2a-go import.

The presence reaper (`internal/presence/reaper.go`) also ports directly into `backend/internal/rooms/presence/`.

---

## Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| `golang-migrate` CLI | Add rooms/agent_presence/messages migrations to Solvr | Already in use for Solvr. Create 000073-000075 with the existing `migrate create` command. Do NOT add goose. |
| `sqlc` (optional, local dev only) | Re-generate Quorum queries if schema evolves | Only needed if the team wants to keep sqlc for future rooms queries. Not required for the initial merge — translate manually. |

---

## Installation

```bash
# Add new dependencies to Solvr backend
cd /Users/fcavalcanti/dev/solvr/backend

# A2A protocol types
go get github.com/a2aproject/a2a-go@v0.3.12

# Chi JWT auth middleware (Quorum pattern, used for room routes)
go get github.com/go-chi/jwtauth/v5@v5.4.0

# Per-route IP rate limiting
go get github.com/go-chi/httprate@v0.15.0

# Version upgrades (Quorum is ahead of Solvr on these)
go get github.com/go-chi/chi/v5@v5.2.5
go get github.com/jackc/pgx/v5@v5.9.1
go get github.com/golang-jwt/jwt/v5@v5.3.1

go mod tidy
```

---

## Alternatives Considered

| Recommended | Alternative | Why Not |
|-------------|-------------|---------|
| Port Quorum DB queries as raw pgx | Adopt sqlc for all Solvr queries | Requires regenerating 150+ existing queries from scratch. Two-pattern codebase during transition is worse. |
| Remove global WriteTimeout | Per-response deadline extension on SSE only | More correct in theory but adds complexity. Solvr is single-VPS, no load balancer WriteTimeout enforcement. |
| Keep two JWT patterns (jwtauth + hand-rolled) | Normalize all routes to jwtauth | Normalizing touches every existing handler. Deferred to a future phase. |
| golang-migrate for Quorum tables | Run goose at Solvr startup for room migrations | Creates two migration runners in the same binary. One runner principle. |
| Port hub as-is into rooms/ package | Rewrite hub using a pub/sub library | Hub has goroutine-leak tests (`go.uber.org/goleak`), is well-tested, and is already integrated with SSE. Rewriting gains nothing. |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| sqlc in Solvr | Requires full schema codegen; creates two DB patterns | Raw pgx (existing Solvr pattern) for all rooms queries |
| goose in Solvr | Solvr uses golang-migrate; two migration runners cause confusion about state | Convert Quorum migrations to golang-migrate format |
| `caarlos0/env` in Solvr | Solvr has its own config loader; env/v11 adds a pattern inconsistency | Add Quorum's 3 new env vars to Solvr's existing `config.Load()` with `os.Getenv` |
| Separate port or server for rooms/SSE | Unnecessary complexity on single-VPS deployment | Mount `/v1/rooms/` and `/r/` on same Chi router, disable WriteTimeout globally |
| Quorum's OAuth handlers | Solvr already has fully working Google/GitHub OAuth | Use Solvr's existing auth system for room ownership |
| Quorum's refresh_token system | Solvr has its own refresh token system | Use Solvr's existing refresh tokens; Quorum's can be discarded |

---

## Version Compatibility

| Solvr Package | Quorum Package | Status | Resolution |
|---------------|----------------|--------|------------|
| `chi/v5@v5.2.0` | `chi/v5@v5.2.5` | Minor version gap, same API | Upgrade Solvr to v5.2.5 |
| `pgx/v5@v5.7.2` | `pgx/v5@v5.9.1` | Patch gap, wire-compatible | Upgrade to v5.9.1 or keep v5.7.2 (either works) |
| `golang-jwt/jwt/v5@v5.2.1` | `golang-jwt/jwt/v5@v5.3.1` | Patch gap, backward-compatible | Upgrade to v5.3.1 |
| Solvr: no jwtauth | `go-chi/jwtauth/v5@v5.4.0` | New dep, no conflict | Add to Solvr go.mod |
| Solvr: no a2a-go | `a2a-go@v0.3.12` | New dep, no conflict | Add to Solvr go.mod |
| Solvr: no httprate | `httprate@v0.15.0` | New dep, no conflict | Add to Solvr go.mod |
| Solvr: WriteTimeout=15s | Quorum: WriteTimeout=0 | **Conflict** — kills SSE | Remove WriteTimeout from Solvr http.Server |
| Solvr JWT claim: `user_id` | Quorum JWT claim: `sub` | **Claim name conflict** | Keep two patterns; unify in later phase |
| Solvr CORS AllowedHeaders | Quorum adds `A2A-Version`, `X-Agent-Name` | **Missing headers** | Add both to Solvr's CORS config |
| Solvr users schema: `username`, `role`, `bio`, `auth_provider` | Quorum users: `provider`, `provider_id`, no `username`/`role` | **Schema conflict** | Use Solvr's users table; discard Quorum's |

---

## Sources

- `/Users/fcavalcanti/dev/quorum/relay/go.mod` — Quorum dependency list (verified directly)
- `/Users/fcavalcanti/dev/solvr/backend/go.mod` — Solvr dependency list (verified directly)
- `/Users/fcavalcanti/dev/quorum/relay/cmd/server/main.go` — Quorum server setup, missing WriteTimeout (verified directly)
- `/Users/fcavalcanti/dev/solvr/backend/cmd/api/main.go` — Solvr WriteTimeout=15s (verified directly)
- `/Users/fcavalcanti/dev/quorum/relay/internal/handler/sse.go` — SSE implementation (verified directly)
- `/Users/fcavalcanti/dev/quorum/relay/internal/relay/handler.go` — A2A JSON-RPC handler (verified directly)
- `/Users/fcavalcanti/dev/quorum/relay/internal/middleware/a2aversion.go` — A2A version guard (verified directly)
- `/Users/fcavalcanti/dev/quorum/relay/internal/middleware/ssebuffering.go` — SSE buffering fix (verified directly)
- `/Users/fcavalcanti/dev/quorum/relay/internal/db/querier.go` — sqlc Querier interface (verified directly)
- `/Users/fcavalcanti/dev/quorum/relay/internal/db/models.go` — Quorum data models (verified directly)
- `/Users/fcavalcanti/dev/quorum/relay/internal/migrations/` — 3 SQL migrations (verified directly)
- `/Users/fcavalcanti/dev/solvr/backend/internal/auth/jwt.go` — Solvr JWT claims (`user_id` field) (verified directly)
- `/Users/fcavalcanti/dev/solvr/backend/internal/api/router.go` — Solvr CORS AllowedHeaders (verified directly)
- `/Users/fcavalcanti/dev/quorum/relay/internal/token/token.go` — Room bearer token (SHA-256, constant-time) (verified directly)
- `github.com/a2aproject/a2a-go@v0.3.12` — module cache at `~/go/pkg/mod/github.com/a2aproject/` (verified directly)
- `go version go1.25.3 darwin/arm64` — local Go toolchain (verified via `go version`)

---
*Stack research for: Quorum A2A relay merge into Solvr Go backend*
*Researched: 2026-04-02*
