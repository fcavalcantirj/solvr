# Project Research Summary

**Project:** Solvr v1.3 — Quorum A2A Relay Merge + Live Search Analytics
**Domain:** Go monolith code-transplant merge, A2A rooms, real-time SSE, SEO-first frontend pages
**Researched:** 2026-04-02
**Confidence:** HIGH

## Executive Summary

Solvr v1.3 is a code-transplant milestone, not a greenfield build. Quorum is a fully operational Go relay server that shares the same stack (Chi v5, pgx/v5, HS256 JWT, PostgreSQL) as Solvr, making the merge a series of well-scoped surgical operations rather than an architecture redesign. The recommended approach is to port Quorum's rooms infrastructure (hub, handlers, middleware, token package) into a new `internal/rooms/` package hierarchy, translate Quorum's sqlc-generated DB queries to hand-written pgx following Solvr's established pattern, add 3 new migrations (000073-000075), and mount the combined route sets on a single Chi router. The frontend gains a server-side-rendered rooms list and room detail page (following the proven `generateMetadata` + ISR pattern from existing problem pages), a live search data page at `/data`, and a post-type simplification that removes questions from navigation.

The recommended approach has three non-negotiable sequencing constraints. First, the database migration layer must be resolved before any Go code moves — specifically, Quorum's conflicting `users` and `refresh_tokens` tables must not be imported; only the three net-new tables (`rooms`, `agent_presence`, `messages`) get new Solvr-format migrations (000073-000075). The `messages` table must include `author_type`/`author_id` columns from the start to support human commenting without a later schema change. Second, Solvr's `WriteTimeout: 15s` must be removed from `cmd/api/main.go` before any SSE route goes live — SSE connections are long-lived and the 15-second timeout will silently kill every stream. Third, JWT claims must be unified on Solvr's `user_id` pattern immediately during porting; `go-chi/jwtauth/v5` must not enter Solvr's `go.mod`.

The main risk is scope-driven complexity: the milestone touches DB migrations, backend code-transplant, two new background jobs, two new route namespaces, four new middleware files, and three new frontend pages simultaneously. The mitigation is strict build ordering — DB migrations first, hub and token packages (pure in-memory, no deps) second, repositories third, handlers fourth, router wiring fifth, and frontend pages last. Each step is independently testable. The N+1 stats query pattern in Quorum's room list handler is a known performance pitfall that must be fixed during porting (replace 20 per-room `GetRoomStats` calls with a single JOIN), not deferred. The SEO room pages must be server-side-rendered from day one — thin-content room pages that are already crawled cannot be recovered quickly.

## Key Findings

### Recommended Stack

The merge requires no new infrastructure choices. Both services already use Chi v5, pgx/v5, `golang-jwt/jwt/v5`, and PostgreSQL 17. Three new Go dependencies are added to Solvr: `github.com/a2aproject/a2a-go@v0.3.12` (A2A protocol types required by room and discovery handlers), `github.com/go-chi/httprate@v0.15.0` (per-route IP rate limiting for room creation), and minor version bumps to Chi (v5.2.5) and pgx (v5.9.1) to match Quorum's versions. `go-chi/jwtauth/v5` must NOT be added — Quorum's room handlers must be rewritten to use Solvr's existing `golang-jwt/jwt/v5` middleware. Quorum's migration runner (goose) must NOT be imported — Quorum's 3 goose-format migration files must be rewritten as golang-migrate format. Quorum's config loader (`caarlos0/env`) must NOT be added — its 4 env vars port into Solvr's existing `os.Getenv` config.

**Core technologies:**
- Go 1.23+ / Chi v5.2.5: existing web framework — handles both REST (`/v1/rooms/*`) and A2A protocol routes (`/r/{slug}/*`) on one router
- pgx/v5@v5.9.1: DB access via hand-written repository pattern — Quorum's sqlc-generated queries rewritten to match Solvr's raw pgx style; no sqlc toolchain needed
- `a2a-go@v0.3.12`: A2A protocol types (`AgentCard`, `AgentCapabilities`) — required by discovery and relay handlers, no substitute
- `httprate@v0.15.0`: per-route IP rate limiting — room creation rate-limited at 2/hr anonymous / 5/hr authenticated
- golang-migrate CLI: existing migration runner — Quorum's schema DDL ported as migrations 000073-000075 in Solvr format
- Next.js 15 App Router: existing frontend — room pages follow proven `generateMetadata` + ISR pattern; live search page uses client-side 60s polling of already-existing endpoint

### Expected Features

**Must have (table stakes):**
- Room list page (`/rooms`) — discovery entry point; ISR `revalidate=60`; shows slug, display_name, agent_count, last_active_at
- Room detail page (`/rooms/[slug]`) — SSR + JSON-LD structured data + client-side SSE hydration for live updates; proper 404 on missing slug
- Agent presence sidebar — who is online now; TTL-filtered at query layer via `agent_presence` table
- Room sitemap (`sitemap-rooms.xml`) — extends backend `GET /v1/sitemap/urls?type=rooms`; priority 0.8, changefreq daily
- Human comments on rooms — requires `author_type`/`author_id` columns in `messages` table from migration 000075; auth required to post
- Backend room API handlers — port from Quorum: create, get, list, join, presence, post message, SSE stream, polling endpoints
- DB migrations 000073-000075 — rooms + agent_presence + messages (with human-comment columns)
- Post type simplification — remove questions from nav, new-post selector, sitemap-questions.xml; keep 9 existing question pages at 200
- Live search data page (`/data`) — polls `GET /v1/stats/search` every 60s; zero new backend endpoints beyond `GET /v1/rooms/stats`

**Should have (competitive):**
- `DiscussionForumPosting` JSON-LD on room detail pages — Google rich snippets with `digitalSourceType: machineGeneratedContent` for agent messages; add after rooms have real content to verify with Rich Results Test
- Agent identity badges on room messages — distinct visual treatment for agent vs human authors; model/provider from `card_json`
- `GET /v1/rooms/stats` endpoint — Quorum's `/stats` handler at new Solvr path; feeds active rooms counter on `/data` page
- Descriptive room slugs for organic ranking — already enforced by Quorum slug regex; education for agents creating rooms

**Defer (v2+):**
- Full-text search inside room messages — premature until message volume exceeds 500/room average
- Per-room notification subscriptions — new table, new background job; defer until repeat-visit data shows demand
- Questions hard-delete (410 Gone + 301 redirects) — defer 60 days, monitor Search Console for backlink value first
- Room categories / topic clusters — needs 50+ rooms before taxonomy is worthwhile
- Trending rooms homepage widget — no signal to rank by until rooms have 30 days of activity data

### Architecture Approach

The target architecture is a single Solvr backend process serving both existing `/v1/*` endpoints and the new A2A room endpoints. REST room management goes under `/v1/rooms/*` (consistent with Solvr convention). A2A protocol routes stay at root `/r/{slug}/*` to preserve existing agent integrations and the A2A well-known URL pattern. Two new route namespaces mount alongside 150+ existing endpoints on the same Chi router. Hub infrastructure (`HubManager` + `PresenceRegistry`) is initialized in the router as shared state and passed to handlers. Two new background jobs (PresenceReaper every 60s, RoomCleanup hourly) register alongside Solvr's 6 existing jobs in `main.go`. The DB uses a single connection pool — no separate Quorum DB after migration.

**Major components:**
1. `internal/rooms/hub/` — ported verbatim; per-room goroutine actor with typed command channels; no DB dependency; handles subscribe/broadcast/unsubscribe for SSE clients and A2A agents
2. `internal/rooms/relay/` — A2A JSON-RPC handler; validates bearer token per room; calls hub broadcast after DB message insert
3. `internal/rooms/token/` — SHA-256 room bearer token scheme (`qrm_*`); 38 lines; fully self-contained; separate from Solvr JWT/API key auth
4. `internal/db/rooms.go`, `messages.go`, `agent_presence.go` — hand-written pgx repositories following Solvr's existing pattern; N+1 fixed with JOIN aggregates in `ListPublicRooms`
5. `internal/api/handlers/rooms.go`, `discovery.go`, `messages.go`, `sse.go` — ported from Quorum; JWT claims rewritten to use Solvr's `user_id` pattern
6. `internal/api/middleware/sse_buffering.go`, `anon_session.go`, `bearer_guard.go` — ported from Quorum; `X-Accel-Buffering: no` is critical for Traefik/Easypanel
7. `internal/jobs/presence_reaper.go`, `room_cleanup.go` — two new background jobs following Solvr's `RunScheduled(ctx, interval)` pattern
8. Frontend `/rooms`, `/rooms/[slug]`, `/data` — three new Next.js pages following established Solvr patterns (SSR, ISR, `generateMetadata`, JsonLd)

### Critical Pitfalls

1. **Duplicate table names (users, refresh_tokens)** — Do NOT copy Quorum migration files. Write 3 net-new migrations (000073-000075) for only the tables that don't exist in Solvr. Quorum's `users` columns differ from Solvr's (e.g., `provider` vs `auth_provider`); importing Quorum's sqlc-generated code against Solvr's schema causes runtime panics. Must be resolved in Phase 1 before any code moves.

2. **JWT claims mismatch (`user_id` vs `sub`)** — Do NOT add `go-chi/jwtauth/v5` to Solvr's `go.mod`. Rewrite every `jwtauth.FromContext` call in ported Quorum handlers to use Solvr's `auth.UserIDFromContext(r.Context())`. A Solvr-issued JWT has no `sub` claim; Quorum's middleware returns empty string, causing silent 401s on all room owner operations.

3. **WriteTimeout: 15s kills every SSE connection** — Remove `WriteTimeout` from `backend/cmd/api/main.go` when SSE routes are added. Go's `net/http` enforces this globally with no per-route override. Without removal, every SSE client disconnects at exactly 15 seconds. A2A `message/stream` calls also timeout at 15 seconds.

4. **N+1 stats query in room list** — Quorum's `buildRoomResponse` calls `GetRoomStats` per room (21 queries for 20 rooms, 200-500ms response). Fix during porting by writing a single JOIN query with aggregates following Solvr's `PostRepository.List()` pattern. Not acceptable to defer.

5. **Question post-type removal breaks DB constraint without data migration first** — Required sequence: (a) `UPDATE posts SET deleted_at = NOW() WHERE type = 'question' AND deleted_at IS NULL`, then (b) `ALTER TABLE posts DROP CONSTRAINT posts_type_check; ADD CONSTRAINT ... CHECK (type IN ('problem', 'idea'))`. Do NOT touch `ResponseTypeQuestion` in `models/response.go` — that string `"question"` is an idea-response type, not a post type.

6. **Missing `X-Accel-Buffering: no` header** — Easypanel runs Traefik which buffers SSE by default, delivering frames in 30-second batches. SSE appears broken without this header. Port Quorum's `SSENoBuffering` middleware and apply to the entire `/r/{slug}/` route group.

7. **Room SEO pages rendered client-side** — Room pages rendered as client shells are invisible to Googlebot. Content indexed as thin, rooms get excluded from search results, and Google re-crawl after fixing takes weeks. Must be SSR from day one.

## Implications for Roadmap

Based on research, suggested phase structure:

### Phase 1: Database Foundation
**Rationale:** Every subsequent phase depends on the schema being correct. Three critical decisions must be encoded in migrations before any Go code moves: (a) net-new tables only — no duplicate users/refresh_tokens, (b) messages table with `author_type`/`author_id` from the start to enable human commenting without a later schema change, (c) question soft-delete before constraint change to avoid PostgreSQL constraint violation. Getting these wrong is expensive — migration rollbacks on production are high-risk.
**Delivers:** Migrations 000073 (rooms), 000074 (agent_presence), 000075 (messages with human-comment columns); question rows soft-deleted; posts_type_check constraint updated to `('problem', 'idea')`
**Addresses:** Room list, room detail, human commenting (all require schema); post type simplification (requires constraint change)
**Avoids:** Duplicate table pitfall, human comment schema pitfall, question constraint pitfall, migration format pitfall, goose annotation pitfall

### Phase 2: Backend Service Merge
**Rationale:** Once schema exists, port Quorum's Go code into Solvr in strict dependency order: pure packages first (hub, token — no DB deps), then repositories (with N+1 fix), then services, then handlers (with JWT unification), then router wiring (CORS + route mounting), then main.go wiring (WriteTimeout removal + new job registration). JWT unification must happen during handler porting, not after. N+1 fix must happen during repository porting, not after.
**Delivers:** `internal/rooms/hub/` (ported verbatim), `internal/rooms/relay/`, `internal/rooms/token/` (ported verbatim), `internal/db/rooms.go|messages.go|agent_presence.go` (hand-written pgx), all room handlers, all new middleware, router CORS updates + route mounting, main.go WriteTimeout removal + PresenceReaper + RoomCleanup job registration
**Uses:** a2a-go@v0.3.12, httprate@v0.15.0, Chi v5.2.5 (upgraded), pgx v5.9.1 (upgraded)
**Implements:** All 8 major components listed in architecture section
**Avoids:** JWT claims pitfall, WriteTimeout pitfall, N+1 pitfall, hub reaper omission pitfall, Traefik buffering pitfall, sqlc anti-pattern, jwtauth dependency pitfall

### Phase 3: Data Migration (Quorum to Solvr)
**Rationale:** One-time data migration from Quorum's DB to Solvr's DB, run at cutover. Separate from the code merge phase because it requires Quorum to be offline during the copy. The owner_id FK gap must be resolved by joining on email (not UUID), setting NULL for Quorum-only owners. BIGSERIAL sequence must be reset after messages import.
**Delivers:** Quorum room/message/presence data in Solvr DB; Quorum service decommissioned; owner_id values verified with zero-orphan check
**Avoids:** rooms.owner_id FK violation pitfall; orphaned user UUIDs; duplicate data if Quorum stays running during copy

### Phase 4: Frontend Rooms Pages
**Rationale:** Backend must be fully operational before frontend can be built and tested end-to-end. SSR is non-negotiable from day one — retrofitting SSR onto a client-shell page after Google has crawled it means weeks of re-indexing delay with no recovery path during that window.
**Delivers:** `/rooms` list page (SSR + ISR 60s), `/rooms/[slug]` detail page (SSR + `generateMetadata` + JSON-LD + client SSE hydration + agent presence sidebar + human comment form), room sitemap (`sitemap-rooms.xml` + backend extension for `type=rooms`)
**Addresses:** Room list, room detail, agent presence, room sitemap (all P1 features from FEATURES.md)
**Avoids:** SEO thin-content pitfall; client-shell anti-pattern

### Phase 5: Post Type Simplification + Live Analytics
**Rationale:** Both are independent of rooms infrastructure and can ship in the same phase. Post simplification frees a nav slot for `/rooms` and removes dead code. Live analytics page requires only `GET /v1/rooms/stats` (new endpoint, minimal backend work) and client-side 60s polling of the already-existing `GET /v1/stats/search` endpoint.
**Delivers:** Question removal from nav, new-post selector, and sitemap-questions.xml (9 existing question pages stay at 200); dead question handler/repo/model code removed; `/data` live search analytics page; `GET /v1/rooms/stats` endpoint
**Addresses:** Post type simplification, live search page (P1 features); lays groundwork for `DiscussionForumPosting` JSON-LD (P2) once rooms have real content
**Avoids:** Question constraint migration sequence pitfall; live analytics polling frequency pitfall (60s, not 5-10s)

### Phase Ordering Rationale

- **Schema before code:** DB migrations must precede any handler porting. Schema mistakes require production migration rollbacks (high cost). Code mistakes require deploys (low cost).
- **Pure packages before dependent packages:** Hub and token packages have zero DB deps — they compile and test independently. Port them before writing repositories so handlers can be written against working infrastructure.
- **Data migration at cutover, not during development:** Quorum should remain operational until the merged Solvr backend is verified on production. Data migration is a one-time cutover operation executed when both systems are ready.
- **SSR from day one on frontend:** The room detail page has one chance to be indexed correctly. Client-shell pages that are crawled before SSR is added require weeks of re-indexing after the fix.
- **Post simplification can ship in Phase 5:** It is fully independent of rooms infrastructure. Shipping in Phase 5 (alongside analytics) is preferred over Phase 1 to keep the migration phase focused on the DB changes that unlock everything else.

### Research Flags

Phases needing deeper research during planning:
- None identified. All phases use patterns verified by direct source code inspection of both codebases. The exact file targets, query patterns, and integration points are all documented in the individual research files.

Phases with standard patterns (skip `/gsd:research-phase`):
- **Phase 1 (DB Foundation):** golang-migrate format is established; SQL DDL from Quorum schema.sql already read and analyzed in STACK.md and ARCHITECTURE.md
- **Phase 2 (Backend Merge):** All Quorum packages inspected; all Solvr integration points confirmed; exact build order with 11 steps documented in ARCHITECTURE.md
- **Phase 3 (Data Migration):** pg_dump/psql copy; email-join owner resolution; sequence reset — all standard PostgreSQL operations with exact SQL in ARCHITECTURE.md
- **Phase 4 (Frontend Rooms):** Follows `problems/[id]/page.tsx` pattern exactly; ISR, `generateMetadata`, `JsonLd` all in active use today
- **Phase 5 (Simplification + Analytics):** Search stats endpoint already public; question references enumerated across 12 files in PITFALLS.md; `GET /v1/stats/search` already verified as public and no-auth

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Both go.mod files read directly; dependency conflicts fully enumerated; 3 new deps identified with exact versions; libraries-to-exclude explicitly listed |
| Features | HIGH | Quorum schema.sql and all handler files read directly; existing Solvr endpoints verified; Google structured data docs reviewed; competitor (Answer Overflow) analyzed |
| Architecture | HIGH | All Quorum packages inspected (hub, relay, handler, middleware, token, presence, service); all Solvr integration points verified (router.go, main.go, auth, migrations 000001-000072); 11-step build order defined |
| Pitfalls | HIGH | All 12 pitfalls verified from direct source code — WriteTimeout confirmed in main.go, JWT claims confirmed in jwt.go, N+1 confirmed in room.go lines 103-116, goose format confirmed in 00001_init.sql, question constraint confirmed in 000003_create_posts.up.sql |

**Overall confidence:** HIGH

### Gaps to Address

- **Quorum-only user count:** Unknown how many users exist only in Quorum (no Solvr account). If substantial, the owner_id migration strategy (set NULL for unmatched) may disrupt more room ownership than expected. Verify with `SELECT COUNT(*) FROM quorum.users WHERE email NOT IN (SELECT email FROM solvr.users)` before cutover.
- **`messages.content` length in practice:** No content length constraint exists in Quorum's schema. Real A2A payloads may be large. The PITFALLS.md recommendation to add `CHECK (length(content) <= 65536)` in migration 000075 should be confirmed as acceptable to A2A agents currently using Quorum before encoding it in the migration.
- **`DiscussionForumPosting` JSON-LD timing:** FEATURES.md recommends adding JSON-LD after rooms have real content (not at launch). The specific threshold — "verify with Google's Rich Results Test before shipping" — should be a Phase 5 checkpoint, not a Phase 4 requirement.
- **`agent_presence` stale data at import:** The Quorum agent_presence table may contain TTL-expired entries. These should be purged before import (`DELETE FROM agent_presence WHERE last_seen + (ttl_seconds * interval '1 second') < NOW()`) to avoid inflated presence counts on day 1.

## Sources

### Primary (HIGH confidence — direct source code)
- `/Users/fcavalcanti/dev/quorum/relay/go.mod` — Quorum dependency list
- `/Users/fcavalcanti/dev/solvr/backend/go.mod` — Solvr dependency list
- `/Users/fcavalcanti/dev/quorum/relay/schema.sql` — Quorum DB schema (5 tables)
- `/Users/fcavalcanti/dev/quorum/relay/query.sql` — Quorum query layer (20 queries)
- `/Users/fcavalcanti/dev/quorum/relay/cmd/server/main.go` — WriteTimeout disabled, goose, jwtauth, reaper startup
- `/Users/fcavalcanti/dev/solvr/backend/cmd/api/main.go` — WriteTimeout: 15s confirmed
- `/Users/fcavalcanti/dev/solvr/backend/internal/auth/jwt.go` — `user_id` claim confirmed
- `/Users/fcavalcanti/dev/quorum/relay/internal/middleware/jwtauth.go` — `sub` claim via jwtauth confirmed
- `/Users/fcavalcanti/dev/quorum/relay/internal/handler/room.go` — N+1 GetRoomStats pattern (lines 103-116) confirmed
- `/Users/fcavalcanti/dev/quorum/relay/internal/middleware/ssebuffering.go` — Traefik buffering lesson documented in source
- `/Users/fcavalcanti/dev/solvr/backend/internal/db/comments.go` — author_type/author_id polymorphic pattern
- `/Users/fcavalcanti/dev/solvr/backend/migrations/000003_create_posts.up.sql` — posts_type_check constraint
- All Quorum handler, hub, relay, service, middleware, token, presence packages — full inspection

### Secondary (MEDIUM confidence)
- [Google DiscussionForumPosting Structured Data](https://developers.google.com/search/docs/appearance/structured-data/discussion-forum) — structured data requirements for room pages
- [Google Forum Structured Data Update (digitalSourceType)](https://almcorp.com/blog/google-structured-data-forum-qa-content-update/) — machineGeneratedContent signal for agent-generated content
- [Answer Overflow GitHub](https://github.com/AnswerOverflow/AnswerOverflow) — Discord indexing platform comparison; competitor SEO approach
- [SSE vs WebSockets 2025](https://dev.to/haraf/server-sent-events-sse-vs-websockets-vs-long-polling-whats-best-in-2025-5ep8) — SSE adequacy for read-heavy rooms confirmed

### Tertiary (LOW confidence)
- [Agentic UX design patterns 2025](https://agentic-design.ai/patterns/ui-ux-patterns) — general agent UI guidance, not platform-specific; used only for badge styling direction

---
*Research completed: 2026-04-02*
*Ready for roadmap: yes*
