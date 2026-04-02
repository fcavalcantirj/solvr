# Pitfalls Research

**Domain:** Go monolith merge — two Go services into one, rooms + SSE addition, post type removal, live search analytics
**Researched:** 2026-04-02
**Confidence:** HIGH — all findings verified directly from source code of both services

---

## Critical Pitfalls

### Pitfall 1: Duplicate Table Names Will Silently Corrupt Data

**What goes wrong:**
Both services have `users` and `refresh_tokens` tables in PostgreSQL. Quorum's `users` has 6 columns (`id`, `email`, `display_name`, `avatar_url`, `provider`, `provider_id`). Solvr's `users` has 14+ columns (`username`, `auth_provider`, `auth_provider_id`, `bio`, `role`, `referral_code`, `password_hash`, `storage_used_bytes`, etc.). The column names for the same concept differ: Quorum uses `provider` and `provider_id`, Solvr uses `auth_provider` and `auth_provider_id`.

If Quorum's goose migration `00001_init.sql` runs against Solvr's database, `CREATE TABLE users` fails with a duplicate table error. The tempting workaround — adding `IF NOT EXISTS` — silently keeps the wrong schema. Quorum's sqlc-generated `db.Queries` will then try to scan a `provider` column that does not exist and panic at runtime. Both `refresh_tokens` tables have identical structure, making this one safe to skip, but the migration system must not attempt to create it.

**Why it happens:**
Both services were built independently as standalone apps. They both needed users and refresh tokens. Nobody noticed the naming conflict because they ran on separate databases.

**How to avoid:**
Do NOT copy Quorum's migration files. Write new Solvr migrations (000073+) that add ONLY the three net-new tables: `rooms`, `messages`, and `agent_presence`. Rewrite Quorum's DB query layer to use Solvr's `users` column names. Do not use Quorum's sqlc-generated `db.Queries` directly in the merged service.

**Warning signs:**
- Any migration file with `CREATE TABLE users` or `CREATE TABLE refresh_tokens`
- `ERROR: column "provider" of relation "users" does not exist` at runtime
- `go-chi/jwtauth` or `go-chi/jwtauth/v5` appearing in `backend/go.mod`

**Phase to address:**
Phase 1 (Database Migration) — must be resolved before any code is merged.

---

### Pitfall 2: JWT Claims Structure Mismatch — Quorum Tokens Rejected by Solvr Auth

**What goes wrong:**
Solvr JWT claims use a custom `user_id` field (verified in `backend/internal/auth/jwt.go`: `UserID string json:"user_id"`). Quorum JWT claims use the standard RFC 7519 `sub` field (verified in `relay/internal/middleware/jwtauth.go`: `jwtauth.FromContext` reads `sub` via `go-chi/jwtauth/v5`). A Solvr-issued JWT passed to Quorum's middleware returns empty string from `UserIDFromContext` because `sub` is absent. A Quorum-issued JWT passed to Solvr's `auth.ValidateJWT` fails because `user_id` is absent.

Additionally, Quorum uses `go-chi/jwtauth/v5` (wraps `lestrrat-go/jwx/v3`) while Solvr uses `golang-jwt/jwt/v5` directly. These are different libraries with different validation paths and error types. After merge, only one JWT library should exist in the binary.

**Why it happens:**
Each service chose its JWT library independently. `go-chi/jwtauth` standardizes on RFC 7519 `sub` claim. Solvr was built before the Quorum pattern was established.

**How to avoid:**
Unify on Solvr's `golang-jwt/jwt/v5` pattern — it is already in `go.mod` and has no competing dependency. When porting Quorum's room and auth handlers:
1. Replace `jwtauth.FromContext(ctx)` calls with Solvr's middleware pattern that reads `user_id` from context.
2. Do not add `go-chi/jwtauth` to Solvr's `go.mod` — it pulls the entire `lestrrat-go/jwx` stack unnecessarily.
3. If Quorum has live users with unexpired tokens when the merge happens, those tokens will be invalid on the merged service. Acceptable since Quorum users will re-auth to Solvr.

**Warning signs:**
- 401 Unauthorized on room endpoints for users logged into Solvr
- `go-chi/jwtauth/v5` in `backend/go.mod`
- `userIDStr, ok := mw.UserIDFromContext(ctx)` returning `ok = false` for Solvr-issued tokens

**Phase to address:**
Phase 2 (Service Merge) — auth middleware must be unified in the first working merge.

---

### Pitfall 3: Solvr's `WriteTimeout: 15s` Kills Every SSE Connection After 15 Seconds

**What goes wrong:**
Solvr's HTTP server is configured with `WriteTimeout: 15 * time.Second` (verified in `backend/cmd/api/main.go`). SSE connections are long-lived — a browser holds the connection open for minutes. When `WriteTimeout` is set globally, Go's `net/http` forcibly closes any response that has not completed within the timeout. SSE connections never "complete" by design. Result: every SSE client gets disconnected after exactly 15 seconds.

Quorum explicitly disables `WriteTimeout` with a comment: "WriteTimeout disabled — SSE connections are long-lived" (verified in `relay/cmd/server/main.go`). Solvr has never needed this because it has no streaming endpoints.

The same `WriteTimeout` also kills the A2A JSON-RPC `message/stream` handler, breaking all A2A agent streaming interactions, not just browser SSE clients.

**Why it happens:**
`WriteTimeout` is standard best practice for REST APIs. Nobody removes it when adding streaming because the failure only appears at runtime after exactly 15 seconds — not immediately.

**How to avoid:**
Remove `WriteTimeout` from the global `http.Server` config in `cmd/api/main.go` when SSE routes are added. This is intentional. Add a comment explaining why. There is no way to set per-route write timeouts in Go's `net/http`.

**Warning signs:**
- SSE client always disconnects at exactly 15 seconds
- `context deadline exceeded` errors in logs from SSE handlers
- Browser EventSource auto-reconnects every 15 seconds
- A2A `message/stream` calls timing out at exactly 15 seconds

**Phase to address:**
Phase 3 (SSE Integration) — must change server config before any SSE routes go live.

---

### Pitfall 4: Removing the `question` Post Type Breaks DB Constraint Without Migrating Data First

**What goes wrong:**
The `posts` table has `CONSTRAINT posts_type_check CHECK (type IN ('problem', 'question', 'idea'))`. There are 9 existing question posts in production. If the migration drops 'question' from this constraint before soft-deleting those rows, PostgreSQL will reject the `ALTER TABLE` with `ERROR: check constraint "posts_type_check" of relation "posts" is violated by some row`. The 9 existing rows prevent the constraint change from applying.

Additionally, `question` is referenced in approximately 12 distinct files. One specific case is dangerous: `ResponseTypeQuestion = "question"` in `models/response.go` is a DIFFERENT concept — it is a response type for ideas (not a post type) and must NOT be removed when removing the question post type.

Files with post-type `question` references that all need updating:
- `models/post.go` — `PostTypeQuestion` constant
- `db/questions.go` — entire file (questions-specific repository)
- `db/stats_questions.go` — stats queries with `type = 'question'`
- `db/feed.go` — feed queries filtering `p.type = 'question'`
- `db/agents.go` — `COUNT(*) FILTER (WHERE type = 'question')`
- `api/handlers/questions.go` — entire file
- `api/openapi_schemas.go` — enum values include 'question'
- `api/handlers/posts.go` — validation error message mentions 'question'
- `api/handlers/mcp.go` — MCP schema enum includes 'question'
- `api/handlers/search.go` — type filter documentation
- `api/openapi_paths.go` — filter documentation

**Why it happens:**
The question type is deeply threaded through the codebase. The risk of accidentally removing `ResponseTypeQuestion` (idea responses) while removing `PostTypeQuestion` (post type) is real because both use the string `"question"`.

**How to avoid:**
Exact required sequence:
1. Migration: `UPDATE posts SET deleted_at = NOW() WHERE type = 'question' AND deleted_at IS NULL` (soft-delete 9 rows)
2. Migration: `ALTER TABLE posts DROP CONSTRAINT posts_type_check; ALTER TABLE posts ADD CONSTRAINT posts_type_check CHECK (type IN ('problem', 'idea'))`
3. Code: remove all handler, model, and db files for question post type
4. Do NOT touch `ResponseTypeQuestion` in `models/response.go` — that is an idea response type

**Warning signs:**
- `ERROR: check constraint "posts_type_check" of relation "posts" is violated by some row` on migration
- Ideas losing the ability to receive "question" responses after the type removal
- 9 questions still visible in UI after removal

**Phase to address:**
Phase 1 (Database Migration) — constraint change must happen in the same migration that soft-deletes the existing rows.

---

### Pitfall 5: Migration Tool Mismatch — Goose Format vs Golang-Migrate Format

**What goes wrong:**
Quorum uses `pressly/goose/v3` with SQL files annotated with `-- +goose Up` / `-- +goose Down` markers. Solvr uses the external `golang-migrate` CLI with separate `.up.sql` / `.down.sql` files and no annotations. If Quorum's migration files are copied into Solvr's `migrations/` directory as-is, `golang-migrate` fails with a syntax error on the `-- +goose Up` annotation. Or worse, the annotation line is treated as a comment and partially executes, leaving the migration state table in an inconsistent state.

Solvr's last migration is `000072`. The next migration number must be `000073`.

**Why it happens:**
Both are valid migration systems with incompatible file formats. The Quorum files are self-contained with goose annotations; Solvr files are pure SQL split into two files.

**How to avoid:**
Write new Solvr-format migrations from scratch using the table DDL from Quorum's `schema.sql` as reference. Never copy Quorum's `.sql` migration files:
- `000073_create_rooms.up.sql` / `.down.sql` — rooms table only
- `000074_create_messages.up.sql` / `.down.sql` — messages table
- `000075_create_agent_presence.up.sql` / `.down.sql` — agent_presence table

**Warning signs:**
- `ERROR: syntax error at or near "--"` during migration
- `-- +goose Up` appearing as content in Solvr's migrations directory
- Both `schema_migrations` (golang-migrate) and `goose_db_version` (goose) tables existing in the same database

**Phase to address:**
Phase 1 (Database Migration) — the first migration written must use the correct format.

---

### Pitfall 6: Quorum's `rooms.owner_id` FK References a Different `users` Population

**What goes wrong:**
In Quorum's schema, `rooms.owner_id` is a FK to Quorum's `users.id`. Quorum's `users` table only has users who authenticated via OAuth on Quorum. Solvr's `users` table has users who authenticated via Solvr. These are different user populations with different UUIDs, even if some individuals have accounts in both.

When migrating Quorum's room data to Solvr's database, a naive FK-preserving copy (`INSERT INTO rooms SELECT * FROM quorum.rooms`) will violate the FK constraint because Quorum's user UUIDs don't exist in Solvr's `users` table. The matching must be done by email (the common identity anchor).

Additionally, some Quorum room owners may never have signed into Solvr. These rooms should have `owner_id = NULL` after migration (preserving the `ON DELETE SET NULL` behavior).

**Why it happens:**
Cross-service data migration where two `users` tables represent the same concept but contain different user populations with different primary keys.

**How to avoid:**
Data migration script must join on email: `INSERT INTO rooms (..., owner_id) SELECT ..., su.id FROM quorum_rooms qr LEFT JOIN quorum_users qu ON qu.id = qr.owner_id LEFT JOIN solvr_users su ON su.email = qu.email`. Rooms whose owners don't exist in Solvr get `owner_id = NULL`. Verify post-migration with `SELECT COUNT(*) FROM rooms WHERE owner_id IS NOT NULL AND owner_id NOT IN (SELECT id FROM users)` — should be 0.

**Warning signs:**
- FK violation errors during data migration INSERT
- Rooms showing `owner_id = NULL` for rooms that had owners in Quorum
- Users unable to manage rooms they created in Quorum

**Phase to address:**
Phase 1 (Database Migration) — data migration script must be written and verified before going live.

---

### Pitfall 7: Quorum's Hub In-Memory State Is Not Initialized on Solvr Startup

**What goes wrong:**
Quorum's `HubManager` and `PresenceRegistry` are in-memory. They are populated lazily as agents join rooms. After a server restart (deploy), all in-memory hub state is gone. Agents that were "present" before restart don't reappear until they send a heartbeat. This is fine. But `presence.StartReaper` (the background goroutine that evicts TTL-expired agents from `agent_presence` DB rows and the in-memory registry) must be started in Solvr's `main.go` or agents accumulate in `agent_presence` forever and appear as present when they are not.

**Why it happens:**
Quorum calls `presence.StartReaper(ctx, ...)` in its own `main.go`. When porting the hub infrastructure into Solvr's `main.go`, it is easy to wire up the hub manager and registry but forget the reaper goroutine.

**How to avoid:**
Port the complete hub startup from Quorum's `main.go` into Solvr's `main.go`: registry, hub manager, and reaper — all three. Add a test that verifies the reaper cleans up stale `agent_presence` rows after TTL expiry.

**Warning signs:**
- `agent_presence` table growing unboundedly after rooms become inactive
- Agents showing as present in room even after they've been offline for hours
- `GetRoomStats.UniqueAgents` count inflating over time

**Phase to address:**
Phase 2 (Service Merge) — reaper must be started alongside the hub manager.

---

### Pitfall 8: Solvr's N+1 Query Pattern in Room List Will Break Under Load

**What goes wrong:**
Quorum's `buildRoomResponse` calls `h.queries.GetRoomStats(ctx, room.ID)` for each room in the list (verified in `relay/internal/handler/room.go:103-116`). For `GET /rooms` returning 20 rooms, this executes 21 queries: 1 to list rooms + 20 individual stats queries. At small scale (Quorum's current usage) this is invisible. When rooms list is used on the Solvr homepage or rooms index page, this pattern hits the DB hard on every page load.

**Why it happens:**
The stats query (`TotalMessages`, `UniqueAgents`, `LastMessageAt`) is a natural addition per-room after the main list query. The N+1 pattern is the path of least resistance.

**How to avoid:**
When porting `ListPublicRooms` to Solvr, replace the per-room stats call with a single JOIN query that aggregates stats inline. Use a lateral join or window function to compute `total_messages`, `unique_agents`, and `last_message_at` in one query. Solvr's codebase already demonstrates this pattern with `LEFT JOIN aggregate subqueries` in `PostRepository.List()`.

**Warning signs:**
- `duration_ms` for `GET /rooms` is 200-500ms when 20 rooms are returned
- Database query count in logs showing 21 queries for a single list request
- Slow rooms page under moderate traffic

**Phase to address:**
Phase 2 (Service Merge) — fix the query during porting, not after.

---

### Pitfall 9: Human + Agent Commenting on Rooms Requires Schema Change, Not Just Logic Change

**What goes wrong:**
Quorum's `messages` table has `agent_name TEXT NOT NULL DEFAULT ''` — no user/author concept, no FK to users, no distinction between human and agent messages. If human commenting on rooms is added by simply inserting rows with `agent_name = user.display_name`, the data model conflates identity types. A user with display name "gpt-4" would be indistinguishable from an agent named "gpt-4". Auth checks (can a user delete their own comment?) require a `user_id` FK, which doesn't exist.

Solvr's comments table uses a `author_type` (human/agent) + `author_id` polymorphic pattern (verified in `db/comments.go`) which is exactly what rooms messages need. But rooms messages are currently a separate `messages` table with a different structure.

**Why it happens:**
Quorum was built for A2A-only agent communication. Humans were never expected to post messages. Retrofitting human identity requires a schema change.

**How to avoid:**
When creating the `messages` migration for Solvr, add author tracking from the start:
- Add `author_type VARCHAR(10) NOT NULL DEFAULT 'agent' CHECK (author_type IN ('human', 'agent'))`
- Add `author_id VARCHAR(255)` — UUID for humans, agent name/ID for agents
- Keep `agent_name TEXT` for backwards compatibility with A2A protocol (A2A sends agent_name, not agent_id)

This matches Solvr's established `author_type / author_id` pattern in comments and avoids a later schema migration to add it.

**Warning signs:**
- Human display names appearing in agent name field
- No FK or reference to `users.id` in messages table
- Unable to filter "messages by this human user" in room history

**Phase to address:**
Phase 1 (Database Migration) — schema must be correct before any human commenting is built.

---

### Pitfall 10: SEO Room Pages Return Thin Content if Rendered as Client-Side Shell

**What goes wrong:**
Next.js App Router defaults to static rendering for pages without dynamic data. A room page at `/rooms/[slug]` that fetches messages via client-side JavaScript will render an empty shell to Googlebot — only the room name and possibly a loading spinner. Googlebot does execute JavaScript but often does not wait for client-side data fetching. Answer Overflow (the Discord indexing platform) succeeds at SEO precisely because it SSRs message content into the initial HTML response.

Additionally, if room slugs are opaque (`/rooms/abc-123-xyz`) rather than descriptive (`/rooms/python-async-debugging`), the URL itself provides no keyword signal to Google.

**Why it happens:**
Client-side fetching is easier and avoids SSR complexity. Developers ship a working room page without realizing it's invisible to search engines.

**How to avoid:**
Use `generateStaticParams` or server-side `fetch` in the page component so room content is in the initial HTML. Include room `description`, `tags`, first N messages (as text), and agent names in the SSR payload. Add `og:title`, `og:description`, and a JSON-LD `DiscussionForumPosting` structured data block. Ensure room slugs are human-readable and keyword-relevant — the slug regex in Quorum (`^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$`) supports descriptive slugs.

**Warning signs:**
- `curl -A Googlebot https://solvr.dev/rooms/test-room` returns HTML with no message content
- Google Search Console shows room pages as "Crawled but not indexed" with "thin content" note
- Room page HTML source has only `<div id="__next"><div class="loading" /></div>`

**Phase to address:**
Phase 4 (Frontend Rooms) — SSR must be built into the room page from the start, not retrofitted.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Keep `go-chi/jwtauth` alongside `golang-jwt/jwt/v5` | Faster copy of Quorum auth code | Two JWT libraries, doubled dep surface, version drift risk | Never — unify on Solvr's pattern |
| Copy Quorum's sqlc `db.Queries` as-is | Less rewriting | Scans `provider`/`provider_id` columns that don't exist in Solvr's schema — runtime panics | Never |
| Soft-delete questions without removing code | Avoids deletion work in Phase 1 | Question endpoints remain in production, dead code accumulates, agents try to use them | Acceptable as first step only — code removal must follow in Phase 2 |
| Keep Quorum hub in-memory (no persistence) | Works immediately | Hub state lost on redeploy, rooms appear empty until agents rejoin | Acceptable for v1.3 — hub is for live presence, not history |
| Disable `WriteTimeout` globally | Simple fix | Removes slow-loris protection on non-SSE routes | Acceptable — Traefik at VPS edge already handles connection limits |
| N+1 stats query per room | Simpler initial port | Slow list endpoint under load | Not acceptable — fix during porting |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Traefik + SSE | Forgetting `X-Accel-Buffering: no` header | Port Quorum's `SSENoBuffering` middleware directly — apply to all `/rooms/{slug}/` routes |
| Traefik + SSE | Setting header only on `/events` route, forgetting A2A route | Apply to the entire `/rooms/{slug}/` route group, not individual endpoints |
| SSE + CORS | Missing SSE endpoint in CORS allowed methods | SSE uses `GET` — already in Solvr's CORS config; verify `AllowedOrigins` includes frontend URL |
| Solvr pgxpool wrapper + Quorum hub Queries | Quorum uses raw `pgxpool.Pool`, Solvr wraps it in `db.Pool` | Create a `db.NewQuorumRoomRepository(pool)` in Solvr style, don't pass raw pool |
| sqlc generated code in Solvr | Running `sqlc generate` with merged schemas | Solvr does not use sqlc — delete Quorum's generated `db.Queries` entirely; rewrite as raw pgx |
| golang-migrate + goose coexisting | Importing goose as dependency for Quorum's migrations | Use only golang-migrate for all migrations; do not import goose into Solvr |
| A2A route mounting | `relay.MountA2ARoutes` uses Quorum's module path | After merge, update import paths to Solvr's module name (`github.com/fcavalcantirj/solvr`) |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| `search_queries` table live-queried for analytics page | Dashboard loads 2-5s; concurrent searches slow down | Time-bound all queries; add `LIMIT`; consider materialized view for hourly aggregates | At ~50k rows — weeks away given 49% cron-loop searches |
| N+1 stats per room in `GET /rooms` list | 21 queries for 20-room list; 200-500ms response | Single JOIN query with aggregates inline | At 20+ rooms |
| SSE goroutine leak on client disconnect | Goroutine count rises; memory grows | Quorum's SSE handler uses `r.Context().Done()` correctly — preserve this on port | Goroutine leak visible within hours of real traffic |
| `agent_presence` rows accumulate without reaper | Stale agents show as present; stats inflated | Port `presence.StartReaper` — runs every 60s | Day 1 without reaper |
| `messages` table with no content length limit | Large A2A payloads stored in full; DB grows quickly | Add `CHECK (length(content) <= 65536)` in migration | Depends on agent verbosity |
| Live analytics page polling every 5-10s | DB CPU spikes every 5-10s during active sessions | Manual refresh or 60s polling; analytics data doesn't need to be real-time | At moderate traffic |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Sharing JWT_SECRET between merged Quorum routes and Solvr admin routes | A compromised Quorum room token grants admin access | Use single Solvr JWT_SECRET after merge; retire Quorum's separate secret |
| Exposing `token_hash` in room API response | Bearer token for A2A room access leaked in room listing | Never include `token_hash` in list/get responses — only return on room creation (Quorum already does this correctly) |
| SSE endpoint with no auth for private rooms | Anonymous users stream private room messages | Quorum's `StreamEvents` has no auth check by design for public rooms — add check for `is_private = true` rooms |
| `messages.agent_name` is free-form text with no validation | Any string accepted as agent name — no verification against registered agents | When merging, validate `agent_name` against `agent_presence` table for non-human senders |
| Quorum OAuth callback URL registered to old domain | OAuth callbacks fail silently after merge | Update Google/GitHub OAuth app settings to point to `api.solvr.dev` not Quorum's old domain |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Removing `question` type with no redirect | 9 existing question URLs return 404 — some may be indexed by Google | Soft-delete first; `GET /v1/questions/{id}` returns 301 to `/v1/posts/{id}` or 410 Gone |
| Room SEO pages with only agent names as visible text | Google classifies as thin content; no indexing | Include room description, first 5 messages as HTML, agent names, and tags in SSR |
| Live search analytics page with auto-refresh | Admin page creates constant DB load | Manual refresh button; or 60s polling interval |
| Room slugs not shown with description on index page | Rooms list looks like a wall of opaque names | Show `description` (truncated) and `tags` alongside `display_name` in room list UI |

## "Looks Done But Isn't" Checklist

- [ ] **SSE integration:** `WriteTimeout` removed from `backend/cmd/api/main.go` — verify with `grep WriteTimeout backend/cmd/api/main.go`
- [ ] **SSE integration:** SSE client stays connected for > 60 seconds — test manually with `curl -N https://api.solvr.dev/v1/rooms/{slug}/events`
- [ ] **Question removal:** All 9 production question rows soft-deleted before constraint change — `SELECT COUNT(*) FROM posts WHERE type = 'question' AND deleted_at IS NULL` must return 0 before migration runs
- [ ] **Question removal:** `ResponseTypeQuestion` in `models/response.go` preserved — ideas can still receive 'question' response type
- [ ] **Migration format:** No goose annotations in new migrations — `grep -r "goose" backend/migrations/` returns nothing
- [ ] **Auth merge:** No `go-chi/jwtauth` in `backend/go.mod` — Quorum handlers use Solvr's `auth.ValidateJWT`
- [ ] **Hub port:** `presence.StartReaper` started in Solvr's `main.go` — verify agents get evicted after TTL with an integration test
- [ ] **Data migration:** No orphaned room owner UUIDs — `SELECT COUNT(*) FROM rooms WHERE owner_id IS NOT NULL AND owner_id NOT IN (SELECT id FROM users)` returns 0
- [ ] **SEO:** Room pages return message content in SSR HTML — `curl -A Googlebot https://solvr.dev/rooms/{slug}` contains message text
- [ ] **Traefik buffering:** `X-Accel-Buffering: no` header present on SSE and A2A routes — verify with `curl -I https://api.solvr.dev/v1/rooms/{slug}/events | grep -i x-accel`

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Wrong migration format ran on production | HIGH | Run `migrate down N`; if goose polluted the schema — manual `DROP TABLE` and re-run clean migration |
| JWT secret mismatch (Quorum sessions rejected after merge) | LOW | All Quorum sessions expire within 30 days; users re-auth transparently to Solvr |
| Question constraint dropped before data migration | MEDIUM | `ALTER TABLE posts ADD CONSTRAINT ... NOT VALID` bypasses row validation; manually soft-delete remaining rows; `VALIDATE CONSTRAINT` |
| SSE WriteTimeout not removed in production deploy | LOW | Hotfix deploy with `WriteTimeout: 0`; zero data loss |
| Room owner_id FK violations during data migration | MEDIUM | Set `owner_id = NULL` for orphaned rooms; accept loss of ownership for users who only existed in Quorum |
| Hub goroutines leaked (missing context cancellation) | MEDIUM | Rolling restart recovers immediately; fix context propagation in SSE handler; deploy; monitor with pprof goroutine count |
| Thin content SEO for room pages | HIGH | Cannot recover crawled pages quickly; Google re-crawl takes weeks; rebuild as SSR from the start |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Duplicate table names (users, refresh_tokens) | Phase 1: DB Migration | New migration only creates rooms, agent_presence, messages — `\dt` in psql confirms no duplicates |
| JWT claims mismatch | Phase 2: Service Merge | Integration test: Solvr-issued JWT accepted by merged room endpoint with 200, not 401 |
| WriteTimeout kills SSE | Phase 3: SSE Integration | Manual test: SSE client stays connected 60+ seconds without disconnect |
| Question type removal breaks constraint | Phase 1: DB Migration | Soft-delete migration runs first; then constraint-change migration runs; `\d posts` shows new constraint |
| Migration tool format mismatch | Phase 1: DB Migration | `grep -r "goose" backend/migrations/` returns nothing |
| rooms.owner_id FK to wrong users population | Phase 1: DB Migration | Zero orphaned owner_id values after migration script |
| A2A streaming broken by WriteTimeout | Phase 3: SSE Integration | A2A streaming test: agent sends and receives `message/stream` without 15s timeout |
| Hub reaper not started | Phase 2: Service Merge | Integration test: agent_presence row evicted after TTL from both DB and in-memory registry |
| N+1 room list query | Phase 2: Service Merge | `GET /v1/rooms` executes one DB query, not N+1 — verify with query count logging |
| Human commenting needs schema change | Phase 1: DB Migration | messages table has `author_type` and `author_id` columns from the start |
| Room SEO pages return empty shell | Phase 4: Frontend Rooms | `curl -A Googlebot` on room URL returns messages in HTML body |
| search_queries analytics performance | Phase 5: Analytics Page | EXPLAIN ANALYZE on dashboard query; page loads < 500ms with real data |

## Sources

- Direct code inspection: `/Users/fcavalcanti/dev/solvr/backend/cmd/api/main.go` — `WriteTimeout: 15 * time.Second` confirmed
- Direct code inspection: `/Users/fcavalcanti/dev/quorum/relay/cmd/server/main.go` — WriteTimeout disabled, goose migrations, jwtauth, reaper startup
- Direct code inspection: `/Users/fcavalcanti/dev/solvr/backend/internal/auth/jwt.go` — custom `user_id` claim confirmed (not `sub`)
- Direct code inspection: `/Users/fcavalcanti/dev/quorum/relay/internal/middleware/jwtauth.go` — `sub` claim via `go-chi/jwtauth/v5` confirmed
- Direct code inspection: `/Users/fcavalcanti/dev/solvr/backend/migrations/000003_create_posts.up.sql` — `posts_type_check` constraint
- Direct code inspection: `/Users/fcavalcanti/dev/solvr/backend/migrations/000001_create_users.up.sql` — Solvr users schema (14+ columns, `auth_provider`)
- Direct code inspection: `/Users/fcavalcanti/dev/quorum/relay/schema.sql` — Quorum users schema (6 columns, `provider`, `provider_id`)
- Direct code inspection: `/Users/fcavalcanti/dev/quorum/relay/internal/migrations/00001_init.sql` — goose format with `-- +goose Up` confirmed
- Direct code inspection: `/Users/fcavalcanti/dev/solvr/backend/go.mod` — no goose, no jwtauth; golang-migrate used as external CLI
- Direct code inspection: `/Users/fcavalcanti/dev/quorum/relay/go.mod` — `pressly/goose/v3`, `go-chi/jwtauth/v5` confirmed
- Direct code inspection: `/Users/fcavalcanti/dev/quorum/relay/internal/middleware/ssebuffering.go` — Traefik buffering pitfall documented in comments
- Direct code inspection: `/Users/fcavalcanti/dev/quorum/relay/internal/handler/room.go:103-116` — N+1 `GetRoomStats` per room confirmed
- Direct code inspection: `/Users/fcavalcanti/dev/solvr/backend/internal/db/comments.go` — `author_type`/`author_id` polymorphic pattern
- Grep: `question` references — 62 in `api/handlers/`, 138 in `internal/db/` across 12 files
- Grep: `ResponseTypeQuestion` in `models/response.go` — confirmed as separate idea-response concept

---
*Pitfalls research for: Solvr v1.3 Quorum Merge — Go service merge, rooms + SSE addition, question type removal, live search analytics*
*Researched: 2026-04-02*
