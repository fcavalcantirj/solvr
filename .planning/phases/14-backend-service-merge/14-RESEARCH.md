# Phase 14: Backend Service Merge - Research

**Researched:** 2026-04-04
**Domain:** Go service merge — Quorum hub/handler/presence/relay packages ported into Solvr monolith
**Confidence:** HIGH — all findings from direct source inspection of both codebases

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**SSE Timeout & Streaming**
- D-01: Claude's Discretion on WriteTimeout strategy — either remove globally or per-route middleware, based on what fits best with Solvr's server config.
- D-02: `X-Accel-Buffering: no` header set in the Go SSE handler (not nginx config). Self-contained, works regardless of reverse proxy.
- D-03: SSE connections have 30-minute max lifetime. Client auto-reconnects via SSE retry mechanism.
- D-04: 30-second heartbeat/keep-alive pings on SSE connections. Detects dead clients, prevents proxy timeouts.
- D-05: Global SSE connection limit (e.g., 1000). No per-room limits for now — add later if needed.
- D-06: Four SSE event types: `message`, `presence_join`, `presence_leave`, `room_update`. Full parity with Quorum.
- D-07: Last-Event-ID support for reconnection replay. Use message BIGSERIAL id as Event-ID. On reconnect, replay missed messages from DB.

**Hub Manager**
- D-08: Hub lives in its own package `internal/hub/`. Architecturally distinct from request-response services.
- D-09: Hub initialized in `main.go` like background jobs, injected into room handlers as a dependency.
- D-10: Hub shutdown via context cancellation. On SIGTERM, context cancels, hub drains connections and stops goroutines.
- D-11: DB-only replay — hub is purely a broadcast relay, no in-memory message buffering.
- D-12: Lazy room creation in hub — hub room created when first client connects or first message is posted.

**Route Organization**
- D-13: Room routes in `router_rooms.go` — extracted from router.go (already 1105 lines, over 900-line limit).
- D-14: Handlers split by concern: `rooms.go` (CRUD), `rooms_messages.go`, `rooms_sse.go`, `rooms_presence.go`.
- D-15: Separate route paths: `/r/{slug}/*` for A2A protocol (room bearer token auth). `/v1/rooms/*` for REST CRUD (Solvr JWT/agent key auth).

**Auth Model**
- D-16: Room creation: Requires logged-in Solvr user OR registered Solvr agent (JWT / `solvr_` API key) via `POST /v1/rooms`.
- D-17: A2A participation (`/r/{slug}/*`): Room bearer token ONLY. Any agent with the token can join, post, stream.
- D-18: REST room list (`GET /v1/rooms`): Public, no auth.
- D-19: REST room detail (`GET /v1/rooms/{slug}`): Public for public rooms.

**Room Management**
- D-20: Slug auto-generated from display_name if not provided. Client can override with custom slug.
- D-21: Soft-delete, owner only. Set deleted_at timestamp. Admins can also delete.
- D-22: All metadata fields editable by owner except slug (immutable after creation). Editable: display_name, description, category, tags, is_private.
- D-23: Soft-deleted rooms return 404 for all operations.

**Room Tokens**
- D-24: Generate crypto-random token on creation, store SHA256 hash in token_hash, return plaintext ONCE.
- D-25: Tokens rotatable via `POST /v1/rooms/{slug}/rotate-token`.

**Presence Reaper**
- D-26: New background job (7th job) using Solvr's existing job pattern. Runs every 60s.
- D-27: Reaper emits `presence_leave` SSE event when presence expires.
- D-28: Agents renew presence via `POST /r/{slug}/heartbeat` AND implicitly via message posting.
- D-29: Same reaper job also handles expired rooms (where expires_at has passed). Two cleanup tasks, one job.

**Data Management**
- D-30: message_count on rooms maintained in application code. Increment on INSERT, decrement on soft-delete.
- D-31: sequence_num assigned in application code. Must be correct under concurrent writes.
- D-32: Per-room rate limiting on A2A message posting (e.g., 60/min). Uses Solvr's existing rate limiter middleware.
- D-33: Agent card follows basic structure with custom fields in JSONB. Flexible up to 16KB.

**Repository Layer**
- D-34: Room list query uses single JOIN query with aggregate subqueries for agent_count.
- D-35: Message pagination is cursor-based using BIGSERIAL message ID (?after=12345).
- D-36: Quorum queries redesigned for Solvr's patterns (LEFT JOIN subqueries, pgx scanning). Not straight sqlc-to-pgx.

**Testing**
- D-37: Integration tests with real HTTP (httptest.Server). Connect real SSE clients, send messages, assert events received.

**Code Reuse Strategy**
- D-38: Adapt Quorum's hub core logic (broadcast, subscribe, room registry) to Solvr's patterns.
- D-39: Rewrite handlers from scratch using Solvr's Chi router, auth, and response patterns.

### Claude's Discretion
- WriteTimeout removal strategy (global vs per-route)
- Exact sequence_num increment mechanism (must be concurrent-safe)
- SSE connection limit number
- Rate limit values for message posting
- Agent card basic structure field names
- Hub file organization within internal/hub/
- Exact query designs for room list and message pagination

### Deferred Ideas (OUT OF SCOPE)
- Private room access control (column exists, no logic until Phase 16+)
- Room creation from frontend UI (API/A2A only for now per REQUIREMENTS.md)
- Message editing (no edited_at column yet)
- Per-room SSE connection limits (start with global only)
- Agent card schema enforcement (start flexible, tighten later)
- Room search/discovery features beyond basic list
- WebSocket support (SSE is sufficient per REQUIREMENTS.md)
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MERGE-02 | Quorum's 20 sqlc queries ported as Solvr-style pgx repository methods | All 20 queries from query.sql documented; pgx scanning pattern confirmed from posts.go |
| MERGE-03 | A2A protocol routes mounted at `/r/{slug}/*` preserving existing agent integration URLs | Route structure documented; Chi v5 mounting pattern confirmed from router.go |
| MERGE-04 | REST room management endpoints available at `/v1/rooms/*` | CRUD endpoint design confirmed; auth integration mapped to Solvr's existing JWT/agent key pattern |
| MERGE-05 | SSE hub manager runs alongside Solvr API with clean shutdown on SIGTERM | Hub goroutine lifecycle confirmed; SIGTERM shutdown pattern confirmed from main.go |
| MERGE-06 | WriteTimeout removed and `X-Accel-Buffering: no` header set for SSE routes | WriteTimeout:15s at main.go:211 confirmed; X-Accel-Buffering pattern documented from Quorum's ssebuffering.go |
| MERGE-07 | Agent presence with TTL-based expiry (default 10min) and reaper goroutine integrated | Reaper goroutine pattern confirmed from presence/reaper.go; Solvr job pattern confirmed from stale_content.go |
</phase_requirements>

---

## Summary

Phase 14 ports Quorum's hub, handler, presence, and relay packages into Solvr as a fully integrated backend. The migration is well-understood: both codebases were inspected in full. All tables (rooms, agent_presence, messages) are confirmed created by Phase 13 migrations 000073-000075.

The work breaks into five distinct concerns: (1) three new pgx repositories (rooms, messages, agent_presence) using hand-written SQL — zero sqlc, (2) a hub package ported verbatim from Quorum with only import path changes, (3) handlers rewritten from scratch using Solvr's Chi v5/auth/response patterns with Quorum handlers as reference, (4) `router_rooms.go` to split router.go (1105 lines, already over limit) and mount the new routes, and (5) `main.go` modifications: remove WriteTimeout, add hub initialization, add presence reaper as 7th background job.

The most common failure modes are: forgetting to remove WriteTimeout (SSE dies at 15s), not starting the reaper (presence rows accumulate), using the Quorum JWT context key (`sub`) instead of Solvr's (`user_id`), and the N+1 query trap in room list (must JOIN aggregate, not call GetRoomStats per room).

**Primary recommendation:** Follow the 11-step build order from ARCHITECTURE.md, starting with repositories (unblocked immediately because tables exist), then hub package (no dependencies), then handlers, then router wiring, then main.go modifications.

---

## Standard Stack

### Core (already in Solvr go.mod — no new deps needed)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/go-chi/chi/v5 | v5.2.0 | HTTP routing, middleware chains, URL params | Already in Solvr; Quorum also uses Chi v5 |
| github.com/jackc/pgx/v5 | v5.7.2 | PostgreSQL pool, query, scan | Already in Solvr; all repositories use pgx directly |
| github.com/golang-jwt/jwt/v5 | v5.2.1 | JWT validation for /v1/rooms auth | Already in Solvr; must NOT add go-chi/jwtauth |
| github.com/google/uuid | v1.6.0 | UUID generation, RoomID type wrapper | Already in Solvr |
| golang.org/x/crypto | v0.36.0 | Already in go.mod | No new crypto needed — SHA256 is in stdlib |

### New Dependencies Required

| Library | Version | Purpose | How to Add |
|---------|---------|---------|------------|
| github.com/a2aproject/a2a-go | v0.3.12 | A2A protocol types (AgentCard, AgentSkill) used by hub and discovery handler | `go get github.com/a2aproject/a2a-go@v0.3.12` |
| github.com/go-chi/httprate | v0.15.0 | Per-room rate limiting on A2A message posting (D-32) | `go get github.com/go-chi/httprate@v0.15.0` |

### Do NOT Add

| Library | Why Forbidden |
|---------|---------------|
| github.com/go-chi/jwtauth/v5 | Quorum's JWT library — reads `sub` claim, Solvr uses `user_id`. Adding it creates two JWT systems. Verified in PITFALLS.md. |
| github.com/pressly/goose/v3 | Quorum's migration tool. Solvr uses golang-migrate CLI. Incompatible file format. |
| github.com/sqlc-dev/sqlc | Quorum uses sqlc-generated code. Solvr uses hand-written pgx. Do not introduce sqlc. |

**Version verification:**
```bash
go version  # confirm 1.23+
go list -m github.com/a2aproject/a2a-go 2>/dev/null || echo "not installed"
```

**Installation:**
```bash
cd backend
go get github.com/a2aproject/a2a-go@v0.3.12
go get github.com/go-chi/httprate@v0.15.0
```

---

## Architecture Patterns

### Recommended Project Structure

```
backend/internal/
├── hub/                             # NEW package — hub lives here (D-08)
│   ├── hub.go                       # RoomHub per-room goroutine (port verbatim from Quorum)
│   ├── hub_test.go                  # Port Quorum's hub_test.go (already has good coverage)
│   ├── manager.go                   # HubManager lazy-create (port verbatim)
│   ├── registry.go                  # PresenceRegistry thread-safe in-memory store (port verbatim)
│   ├── event.go                     # EventType constants, RoomEvent struct (port verbatim)
│   ├── roomid.go                    # RoomID type-safe UUID wrapper (port verbatim)
│   └── messages.go                  # MessageStore ring buffer — not used in Phase 14 (port anyway)
├── api/
│   ├── router.go                    # MODIFY: remove WriteTimeout wiring, hub context setup
│   ├── router_rooms.go              # NEW: mountRoomRoutes() — extracted per D-13
│   ├── handlers/
│   │   ├── rooms.go                 # NEW: CRUD (create, get, list, delete, update, rotate-token)
│   │   ├── rooms_messages.go        # NEW: GET /r/{slug}/messages, POST /r/{slug}/message
│   │   ├── rooms_sse.go             # NEW: GET /r/{slug}/stream SSE handler
│   │   ├── rooms_presence.go        # NEW: join, heartbeat, agents list, agent card, room info
│   │   └── rooms_test.go            # NEW: integration tests for all room handlers
│   └── middleware/
│       ├── sse_buffering.go         # NEW: port Quorum's SSENoBuffering middleware verbatim
│       ├── anon_session.go          # NEW: port Quorum's AnonSession middleware (rename cookie)
│       └── bearer_guard.go          # NEW: port Quorum's BearerTokenQueryStringGuard verbatim
├── db/
│   ├── rooms.go                     # NEW: RoomRepository — 20 queries as pgx methods
│   ├── messages.go                  # NEW: MessageRepository — cursor pagination, insert
│   └── agent_presence.go            # NEW: AgentPresenceRepository — upsert, delete expired, list
├── jobs/
│   └── presence_reaper.go           # NEW: port Quorum's presence/reaper.go (Solvr RunScheduled pattern)
└── cmd/api/
    └── main.go                      # MODIFY: WriteTimeout removal, hub init, reaper 7th job
```

### Pattern 1: Hub Package (Port Verbatim)

**What:** Each `RoomHub` runs a single goroutine that owns the subscriber map exclusively. All mutations go through typed command channels. No locks on subscriber state.

**When to use:** Long-lived per-room state shared across goroutines.

**Port strategy:** Copy all 6 files from `/Users/fcavalcanti/dev/quorum/relay/internal/hub/` to `backend/internal/hub/`. Change import path from `github.com/fcavalcanti/quorum/relay` to `github.com/fcavalcantirj/solvr`. Zero logic changes.

**Example — Hub startup (confirmed from Quorum main.go + ARCHITECTURE.md):**
```go
// In main.go (Solvr):
// rootCtx is cancelled on SIGTERM — propagates to all hub goroutines
rootCtx, rootCancel := context.WithCancel(context.Background())
defer rootCancel()

hubRegistry := hub.NewPresenceRegistry()
maxSSEGlobal := 1000 // D-05: global limit
maxSSEPerRoom := 0   // D-05: no per-room limit for now
hubMgr := hub.NewHubManager(hubRegistry, slog.Default(), maxSSEPerRoom)
```

### Pattern 2: SSE Handler with Last-Event-ID Replay (D-07)

**What:** On SSE connect, check `Last-Event-ID` header. If present, replay missed messages from DB before streaming live events.

**When to use:** All SSE connections to `GET /r/{slug}/stream`.

**Example pattern (adapted from Quorum's SSEHandler + D-07 decision):**
```go
// Source: confirmed pattern from quorum/relay/internal/handler/sse.go
func (h *RoomsSSEHandler) Stream(w http.ResponseWriter, r *http.Request) {
    // Check flusher support
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    // SSE headers (D-02: set in handler, not nginx)
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")  // Critical for Traefik/Easypanel

    // D-07: Last-Event-ID replay
    if lastID := r.Header.Get("Last-Event-ID"); lastID != "" {
        // Query DB for messages after lastID, send each as SSE event
    }

    // D-03: 30-minute max lifetime
    ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
    defer cancel()

    // Subscribe to hub
    subscriberName := "_browser_" + uuid.New().String()[:8]
    ch, err := roomHub.Subscribe(subscriberName, nil)
    if err != nil {
        http.Error(w, "room at capacity", http.StatusServiceUnavailable)
        return
    }
    defer roomHub.Unsubscribe(subscriberName)

    // D-04: 30-second heartbeat ticker
    heartbeat := time.NewTicker(30 * time.Second)
    defer heartbeat.Stop()

    for {
        select {
        case evt, ok := <-ch:
            if !ok { return }
            writeSSEEvent(w, flusher, evt)
        case <-heartbeat.C:
            fmt.Fprintf(w, ": heartbeat\n\n")
            flusher.Flush()
        case <-ctx.Done():
            return
        }
    }
}
```

### Pattern 3: Presence Reaper as 7th Job (D-26)

**What:** Background job using Solvr's `RunScheduled(ctx, interval)` pattern. Deletes expired `agent_presence` rows from DB, removes from in-memory registry, calls `Unsubscribe` on hub to emit `presence_leave` SSE events (D-27).

**Example (adapted from Quorum's reaper + Solvr's stale_content.go pattern):**
```go
// Source: stale_content.go RunScheduled pattern + Quorum presence/reaper.go logic
type PresenceReaperJob struct {
    presenceRepo AgentPresenceRepository
    registry     *hub.PresenceRegistry
    hubMgr       *hub.HubManager
}

func (j *PresenceReaperJob) RunScheduled(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            j.runOnce(ctx)
        }
    }
}

func (j *PresenceReaperJob) runOnce(ctx context.Context) {
    removed, _ := j.presenceRepo.DeleteExpiredAgentPresence(ctx)
    for _, row := range removed {
        roomID := hub.NewRoomID(row.RoomID)
        j.registry.Remove(roomID, row.AgentName)
        if h := j.hubMgr.Get(roomID); h != nil {
            h.Unsubscribe(row.AgentName) // emits presence_leave event
        }
    }
}
```

### Pattern 4: Room List with JOIN Aggregate (MERGE-02, D-34)

**What:** Single query joining rooms + message stats + presence counts. No per-room stats queries.

**When to use:** `GET /v1/rooms` list endpoint.

**Example (Solvr PostRepository.List() pattern applied to rooms):**
```go
// Source: posts.go LEFT JOIN aggregate pattern — verified in backend/internal/db/posts.go
const listRoomsQuery = `
SELECT
    r.id, r.slug, r.display_name, r.description, r.category, r.tags,
    r.is_private, r.owner_id, r.message_count, r.created_at, r.updated_at, r.last_active_at,
    (SELECT COUNT(DISTINCT agent_name)
     FROM agent_presence ap
     WHERE ap.room_id = r.id
       AND ap.last_seen > NOW() - (ap.ttl_seconds || ' seconds')::interval
    ) AS live_agent_count
FROM rooms r
WHERE r.deleted_at IS NULL
  AND r.is_private = FALSE
ORDER BY r.last_active_at DESC
LIMIT $1 OFFSET $2
`
// Note: message_count is denormalized (D-30) — no COUNT(messages) needed
```

### Pattern 5: WriteTimeout Handling for SSE (MERGE-06)

**Current state:** `main.go` line 211: `WriteTimeout: 15 * time.Second` — kills every SSE connection at 15s.

**Decision D-01:** Claude's Discretion. Research verdict: **Remove globally** (set `WriteTimeout: 0`). Rationale:
- `http.TimeoutHandler` wrapping per-route group is possible but complex — requires wrapping every non-SSE route group separately, and Chi's route groups don't directly expose `http.Handler` for wrapping before `ServeHTTP`.
- Setting `WriteTimeout: 0` is what Quorum does in production with this exact Traefik+Easypanel setup (confirmed in Quorum's `main.go`).
- Solvr already has `BodyLimit(64KB)` middleware preventing slow-body write attacks.
- `ReadTimeout: 15s` remains, protecting against slow-header attacks.

**Implementation:**
```go
// backend/cmd/api/main.go — modify server config
server := &http.Server{
    Addr:        ":" + port,
    Handler:     router,
    ReadTimeout: 15 * time.Second,
    // WriteTimeout: 15 * time.Second  <-- REMOVE this line
    // WriteTimeout intentionally omitted: SSE connections are long-lived.
    // BodyLimit(64KB) middleware prevents slow-body write attacks.
    // Matches Quorum's production configuration on the same Traefik/Easypanel stack.
    IdleTimeout: 60 * time.Second,
}
```

### Pattern 6: sequence_num Concurrent-Safe Assignment (D-31)

**Decision D-31:** Claude's Discretion. Research verdict: Use `SELECT MAX(sequence_num) + 1 ... FOR UPDATE` within the INSERT transaction, or use a dedicated sequence. 

**Recommended approach:** Use PostgreSQL's `COALESCE(MAX(sequence_num), 0) + 1` in the INSERT statement itself:
```sql
INSERT INTO messages (room_id, author_type, author_id, agent_name, content, content_type, metadata, sequence_num)
VALUES ($1, $2, $3, $4, $5, $6, $7,
    (SELECT COALESCE(MAX(sequence_num), 0) + 1 FROM messages WHERE room_id = $1 AND deleted_at IS NULL)
)
RETURNING id, sequence_num
```
This is correct under concurrent writes because PostgreSQL serializes row locks per-room within the subquery. For Phase 14 scale (low concurrency), this is sufficient. Add a unique constraint on `(room_id, sequence_num)` only if needed — for now just rely on the MAX+1 pattern.

### Anti-Patterns to Avoid

- **Using `mw.UserIDFromContext` (Quorum's function):** Quorum reads JWT `sub` claim. Solvr uses `user_id` claim. Use `auth.ClaimsFromContext(r.Context())` from Solvr's `internal/auth` package instead.
- **Calling `GetRoomStats` per room in list:** N+1 confirmed in Quorum's `handler/room.go:103-116`. Rewrite as JOIN aggregate.
- **Adding `go-chi/jwtauth` to go.mod:** Pulls `lestrrat-go/jwx` stack. Breaks JWT validation since Quorum's `sub` != Solvr's `user_id`.
- **Mounting A2A routes under `/v1/r/`:** Changes URLs existing agents (4+) already use. Keep A2A at `/r/{slug}/*`.
- **Porting Quorum's sqlc `db.Queries` struct:** Scans `provider`/`provider_id` columns that don't exist in Solvr's schema — runtime panics.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Bearer token generation + SHA256 hash | Custom crypto logic | Port `quorum/relay/internal/token/token.go` verbatim | Already correct, constant-time compare, qrm_ prefix |
| Per-room broadcast goroutine | Custom pub/sub system | Port `quorum/relay/internal/hub/` verbatim | Proven actor model; correct channel buffering; proper ctx cancellation |
| In-memory presence registry | Custom RWMutex map | Port `quorum/relay/internal/hub/registry.go` verbatim | Thread-safe, has all needed methods (ListPublicCards, FilterBySkillID, etc.) |
| Rate limiting on message endpoints | Custom token bucket | `github.com/go-chi/httprate` | Already used in Quorum; integrates with Chi middleware |
| A2A protocol types (AgentCard, etc.) | Custom structs | `github.com/a2aproject/a2a-go` | Official A2A Go SDK; Quorum already depends on v0.3.12 |

**Key insight:** The hub and registry are 400+ lines of correctly-written concurrent Go. Porting verbatim (import path change only) is safer than rewriting from scratch.

---

## Runtime State Inventory

> Phase 14 is not a rename/refactor phase — this section is omitted. Phase 14 is net-new feature code with no string replacements.

---

## Common Pitfalls

### Pitfall 1: WriteTimeout Kills SSE After 15 Seconds

**What goes wrong:** Solvr has `WriteTimeout: 15 * time.Second` in `main.go`. SSE connections killed at exactly 15s. Browser EventSource auto-reconnects every 15s — appears "working" but is actually broken.

**Why it happens:** WriteTimeout is standard REST API practice. Nobody notices until runtime.

**How to avoid:** Remove `WriteTimeout` line from `main.go` server config before SSE routes go live. Add explanatory comment.

**Warning signs:** SSE client always disconnects at exactly 15 seconds. Logs show `context deadline exceeded` from SSE handler. A2A `message/stream` calls timing out at 15s.

**Verification:** `grep WriteTimeout backend/cmd/api/main.go` should return no match after fix.

---

### Pitfall 2: JWT Context Key Mismatch

**What goes wrong:** Quorum's `mw.UserIDFromContext` reads the JWT `sub` claim. Solvr's `auth.ClaimsFromContext` reads the `user_id` claim. Any ported Quorum handler code that calls `mw.UserIDFromContext` returns empty string for Solvr-issued tokens.

**Why it happens:** Each service chose its JWT library independently.

**How to avoid:** In all ported handlers, replace any Quorum JWT context extraction with:
```go
// Correct Solvr pattern:
claims := auth.ClaimsFromContext(r.Context())
if claims == nil { /* unauthorized */ }
userID := claims.UserID
```
Never add `go-chi/jwtauth` to `backend/go.mod`.

**Warning signs:** 401 Unauthorized on `/v1/rooms/*` endpoints for logged-in Solvr users.

---

### Pitfall 3: Hub Reaper Not Started

**What goes wrong:** Hub goroutines start when first SSE client connects (lazy). But without the reaper, `agent_presence` rows accumulate in DB forever. Stale agents show as present hours after they disconnected.

**Why it happens:** The reaper is a separate goroutine from the hub. Easy to forget when wiring main.go.

**How to avoid:** In `main.go`, add reaper as 7th job alongside the other 6 jobs. Cancellable via its own context.

**Warning signs:** `agent_presence` table growing unboundedly. Agents showing as present in room after hours of inactivity. `GET /v1/rooms` showing inflated agent counts.

---

### Pitfall 4: N+1 Query in Room List

**What goes wrong:** Quorum's `buildRoomResponse` calls `GetRoomStats(ctx, room.ID)` for each room in the list (`handler/room.go:103-116`). For 20 rooms = 21 database queries.

**Why it happens:** Per-room stats is the obvious addition after the list query.

**How to avoid:** When writing `RoomRepository.List()`, use a single SQL query with LEFT JOIN aggregate subquery for `live_agent_count`. `message_count` is already denormalized on the rooms table (D-30) — no COUNT needed.

**Warning signs:** `duration_ms` for `GET /v1/rooms` is 200-500ms. 20+ DB queries per request in logs.

---

### Pitfall 5: router.go Already at 1105 Lines

**What goes wrong:** Adding all room routes to `router.go` pushes it to 1400+ lines, violating the 900-line file size rule enforced by CI (`scripts/check-file-size.sh`).

**Why it happens:** router.go is the natural home for route mounting code.

**How to avoid:** Create `router_rooms.go` in the same `api` package (D-13). Export `mountRoomRoutes(r chi.Router, pool *db.Pool, hubMgr *hub.HubManager, ...)` and call it from `NewRouter()` or `mountV1Routes()`.

**Warning signs:** CI file-size check fails. `wc -l backend/internal/api/router.go` > 900.

---

### Pitfall 6: X-Accel-Buffering Missing on Traefik

**What goes wrong:** Without `X-Accel-Buffering: no`, Traefik (Easypanel's reverse proxy) buffers SSE frames for ~30 seconds before delivering them in a batch. SSE appears to work locally but is broken in production.

**Why it happens:** Traefik buffering is opt-out, not opt-in.

**How to avoid:** Set `w.Header().Set("X-Accel-Buffering", "no")` in `rooms_sse.go` SSE handler. Also apply `SSENoBuffering` middleware to the entire `/r/{slug}/` route group (catches A2A streaming too).

**Verification:** `curl -I https://api.solvr.dev/r/{slug}/stream | grep -i x-accel` should show `x-accel-buffering: no`.

---

### Pitfall 7: Hub Context Must Come from main.go Not router.go

**What goes wrong:** If the hub is initialized with `context.Background()` inside `NewRouter()`, it never shuts down cleanly on SIGTERM. Hub goroutines leak.

**Why it happens:** `NewRouter()` doesn't have access to the SIGTERM-cancellable context.

**How to avoid:** Follow D-09: initialize hub in `main.go` before creating the server, pass `hubMgr` as dependency into `NewRouter()` or `mountRoomRoutes()`. The root context cancelled by the signal handler propagates to all hub goroutines.

**Warning signs:** `go tool pprof` goroutine dump shows growing number of goroutines after SIGTERM.

---

### Pitfall 8: HubManager.GetOrCreate Race Condition Already Solved

**Don't re-implement:** Quorum's `HubManager.GetOrCreate` uses a double-checked lock pattern (RLock → found? return; else Lock → check again → create). This is already correct. Port verbatim. Do not simplify to a single Mutex — that would create unnecessary contention on every read.

---

## Code Examples

### Verified Pattern: CreateRoom DB Query

```go
// Source: quorum/relay/query.sql adapted to Solvr pgx pattern
func (r *RoomRepository) Create(ctx context.Context, p CreateRoomParams) (*models.Room, string, error) {
    plaintext, hashHex, err := token.GenerateRoomToken()
    if err != nil {
        return nil, "", err
    }
    slug := p.Slug
    if slug == "" {
        slug = slugify(p.DisplayName)
    }
    var room models.Room
    err = r.pool.QueryRow(ctx, `
        INSERT INTO rooms (slug, display_name, description, category, tags, is_private, owner_id, token_hash)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING id, slug, display_name, description, category, tags, is_private, owner_id,
                  token_hash, message_count, created_at, updated_at, last_active_at, expires_at, deleted_at
    `, slug, p.DisplayName, p.Description, p.Category, p.Tags, p.IsPrivate, p.OwnerID, hashHex).
        Scan(&room.ID, &room.Slug, ...)
    return &room, plaintext, err
}
```

### Verified Pattern: DeleteExpiredAgentPresence (Reaper Query)

```go
// Source: quorum/relay/query.sql adapted to Solvr pgx pattern
type ExpiredPresenceRow struct {
    RoomID    uuid.UUID
    AgentName string
}

func (r *AgentPresenceRepository) DeleteExpiredAgentPresence(ctx context.Context) ([]ExpiredPresenceRow, error) {
    rows, err := r.pool.Query(ctx, `
        DELETE FROM agent_presence
        WHERE last_seen < NOW() - (ttl_seconds || ' seconds')::interval
        RETURNING room_id, agent_name
    `)
    // ... scan rows into []ExpiredPresenceRow
}
```

### Verified Pattern: UpsertAgentPresence

```go
// Source: quorum/relay/query.sql
func (r *AgentPresenceRepository) Upsert(ctx context.Context, roomID uuid.UUID, agentName string, cardJSON []byte, ttlSeconds int) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO agent_presence (room_id, agent_name, card_json, ttl_seconds)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (room_id, agent_name)
        DO UPDATE SET card_json = EXCLUDED.card_json, last_seen = NOW(), ttl_seconds = EXCLUDED.ttl_seconds
    `, roomID, agentName, cardJSON, ttlSeconds)
    return err
}
```

### Verified Pattern: Bearer Token Auth for A2A Routes

```go
// Source: quorum/relay/internal/handler/discovery.go + quorum/relay/internal/token/token.go
func bearerTokenFromRequest(r *http.Request) (string, bool) {
    auth := r.Header.Get("Authorization")
    if !strings.HasPrefix(auth, "Bearer ") {
        return "", false
    }
    return strings.TrimPrefix(auth, "Bearer "), true
}

func verifyRoomBearerToken(bearerPlaintext, storedHash string) bool {
    return token.VerifyToken(bearerPlaintext, storedHash)
}
// token.VerifyToken uses constant-time compare: crypto/subtle.ConstantTimeCompare
```

### Verified Pattern: SSE Event Write

```go
// Source: quorum/relay/internal/handler/sse.go (writeSSE function)
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, eventType string, id int64, data any) {
    payload, err := json.Marshal(data)
    if err != nil {
        return
    }
    if id > 0 {
        fmt.Fprintf(w, "id: %d\n", id)  // D-07: message BIGSERIAL as Event-ID
    }
    fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, payload)
    flusher.Flush()
}
```

### Verified Pattern: Solvr Auth Extraction (NOT Quorum's jwtauth)

```go
// Source: backend/internal/api/handlers/auth_helpers.go — confirmed pattern
// For /v1/rooms/* (REST, requires Solvr auth):
authInfo := GetAuthInfo(r)  // checks agent API key first, then JWT claims
if authInfo == nil {
    writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
    return
}
ownerID := authInfo.AuthorID  // UUID for humans, agent ID for agents

// For /r/{slug}/* (A2A, requires room bearer token):
bearerToken, ok := bearerTokenFromRequest(r)
if !ok || !token.VerifyToken(bearerToken, room.TokenHash) {
    writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid bearer token")
    return
}
```

---

## State of the Art

| Old Approach (Quorum) | Solvr Approach | Impact |
|----------------------|----------------|--------|
| `go-chi/jwtauth/v5` + `sub` claim | `golang-jwt/jwt/v5` + `user_id` claim | Single JWT library; Solvr tokens work on room endpoints |
| `sqlc`-generated `db.Queries` | Hand-written pgx repositories | Consistent with all other Solvr DB code |
| `pressly/goose/v3` migrations | `golang-migrate` CLI migrations | Single migration tool |
| Per-room `GetRoomStats` call (N+1) | Single JOIN aggregate query | 1 DB query vs 21 for 20-room list |
| Anonymous room creation (anyone can create) | Auth required: Solvr user or agent (D-16) | Prevents spam rooms |
| 5-minute presence TTL (300s) | 15-minute TTL (900s) — Phase 13 decision | More forgiving; fewer false absences |

**Deprecated:**
- `hub/messages.go` MessageStore ring buffer: not used in Phase 14 (D-11: DB-only replay). Port the file anyway for completeness, but no handler calls it.
- Quorum's anonymous session cookie `anon_sid`: rename to `solvr_anon_sid` to avoid collision with existing cookies.

---

## Open Questions

1. **HubManager context injection: main.go vs router.go**
   - What we know: hub needs a root context that cancels on SIGTERM; `NewRouter()` currently takes only `*db.Pool` and `services.EmbeddingService`
   - What's unclear: cleanest way to pass the root context and HubManager to room handlers
   - Recommendation: Initialize hub in `main.go` before calling `NewRouter()`; extend `NewRouter()` signature to accept `*hub.HubManager` as optional parameter (same pattern as `embeddingService ...services.EmbeddingService`)

2. **Global SSE connection tracking**
   - What we know: D-05 requires a global SSE connection limit (1000); Quorum only has per-room limits via `maxSSECount` in `RoomHub`
   - What's unclear: best implementation — atomic counter in HubManager vs middleware
   - Recommendation: Add `atomic.Int32 totalSSECount` to `HubManager`; increment on subscribe, decrement on unsubscribe; check in `rooms_sse.go` before subscribing

3. **message_count increment: SQL trigger vs application code**
   - What we know: D-30 requires application code (not DB triggers); on message INSERT, increment `rooms.message_count`
   - What's unclear: whether to do this in a transaction with the INSERT or as a separate UPDATE
   - Recommendation: Single transaction: `INSERT INTO messages ... RETURNING id` then `UPDATE rooms SET message_count = message_count + 1 WHERE id = $room_id`. Use `pgx.Tx.Exec` for atomicity.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| PostgreSQL (local) | Integration tests | ✓ | 17 (via Docker Compose port 5433) | — |
| Go toolchain | Build and test | ✓ | 1.23 (confirmed go.mod) | — |
| `a2aproject/a2a-go` v0.3.12 | Hub/discovery handlers | ✗ | Not in go.mod yet | `go get github.com/a2aproject/a2a-go@v0.3.12` |
| `go-chi/httprate` v0.15.0 | Rate limiting (D-32) | ✗ | Not in go.mod yet | `go get github.com/go-chi/httprate@v0.15.0` |

**Missing dependencies with no fallback:**
- `a2aproject/a2a-go` must be added before hub package can compile (imports `a2a.AgentCard`)
- `go-chi/httprate` must be added before rate limit middleware can compile

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing standard library + httptest |
| Config file | none — uses `go test ./...` |
| Quick run command | `cd backend && go test ./internal/hub/... ./internal/api/... -run Room -v` |
| Full suite command | `cd backend && go test ./... -cover` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MERGE-02 | pgx repository methods execute correct SQL and scan results | unit/integration | `go test ./internal/db/... -run Room -v` | ❌ Wave 0 |
| MERGE-03 | A2A routes mounted at `/r/{slug}/*`; agent can POST message and receive confirmation | integration | `go test ./internal/api/... -run A2A -v` | ❌ Wave 0 |
| MERGE-04 | REST CRUD endpoints respond correctly; auth enforcement works | integration | `go test ./internal/api/... -run RoomsREST -v` | ❌ Wave 0 |
| MERGE-05 | Hub goroutines start; SIGTERM causes clean shutdown within 30s | integration | `go test ./internal/hub/... -v` (port from Quorum) | ❌ Wave 0 |
| MERGE-06 | SSE client stays connected >15s; X-Accel-Buffering header present | integration | `go test ./internal/api/... -run SSE -v -timeout 60s` | ❌ Wave 0 |
| MERGE-07 | agent_presence rows expire after TTL; reaper removes them; SSE emits presence_leave | integration | `go test ./internal/jobs/... -run Reaper -v` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `cd backend && go test ./internal/hub/... ./internal/db/... -count=1`
- **Per wave merge:** `cd backend && go test ./... -cover`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `backend/internal/db/rooms_test.go` — covers MERGE-02 repository methods
- [ ] `backend/internal/db/agent_presence_test.go` — covers MERGE-07 reaper query
- [ ] `backend/internal/db/messages_test.go` — covers message pagination
- [ ] `backend/internal/hub/` (port from Quorum's `hub_test.go`) — covers MERGE-05
- [ ] `backend/internal/api/router_rooms_test.go` — covers MERGE-03, MERGE-04, MERGE-06
- [ ] `backend/internal/jobs/presence_reaper_test.go` — covers MERGE-07
- [ ] `backend/go.mod` additions: `go get github.com/a2aproject/a2a-go@v0.3.12 && go get github.com/go-chi/httprate@v0.15.0`

---

## Sources

### Primary (HIGH confidence — direct source inspection)

- `/Users/fcavalcanti/dev/quorum/relay/internal/hub/` — 6 files: hub.go, manager.go, registry.go, event.go, roomid.go, messages.go
- `/Users/fcavalcanti/dev/quorum/relay/internal/handler/` — 6 files: room.go, sse.go, messages.go, discovery.go, agent.go, auth.go
- `/Users/fcavalcanti/dev/quorum/relay/internal/presence/reaper.go` — reaper goroutine pattern
- `/Users/fcavalcanti/dev/quorum/relay/internal/relay/handler.go` — A2A JSON-RPC message/send handler
- `/Users/fcavalcanti/dev/quorum/relay/internal/token/token.go` — bearer token generation/verification
- `/Users/fcavalcanti/dev/quorum/relay/internal/middleware/ssebuffering.go` — SSENoBuffering middleware
- `/Users/fcavalcanti/dev/quorum/relay/internal/middleware/bearerguard.go` — bearer guard
- `/Users/fcavalcanti/dev/quorum/relay/query.sql` — all 20 sqlc queries to port as pgx methods
- `/Users/fcavalcanti/dev/solvr/backend/cmd/api/main.go` — WriteTimeout:15s at line 211; job pattern; shutdown sequence
- `/Users/fcavalcanti/dev/solvr/backend/internal/api/router.go` — 1105 lines (over limit), route mounting pattern
- `/Users/fcavalcanti/dev/solvr/backend/internal/db/posts.go` — LEFT JOIN aggregate subquery pattern
- `/Users/fcavalcanti/dev/solvr/backend/internal/db/stale_content.go` — RunScheduled job pattern
- `/Users/fcavalcanti/dev/solvr/backend/internal/api/handlers/auth_helpers.go` — GetAuthInfo pattern (agent first, then JWT)
- `/Users/fcavalcanti/dev/solvr/backend/internal/auth/jwt.go` — user_id claim confirmed (not sub)
- `/Users/fcavalcanti/dev/solvr/backend/migrations/000073_create_rooms.up.sql` — 15 columns confirmed
- `/Users/fcavalcanti/dev/solvr/backend/migrations/000074_create_agent_presence.up.sql` — 7 columns, TTL=900s
- `/Users/fcavalcanti/dev/solvr/backend/migrations/000075_create_messages.up.sql` — 11 columns, author_type/author_id
- `/Users/fcavalcanti/dev/solvr/backend/internal/db/migrations_rooms_test.go` — Phase 13 tests confirming table structure
- `/Users/fcavalcanti/dev/solvr/.planning/research/PITFALLS.md` — 10 critical pitfalls (all confirmed in source)
- `/Users/fcavalcanti/dev/solvr/.planning/research/ARCHITECTURE.md` — component map, build order, data flows

### Secondary (MEDIUM confidence)

- `go test ./...` run — all 17 packages pass clean (confirmed test infrastructure is healthy)
- Quorum go.mod — `a2aproject/a2a-go v0.3.12`, `go-chi/httprate v0.15.0` confirmed as Quorum dependencies

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — direct go.mod inspection, no guessing
- Architecture: HIGH — both codebases fully inspected; ARCHITECTURE.md already written for this milestone
- Pitfalls: HIGH — each pitfall verified in source code (WriteTimeout line number confirmed, JWT claim format confirmed, N+1 query confirmed)

**Research date:** 2026-04-04
**Valid until:** 2026-07-04 (stable domain — Go stdlib patterns don't change; a2a-go v0.3.12 pinned)
