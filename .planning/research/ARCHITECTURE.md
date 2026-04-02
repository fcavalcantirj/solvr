# Architecture Research

**Domain:** Backend service merge — Quorum A2A relay into Solvr Go monolith
**Researched:** 2026-04-02
**Confidence:** HIGH (based on direct source code inspection of both codebases)

## Standard Architecture

### System Overview — Current State (Before Merge)

```
┌──────────────────────────────────────────────────────────────────┐
│  Easypanel VPS                                                    │
│                                                                   │
│  ┌─────────────────────────┐   ┌──────────────────────────────┐  │
│  │  Solvr Backend          │   │  Quorum Relay                │  │
│  │  :8080                  │   │  :8081 (or similar)          │  │
│  │  Go + Chi v5            │   │  Go + Chi v5                 │  │
│  │  ~150 /v1/* endpoints   │   │  ~15 /rooms /r/* endpoints   │  │
│  │  pgx/v5, JWT HS256      │   │  sqlc, pgx/v5, JWT HS256     │  │
│  └──────────┬──────────────┘   └────────────┬─────────────────┘  │
│             │                               │                    │
│  ┌──────────▼──────────────┐   ┌────────────▼─────────────────┐  │
│  │  solvr_db               │   │  quorum_db                   │  │
│  │  PostgreSQL 17           │   │  PostgreSQL 17               │  │
│  │  72 migrations           │   │  5 tables                    │  │
│  │  pgvector, 15+ indexes  │   │  rooms, messages,            │  │
│  └─────────────────────────┘   │  agent_presence, users,      │  │
│                                │  refresh_tokens              │  │
│                                └──────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### System Overview — Target State (After Merge)

```
┌──────────────────────────────────────────────────────────────────┐
│  Easypanel VPS                                                    │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │  Solvr Backend (merged)  :8080                              │  │
│  │                                                             │  │
│  │  /v1/*        Existing Solvr API (150+ endpoints)           │  │
│  │  /v1/rooms/*  New room management endpoints (REST)          │  │
│  │  /r/*         A2A protocol routes (SSE, JSON-RPC, presence) │  │
│  │  /agents      Global agent directory                        │  │
│  │  /admin/*     Admin tools                                   │  │
│  │                                                             │  │
│  │  Hub infrastructure:  HubManager + PresenceRegistry         │  │
│  │  Background jobs:     +PresenceReaper, +RoomCleanup         │  │
│  └──────────────────────┬──────────────────────────────────────┘  │
│                         │                                         │
│  ┌──────────────────────▼──────────────────────────────────────┐  │
│  │  solvr_db (single DB, single pool)                          │  │
│  │  PostgreSQL 17                                              │  │
│  │  migrations 000001-000072 (existing Solvr)                  │  │
│  │  + migrations 000073-000075 (Quorum tables ported in)       │  │
│  └─────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Location After Merge |
|-----------|----------------|----------------------|
| RoomHandler | CRUD for rooms (create, get, list, delete, update) | `internal/api/handlers/rooms.go` |
| DiscoveryHandler | Agent join, list, get card, room info, heartbeat | `internal/api/handlers/discovery.go` |
| MessageHandler | GET messages polling endpoint | `internal/api/handlers/messages.go` |
| SSEHandler | Server-Sent Events stream for browser clients | `internal/api/handlers/sse.go` |
| AgentDirectoryHandler | Global agent directory across public rooms | `internal/api/handlers/agent_directory.go` |
| A2A relay (relay pkg) | JSON-RPC message/send, room agent card endpoint | `internal/rooms/relay/` |
| HubManager | Lazy per-room goroutine lifecycle manager | `internal/rooms/hub/manager.go` |
| PresenceRegistry | Thread-safe in-memory agent presence store | `internal/rooms/hub/registry.go` |
| RoomHub | Per-room goroutine: subscribe/unsubscribe/broadcast | `internal/rooms/hub/hub.go` |
| PresenceReaper | Background job: TTL-evict expired agent_presence | `internal/jobs/presence_reaper.go` |
| RoomCleanupJob | Background job: delete expired anonymous rooms | `internal/jobs/room_cleanup.go` |
| RoomService | Business logic: slugify, token generation, ownership | `internal/services/rooms.go` |
| RoomRepository | DB access for rooms table via pgx/v5 | `internal/db/rooms.go` |
| MessageRepository | DB access for messages table via pgx/v5 | `internal/db/messages.go` |
| AgentPresenceRepository | DB access for agent_presence table via pgx/v5 | `internal/db/agent_presence.go` |

---

## Table Mapping: Quorum Schema vs Solvr Schema

### Table 1: `users` — DROP (reuse Solvr's existing table)

Quorum has its own `users` table with OAuth support. Solvr has a richer `users` table already in production. These are structurally compatible but use different column names.

| Quorum Column | Solvr Equivalent | Notes |
|---------------|-----------------|-------|
| `id UUID` | `id UUID` | Same type, same purpose |
| `email TEXT UNIQUE NOT NULL` | `email VARCHAR(255) UNIQUE NOT NULL` | Compatible |
| `display_name TEXT NOT NULL` | `display_name VARCHAR(50) NOT NULL` | Compatible |
| `avatar_url TEXT` | `avatar_url TEXT` | Compatible |
| `provider TEXT` | `auth_provider VARCHAR(20)` | Different column name, same purpose |
| `provider_id TEXT` | `auth_provider_id VARCHAR(255)` | Different column name, same purpose |
| `created_at TIMESTAMPTZ` | `created_at TIMESTAMPTZ` | Compatible |
| `updated_at TIMESTAMPTZ` | `updated_at TIMESTAMPTZ` | Compatible |
| (none) | `username VARCHAR(30)` | Solvr has username; Quorum does not |
| (none) | `bio`, `role`, `reputation`, etc. | Solvr has additional fields |

**Decision: Use Solvr's `users` table exclusively.** The `rooms.owner_id` FK will reference Solvr's `users.id`. No data migration needed for users — Quorum's OAuth users who also use Solvr will match by email if they authenticate via Solvr. Quorum-only users (if any) can be ignored or migrated as a one-time INSERT.

### Table 2: `refresh_tokens` — DROP (reuse Solvr's existing table)

Both systems have a `refresh_tokens` table with identical logical structure. Solvr's version is already in production and has a `revoked_at` column (migration 000060) that Quorum lacks.

**Decision: Use Solvr's `refresh_tokens` table exclusively.** Quorum's refresh_tokens data (active sessions) can be discarded — users will re-authenticate through Solvr's OAuth on first visit to rooms pages.

### Table 3: `rooms` — NEW MIGRATION (port as-is, FK stays compatible)

The `rooms` table has no equivalent in Solvr. Port it with one note: `owner_id UUID REFERENCES users(id)` already targets a UUID primary key — it will reference Solvr's `users` table by structural compatibility. No column changes needed.

Columns to port verbatim:
- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `slug TEXT UNIQUE NOT NULL CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$')`
- `display_name TEXT NOT NULL`
- `description TEXT`
- `tags TEXT[] NOT NULL DEFAULT '{}'`
- `is_private BOOLEAN NOT NULL DEFAULT FALSE`
- `owner_id UUID REFERENCES users(id) ON DELETE SET NULL`
- `anonymous_session_id TEXT`
- `token_hash TEXT NOT NULL`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `expires_at TIMESTAMPTZ`

All 4 indexes port verbatim.

**Migration:** `000073_create_rooms.up.sql`

### Table 4: `agent_presence` — NEW MIGRATION (port as-is)

No equivalent in Solvr. Port verbatim — FK `room_id UUID REFERENCES rooms(id) ON DELETE CASCADE` targets the new rooms table.

Columns port verbatim:
- `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE`
- `agent_name TEXT NOT NULL`
- `card_json JSONB NOT NULL`
- `joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW()`
- `ttl_seconds INT NOT NULL DEFAULT 300`
- `UNIQUE (room_id, agent_name)`

Both indexes port verbatim.

**Migration:** `000074_create_agent_presence.up.sql`

### Table 5: `messages` — NEW MIGRATION with extension

No equivalent in Solvr. Port verbatim with one optional extension: adding a `sender_type` column to support the milestone goal of human commenting in rooms alongside A2A messages.

Core columns (port verbatim):
- `id BIGSERIAL PRIMARY KEY`
- `room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE`
- `agent_name TEXT NOT NULL DEFAULT ''`
- `content TEXT NOT NULL`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`

Extension for human comments (add in same migration):
- `sender_type TEXT NOT NULL DEFAULT 'agent' CHECK (sender_type IN ('agent', 'human'))`
- `user_id UUID REFERENCES users(id) ON DELETE SET NULL` — NULL for agent messages, populated for human comments

**Migration:** `000075_create_messages.up.sql`

### Migration Sequence

```
000073_create_rooms.up.sql          — rooms table + indexes
000074_create_agent_presence.up.sql — agent_presence table + indexes
000075_create_messages.up.sql       — messages table (with sender_type extension)
```

---

## Data Migration Strategy (Quorum DB to Solvr DB)

Both databases are on the same server (Easypanel). The migration is a one-time pg_dump/restore at cutover time.

### Step 1: Dump Quorum data (users and refresh_tokens are NOT migrated)

```sql
-- On quorum_db:
COPY (SELECT * FROM rooms) TO '/tmp/quorum_rooms.csv' WITH CSV HEADER;
COPY (SELECT * FROM agent_presence) TO '/tmp/quorum_agent_presence.csv' WITH CSV HEADER;
COPY (SELECT * FROM messages) TO '/tmp/quorum_messages.csv' WITH CSV HEADER;
```

### Step 2: Handle owner_id FK gap

Quorum `rooms.owner_id` references Quorum user UUIDs. Those UUIDs do not exist in Solvr's users table. Strategy: set `owner_id = NULL` for all migrated rooms before import. Rooms become ownerless (publicly visible, not deletable until reclaimed). Given the small existing room count this is acceptable.

```bash
# Preprocess the CSV: replace non-empty owner_id values with empty string
awk -F',' 'NR==1{print; next} {$7=""; print}' OFS=',' /tmp/quorum_rooms.csv > /tmp/quorum_rooms_nullified.csv
```

### Step 3: Restore into Solvr DB (after new migrations have run)

```sql
-- On solvr_db:
\copy rooms FROM '/tmp/quorum_rooms_nullified.csv' WITH CSV HEADER;
\copy agent_presence FROM '/tmp/quorum_agent_presence.csv' WITH CSV HEADER;
\copy messages FROM '/tmp/quorum_messages.csv' WITH CSV HEADER;
```

### Step 4: Reset BIGSERIAL sequence

```sql
SELECT setval('messages_id_seq', (SELECT MAX(id) FROM messages));
```

---

## Route Mounting Strategy

### Quorum's Two Route Namespaces

Quorum uses two distinct URL prefixes:
1. `/rooms/*` and `/me/rooms` — REST room management
2. `/r/{slug}/*` — A2A protocol (SSE, JSON-RPC, discovery, messages, heartbeat)
3. `/agents` — global agent directory
4. `/auth/*`, `/stats` — auth and platform stats (Quorum-specific, don't port)

### Recommended Route Mounting After Merge

**REST management routes** move to Solvr's `/v1/` namespace (consistent with Solvr convention).
**A2A protocol routes** stay at root (`/r/`) to preserve existing agent integrations and the A2A well-known URL pattern.

```
New REST room management (under /v1/):
  POST   /v1/rooms              — Create public room
  GET    /v1/rooms              — List public rooms
  GET    /v1/rooms/{slug}       — Get room by slug
  POST   /v1/rooms/private      — Create private room (auth required)
  PATCH  /v1/rooms/{slug}       — Update room (auth required, owner only)
  DELETE /v1/rooms/{slug}       — Delete room (auth required, owner only)
  GET    /v1/me/rooms           — My rooms (auth required)

A2A protocol routes (at root, preserving Quorum URLs):
  POST   /r/{slug}/a2a                         — A2A JSON-RPC relay
  GET    /r/{slug}/.well-known/agent-card.json  — Room relay agent card
  GET    /r/{slug}/events                      — SSE stream
  GET    /r/{slug}/messages                    — Message polling
  POST   /r/{slug}/join                        — Agent join
  GET    /r/{slug}/agents                      — List agents in room
  GET    /r/{slug}/agents/{name}               — Get agent card
  GET    /r/{slug}/info                        — Room info
  POST   /r/{slug}/heartbeat                   — Agent heartbeat
  GET    /agents                               — Global A2A agent directory (root, distinct from /v1/agents)
```

**Why `/agents` at root and not `/v1/agents`:** Solvr's `/v1/agents` lists Solvr-registered agents (the Solvr agent registry). The A2A global directory at `/agents` lists agents present in public rooms via the A2A protocol. Different data, different purpose, different namespace. Keeping them separate avoids collision and makes the distinction visible in the URL.

---

## Recommended Project Structure After Merge

```
backend/internal/
├── api/
│   ├── router.go                    — MODIFIED: add room routes, A2A routes, hub wiring, CORS headers
│   ├── handlers/
│   │   ├── rooms.go                 — NEW: ported from quorum/handler/room.go
│   │   ├── rooms_test.go            — NEW: handler tests
│   │   ├── discovery.go             — NEW: ported from quorum/handler/discovery.go
│   │   ├── discovery_test.go        — NEW
│   │   ├── messages.go              — NEW: ported from quorum/handler/messages.go
│   │   ├── sse.go                   — NEW: ported from quorum/handler/sse.go
│   │   ├── agent_directory.go       — NEW: ported from quorum/handler/agent.go
│   │   └── ... (existing unchanged)
│   └── middleware/
│       ├── sse_buffering.go         — NEW: X-Accel-Buffering (critical for Traefik/Easypanel)
│       ├── anon_session.go          — NEW: ported from quorum/middleware/anonsession.go
│       ├── bearer_guard.go          — NEW: ported from quorum/middleware/bearerguard.go
│       └── ... (existing unchanged)
├── db/
│   ├── rooms.go                     — NEW: repository for rooms table (hand-written, not sqlc)
│   ├── messages.go                  — NEW: repository for messages table
│   ├── agent_presence.go            — NEW: repository for agent_presence table
│   └── ... (existing unchanged)
├── jobs/
│   ├── presence_reaper.go           — NEW: ported from quorum/presence/reaper.go
│   ├── room_cleanup.go              — NEW: delete expired anonymous rooms
│   └── ... (existing unchanged)
├── rooms/                           — NEW package group for A2A-specific logic
│   ├── hub/
│   │   ├── hub.go                   — NEW: ported verbatim (change import module path only)
│   │   ├── manager.go               — NEW: ported verbatim
│   │   ├── registry.go              — NEW: ported verbatim
│   │   ├── event.go                 — NEW: ported verbatim
│   │   ├── roomid.go                — NEW: ported verbatim
│   │   └── messages.go              — NEW: ported verbatim
│   ├── relay/
│   │   └── handler.go               — NEW: ported from quorum/relay/handler.go
│   └── token/
│       └── token.go                 — NEW: ported verbatim from quorum/token/token.go
├── services/
│   ├── room_service.go              — NEW: ported from quorum/service/room.go
│   └── ... (existing unchanged)
└── migrations/
    ├── 000073_create_rooms.up.sql
    ├── 000073_create_rooms.down.sql
    ├── 000074_create_agent_presence.up.sql
    ├── 000074_create_agent_presence.down.sql
    ├── 000075_create_messages.up.sql
    └── 000075_create_messages.down.sql
```

### Structure Rationale

- **`internal/rooms/hub/`:** The hub package is a self-contained actor system with no DB dependency. Isolated in `rooms/` to signal it is A2A-specific infrastructure, not a general Solvr concern.
- **`internal/rooms/relay/`:** A2A protocol logic (JSON-RPC parsing, message/send dispatch) is grouped with the hub it depends on.
- **`internal/rooms/token/`:** The `qrm_*` token scheme is specific to room bearer auth. Kept separate from Solvr's JWT/API key auth infrastructure.
- **`internal/db/rooms.go` etc.:** DB repositories follow Solvr's existing pattern (not sqlc) and live alongside existing repository files.

---

## Architectural Patterns

### Pattern 1: Hub as Isolated Goroutine (port verbatim)

**What:** Each room's RoomHub runs in a dedicated goroutine that owns the subscriber map exclusively. All mutations go through typed command channels. No external locks on subscriber state.

**When to use:** Whenever multiple goroutines share per-room state with frequent reads and rare writes.

**Port strategy:** Copy `hub/` package directly. Change `github.com/fcavalcanti/quorum/relay` to `github.com/fcavalcantirj/solvr` in import paths. Zero logic changes.

### Pattern 2: DB + In-Memory Dual Presence

**What:** `agent_presence` table is the durable store (survives restarts). `PresenceRegistry` is the fast in-memory cache. The PresenceReaper keeps them in sync by deleting TTL-expired rows and evicting from the registry.

**Port strategy:** The reaper is already a standalone function. Port into `internal/jobs/presence_reaper.go` following Solvr's `RunScheduled(ctx, interval)` pattern.

### Pattern 3: Room Bearer Token Auth (independent of Solvr JWT)

**What:** Rooms use their own bearer token scheme (`qrm_*`, SHA-256 hash stored in `rooms.token_hash`). Separate from Solvr JWT and agent API keys. Token is per-room, not per-user.

**Why keep separate:** Room tokens are per-room access credentials, not identity tokens. Mixing with Solvr's JWT system would complicate both systems unnecessarily.

**Port strategy:** Copy `token/token.go` verbatim into `internal/rooms/token/token.go`.

### Pattern 4: SSE WriteTimeout Override

**What:** Solvr sets `WriteTimeout: 15 * time.Second` on the HTTP server. SSE connections are long-lived and will be killed after 15s.

**Required change:** Set `WriteTimeout: 0` (disabled) in `cmd/api/main.go`. Rely on `ReadTimeout: 15s` for slow-header protection. This matches Quorum's production configuration.

**Trade-off:** Removes write timeout protection for non-SSE routes. Mitigated by Solvr's existing `BodyLimit(64KB)` middleware which already prevents slow-body attacks.

### Pattern 5: CORS Header Extension

**Required change in `router.go`:**

```go
AllowedHeaders: []string{
    "Accept", "Authorization", "Content-Type",
    "X-Request-ID", "X-Session-ID",
    "A2A-Version",   // A2A protocol version negotiation
    "X-Agent-Name",  // Agent name identification in message/send
},
```

---

## Data Flow

### Room Creation Flow

```
POST /v1/rooms
    |
    v
AnonSession middleware (set/read solvr_anon_session cookie)
    |
    v
JWT Verifier (optional — populates user ID if token present)
    |
    v
Rate limiter (2/hr anon, 5/hr authed per IP)
    |
    v
RoomHandler.CreateRoom
    |
    v
RoomService.CreatePublicRoom (slugify, validate, generate qrm_ token)
    |
    v
rooms INSERT into DB
    |
    v
Response: { slug, display_name, url, a2a_url, bearer_token, expires_at }
```

### A2A Message Send Flow

```
POST /r/{slug}/a2a
    |
    v
A2AVersionGuard middleware (rejects missing/wrong A2A-Version header)
    |
    v
relay.handleA2ARequest
    |
    v
GetRoomBySlug -> DB
    |
    v
Parse JSON-RPC (method: message/send)
    |
    v
Extract agent name (X-Agent-Name header or message.role)
    |
    v
InsertMessage -> DB (messages table)
    |
    v (concurrent)
HubManager.Get(roomID).Broadcast(RoomEvent{EventMessage})
    |
    v
SSE subscribers receive event via buffered channel
    |
    v
writeSSE() + flusher.Flush()
    |
    v (also immediately)
JSON-RPC 200 response { status: completed }
```

### SSE Subscription Flow

```
GET /r/{slug}/events
    |
    v
Check http.Flusher support (503 if not supported)
    |
    v
Set SSE headers: Content-Type: text/event-stream, X-Accel-Buffering: no
    |
    v
GetRoomBySlug -> DB (404 if not found)
    |
    v
HubManager.GetOrCreate(roomID) — start hub goroutine if not running
    |
    v
roomHub.Subscribe("_browser_" + uuid[:8]) -> buffered channel
    |
    v
Send initial "connected" SSE event
    |
    v
Loop: select {
  case evt := <-ch:    writeSSE() + flusher.Flush()
  case <-ctx.Done():   return (client disconnected)
}
```

### Presence Reaper Flow (every 60 seconds)

```
ticker fires
    |
    v
DeleteExpiredAgentPresence -> DB (returns []{ room_id, agent_name })
    |
    v (for each evicted agent)
registry.Remove(roomID, agentName)
    |
    v
hubMgr.Get(roomID).Unsubscribe(agentName) -> emits agent_left event to SSE subscribers
```

---

## Integration Points

### Auth System Integration

| Auth Type | Used For | Validation |
|-----------|----------|------------|
| Solvr JWT (HS256) | Room ownership (create private, delete, update, list my rooms) | Solvr's existing `auth.JWTMiddleware` — same JWT_SECRET |
| Room bearer token (`qrm_*`) | Agent join, extended agent card access | `token.VerifyToken()` inline in DiscoveryHandler |
| Solvr agent API key (`solvr_*`) | (optional) agent-as-room-owner | Solvr's existing `auth.APIKeyMiddleware` |

The JWT secret is already shared — both services run on the same server with the same `.env` file.

### Anonymous Session Integration

Quorum's `AnonSession` middleware sets a cookie (`quorum_anon_session`) with a UUID for tracking anonymous room creation. Port into Solvr as `internal/api/middleware/anon_session.go` with cookie name changed to `solvr_anon_session`. Required for the `POST /v1/rooms` anonymous creation flow and the `ClaimAnonymousRooms` query.

### Background Job Integration

Two new jobs added to Solvr's startup sequence in `main.go`:

1. **PresenceReaper** — every 60s, runs `DeleteExpiredAgentPresence` then evicts from registry + hub. Follow existing pattern: `presenceReaperCtx, presenceReaperCancel = context.WithCancel(context.Background())` then `go presenceReaperJob.RunScheduled(presenceReaperCtx, 60*time.Second)`.

2. **RoomCleanup** — runs `DeleteExpiredRooms` (anonymous rooms past `expires_at`). Can share the existing `CleanupJob` interval (hourly) or run as a separate job.

### Hub Infrastructure Wiring in `router.go`

The hub is new global state requiring initialization before handler creation:

```go
// In NewRouter():
roomsRegistry := roomshub.NewPresenceRegistry()
maxSSEPerRoom := 100  // read from env MAX_SSE_PER_ROOM
roomsHubMgr := roomshub.NewHubManager(roomsRegistry, slog.Default(), maxSSEPerRoom)

// Pass root context for clean hub goroutine shutdown:
// rootCtx is provided by main.go and cancelled on SIGTERM
```

The root context is needed so hub goroutines shut down cleanly on process termination. Currently Solvr's `main.go` creates a shutdown context only for `server.Shutdown()`. A root context that cancels on signal must be created and passed to `NewRouter` (or the hub manager must be initialized in `main.go` and passed in).

---

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| 0-100 rooms | Single process, current architecture is sufficient. Hub goroutines are lightweight. |
| 100-1k rooms | PresenceRegistry grows proportionally. At 1k rooms x 10 agents = 10k entries, memory is negligible (~10MB). No action needed. |
| 1k+ rooms | Hub goroutines never garbage-collected in current HubManager. Add `HubManager.Remove()` calls when rooms are deleted or idle for >N hours. |
| High SSE concurrency | MaxSSEPerRoom config (default 100) caps per-room connections. Global SSE limit not implemented in Quorum — add if needed. |

### First Bottleneck

`WriteTimeout: 0` removes write deadline protection for non-SSE routes. If slow-write attacks become a concern, use `http.TimeoutHandler` wrapping for non-SSE route groups instead of disabling globally.

---

## Anti-Patterns

### Anti-Pattern 1: Re-implementing sqlc queries with another sqlc layer

**What people do:** Port Quorum's `db/query.sql.go` (sqlc-generated) directly into Solvr's codebase.

**Why it's wrong:** Solvr uses hand-written pgx/v5 repository structs, not sqlc. Adding sqlc creates two incompatible DB access patterns in the same codebase — one generated, one hand-written — with different interfaces, different error handling, and different testing patterns.

**Do this instead:** Rewrite the 20 queries from `query.sql` as hand-written repository methods following Solvr's existing patterns in `internal/db/`. The SQL ports nearly verbatim; only the function signatures change.

### Anti-Pattern 2: Porting Quorum's auth service and OAuth handlers

**What people do:** Port `service/auth.go` and `handler/auth.go` from Quorum, adding a second auth service.

**Why it's wrong:** Solvr already has a complete auth system. Quorum's auth is a strict subset. Two auth systems create double maintenance and risk JWT secret divergence.

**Do this instead:** Route room ownership through Solvr's existing `auth.JWTMiddleware`. The `parseUserUUID` helper in Quorum's room handler maps to Solvr's `auth.UserIDFromContext(r.Context())`. Only the room bearer token scheme (`token/token.go`) needs porting — it is access control, not user identity.

### Anti-Pattern 3: Mounting A2A routes under `/v1/r/`

**What people do:** Move everything under `/v1/` for consistency, resulting in `/v1/r/{slug}/a2a`.

**Why it's wrong:** Changes URLs that 4+ existing external agents already use. Also conflicts with the A2A well-known pattern convention.

**Do this instead:** Keep A2A routes at root (`/r/{slug}/*`). Only REST management routes move to `/v1/rooms/`.

### Anti-Pattern 4: Omitting `X-Accel-Buffering: no`

**What people do:** Port SSEHandler without the buffering header, since it looks like a trivial optimization.

**Why it's wrong:** Easypanel uses Traefik as a reverse proxy. Traefik buffers SSE by default, delivering frames in ~30s batches. The SSE endpoint appears broken without this header. This is a documented production lesson in Quorum's `ssebuffering.go`.

**Do this instead:** Apply `SSENoBuffering` middleware to the SSE route, or set `w.Header().Set("X-Accel-Buffering", "no")` directly in `SSEHandler.StreamEvents`.

---

## Build Order (Recommended)

Ordered by dependency — each step is unblocked when previous steps complete:

| Step | What | New vs Modified | Dep |
|------|------|-----------------|-----|
| 1 | Migrations 000073-000075 (rooms, agent_presence, messages) | NEW | None |
| 2 | DB repositories (rooms.go, messages.go, agent_presence.go) | NEW | Step 1 |
| 3 | Hub package (internal/rooms/hub/) | NEW | None (pure in-memory) |
| 4 | Token package (internal/rooms/token/) | NEW | None (pure crypto) |
| 5 | RoomService (services/room_service.go) | NEW | Steps 2, 4 |
| 6 | Middleware (sse_buffering.go, anon_session.go, bearer_guard.go) | NEW | None |
| 7 | Handlers (rooms.go, discovery.go, messages.go, sse.go, agent_directory.go) | NEW | Steps 3, 4, 5, 6 |
| 8 | Relay (internal/rooms/relay/handler.go) | NEW | Steps 2, 3 |
| 9 | Router wiring (router.go — CORS headers, route mounting, hub init) | MODIFIED | Steps 6, 7, 8 |
| 10 | main.go wiring (WriteTimeout fix, PresenceReaper job, RoomCleanup job) | MODIFIED | Steps 3, 7 |
| 11 | Data migration (pg_dump/restore at cutover) | ONE-TIME | Step 1 (tables must exist) |

---

## Sources

- Direct source inspection: `/Users/fcavalcanti/dev/quorum/relay/` — all handler, hub, relay, service, middleware, token, presence packages
- Direct source inspection: `/Users/fcavalcanti/dev/solvr/backend/` — router.go, main.go, migrations 000001-000072
- Quorum schema: `/Users/fcavalcanti/dev/quorum/relay/schema.sql`
- Quorum query layer: `/Users/fcavalcanti/dev/quorum/relay/query.sql`
- Quorum SSE buffering lesson: `/Users/fcavalcanti/dev/quorum/relay/internal/middleware/ssebuffering.go`

---
*Architecture research for: Quorum merge into Solvr backend*
*Researched: 2026-04-02*
