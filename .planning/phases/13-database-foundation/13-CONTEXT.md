# Phase 13: Database Foundation - Context

**Gathered:** 2026-04-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Create PostgreSQL migrations 000073-000075 that add rooms, agent_presence, and messages tables to Solvr's database. These tables merge Quorum's A2A room infrastructure into Solvr with schema adaptations for human+agent unified messaging. No repository code or API endpoints — just migrations and tests.

</domain>

<decisions>
## Implementation Decisions

### Room Schema (Migration 000073)
- **D-01:** No anonymous room creation — drop `anonymous_session_id` column. Only authenticated users/agents can create rooms.
- **D-02:** Keep `last_active_at` TIMESTAMPTZ NOT NULL DEFAULT NOW() — useful for sorting, stale room cleanup, room list ordering.
- **D-03:** Keep Quorum slug regex: `CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$')` — 3-40 chars, lowercase alphanumeric + hyphens.
- **D-04:** `owner_id` nullable with `ON DELETE SET NULL` — rooms survive owner deletion, needed for Phase 15 data migration.
- **D-05:** `token_hash` NOT NULL — every room gets a bearer token at creation, even public rooms.
- **D-06:** `expires_at` nullable (NULL = permanent). No auto-cleanup in this phase — deferred to Phase 14+.
- **D-07:** `is_private` BOOLEAN NOT NULL DEFAULT FALSE — column exists but no private room logic built yet.
- **D-08:** Tags: `TEXT[] NOT NULL DEFAULT '{}'` with `CHECK (array_length(tags, 1) <= 10)` — max 10 tags per room.
- **D-09:** Add `deleted_at` TIMESTAMPTZ for soft-delete — matches Solvr's existing posts pattern.
- **D-10:** `description` as `VARCHAR(1000)` — bounded but generous.
- **D-11:** `display_name` as `VARCHAR(200)`.
- **D-12:** Add `message_count` INT NOT NULL DEFAULT 0 — denormalized count to avoid COUNT(*) on list queries.
- **D-13:** Add `category` VARCHAR(50) — for room topic grouping/discovery.
- **D-14:** Add `updated_at` TIMESTAMPTZ NOT NULL DEFAULT NOW() — standard Solvr pattern for metadata changes.

### Messages Schema (Migration 000075) — UNIFIED, No Separate room_comments
- **D-15:** Drop `room_comments` table entirely. One unified `messages` table handles both agent A2A messages and human messages.
- **D-16:** COMMENT-02 requirement satisfied by `author_type`/`author_id` columns on messages, not a separate table.
- **D-17:** `id` BIGSERIAL PRIMARY KEY — auto-incrementing for natural ordering and efficient pagination.
- **D-18:** `author_type` VARCHAR(10) NOT NULL DEFAULT 'agent' CHECK (author_type IN ('human', 'agent', 'system')) — includes 'system' for automated events (room created, agent joined/left).
- **D-19:** `author_id` VARCHAR(255) — stores user UUID for humans, agent ID for agents. No FK constraint, text field matching Solvr's existing comments pattern.
- **D-20:** `agent_name` VARCHAR(100) NOT NULL — populated for ALL message types. For humans: display name or email. For agents: agent name. For system: 'system'.
- **D-21:** `content` TEXT NOT NULL with `CHECK (length(content) <= 65536)` — 64KB max.
- **D-22:** `content_type` VARCHAR(20) NOT NULL DEFAULT 'text' CHECK (content_type IN ('text', 'markdown', 'json')).
- **D-23:** `deleted_at` TIMESTAMPTZ for soft-delete — allows message moderation.
- **D-24:** No `edited_at` column — messages are immutable for now.
- **D-25:** `metadata` JSONB NOT NULL DEFAULT '{}' — flexible field for A2A protocol extensions, tool results, structured data.
- **D-26:** `sequence_num` INT — per-room sequence number, application-managed (Phase 14 handles increment logic). Nullable until Phase 14 populates it.

### Agent Presence Schema (Migration 000074)
- **D-27:** `id` UUID PRIMARY KEY DEFAULT gen_random_uuid() — matches Solvr's entity ID convention.
- **D-28:** UNIQUE (room_id, agent_name) constraint — one presence record per agent per room.
- **D-29:** `ttl_seconds` INT NOT NULL DEFAULT 900 — 15 minutes (user override from Quorum's 300 and original plan's 600).
- **D-30:** `card_json` JSONB NOT NULL with `CHECK (length(card_json::text) <= 16384)` — 16KB limit for agent cards.

### Index Strategy
- **D-31:** Port all Quorum indexes: `idx_rooms_owner_id`, `idx_rooms_expires_at` (partial WHERE expires_at IS NOT NULL), `idx_agent_presence_room_id`, `idx_agent_presence_last_seen`.
- **D-32:** Add composite index: `idx_messages_room_created ON messages(room_id, created_at)` for room timeline queries.
- **D-33:** Use partial indexes WHERE deleted_at IS NULL on both rooms and messages — matches Solvr's migration 000030 pattern.
- **D-34:** Room slug UNIQUE constraint already provides implicit index.

### Down Migrations
- **D-35:** `DROP TABLE IF EXISTS ... CASCADE` for all down migrations. Simple, clean rollback for net-new tables.
- **D-36:** Rely on CASCADE for index/constraint cleanup — no explicit DROP INDEX.

### Migration Testing
- **D-37:** Integration tests against real DB using `os.Getenv("DATABASE_URL")` pattern.
- **D-38:** Migration tests only (no repository code). Test: tables exist, columns are correct, constraints work, up+down idempotent.
- **D-39:** Repository scaffolding deferred to Phase 14.

### Quorum Data Compatibility (Phase 15 context)
- **D-40:** All Quorum users are Solvr users — derive agent_name from username/email if needed.
- **D-41:** Default values handle migration gap: author_type DEFAULT 'agent', content_type DEFAULT 'text', deleted_at NULL.
- **D-42:** Anonymous-created Quorum rooms get NULL owner_id — acceptable.

### Claude's Discretion
- Exact migration file naming and SQL formatting
- Order of CREATE INDEX statements within each migration
- Test file organization and naming
- Whether to use IF NOT EXISTS guards on CREATE TABLE (recommended: no, let it fail loudly on conflicts)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Quorum Source Schema
- `/Users/fcavalcanti/dev/quorum/relay/schema.sql` — Original table definitions being adapted (rooms, agent_presence, messages)

### Solvr Migration Patterns
- `backend/migrations/000001_create_users.up.sql` — Solvr users table schema (14+ columns, auth_provider pattern)
- `backend/migrations/000072_add_email_unsubscribed_at.up.sql` — Latest migration, shows current golang-migrate format
- `backend/migrations/000030_add_partial_indexes.up.sql` — Partial index pattern (WHERE deleted_at IS NULL) referenced in D-33

### Solvr Code Patterns
- `backend/internal/db/comments.go` — Existing author_type/author_id polymorphic pattern to match

### Research
- `.planning/research/PITFALLS.md` — 10 critical pitfalls with prevention strategies (MUST READ)
- `.planning/research/ARCHITECTURE.md` — Architecture decisions for the merge
- `.planning/STATE.md` — Session context, decisions log, Quorum codebase reference map

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `comments` table `author_type`/`author_id` pattern — proven polymorphic author model to replicate in messages
- Partial index pattern from migration 000030 — reuse WHERE deleted_at IS NULL

### Established Patterns
- golang-migrate format: separate `.up.sql` / `.down.sql` files, pure SQL, no annotations
- UUID primary keys for entities, BIGSERIAL for ordered sequences
- TIMESTAMPTZ for all timestamps with DEFAULT NOW()
- Soft-delete via `deleted_at TIMESTAMPTZ` column
- Integration tests using `os.Getenv("DATABASE_URL")` to skip when no DB available

### Integration Points
- Last migration: 000072. Next must be 000073, 000074, 000075
- No schema_migrations table on production — migrations applied manually via admin query route
- Solvr users table has `auth_provider`/`auth_provider_id` (NOT Quorum's `provider`/`provider_id`)

</code_context>

<specifics>
## Specific Ideas

- 3 migration files mapping: 000073 = rooms, 000074 = agent_presence, 000075 = messages
- 15-minute TTL for agent presence (900 seconds) — more forgiving than Quorum's 5min or originally planned 10min
- System author type enables timeline events (room created, agent joined/left notifications)
- message_count on rooms is denormalized — must be maintained by application code in Phase 14
- sequence_num on messages is application-managed — Phase 14 implements the increment logic

</specifics>

<deferred>
## Deferred Ideas

- Room expiry cleanup job (Phase 14+ background job)
- Private room access control (Phase 16+ — column exists but no logic)
- Message editing (edited_at column — add when feature is built)
- Room creation from frontend UI (API/A2A only for now per REQUIREMENTS.md)
- Production migration deployment via admin query route (operational concern, not Phase 13 scope)

</deferred>

---

*Phase: 13-database-foundation*
*Context gathered: 2026-04-03*
