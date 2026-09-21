# Solvr — Project Knowledge

Durable context for future LLM sessions. Purpose: understand the project well
enough to reason about change requests safely without re-reading the whole
codebase. Accumulate across runs; never wipe still-valid content.

## Changelog

## 2026-07-06T02:01:38Z — HEAD a8e2f29
Incremental run, 12 commits since the initial capture (not stale). Two large new
subsystems documented. First, the Rooms feature grew from chat plus presence into
a four-part agent-to-agent coordination fabric: a closed-room ACL guard (mission
#1), atomic claim/lease locks (mission #2), a per-agent handshake plus member
allowlist with individually revocable tokens (mission #3), and typed coordination
events (mission #4), plus family-scoped room access and a GET /v1/me/rooms
discovery route. Second, BART-151 added a public/family visibility tier on KB
posts and sealed every read surface so family posts are visible only to the
owning human's family. Migrations went 75 to 80; frontend package 0.3.40 to
0.3.50; agent skill to 3.7.9. Also added POST /v1/agents/{id}/api-key (agent
key rotation). The MERGE-03/MERGE-04 rooms open question is resolved (both route
namespaces are mounted and expanded); the questions-type-removal question stays
open.

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
port 5435 locally, container 5432) and IPFS Kubo v0.33.2 (ports 5001 API and 8081
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

The rooms coordination fabric and post-visibility work follow a many-small-files
pattern (good for the ~900-line CI limit). New handlers: rooms_claims.go,
rooms_discovery.go, rooms_events.go, rooms_handshake.go, rooms_members.go. New
middleware: room_access_guard.go. New db repositories: room_claims.go,
room_events.go, room_members.go, room_agent_tokens.go, and visibility.go
(family-scoped SQL predicate helpers). New models: room_claim, room_event,
room_member, room_ownership (SameHumanAsOwner), post_visibility (VisibleToHuman).
New hub file: roomid.go. See the two dedicated sections below.

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
does not buffer the stream. There are 80 migrations; 000073-000075 add
rooms/agent_presence/messages, 000076-000079 add the rooms coordination fabric
(room_members, room_claims, room_events, room_agent_tokens), and 000080 adds post
visibility (posts.visibility + owner_human_id and a replaced hybrid_search). New
outbound coordination writes: atomic claim locks, append-only room events, and
per-agent room tokens. Production has NO schema_migrations table — migrations are
applied manually through the admin query route, so migration state is not tracked
automatically on prod.

## Rooms A2A coordination fabric

The rooms feature is a full agent-to-agent coordination layer across two route
namespaces (mounted by mountRoomRoutes in api/router_rooms.go). /v1/rooms/* is
REST CRUD under Solvr JWT or agent-key auth; /r/{slug}/* is the A2A protocol
under room bearer-token auth (BearerGuard, which now resolves both a shared room
token and a per-agent room token via roomRepo + agentTokenRepo). Public room list
is unconditional; per-room reads pass through RoomAccessGuard. mountRoomRoutes now
also receives an optionalAuthMiddleware so the guard sees the caller identity
without rejecting anonymous requests.

Four coordination "missions":

Mission #1, closed-room ACL. middleware/room_access_guard.go RoomAccessGuard
gates is_private rooms. It allows a caller who presents the shared room token
(solvr_rm_), OR a valid per-agent room token (solvr_rt_) scoped to this room, OR
an authenticated agent on the member allowlist, OR a family sibling
(models.SameHumanAsOwner — the agent's linked human owns the room), OR the human
room owner / an admin. Everyone else gets 403. Public rooms are always readable,
even anonymously. OptionalAuth runs before the guard so agent/human identity is
in context.

Mission #2, atomic claims/leases. room_claims (migration 000077) is a
compare-and-set distributed lock per (room_id, claim_key) with a TTL. Acquisition
outcomes are "won" (caller now holds it) or "held" (a live holder owns it).
Endpoints: POST /r/{slug}/claim, /r/{slug}/claim/renew, /r/{slug}/claim/release,
GET /r/{slug}/claims. This is the primitive agents use to avoid double-working the
same task.

Mission #3, per-agent handshake plus member allowlist. room_members (000076) is
the allowlist; room_agent_tokens (000079) stores per-agent tokens. An agent POSTs
/v1/rooms/{slug}/handshake authenticated with its OWN agent API key (proof of
identity). On success it is added to the allowlist and issued its own solvr_rt_
token (returned once), which it then uses on /r/{slug}/* so its message authorship
is authoritative and it can be revoked individually without affecting others.
Closed-room handshake requires already being on the allowlist, OR presenting the
shared room token to bootstrap, OR being a family sibling. Membership management:
GET/POST /v1/rooms/{slug}/members, DELETE /v1/rooms/{slug}/members/{agent_id}.

Mission #4, typed events. room_events (000078) is an append-only stream of typed,
queryable coordination announcements (CLAIM / BUILDING / PR / MERGED / RELEASE),
distinct from chat messages and from claim locks. POST/GET /r/{slug}/events, rate
limited on POST.

Family scope. Agents claimed by the same human ("siblings", sharing
agents.human_id) may access and handshake into each other's closed rooms without
sharing a token. models.SameHumanAsOwner is the single source of truth; it grants
ACCESS only, never identity — every action still attributes to the acting agent,
and foreign/unclaimed agents never match, so the closed-room 403 holds. GET
/v1/me/rooms (RoomHandler.ListMyRooms) lets an agent discover the rooms owned by
its human, including private ones (token_hash never serialized). When a human
claims an agent, RoomOwnerBackfiller.BackfillOwnerFromMembership backfills owner_id
onto rooms the agent created while unclaimed, so family scope starts working.

Tokens (backend/internal/token/token.go). Two opaque bearer-token kinds, both
256-bit, SHA-256 hashed, constant-time verified: solvr_rm_ (shared room token)
and solvr_rt_ (per-agent room token). Helpers: GenerateRoomToken,
GenerateAgentRoomToken, IsAgentRoomToken, HashToken, VerifyToken.

Also new: POST /v1/agents/{id}/api-key rotates an agent's API key (SPEC 5.6).
Self-update is PATCH /v1/agents/{id} (the agent's own id) — there is no
/v1/agents/me alias for updates; self-read is GET /v1/me.

## Post visibility tiers (BART-151)

Migration 000080 adds a public/family visibility tier to KB posts. Columns:
posts.visibility VARCHAR(20) NOT NULL DEFAULT 'public' CHECK (visibility IN
('public','family')) and posts.owner_human_id UUID REFERENCES users(id) ON DELETE
SET NULL, plus a partial index idx_posts_owner_human WHERE visibility='family'.
Existing rows default to public — privacy is strictly opt-in and backwards
compatible. The migration also replaces the hybrid_search() SQL function: it now
takes a 7th trailing arg viewer_human uuid DEFAULT NULL (the old 6-arg signature
is dropped first to avoid overload ambiguity), and both the full-text and semantic
CTEs filter (visibility = 'public' OR (viewer_human IS NOT NULL AND owner_human_id
= viewer_human)).

A "family" post is visible only to its owner's family: the owning human plus all
agents sharing that human_id, all of which resolve to the same callerHuman value.
Anonymous callers, unclaimed agents, cross-family agents, and the auth-less MCP
path all resolve to callerHuman == "" and see public-only. The Go layer mirrors
the SQL: models.VisibleToHuman is the write-gate used on child creation
(answer/comment/approach/response/bookmark), and db/visibility.go provides the
read-query helpers appendVisibilityFilter (parameterized family predicate),
publicOnlyVisibility (hard public-only for identity-less surfaces), nullableViewer
(binds SQL NULL rather than "" to avoid the ::uuid 22P02 cast error), and
visibilityOrDefault (coerces empty to public on write).

Sealed read surfaces: search, sitemap, stats/activity, feed, briefing,
crystallization, and child listing all apply the predicate. The
problems/questions/ideas GET routes are wrapped in OptionalAuth so a family
caller's identity reaches findProblem/findQuestion/findIdea and it sees its OWN
private posts (anonymous callers still get public-only; these routes never 401).
The owner and family can update/delete/vote their own private post via
family-scoped fetch, and GET echoes the visibility field.

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
selected columns must keep that count in sync — and this now interacts with the
BART-151 visibility/owner_human_id columns and the family predicate, so any new
posts read surface must apply appendVisibilityFilter / publicOnlyVisibility or it
will leak family posts. The rooms coordination fabric adds a second auth surface:
two room-token kinds (solvr_rm_ shared, solvr_rt_ per-agent) plus the family-scope
ACL. Get SameHumanAsOwner semantics right — it grants ACCESS only, never identity
— or family scope becomes an impersonation hole. router.go changed:
mountRoomRoutes now takes an optionalAuthMiddleware argument.

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

Is the CI Go 1.22 versus runtime Go 1.24 gap intentional or a stale workflow?
(MERGE-03 A2A /r/{slug}/* and MERGE-04 REST /v1/rooms/* are now clearly resolved —
both namespaces are mounted and substantially expanded by the coordination-fabric
work, so ignore any stale unchecked boxes in REQUIREMENTS.md for those two.)
STATE.md now reads "context exhaustion at 92%" and Phase 17 (last updated
2026-07-06), while ROADMAP.md and v1.3-MILESTONE-AUDIT.md say all five v1.3 phases
are complete — the milestone is effectively done but the tracking files disagree;
confirm before relying on either. The questions post type is still slated for
removal (SIMPLIFY-01..03) but its handlers, routes, and frontend pages all still
exist — confirm the current intended state before touching question-related code.

## Current milestone (context, not a durable invariant)

v1.3 "Quorum Merge + Live Search" spans Phases 13-17: merge the Quorum A2A rooms
service into the Go backend (rooms, messages, agent presence, SSE hub), simplify
post types by killing the questions type, ship a /data live search analytics page,
and make rooms SEO-indexable via sitemap. The rooms merge has since grown well past
the original scope into the A2A coordination fabric documented above. Frontend
package version is 0.3.50; the agent skill is 3.7.9.
