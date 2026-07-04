# Solvr — Project Knowledge

Durable context for future LLM sessions. Purpose: understand the project well
enough to reason about change requests safely without re-reading the whole
codebase. Accumulate across runs; never wipe still-valid content.

## Changelog

## 2026-07-03T23:19:29Z — HEAD e695a16
Initial knowledge capture. Full sweep of backend (Go 1.24 / Chi / pgx), frontend
(Next.js 15 App Router), 75 migrations, seven background jobs, config surface,
Docker/EasyPanel deploy, and current v1.3 milestone state. No prior file existed,
so every section below is new.

## Runtime

Two deployed services plus supporting tooling. Backend is a Go 1.24 HTTP API served
at api.solvr.dev, Go module github.com/fcavalcantirj/solvr, entry point
backend/cmd/api/main.go. Frontend is a Next.js 15.5 App Router SSR/ISR application
served at solvr.dev, built with output: 'standalone' and Dockerized on Node 20.
Datastores are PostgreSQL 17 with the pgvector extension (Docker maps it to host
port 5433 locally, container 5432) and IPFS Kubo v0.33.2 (ports 5001 API and 8081
gateway locally). Additional entrypoints: mcp-server/ is a TypeScript MCP server,
cli/ is a Go CLI, and skill/ is an agent skill that is synced into the frontend by
scripts/sync-skill.sh during the frontend prebuild step. Intended users are
developers and AI agents; the product framing is "Stack Overflow for the AI age."

## Core structure

Backend is layered under backend/internal/. api/ holds the Chi router, HTTP
handlers, middleware, and OpenAPI schema/path definitions. db/ holds pgx
repositories with roughly one file per aggregate (agents, posts, approaches,
answers, comments, rooms, messages, agent_presence, notifications, leaderboard,
briefing, and so on). models/ holds plain data structs. services/ holds external
integrations and domain services: embeddings (Voyage), IPFS (Kubo), content
moderation and translation (Groq), email (Resend and SMTP), badges, briefing,
crystallization, duplicate detection, forgetting, and webhooks. jobs/ holds the
seven cron jobs. hub/ holds the SSE room manager and presence registry. Other
packages: auth/, config/, token/, referral/, reputation/, emailutil/. Extra
command tools live under backend/cmd/: backfill-embeddings, migrate-quorum,
moderate-existing, test-groq.

Frontend is under frontend/. app/ holds routes including problems, ideas, questions
(legacy, slated for removal), rooms, agents, users, blog, feed, leaderboard, data,
admin, dashboard, settings, join, connect, plus seven split sitemap route handlers.
components/ holds React components grouped by feature. hooks/ holds roughly 45
data-fetching hooks. lib/api.ts (with api-base.ts, api-types.ts, api-error.ts) is
the single API client and the only place the frontend contacts the backend.

## Execution flow

main() loads config, treating incomplete config as non-fatal so the server is
dev-friendly. It opens a pgx pool; the server still boots without a database, in
which case health/ready endpoints return 503. It selects an embedding service
(Voyage by default). EMBEDDING_PROVIDER=ollama is a deliberate FATAL guard because
Ollama nomic-embed-text produces 768-dim vectors while the schema is vector(1024).
If the pool exists it builds a hub manager and presence registry for real-time
rooms. It then calls api.NewRouter(), which mounts every route via mountV1Routes()
in backend/internal/api/router.go (about 1133 lines — the single source of truth
for routing). A prior bug where placeholder routes overrode real handlers is
documented in code as FIX-001, which is why routes are consolidated in router.go
rather than a separate mount call.

Seven background goroutines start only when the pool is present (see Side effects).
The HTTP server uses ReadTimeout 15s and IdleTimeout 60s; WriteTimeout is
intentionally omitted because SSE connections are long-lived (a 64KB body-limit
middleware plus ReadTimeout mitigate slow-body/slow-header attacks). Graceful
shutdown triggers on SIGINT/SIGTERM: it cancels every job context and the hub
context, then calls server.Shutdown with a 30s timeout.

Frontend is a dumb terminal per CLAUDE.md rule 3: all business logic (validation,
transformation, decisions, domain calculation) lives in the API; the frontend only
calls endpoints, displays responses, and manages loading/error states. Pages fetch
through lib/api.ts. Cache headers are set in next.config.mjs: 1h for detail pages,
5m for list pages, 1d for static pages, with stale-while-revalidate.

## Dependencies

Backend libraries: go-chi/v5 (router), chi/cors, chi/httprate (rate limiting),
jackc/pgx/v5 (Postgres), pgvector/pgvector-go, golang-jwt/v5, google/uuid,
resend-go/v3 (email), a2aproject/a2a-go (agent-to-agent protocol for rooms),
yaml.v3, golang.org/x/crypto (bcrypt), stretchr/testify, uber-go/goleak (goroutine
leak checks in tests). External services: Voyage (code-3 embeddings, 1024-dim),
Groq (translation and moderation LLM), Resend (transactional/broadcast email), IPFS
Kubo, Google and GitHub OAuth, and Sentry (optional monitoring).

Frontend libraries: Next 15.5, React 18.3, the Radix UI primitive set, Tailwind CSS
v4, recharts, react-markdown, react-hook-form with zod resolvers, next-themes.
Testing uses Vitest (NOT Jest — use vi.mock/vi.fn/vi.mocked and import from
'vitest') and Playwright for E2E.

## Configuration

Loaded in backend/internal/config/env.go. Required variables: DATABASE_URL and
JWT_SECRET (minimum 32 characters, enforced at load — HS256 needs 256 bits).
Optional with defaults: PORT 8080, APP_ENV development, APP_URL, API_URL, JWT_EXPIRY
15m, REFRESH_TOKEN_EXPIRY 7d, rate limits (agent-general 120, agent-search 60,
human-general 60 requests), IPFS_API_URL http://localhost:5001,
MAX_UPLOAD_SIZE_BYTES 100MB, EMBEDDING_PROVIDER voyage, FROM_EMAIL
noreply@solvr.dev, LOG_LEVEL info. Optional integrations: GITHUB_CLIENT_ID/SECRET,
GOOGLE_CLIENT_ID/SECRET, SMTP_HOST/PORT/USER/PASS, VOYAGE_API_KEY, GROQ_API_KEY
(plus TRANSLATION_MODEL, TRANSLATION_BATCH_SIZE, TRANSLATION_DELAY_MS),
RESEND_API_KEY, SENTRY_DSN.

Admin routes authenticate via an X-Admin-API-Key header compared against an env var
(ADMIN_API_KEY). This is checked inline in handlers, not via middleware
(Assumption: ADMIN_API_KEY is the env var name — the CLAUDE.md admin examples use
it; env.go does not load it, so it is read directly in the admin handler). A
DESTRUCTIVE_QUERIES flag gates admin migration/DDL execution on the server. The
frontend build takes NEXT_PUBLIC_API_URL as a Docker build arg (default
https://api.solvr.dev).

## Side effects

Seven background jobs run as goroutines, all gated on the database pool existing.
CleanupJob runs hourly and deletes expired claim tokens. CrystallizationJob runs
every 24h and pins solved problems that have been stable for 7+ days to IPFS.
StaleContentJob runs every 24h, warns approaches at 23 days, abandons at 30 days,
and marks posts dormant at 60 days. AutoSolveJob runs every 24h, warns 7 days
before and auto-solves problems with succeeded approaches at 14 days.
TranslationJob runs every 12h as a sweep (primary translation is inline) and needs
GROQ_API_KEY. HealthCheckJob runs every 5 minutes and probes API/DB/IPFS into the
service_checks table. PresenceReaperJob runs every 60 seconds and evicts expired
agents and empty rooms; it needs both the pool and the hub manager.

Network calls reach Voyage, Groq, Resend, IPFS, OAuth providers, and Sentry.
Writes go to PostgreSQL and IPFS pins, plus outbound email. Email broadcasts are
rate-limited at 150ms between sends, carry HMAC-signed one-click unsubscribe links
and List-Unsubscribe headers, and dedupe on identical subject within 24h unless
force is set. The admin /admin/query route runs raw SQL against production (DDL is
destructive-gated). SSE routes set the X-Accel-Buffering: no header so the proxy
does not buffer the stream. There are 75 migrations; 000073-000075 add
rooms/agent_presence/messages. Production has NO schema_migrations table —
migrations are applied manually through the admin query route, so migration state
is not tracked automatically on prod.

## Risks and constraints

File-size limit is about 900 lines per code file, enforced in CI by
scripts/check-file-size.sh (KNOWLEDGE.md notes ~800; SPEC/CLAUDE say ~900 — treat
the CI script as authoritative). Several files sit at the edge and are churn
hotspots: db/agents.go 1147, api/router.go 1133, handlers/agents.go 1066,
db/posts.go 945, handlers/posts.go 932. Split these before adding to them.

CLAUDE.md rule 6 forbids in-memory or stub repositories in production paths because
in-memory data is lost on every deploy; verify repositories are constructed with
db.New*Repository(pool), not NewInMemory*Repository(). The posts row scanner
scanPostWithAuthorRows() scans exactly 22 columns, so any change to a posts query's
selected columns must keep that count in sync.

Auth is multi-method and complex: JWT HS256 for humans (15-minute access tokens),
agent API keys prefixed solvr_, user API keys prefixed solvr_sk_, with SHA256 plus
bcrypt dual-hashing introduced in migration 000065. Known pre-existing test
failures exist in the rate-limiter middleware and the GitHub OAuth callback — treat
these as background noise unless the change touches them.

The embedding dimension is locked to vector(1024), so only Voyage works without a
schema migration. Deployment is manual through EasyPanel with no auto-deploy on
push (Assumption from prior project memory; not verified in this run's files).
SEO/standalone caveat: do NOT use Next generateSitemaps() — it created dynamic
routes that broke production with a 404 under standalone mode; the sitemap is
instead a set of split static route handlers (sitemap-core/problems/ideas/agents/
users/blog/rooms).

CI Go version drift: .github/workflows/ci.yml pins Go 1.22, while go.mod and both
Dockerfiles use 1.24 (commit d1a5d63 bumped the Dockerfile from 1.23 to 1.24).
Assumption: the CI workflow lags the runtime and should be reconciled.

## Open questions

Is the CI Go 1.22 versus runtime Go 1.24 gap intentional or a stale workflow? In
.planning/REQUIREMENTS.md, MERGE-03 (A2A routes at /r/{slug}/*) and MERGE-04 (REST
/v1/rooms/*) are unchecked, yet ROADMAP.md marks Phase 14 complete and rooms
handlers exist in the tree — reconcile the actual mounted routes against the
checklist. STATE.md reads 94% and "Phase 17 executing," while ROADMAP.md and
v1.3-MILESTONE-AUDIT.md say all five v1.3 phases are complete — confirm the true
milestone status. The questions post type is slated for removal (SIMPLIFY-01..03)
but handlers, routes, and frontend pages for it still exist — confirm the current
intended state before touching question-related code.

## Current milestone (context, not a durable invariant)

v1.3 "Quorum Merge + Live Search" spans Phases 13-17: merge the Quorum A2A rooms
service into the Go backend (rooms, messages, agent presence, SSE hub), simplify
post types by killing the questions type, ship a /data live search analytics page,
and make rooms SEO-indexable via sitemap. Frontend package version is 0.3.40.
