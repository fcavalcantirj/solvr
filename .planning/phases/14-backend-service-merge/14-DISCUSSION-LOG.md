# Phase 14: Backend Service Merge - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-04
**Phase:** 14-backend-service-merge
**Areas discussed:** SSE timeout strategy, Hub manager placement, Route organization, Presence reaper design, Repository layer design, Room token management, Message sequence numbering, Error handling patterns, Agent card handling, Room update/edit permissions, Testing strategy, Quorum code reuse vs rewrite

---

## SSE Timeout Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Remove WriteTimeout globally | Set WriteTimeout: 0 for the whole server. Simpler, runs behind nginx/Cloudflare. | |
| Per-route middleware | Keep 15s for normal routes, disable for SSE. More complex. | |
| You decide | Claude picks best approach | ✓ |

**User's choice:** You decide
**Notes:** Claude has discretion on WriteTimeout strategy

---

### X-Accel-Buffering Header

| Option | Description | Selected |
|--------|-------------|----------|
| Go handler | Set in SSE handler itself. Self-contained. Quorum does this. | ✓ |
| Nginx config | Configure nginx for /r/*/stream routes. | |
| Both | Belt and suspenders. | |

**User's choice:** Go handler (Recommended)

---

### SSE Connection Lifetime

| Option | Description | Selected |
|--------|-------------|----------|
| 30-minute max | Force reconnect after 30min. Client auto-reconnects. | ✓ |
| Indefinite | Keep open as long as client stays. | |

**User's choice:** 30-minute max

---

### SSE Heartbeat

| Option | Description | Selected |
|--------|-------------|----------|
| 30s heartbeat | Send SSE comment ping every 30s. Standard practice. | ✓ |
| No heartbeat | Rely on TCP keepalive only. | |

**User's choice:** 30s heartbeat

---

### SSE Connection Limits

| Option | Description | Selected |
|--------|-------------|----------|
| Global limit only | Cap total SSE connections (e.g., 1000). | ✓ |
| Per-room + global | Limit per room AND globally. | |
| No limits | Skip limits for v0. | |

**User's choice:** Global limit only

---

### SSE Event Types

| Option | Description | Selected |
|--------|-------------|----------|
| All four | message + presence_join + presence_leave + room_update | ✓ |
| Messages only | Only message events. | |

**User's choice:** All four (Recommended)

---

### SSE Reconnection Replay

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, with message ID | Use BIGSERIAL id as Event-ID. Replay from DB. | ✓ |
| No replay | Reconnecting clients start fresh. | |

**User's choice:** Yes, with message ID

---

## Hub Manager Placement

| Option | Description | Selected |
|--------|-------------|----------|
| Own package: internal/hub | Mirror Quorum's structure. Clean separation. | ✓ |
| Inside internal/services | Add hub as a service. Follows Solvr's existing pattern. | |

**User's choice:** Initially preferred following Solvr's architecture, but agreed to internal/hub as own package after discussion about hub's unique lifecycle/concurrency concerns. Initialized in main.go like background jobs, injected as dependency.

---

### Hub Shutdown

| Option | Description | Selected |
|--------|-------------|----------|
| Context cancellation | Hub accepts context, cancels on SIGTERM. Same as http.Server. | ✓ |
| Explicit Stop() method | Called after server.Shutdown(). | |

**User's choice:** Context cancellation

---

### Hub Message History

| Option | Description | Selected |
|--------|-------------|----------|
| DB-only replay | Hub is purely a broadcast relay. No message buffering. | ✓ |
| Small ring buffer | Keep last N messages per room in memory. | |

**User's choice:** DB-only replay

---

### Hub Room Creation

| Option | Description | Selected |
|--------|-------------|----------|
| Lazy creation | Hub room created on first connection/message. | ✓ |
| Eager load | Load all rooms at startup. | |

**User's choice:** Lazy creation

---

## Route Organization

### Router Split

| Option | Description | Selected |
|--------|-------------|----------|
| router_rooms.go | Extract room routes into separate file. | ✓ |
| router_v1.go + router_a2a.go | Split by API version. | |

**User's choice:** router_rooms.go

---

### Handler Files

| Option | Description | Selected |
|--------|-------------|----------|
| Split by concern | rooms.go, rooms_messages.go, rooms_sse.go, rooms_presence.go | ✓ |
| Single rooms.go | All room handlers in one file. | |

**User's choice:** Split by concern

---

### Route Paths

| Option | Description | Selected |
|--------|-------------|----------|
| Separate paths | /r/{slug}/* for A2A, /v1/rooms/* for REST. Different auth. | ✓ |
| Unified under /v1 | Everything under /v1/rooms. | |
| Both paths, same handlers | Dual routing, same code. | |

**User's choice:** Separate paths

---

### A2A Auth

| Option | Description | Selected |
|--------|-------------|----------|
| Room bearer token | Each room has token_hash. Agents authenticate with Bearer token. | ✓ |
| Solvr agent API keys | Use existing solvr_ keys. | |
| Both accepted | Accept either. | |

**User's choice:** Room bearer token
**Notes:** User clarified: ALL agents should be able to join rooms (like Quorum), not just Solvr-registered agents. Room token is the auth — no Solvr account needed for A2A participation.

---

### REST Auth

**User's choice:** Solvr users OR registered Solvr agents (JWT / solvr_ API key) can create rooms. A2A participation = room token only.

---

### Room List & Detail Access

| Option | Description | Selected |
|--------|-------------|----------|
| Public list | GET /v1/rooms is public. Needed for SSR and SEO. | ✓ |
| Public detail | GET /v1/rooms/{slug} is public for public rooms. | ✓ |

---

### Slug Generation

**User's choice:** Auto-generate from display_name if not present. Client can override with custom slug.

---

### Room Deletion

| Option | Description | Selected |
|--------|-------------|----------|
| Soft-delete, owner only | Set deleted_at. Owner + admin can delete. | ✓ |
| Hard-delete | Remove row permanently. | |

**User's choice:** Soft-delete, owner only

---

## Presence Reaper Design

### Integration

| Option | Description | Selected |
|--------|-------------|----------|
| New background job | 7th job using existing pattern. Every 60s. | ✓ |
| Hub-internal goroutine | Reaper inside hub package. | |

**User's choice:** New background job

---

### SSE Events on Expiry

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, emit event | Broadcast presence_leave when presence expires. | ✓ |
| Silent cleanup | Just delete DB records. | |

**User's choice:** Yes, emit event

---

### Presence Renewal

| Option | Description | Selected |
|--------|-------------|----------|
| POST heartbeat | Explicit heartbeat endpoint. | |
| Implicit via messages | Messages auto-renew presence. | |
| Both | Heartbeat AND message auto-renewal. | ✓ |

**User's choice:** Both

---

### Room Expiry

| Option | Description | Selected |
|--------|-------------|----------|
| Same reaper job | Handles expired presence AND expired rooms. | ✓ |
| Separate job | Own background job for room expiry. | |
| Defer | Don't implement room expiry yet. | |

**User's choice:** Same reaper job

---

## Repository Layer Design

### Room List Query

| Option | Description | Selected |
|--------|-------------|----------|
| Single JOIN query | LEFT JOIN aggregate subqueries. Matches PostRepository.List(). | ✓ |
| Separate queries | Fetch rooms, then batch-fetch counts. | |

**User's choice:** Single JOIN query

---

### Message Pagination

| Option | Description | Selected |
|--------|-------------|----------|
| Cursor-based on message ID | Use BIGSERIAL as cursor (?after=12345). | ✓ |
| Offset-based | Traditional ?page=1&per_page=50. | |

**User's choice:** Cursor-based on message ID

---

## Room Token Management

### Token Generation

| Option | Description | Selected |
|--------|-------------|----------|
| Same as Quorum | Crypto-random, SHA256 hash, plaintext once on creation. | ✓ |
| Solvr's dual-hash | SHA256+bcrypt. More secure but slower. | |

**User's choice:** Same as Quorum

---

### Token Rotation

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, rotatable | POST /v1/rooms/{slug}/rotate-token. | ✓ |
| No rotation | Token is permanent. | |

**User's choice:** Yes, rotatable

---

## Message Sequence Numbering

**User's choice:** Application code (per Phase 13 D-26). Claude's discretion on exact mechanism — must be correct under concurrent writes.

---

## Error Handling

### Rate Limiting

| Option | Description | Selected |
|--------|-------------|----------|
| Per-room rate limit | Limit messages per room per minute. | ✓ |
| Per-agent per-room | More granular but complex with bearer tokens. | |
| No rate limit | Skip for v0. | |

**User's choice:** Per-room rate limit

---

### Deleted Room Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| 404 Not Found | Soft-deleted rooms return 404 for all operations. | ✓ |
| 410 Gone | More informative status code. | |

**User's choice:** 404 Not Found

---

## Agent Card Handling

**User's choice:** Follow basic structure (name, description, avatar_url, capabilities) but allow additional custom fields in JSONB. Not fully open — has expected fields but extensible.

---

## Room Update/Edit Permissions

**User's choice:** All metadata fields editable by owner except slug (immutable after creation). Editable: display_name, description, category, tags, is_private.

---

## Testing Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Integration tests with real HTTP | httptest.Server, real SSE clients, assert events. | ✓ |
| Unit tests with mocks | Mock hub interface, test handlers isolated. | |
| Both layers | Unit + integration. | |

**User's choice:** Integration tests with real HTTP

---

## Quorum Code Reuse vs Rewrite

| Option | Description | Selected |
|--------|-------------|----------|
| Adapt hub, rewrite handlers | Hub core logic adapted. Handlers rewritten for Solvr's patterns. | ✓ |
| Rewrite everything | Use Quorum as reference only. | |
| Copy-paste and modify | Copy wholesale, then modify. | |

**User's choice:** Adapt hub, rewrite handlers

---

### Query Port Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Translate to pgx | Port each sqlc query to raw pgx. | |
| Redesign queries | Use Quorum as reference, redesign for Solvr's patterns. | ✓ |

**User's choice:** Redesign queries

---

## Claude's Discretion

- WriteTimeout removal strategy (global vs per-route)
- Exact sequence_num increment mechanism
- SSE connection limit number
- Rate limit values for message posting
- Hub file organization within internal/hub/
- Exact query designs for room list and message pagination

## Deferred Ideas

- Private room access control (column exists, no logic until Phase 16+)
- Room creation from frontend UI (API/A2A only for now)
- Message editing
- Per-room SSE connection limits
- Agent card schema enforcement
- Room search/discovery features
- WebSocket support
