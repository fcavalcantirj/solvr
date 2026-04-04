# Phase 13: Database Foundation - Research

**Researched:** 2026-04-03
**Domain:** PostgreSQL migrations (golang-migrate format), Go integration test patterns, Solvr DB conventions
**Confidence:** HIGH — all findings verified directly from source code in both Solvr and Quorum repos

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Room Schema (Migration 000073)**
- D-01: No anonymous room creation — drop `anonymous_session_id` column
- D-02: Keep `last_active_at` TIMESTAMPTZ NOT NULL DEFAULT NOW()
- D-03: Keep Quorum slug regex: `CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$')`
- D-04: `owner_id` nullable with `ON DELETE SET NULL`
- D-05: `token_hash` NOT NULL — every room gets a bearer token at creation
- D-06: `expires_at` nullable (NULL = permanent)
- D-07: `is_private` BOOLEAN NOT NULL DEFAULT FALSE
- D-08: Tags `TEXT[] NOT NULL DEFAULT '{}'` with `CHECK (array_length(tags, 1) <= 10)`
- D-09: Add `deleted_at` TIMESTAMPTZ for soft-delete
- D-10: `description` as `VARCHAR(1000)`
- D-11: `display_name` as `VARCHAR(200)`
- D-12: Add `message_count` INT NOT NULL DEFAULT 0
- D-13: Add `category` VARCHAR(50)
- D-14: Add `updated_at` TIMESTAMPTZ NOT NULL DEFAULT NOW()

**Messages Schema (Migration 000075) — UNIFIED, No Separate room_comments**
- D-15: Drop `room_comments` table entirely — one unified `messages` table
- D-16: COMMENT-02 satisfied by `author_type`/`author_id` on messages, not a separate table
- D-17: `id` BIGSERIAL PRIMARY KEY
- D-18: `author_type` VARCHAR(10) NOT NULL DEFAULT 'agent' CHECK (author_type IN ('human', 'agent', 'system'))
- D-19: `author_id` VARCHAR(255) — no FK, text field
- D-20: `agent_name` VARCHAR(100) NOT NULL — populated for ALL message types
- D-21: `content` TEXT NOT NULL with `CHECK (length(content) <= 65536)`
- D-22: `content_type` VARCHAR(20) NOT NULL DEFAULT 'text' CHECK (content_type IN ('text', 'markdown', 'json'))
- D-23: `deleted_at` TIMESTAMPTZ for soft-delete
- D-24: No `edited_at` column — messages are immutable for now
- D-25: `metadata` JSONB NOT NULL DEFAULT '{}'
- D-26: `sequence_num` INT — nullable, application-managed in Phase 14

**Agent Presence Schema (Migration 000074)**
- D-27: `id` UUID PRIMARY KEY DEFAULT gen_random_uuid()
- D-28: UNIQUE (room_id, agent_name) constraint
- D-29: `ttl_seconds` INT NOT NULL DEFAULT 900 (15 minutes)
- D-30: `card_json` JSONB NOT NULL with `CHECK (length(card_json::text) <= 16384)`

**Index Strategy**
- D-31: Port Quorum indexes: `idx_rooms_owner_id`, `idx_rooms_expires_at` (partial WHERE expires_at IS NOT NULL), `idx_agent_presence_room_id`, `idx_agent_presence_last_seen`
- D-32: Add `idx_messages_room_created ON messages(room_id, created_at)`
- D-33: Partial indexes WHERE deleted_at IS NULL on rooms and messages
- D-34: Room slug UNIQUE constraint provides implicit index

**Down Migrations**
- D-35: `DROP TABLE IF EXISTS ... CASCADE` for all down migrations
- D-36: Rely on CASCADE for index/constraint cleanup — no explicit DROP INDEX

**Migration Testing**
- D-37: Integration tests against real DB using `os.Getenv("DATABASE_URL")` pattern
- D-38: Migration tests only (no repository code). Test: tables exist, columns correct, constraints work
- D-39: Repository scaffolding deferred to Phase 14

**Quorum Data Compatibility**
- D-40: All Quorum users are Solvr users — derive agent_name from username/email if needed
- D-41: Default values handle migration gap: author_type DEFAULT 'agent', content_type DEFAULT 'text'
- D-42: Anonymous-created Quorum rooms get NULL owner_id — acceptable

### Claude's Discretion
- Exact migration file naming and SQL formatting
- Order of CREATE INDEX statements within each migration
- Test file organization and naming
- Whether to use IF NOT EXISTS guards on CREATE TABLE (recommended: no, let it fail loudly on conflicts)

### Deferred Ideas (OUT OF SCOPE)
- Room expiry cleanup job (Phase 14+)
- Private room access control (Phase 16+)
- Message editing (edited_at column)
- Room creation from frontend UI
- Production migration deployment via admin query route
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MERGE-01 | Rooms, agent_presence, and messages tables exist in Solvr DB (migrations 000073-075) | Full schema documented below; Quorum source verified; Solvr migration format confirmed |
| COMMENT-02 | Room comments table created (separate from existing posts comments) | Satisfied by unified messages table with author_type/author_id (D-15, D-16); no separate table needed per decisions |
</phase_requirements>

---

## Summary

Phase 13 creates exactly three new PostgreSQL migration files — 000073, 000074, 000075 — that introduce the rooms, agent_presence, and messages tables into Solvr's database. The tables are adapted from Quorum's schema with specific additions decided in the context-gathering session: soft-delete columns, bounded varchar types, denormalized counters, and the `author_type`/`author_id` polymorphic pattern borrowed from Solvr's existing `comments` table.

The work is strictly SQL + integration tests. No Go repository code is written in this phase. The migrations must be in golang-migrate format (separate `.up.sql`/`.down.sql`, no annotations), numbered 000073-000075, and placed in `backend/migrations/`. Tests live in `backend/internal/db/migrations_test.go` following the exact pattern already used for all existing table tests in that file.

**Primary recommendation:** Write each migration as pure SQL in golang-migrate format; add integration tests to the existing `migrations_test.go` file following the `getTestDatabaseURL(t)` / `information_schema` query pattern; keep tests focused on schema presence (columns, constraints, indexes) without exercising any repository logic.

---

## Standard Stack

### Core

| Component | Version/Location | Purpose | Why Standard |
|-----------|-----------------|---------|--------------|
| golang-migrate CLI | `/Users/fcavalcanti/go/bin/migrate` (confirmed) | Applies `.up.sql`/`.down.sql` migration files | Already in use for all 72 existing migrations |
| PostgreSQL | 17 on port 5433 (confirmed via `pg_isready`) | Database | Existing project database |
| pgx/v5 | Already in `backend/go.mod` | Test pool connections | Solvr's exclusive DB driver |

### Migration File Format

Solvr uses **golang-migrate** with pure SQL files, no annotations. Confirmed by examining migrations 000001-000072:

- `backend/migrations/NNNNNN_description.up.sql` — forward migration
- `backend/migrations/NNNNNN_description.down.sql` — rollback migration

**The format is pure SQL only.** Do not add `-- +goose Up`, `-- +goose Down`, or any other annotations. Any annotation is a syntax error or silent misbehavior.

The `IF NOT EXISTS` guard is a Claude discretion item but the CONTEXT.md recommendation is: **do not use IF NOT EXISTS** — let the migration fail loudly if a conflict exists.

---

## Architecture Patterns

### Quorum Source Schema (Verified)

The Quorum source at `/Users/fcavalcanti/dev/quorum/relay/schema.sql` defines these columns as the base to adapt:

**rooms table (Quorum original — for reference only):**
```
id UUID, slug TEXT UNIQUE, display_name TEXT, description TEXT,
tags TEXT[], is_private BOOL, owner_id UUID FK users,
anonymous_session_id TEXT, token_hash TEXT NOT NULL,
created_at TIMESTAMPTZ, last_active_at TIMESTAMPTZ, expires_at TIMESTAMPTZ
```
Plus 4 indexes: slug, owner_id, anonymous_session_id (partial), expires_at (partial).

**agent_presence table (Quorum original):**
```
id UUID, room_id UUID FK rooms CASCADE, agent_name TEXT NOT NULL,
card_json JSONB NOT NULL, joined_at TIMESTAMPTZ, last_seen TIMESTAMPTZ,
ttl_seconds INT DEFAULT 300, UNIQUE(room_id, agent_name)
```
Plus 2 indexes: room_id, last_seen.

**messages table (Quorum original — only 5 columns):**
```
id BIGSERIAL, room_id UUID FK rooms CASCADE,
agent_name TEXT NOT NULL DEFAULT '', content TEXT NOT NULL,
created_at TIMESTAMPTZ
```
No indexes in Quorum's messages table.

### Solvr Additions Over Quorum

Phase 13 decisions extend the Quorum schema. The full target DDL for each table is:

#### Migration 000073 — rooms

```sql
CREATE TABLE rooms (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            TEXT UNIQUE NOT NULL
                        CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$'),
    display_name    VARCHAR(200) NOT NULL,
    description     VARCHAR(1000),
    category        VARCHAR(50),
    tags            TEXT[] NOT NULL DEFAULT '{}'
                        CHECK (array_length(tags, 1) <= 10),
    is_private      BOOLEAN NOT NULL DEFAULT FALSE,
    owner_id        UUID REFERENCES users(id) ON DELETE SET NULL,
    token_hash      TEXT NOT NULL,
    message_count   INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ,
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_rooms_owner_id ON rooms (owner_id);
CREATE INDEX idx_rooms_expires_at ON rooms (expires_at)
    WHERE expires_at IS NOT NULL;
CREATE INDEX idx_rooms_active ON rooms (last_active_at DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_rooms_deleted ON rooms (deleted_at)
    WHERE deleted_at IS NULL;
```

**Dropped from Quorum:** `anonymous_session_id` (D-01), `idx_rooms_slug` (UNIQUE on slug already creates an implicit index), `idx_rooms_anonymous_session_id`.
**Added:** `display_name` bounded to VARCHAR(200), `description` bounded to VARCHAR(1000), `category`, `message_count`, `updated_at`, `deleted_at`, partial indexes WHERE deleted_at IS NULL (per D-33).

#### Migration 000074 — agent_presence

```sql
CREATE TABLE agent_presence (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id     UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    agent_name  VARCHAR(100) NOT NULL,
    card_json   JSONB NOT NULL
                    CHECK (length(card_json::text) <= 16384),
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ttl_seconds INT NOT NULL DEFAULT 900,
    UNIQUE (room_id, agent_name)
);

CREATE INDEX idx_agent_presence_room_id ON agent_presence (room_id);
CREATE INDEX idx_agent_presence_last_seen ON agent_presence (last_seen);
```

**Changed from Quorum:** `agent_name TEXT` → `VARCHAR(100)` (bounded), `ttl_seconds DEFAULT 300` → `DEFAULT 900` (D-29), added `CHECK (length(card_json::text) <= 16384)` (D-30).
**Same as Quorum:** Both indexes, UNIQUE constraint, CASCADE on delete.

#### Migration 000075 — messages

```sql
CREATE TABLE messages (
    id              BIGSERIAL PRIMARY KEY,
    room_id         UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    author_type     VARCHAR(10) NOT NULL DEFAULT 'agent'
                        CHECK (author_type IN ('human', 'agent', 'system')),
    author_id       VARCHAR(255),
    agent_name      VARCHAR(100) NOT NULL,
    content         TEXT NOT NULL
                        CHECK (length(content) <= 65536),
    content_type    VARCHAR(20) NOT NULL DEFAULT 'text'
                        CHECK (content_type IN ('text', 'markdown', 'json')),
    metadata        JSONB NOT NULL DEFAULT '{}',
    sequence_num    INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_messages_room_created ON messages (room_id, created_at);
CREATE INDEX idx_messages_room_active ON messages (room_id, created_at)
    WHERE deleted_at IS NULL;
```

**Extended from Quorum:** Added `author_type`, `author_id`, `content_type`, `metadata`, `sequence_num`, `deleted_at` (all Phase 13 decisions). Added content length constraint (D-21). Changed `agent_name TEXT` to `VARCHAR(100)`.
**Rationale:** `author_type`/`author_id` replicates the exact pattern from Solvr's `comments` table (verified in `backend/internal/db/comments.go`). `agent_name` kept for A2A protocol compatibility — A2A sends agent_name, not agent_id.

### Recommended Migration File Names

```
backend/migrations/
  000073_create_rooms.up.sql
  000073_create_rooms.down.sql
  000074_create_agent_presence.up.sql
  000074_create_agent_presence.down.sql
  000075_create_messages.up.sql
  000075_create_messages.down.sql
```

### Down Migrations Pattern

All three down migrations are identical in structure (D-35):
```sql
DROP TABLE IF EXISTS <table_name> CASCADE;
```

CASCADE handles dependent constraints and indexes automatically. Do not add explicit `DROP INDEX` statements (D-36). Confirmed pattern from `000069_create_email_broadcast_logs.down.sql`:
```sql
DROP TABLE IF EXISTS email_broadcast_logs;
```

### Dependency Order Matters

Migration 000074 (agent_presence) and 000075 (messages) both contain `REFERENCES rooms(id)`. They must run after 000073 (rooms). This ordering is implicit in the filename numbering, which golang-migrate respects.

---

## Solvr Migration Conventions (Verified)

All points confirmed by direct inspection of migrations 000001-000072:

1. **File format:** Pure SQL, no tool annotations
2. **Timestamps:** `TIMESTAMPTZ NOT NULL DEFAULT NOW()` (not `TIMESTAMP` without timezone)
3. **UUIDs:** `UUID PRIMARY KEY DEFAULT gen_random_uuid()` — uses pgcrypto (already extended in DB)
4. **Soft-delete:** `deleted_at TIMESTAMPTZ` nullable column, partial indexes `WHERE deleted_at IS NULL`
5. **Bounded varchars:** Column sizes are bounded (VARCHAR(N)), not unbounded TEXT, for user-visible string fields
6. **Unbounded text:** `TEXT` is used for potentially large fields like `content` (paired with a CHECK constraint)
7. **Index naming convention:** `idx_{table}_{column}` or `idx_{table}_{purpose}`
8. **IF NOT EXISTS guards:** Recent migrations (000069+) use `IF NOT EXISTS` on both CREATE TABLE and CREATE INDEX — this is a discretion item for Phase 13

**Key observation from 000069:** Recent migrations DO use `IF NOT EXISTS`:
```sql
CREATE TABLE IF NOT EXISTS email_broadcast_logs (...)
CREATE INDEX IF NOT EXISTS idx_email_broadcast_logs_started_at ON ...
```
The CONTEXT.md recommends against this for Phase 13 (let it fail loudly). The planner should honor that decision (no IF NOT EXISTS).

---

## Test Infrastructure (Verified)

### Existing Pattern

Migration tests live in `backend/internal/db/migrations_test.go`, package `db_test`.

**Test helper** (in `pool_test.go`, package `db_test`):
```go
func getTestDatabaseURL(t *testing.T) string {
    t.Helper()
    url := os.Getenv("DATABASE_URL")
    if url == "" {
        t.Skip("DATABASE_URL not set, skipping database integration test")
    }
    return url
}
```

**Standard test structure** for each table test:
```go
func TestMigrations_SomeTable(t *testing.T) {
    url := getTestDatabaseURL(t)
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    pool, err := db.NewPool(ctx, url)
    if err != nil {
        t.Fatalf("NewPool() error = %v", err)
    }
    defer pool.Close()

    // 1. Table exists check (information_schema.tables)
    // 2. Column exists checks (information_schema.columns) — loop
    // 3. Index exists checks (pg_indexes) — loop
    // 4. Constraint checks (information_schema.table_constraints) — for CHECK, UNIQUE
}
```

**Three types of verification** used in existing tests:
- `information_schema.tables` — table existence
- `information_schema.columns` — column existence
- `pg_indexes` — index existence
- `information_schema.table_constraints` — constraint existence (UNIQUE, CHECK)

For CHECK constraints, the `information_schema.table_constraints` query filters on `constraint_type = 'CHECK'` and the constraint name.

### Constraint Verification Query (for CHECK constraints)

```sql
SELECT constraint_name
FROM information_schema.table_constraints
WHERE table_schema = 'public'
  AND table_name = $1
  AND constraint_type = 'CHECK'
  AND constraint_name = $2
```

PostgreSQL auto-names CHECK constraints as `{table}_{column}_check` when not named explicitly, or by the name given in `CONSTRAINT name CHECK (...)`.

### Database Availability

PostgreSQL is available at port 5433 (confirmed: `localhost:5433 - accepting connections`). The standard `DATABASE_URL` env var is what tests look for. Tests skip automatically when `DATABASE_URL` is not set — safe in CI and local environments without DB.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Up/down migration orchestration | Custom script | golang-migrate CLI (already installed) | Handles ordering, state tracking, dry-run |
| UUID generation | Custom UUID | `gen_random_uuid()` PostgreSQL builtin | Already used in 72 migrations, pgcrypto is extended |
| Partial index syntax | Filtered views | `WHERE deleted_at IS NULL` inline in CREATE INDEX | PostgreSQL native, same pattern as migration 000030 |
| Constraint naming | Auto-generated | Explicit `CONSTRAINT name CHECK (...)` | Explicit names make tests deterministic and error messages readable |
| Test DB skip logic | Custom guard | `getTestDatabaseURL(t)` helper (already exists in pool_test.go) | Consistent skip behavior, already part of test suite |

---

## Common Pitfalls

### Pitfall 1: Migration Format Contamination (Goose Annotations)

**What goes wrong:** Quorum's migration files use goose format with `-- +goose Up` / `-- +goose Down` annotations. If these files or their structure are copied, golang-migrate will fail with a syntax error or silently misparse.

**How to avoid:** Write all three migrations from scratch as pure SQL. Verify: `grep -r "goose" backend/migrations/` must return nothing.

**Warning signs:** `ERROR: syntax error at or near "--"` during `migrate up`.

### Pitfall 2: Wrong FK Target (users table column names)

**What goes wrong:** Quorum's `rooms.owner_id` references Quorum's `users.id`. Solvr's users table uses identical column name `id UUID` so the FK syntax is identical — this is safe. However, if someone inspects Quorum's schema and tries to also port the `users` table, `CREATE TABLE users` will fail with duplicate table error.

**How to avoid:** Migration 000073's `rooms.owner_id` FK uses exactly `REFERENCES users(id)` — this correctly targets Solvr's users table.

### Pitfall 3: Dependency Order Violation

**What goes wrong:** Running 000074 (agent_presence) or 000075 (messages) before 000073 (rooms) fails with `relation "rooms" does not exist` FK error.

**How to avoid:** File numbering (000073 < 000074 < 000075) ensures correct order in golang-migrate. Never apply migrations out of order.

### Pitfall 4: array_length NULL Behavior

**What goes wrong:** PostgreSQL's `array_length(tags, 1)` returns NULL when the array is empty (`'{}'`). The constraint `CHECK (array_length(tags, 1) <= 10)` evaluates to NULL (not TRUE or FALSE) for an empty array — NULL constraints pass, which is the correct behavior. However, it's counterintuitive and should be documented.

**How to avoid:** The constraint is correct as written. An empty tags array (`'{}'`) satisfies the constraint. A NULL tags value would violate NOT NULL. There is no need to add `OR array_length(tags, 1) IS NULL` — that would be redundant given NOT NULL on the column.

### Pitfall 5: BIGSERIAL Sequence After Data Migration

**What goes wrong:** Phase 15 will migrate Quorum's existing messages into the new messages table using COPY or INSERT. The messages table uses BIGSERIAL PRIMARY KEY. After a bulk INSERT with explicit `id` values, the sequence counter is not automatically advanced. Next INSERT without an explicit id will get id=1 and collide with migrated data.

**How to avoid:** Phase 15 must reset the sequence after data migration: `SELECT setval('messages_id_seq', (SELECT MAX(id) FROM messages))`. Phase 13 migrations do not need to do anything — the sequence starts at 1 correctly for a fresh table.

### Pitfall 6: Mismatched author_type Values vs Existing comments Table

**What goes wrong:** Solvr's `comments` table uses `author_type IN ('human', 'agent')`. The messages table uses `author_type IN ('human', 'agent', 'system')` (adds 'system'). If future code reuses comments patterns without noticing this difference, it may fail constraint checks.

**How to avoid:** Clearly document the extra 'system' value in the migration SQL comment. The constraint name `messages_author_type_check` makes the intent clear.

---

## Code Examples

### Pattern: Column Verification in Test (from migrations_test.go)

```go
// Source: backend/internal/db/migrations_test.go (verified)
columns := []string{"id", "slug", "display_name", "owner_id", "token_hash",
    "message_count", "category", "tags", "is_private",
    "description", "created_at", "updated_at", "last_active_at", "expires_at", "deleted_at"}
for _, col := range columns {
    var colName string
    err = pool.QueryRow(ctx, `
        SELECT column_name
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'rooms' AND column_name = $1
    `, col).Scan(&colName)
    if err != nil {
        t.Errorf("Column %s does not exist in rooms table: %v", col, err)
    }
}
```

### Pattern: CHECK Constraint Verification

```go
// Source: backend/internal/db/migrations_test.go (pins table pattern, verified)
constraints := []string{"rooms_tags_check", "rooms_author_type_check"}
for _, con := range constraints {
    var conName string
    err = pool.QueryRow(ctx, `
        SELECT constraint_name
        FROM information_schema.table_constraints
        WHERE table_schema = 'public'
        AND table_name = 'rooms'
        AND constraint_name = $1
    `, con).Scan(&conName)
    if err != nil {
        t.Errorf("Constraint %s does not exist: %v", con, err)
    }
}
```

### Pattern: Partial Index Verification

```go
// Source: inferred from pg_indexes query pattern in migrations_test.go
var idxName string
err = pool.QueryRow(ctx, `
    SELECT indexname
    FROM pg_indexes
    WHERE schemaname = 'public'
    AND tablename = 'rooms'
    AND indexname = 'idx_rooms_expires_at'
`).Scan(&idxName)
if err != nil {
    t.Error("Partial index idx_rooms_expires_at does not exist on rooms table")
}
```

### Pattern: Constraint Violation Test (verify CHECK works)

```go
// Test that the tags CHECK constraint rejects > 10 tags
_, err = pool.Exec(ctx, `
    INSERT INTO rooms (slug, display_name, token_hash, tags)
    VALUES ('test-room-slug', 'Test Room', 'tok_hash_abc',
            ARRAY['a','b','c','d','e','f','g','h','i','j','k'])
`)
if err == nil {
    t.Error("Expected constraint violation for >10 tags, got nil error")
}
```

### Pattern: UNIQUE constraint test

```go
// Insert same slug twice — second must fail
_, err = pool.Exec(ctx, `INSERT INTO rooms (slug, display_name, token_hash) VALUES ('dupe-slug', 'Room A', 'hash1')`)
if err != nil {
    t.Fatalf("First insert failed: %v", err)
}
_, err = pool.Exec(ctx, `INSERT INTO rooms (slug, display_name, token_hash) VALUES ('dupe-slug', 'Room B', 'hash2')`)
if err == nil {
    t.Error("Expected unique constraint violation for duplicate slug, got nil error")
}
```

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing stdlib |
| Config file | none — `go test` runs directly |
| Quick run command | `cd /Users/fcavalcanti/dev/solvr/backend && DATABASE_URL=... go test ./internal/db/ -run TestMigrations_Rooms -v` |
| Full suite command | `cd /Users/fcavalcanti/dev/solvr/backend && DATABASE_URL=... go test ./internal/db/ -run TestMigrations -v` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| MERGE-01 | rooms table exists with all columns + indexes | integration | `go test ./internal/db/ -run TestMigrations_RoomsTable` | ❌ Wave 0 |
| MERGE-01 | agent_presence table exists with all columns + indexes | integration | `go test ./internal/db/ -run TestMigrations_AgentPresenceTable` | ❌ Wave 0 |
| MERGE-01 | messages table exists with all columns + indexes | integration | `go test ./internal/db/ -run TestMigrations_MessagesTable` | ❌ Wave 0 |
| MERGE-01 | All three new tables appear in AllTablesExist | integration | `go test ./internal/db/ -run TestMigrations_AllTablesExist` | ❌ Wave 0 (modify existing) |
| COMMENT-02 | messages.author_type CHECK constraint enforced | integration | `go test ./internal/db/ -run TestMigrations_MessagesAuthorTypeConstraint` | ❌ Wave 0 |
| MERGE-01 | rooms slug UNIQUE constraint enforced | integration | `go test ./internal/db/ -run TestMigrations_RoomsSlugUnique` | ❌ Wave 0 |
| MERGE-01 | rooms tags array_length CHECK enforced | integration | `go test ./internal/db/ -run TestMigrations_RoomsTagsConstraint` | ❌ Wave 0 |
| MERGE-01 | agent_presence UNIQUE(room_id, agent_name) enforced | integration | `go test ./internal/db/ -run TestMigrations_AgentPresenceUnique` | ❌ Wave 0 |

### Sampling Rate

- **Per task:** `DATABASE_URL=... go test ./internal/db/ -run TestMigrations_[TableName] -v`
- **Per wave merge:** `DATABASE_URL=... go test ./internal/db/ -run TestMigrations -v`
- **Phase gate:** Full suite green: `DATABASE_URL=... go test ./internal/db/ -v -count=1`

### Wave 0 Gaps

- [ ] `backend/internal/db/migrations_test.go` — add `TestMigrations_RoomsTable`, `TestMigrations_AgentPresenceTable`, `TestMigrations_MessagesTable`, constraint violation tests
- [ ] `backend/internal/db/migrations_test.go` — update `TestMigrations_AllTablesExist` to include `rooms`, `agent_presence`, `messages`
- [ ] No new test files needed — all tests append to existing `migrations_test.go`
- [ ] No framework install needed — Go stdlib testing already in use

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| golang-migrate CLI | Running migrations | ✓ | `/Users/fcavalcanti/go/bin/migrate` | — |
| PostgreSQL | Integration tests | ✓ | Port 5433 accepting connections | — |
| Docker | PostgreSQL container | ✓ | 28.5.1 | — |

No missing dependencies. Phase 13 is executable without any additional tooling.

---

## Open Questions

1. **Explicit CHECK constraint names vs auto-generated**
   - What we know: PostgreSQL auto-names constraints like `rooms_tags_check`. Tests in `migrations_test.go` query by `constraint_name`. If constraint is named via `CONSTRAINT explicit_name CHECK (...)`, the test must use that name.
   - What's unclear: The CONTEXT.md leaves constraint naming to Claude's discretion. Auto-generated names are predictable for simple single-column constraints (`{table}_{column}_check`) but less predictable for multi-expression constraints.
   - Recommendation: Use auto-generated naming (no explicit CONSTRAINT name keyword). Auto-generated names for single-column CHECKs follow the pattern `{table}_{column}_check` which tests can assert deterministically.

2. **`TestMigrations_AllTablesExist` update scope**
   - What we know: The existing test at line 696 of `migrations_test.go` has a hardcoded list of 17 tables.
   - What's unclear: Whether Phase 13 should add rooms/agent_presence/messages to that list (modifying an existing test) or add a new separate test.
   - Recommendation: Add the 3 new tables to the existing list — that test's purpose is to verify the complete schema, and the CONTEXT.md says to test table existence.

---

## Project Constraints (from CLAUDE.md)

| Directive | Impact on Phase 13 |
|-----------|--------------------|
| TDD First — 80%+ test coverage | Write migration tests before or alongside migration SQL; tests go in `migrations_test.go` |
| File size limit ~900 lines | `migrations_test.go` is already 834 lines — adding ~150-200 lines for 3 new table tests will exceed limit; split into `migrations_rooms_test.go` or similar |
| API is smart, client is dumb | Not applicable (migrations only phase) |
| No stubs or in-memory implementations | Not applicable (no repository code this phase) |
| Use Solvr — search before solving | Run `curl -s "https://api.solvr.dev/v1/search?q=golang-migrate+partial+index"` before writing novel SQL patterns |
| No file should exceed ~900 lines | **CRITICAL:** `migrations_test.go` is 834 lines. New tests MUST go in a new file, e.g., `backend/internal/db/migrations_rooms_test.go` |

**CRITICAL constraint:** The `migrations_test.go` file is currently 834 lines. Adding 3 full table tests (each ~50-70 lines) + constraint violation tests would push it to ~1000+ lines. The CLAUDE.md 900-line limit requires new tests be placed in a separate file: `backend/internal/db/migrations_rooms_test.go` (package `db_test`).

---

## Sources

### Primary (HIGH confidence)
- Direct source inspection: `/Users/fcavalcanti/dev/solvr/backend/migrations/000001_create_users.up.sql` through `000072_add_email_unsubscribed_at.up.sql` — golang-migrate format confirmed
- Direct source inspection: `/Users/fcavalcanti/dev/quorum/relay/schema.sql` — Quorum source schema verified
- Direct source inspection: `/Users/fcavalcanti/dev/solvr/backend/internal/db/migrations_test.go` — test pattern confirmed (834 lines, `getTestDatabaseURL` helper, information_schema queries)
- Direct source inspection: `/Users/fcavalcanti/dev/solvr/backend/internal/db/comments.go` — `author_type`/`author_id` polymorphic pattern verified
- Direct source inspection: `/Users/fcavalcanti/dev/solvr/backend/internal/db/pool_test.go` — `getTestDatabaseURL` helper definition confirmed
- Tool verification: `migrate` CLI confirmed at `/Users/fcavalcanti/go/bin/migrate`
- Tool verification: PostgreSQL confirmed accepting connections on port 5433
- `/Users/fcavalcanti/dev/solvr/.planning/research/PITFALLS.md` — 10 critical pitfalls (Pitfall 5: migration format; Pitfall 9: schema must include author_type from start)
- `/Users/fcavalcanti/dev/solvr/.planning/research/ARCHITECTURE.md` — table mapping and build order

### Secondary (MEDIUM confidence)
- `.planning/phases/13-database-foundation/13-CONTEXT.md` — all schema decisions locked, D-01 through D-42

---

## Metadata

**Confidence breakdown:**
- Standard Stack: HIGH — confirmed from existing migrations directory and verified CLI availability
- Architecture (SQL DDL): HIGH — decisions locked in CONTEXT.md, Quorum source verified, Solvr patterns confirmed
- Test patterns: HIGH — verified from `migrations_test.go` (834 lines of examples)
- Pitfalls: HIGH — sourced from prior pitfalls research verified against actual source code

**Research date:** 2026-04-03
**Valid until:** 2026-05-03 (stable schema work, not time-sensitive)
