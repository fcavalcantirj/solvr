# Phase 14: Backend Service Merge - Context

**Gathered:** 2026-04-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Port Quorum's Go packages (hub, handler, presence, relay) into Solvr as a fully integrated rooms backend. Deliver: REST room CRUD at `/v1/rooms/*`, A2A protocol routes at `/r/{slug}/*`, SSE real-time streaming, agent presence with TTL-based expiry, and clean shutdown. No frontend work — backend only.

</domain>

<decisions>
## Implementation Decisions

### SSE Timeout & Streaming
- **D-01:** Claude's Discretion on WriteTimeout strategy — either remove globally or per-route middleware, based on what fits best with Solvr's server config.
- **D-02:** `X-Accel-Buffering: no` header set in the Go SSE handler (not nginx config). Self-contained, works regardless of reverse proxy.
- **D-03:** SSE connections have 30-minute max lifetime. Client auto-reconnects via SSE retry mechanism.
- **D-04:** 30-second heartbeat/keep-alive pings on SSE connections. Detects dead clients, prevents proxy timeouts.
- **D-05:** Global SSE connection limit (e.g., 1000). No per-room limits for now — add later if needed.
- **D-06:** Four SSE event types: `message`, `presence_join`, `presence_leave`, `room_update`. Full parity with Quorum.
- **D-07:** Last-Event-ID support for reconnection replay. Use message BIGSERIAL id as Event-ID. On reconnect, replay missed messages from DB.

### Hub Manager
- **D-08:** Hub lives in its own package `internal/hub/`. Architecturally distinct from request-response services — has unique lifecycle/concurrency concerns.
- **D-09:** Hub initialized in `main.go` like background jobs, injected into room handlers as a dependency.
- **D-10:** Hub shutdown via context cancellation. On SIGTERM, context cancels, hub drains connections and stops goroutines. Same pattern as Go's http.Server.
- **D-11:** DB-only replay — hub is purely a broadcast relay, no in-memory message buffering. Reconnect replay queries DB by Last-Event-ID.
- **D-12:** Lazy room creation in hub — hub room created when first client connects or first message is posted. No startup overhead.

### Route Organization
- **D-13:** Room routes in `router_rooms.go` — extracted from router.go (already 1105 lines, over 900-line limit). Mount `/r/{slug}/*` and `/v1/rooms/*` from there.
- **D-14:** Handlers split by concern: `rooms.go` (CRUD), `rooms_messages.go` (message posting/listing), `rooms_sse.go` (SSE streaming), `rooms_presence.go` (presence management).
- **D-15:** Separate route paths: `/r/{slug}/*` for A2A protocol (legacy agent URLs, room bearer token auth). `/v1/rooms/*` for REST CRUD (Solvr JWT/agent key auth). Different auth, different handlers.

### Auth Model
- **D-16:** Room creation: Requires logged-in Solvr user OR registered Solvr agent (JWT / `solvr_` API key) via `POST /v1/rooms`. Creator becomes owner.
- **D-17:** A2A participation (`/r/{slug}/*`): Room bearer token ONLY. Any agent with the token can join, post messages, stream — no Solvr account needed. This is how Quorum works.
- **D-18:** REST room list (`GET /v1/rooms`): Public, no auth. Anyone can browse public rooms. Needed for Phase 16 SSR and SEO.
- **D-19:** REST room detail (`GET /v1/rooms/{slug}`): Public for public rooms. Returns room metadata, agent presence, and recent messages. Needed for SSR, SEO, JSON-LD.

### Room Management
- **D-20:** Slug auto-generated from display_name if not provided. Client can override with custom slug.
- **D-21:** Soft-delete, owner only. Set deleted_at timestamp. Admins can also delete. Matches Solvr's existing pattern.
- **D-22:** All metadata fields editable by owner except slug (immutable after creation). Editable: display_name, description, category, tags, is_private.
- **D-23:** Soft-deleted rooms return 404 for all operations (messages, presence, stream). Matches Solvr's post soft-delete behavior.

### Room Tokens
- **D-24:** Same as Quorum: generate crypto-random token on creation, store SHA256 hash in token_hash, return plaintext ONCE in creation response.
- **D-25:** Tokens are rotatable via `POST /v1/rooms/{slug}/rotate-token`. Generates new token, updates hash, returns new plaintext. All agents with old token lose access.

### Presence Reaper
- **D-26:** New background job (7th job) using Solvr's existing job pattern. Runs every 60s, deletes expired presence records.
- **D-27:** Reaper emits `presence_leave` SSE event when presence expires. Clients see agents disappear in real-time.
- **D-28:** Agents renew presence via explicit `POST /r/{slug}/heartbeat` AND implicitly via message posting. Both update last_seen.
- **D-29:** Same reaper job also handles expired rooms (where expires_at has passed). Soft-deletes them. Two cleanup tasks, one job.

### Data Management
- **D-30:** message_count on rooms maintained in application code (Go repository layer). Increment on message INSERT, decrement on soft-delete. No DB triggers.
- **D-31:** sequence_num assigned in application code. Claude's discretion on exact mechanism (MAX+1 in transaction or similar). Must be correct under concurrent writes.
- **D-32:** Per-room rate limiting on A2A message posting (e.g., 60/min). Uses Solvr's existing rate limiter middleware.

### Agent Card
- **D-33:** Agent card follows a basic structure (name, description, avatar_url, capabilities) but allows additional custom fields in the JSONB. Flexible up to 16KB (D-30 from Phase 13).

### Repository Layer
- **D-34:** Room list query uses single JOIN query with aggregate subqueries for agent_count. Matches Solvr's PostRepository.List() pattern. message_count is denormalized (no COUNT needed).
- **D-35:** Message pagination is cursor-based using BIGSERIAL message ID (?after=12345). Efficient for real-time feeds, no skip/duplicate issues.
- **D-36:** Quorum queries redesigned for Solvr's patterns (LEFT JOIN subqueries, pgx scanning). Not straight sqlc-to-pgx translation.

### Testing
- **D-37:** Integration tests with real HTTP (httptest.Server). Connect real SSE clients, send messages, assert events received. Full-stack testing. Matches Solvr's router_*_test.go pattern.

### Code Reuse Strategy
- **D-38:** Adapt Quorum's hub core logic (broadcast, subscribe, room registry) to Solvr's patterns. Hub structure is solid.
- **D-39:** Rewrite handlers from scratch using Solvr's Chi router, auth, and response patterns. Quorum handlers as reference only.

### Claude's Discretion
- WriteTimeout removal strategy (global vs per-route)
- Exact sequence_num increment mechanism (must be concurrent-safe)
- SSE connection limit number
- Rate limit values for message posting
- Agent card basic structure field names
- Hub file organization within internal/hub/
- Exact query designs for room list and message pagination

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Quorum Source Code (Reference for Porting)
- `/Users/fcavalcanti/dev/quorum/relay/internal/hub/` — Hub manager, event types, broadcast logic, room registry (7 files)
- `/Users/fcavalcanti/dev/quorum/relay/internal/handler/` — HTTP handlers: room, messages, sse, agent, auth, discovery (8 files)
- `/Users/fcavalcanti/dev/quorum/relay/internal/presence/reaper.go` — Presence TTL expiry goroutine
- `/Users/fcavalcanti/dev/quorum/relay/internal/relay/` — Executor and relay handler (3 files)
- `/Users/fcavalcanti/dev/quorum/relay/query.sql` — 20 sqlc queries to redesign as pgx methods (124 lines)
- `/Users/fcavalcanti/dev/quorum/relay/schema.sql` — Original table definitions

### Solvr Architecture (Patterns to Follow)
- `backend/internal/api/router.go` — Main router (1105 lines, needs splitting per D-13)
- `backend/internal/db/posts.go` — PostRepository.List() pattern with LEFT JOIN subqueries
- `backend/internal/db/comments.go` — Existing author_type/author_id polymorphic pattern
- `backend/cmd/api/main.go` — Server config (WriteTimeout:15s at L211), shutdown (L225-255), background jobs
- `backend/internal/db/stale_content.go` — Background job pattern for reaper reference

### Phase 13 Context (Decisions Carried Forward)
- `.planning/phases/13-database-foundation/13-CONTEXT.md` — 42 schema decisions (D-01 to D-42)
- `backend/migrations/000073_create_rooms.up.sql` — rooms table schema (15 columns, 4 indexes)
- `backend/migrations/000074_create_agent_presence.up.sql` — agent_presence schema (7 columns, 2 indexes)
- `backend/migrations/000075_create_messages.up.sql` — messages schema (11 columns, 2 indexes)

### Research
- `.planning/research/PITFALLS.md` — 10 critical pitfalls with prevention strategies
- `.planning/research/ARCHITECTURE.md` — Architecture decisions for the merge

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `comments` table `author_type`/`author_id` polymorphic pattern — replicate in message repository
- Solvr's rate limiter middleware — reuse for per-room A2A rate limiting
- `auth_helpers.go` — JWT and agent key extraction, reuse for /v1/rooms auth
- Background job pattern in `main.go` — goroutine with ticker, context cancellation
- `httptest.Server` test patterns in `router_*_test.go` — reuse for SSE integration tests

### Established Patterns
- Chi v5 router with `r.Route()` grouping and middleware chains
- pgx/v5 repositories with `Pool.QueryRow()` / `Pool.Query()` scanning
- UUID primary keys for entities, BIGSERIAL for ordered sequences
- LEFT JOIN aggregate subqueries for list endpoints (avoid N+1)
- Soft-delete via `deleted_at` column, filtered in all queries with WHERE deleted_at IS NULL

### Integration Points
- `router.go` needs splitting (1105 lines) — `router_rooms.go` mounts room routes
- `main.go` server config — WriteTimeout modification for SSE
- `main.go` shutdown handler — hub context cancellation integration
- `main.go` background jobs section — add presence reaper as 7th job
- Hub injected as dependency into room handler constructors

</code_context>

<specifics>
## Specific Ideas

- A2A routes preserve Quorum's exact URL structure (/r/{slug}/*) so existing agent integrations don't break
- Room bearer tokens use SHA256 hash (not bcrypt) — fast verification for high-frequency A2A requests
- Hub is a broadcast relay only, no message buffering — DB is source of truth, hub is the real-time overlay
- Reaper job handles both presence AND room expiry — two cleanup concerns, one goroutine
- Message posting auto-renews agent presence (belt and suspenders alongside explicit heartbeat)

</specifics>

<deferred>
## Deferred Ideas

- Private room access control (column exists, no logic until Phase 16+)
- Room creation from frontend UI (API/A2A only for now per REQUIREMENTS.md)
- Message editing (no edited_at column yet)
- Per-room SSE connection limits (start with global only)
- Agent card schema enforcement (start flexible, tighten later)
- Room search/discovery features beyond basic list
- WebSocket support (SSE is sufficient per REQUIREMENTS.md)

</deferred>

---

*Phase: 14-backend-service-merge*
*Context gathered: 2026-04-04*
