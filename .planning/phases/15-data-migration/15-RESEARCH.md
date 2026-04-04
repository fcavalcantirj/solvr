# Phase 15: Data Migration - Research

**Researched:** 2026-04-04
**Domain:** Go CLI tool, dual-PostgreSQL cross-DB migration, idempotent data transfer, agent registration
**Confidence:** HIGH — all findings verified from direct source inspection of Quorum schema, Solvr schema, migrations, models, and existing CLI tools

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Migration Mechanism**
- D-01: Standalone Go script at `backend/cmd/migrate-quorum/`. NOT a golang-migrate migration file.
- D-02: Keep script in repo after cutover (not deleted).
- D-03: Idempotent — uses ON CONFLICT DO NOTHING / check-before-insert. Safe to re-run.
- D-04: Structured logging — logs each step with counts: rooms migrated, messages migrated, agents created, skipped items. Summary at end.
- D-05: Single transaction — all-or-nothing. If anything fails, TX rolls back, Solvr DB untouched.
- D-06: Computes `message_count` on rooms during migration (UPDATE rooms SET message_count = COUNT of inserted messages per room).
- D-07: Assigns `sequence_num` on messages during migration using ROW_NUMBER() OVER (PARTITION BY room_id ORDER BY created_at).
- D-08: Integration tests with prod data dump in local Docker. Assertions: count accuracy, owner mapping, sequence numbers, exclusion of skipped rooms.

**Cross-DB Access**
- D-09: Dual pgx connections — `QUORUM_DB_URL` env var for Quorum, `DATABASE_URL` from .env for Solvr. Both are separate PostgreSQL databases on the same server.
- D-10: Run locally on macOS with SSH tunnels to prod DBs.
- D-11: `--dry-run` flag reads Quorum data and reports what WOULD be migrated without writing.
- D-12: No `--confirm` required — script writes by default, use `--dry-run` to preview.

**Room Selection (5 rooms, 215 messages)**
- D-13: Migrate ONLY these 5 rooms: ballona-trade-v0 (50 msgs), composio-integration (64 msgs), mackjack-ops (45 msgs), jack-mack-msv-trading (9 msgs), solvr-usage-analysys (47 msgs).
- D-14: Skip 9 rooms: 6 empty/test rooms + drope + claudius + fix-solvr-seo.
- D-15: All rooms are public (is_private=false), permanent (expires_at=NULL), effectively closed.

**Owner Reconciliation**
- D-16: Email JOIN: Quorum owner_id -> Quorum users.email -> Solvr users.email -> Solvr users.id.
- D-17: Only 1 Quorum user exists: felipecavalcantirj@gmail.com maps to Solvr user felipe_cavalcanti_1.
- D-18: Hardcoded owner mapping (only 2 owners for 5 rooms): felipe_cavalcanti_1 (composio-integration, solvr-usage-analysys), marcelo_b (ballona-trade-v0, mackjack-ops, jack-mack-msv-trading).
- D-19: No NULL owner_ids — all 5 rooms get explicit Solvr user owners.

**Agent Registration & Author Mapping**
- D-20: Each distinct Quorum agent_name becomes its own Solvr agent. Every message gets author_type='agent' + real author_id. NO NULLs on author_id.
- D-21: For matched agent names (ClaudiusThePirateEmperor, Jack, Mack) — use existing Solvr agent IDs.
- D-22: For unmatched agent names — register as NEW Solvr agents. Display-only (no api_key_hash). ID format: `agent_{quorum_name}`.
- D-23: New agent human_id based on room owner: agents in Felipe's rooms -> Felipe's user ID, agents in Marcelo's rooms -> Marcelo's user ID.
- D-24: ON CONFLICT DO NOTHING for agent ID conflicts.

**Message Content**
- D-25: Auto-detect content_type via regex: markdown patterns (headers, bold, code blocks) -> 'markdown', else -> 'text'.
- D-26: metadata = empty '{}' for all messages.
- D-27: deleted_at = NULL for all messages.
- D-28: Preserve created_at timestamps exactly from Quorum.
- D-29: New BIGSERIAL IDs for messages (auto-assign). Don't preserve Quorum message IDs.
- D-30: All messages within 64KB limit (max=53KB verified).

**Room Metadata**
- D-31: Preserve original UUIDs for rooms.
- D-32: Preserve token_hash as-is from Quorum.
- D-33: Set updated_at = created_at from Quorum.
- D-34: Keep original display_names except clean up special characters.
- D-35: Fix slug typo: 'solvr-usage-analysys' -> 'solvr-usage-analysis'.
- D-36: No description, tags, or category during migration — enrichment done separately by Claude Code post-migration.

**Room Metadata Enrichment (Post-Migration, Separate Step)**
- D-37: LLM enrichment done by Claude Code (uses subscription), NOT by the Go script.
- D-38: Claude Code reads first 25 messages per room, generates SEO-optimized description, category, and tags.
- D-39: Updates rooms via admin query route after migration.
- D-40: Claude decides whether to print for review or insert directly (discretion).

**Agent Presence**
- D-41: Skip ALL 16 agent_presence records — all expired. DATA-02 satisfied.

**Cutover & Rollback**
- D-42: Quorum offline during migration. Stop service before running script.
- D-43: Single-TX rollback is the rollback plan.
- D-44: Script logs final counts, then manually verify GET /v1/rooms.
- D-45: No DNS redirects needed.
- D-46: Prerequisite: apply migrations 000073-075 to Solvr prod via admin query route BEFORE running migration script.

**Credentials**
- D-47: QUORUM_DB_URL passed as env var at runtime. Never stored in files.
- D-48: Solvr DB via existing DATABASE_URL from .env.

**Post-Migration Cleanup**
- D-49: Keep Quorum DB as backup until Phase 16 ships.
- D-50: Stop Quorum process AND Docker container after migration.
- D-51: Quorum web frontend kept running for now.

**Edge Cases**
- D-52: ON CONFLICT DO NOTHING for room UUID conflicts.
- D-53: Check for slug conflicts before inserting — warn and skip if conflict exists.
- D-54: Schema migrations 000073-075 are a prerequisite, not part of the migration script.

### Claude's Discretion
- Room UUID preservation strategy (recommended: preserve original UUIDs)
- display_name cleanup approach for special characters
- Whether to print LLM-generated room metadata for review or insert directly
- updated_at strategy (decided: use created_at from Quorum)
- Quorum DB credentials: env var at runtime vs CLI flag (decided: env var)

### Deferred Ideas (OUT OF SCOPE)
- Room expiry cleanup job (future background job)
- Private room access control
- Quorum web frontend decommissioning (kept running for now)
- Quorum DB deletion (after Phase 16 ships)
- Room metadata editing from frontend UI (API/A2A only for now)
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DATA-01 | Existing Quorum rooms and messages migrated to Solvr DB with owner_id reconciliation by email | Schema mapping section, email-join pattern, agent registration, content-type detection |
| DATA-02 | Quorum agent_presence data migrated (expired entries pruned) | All 16 records expired — skip entirely satisfies this requirement |
| DATA-03 | Message sequence IDs reset correctly after migration | ROW_NUMBER() OVER (PARTITION BY room_id ORDER BY created_at) assigns sequence_num in single TX |
</phase_requirements>

---

## Summary

Phase 15 is a one-time, offline, standalone Go script that reads from Quorum's PostgreSQL database and writes to Solvr's PostgreSQL database inside a single transaction. The scope is small and fully specified: 5 rooms, 215 messages, 22 distinct agent names (3 pre-existing, 19 net-new), 2 human owners. All edge cases (agent presence, Quorum user table, room owner FK mismatch, slug typo) have been resolved in the context session.

The migration script follows the exact same structural pattern as `backend/cmd/backfill-embeddings/main.go`: a `flag`-parsed main function, interface-abstracted DB operations for testability, `slog` structured logging, and `os.Getenv` for credentials. The key difference is dual-DB connections (one for Quorum, one for Solvr) and a wrapping transaction on the Solvr side. Content-type detection is a pure Go regex function with no external dependencies.

Data integrity is guaranteed by: (1) single Solvr transaction with auto-rollback on any error; (2) ON CONFLICT DO NOTHING for room UUIDs and agent IDs; (3) slug conflict check before insert; (4) sequence_num computed via ROW_NUMBER() OVER in the bulk INSERT rather than row-by-row calls to `MessageRepository.Create`, avoiding the max(sequence_num)+1 subquery race condition for bulk inserts.

**Primary recommendation:** Build the script with a testable interface abstraction (`migrationDB interface`) so the core logic can be unit-tested with mocks, and integration tests can run against a local Docker PostgreSQL with a prod data dump.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/jackc/pgx/v5 | v5.7.2 | Both DB connections (Quorum + Solvr) | Already in go.mod; pgxpool.New for pooled, pgx.Connect for single-conn CLI |
| github.com/google/uuid | v1.6.0 | UUID parsing for room IDs | Already in go.mod |
| log/slog | stdlib | Structured logging | Established pattern across all Solvr CLI tools |
| flag | stdlib | `--dry-run` flag parsing | Established pattern in backfill-embeddings |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| regexp | stdlib | Content-type detection (markdown vs text) | Single compile-once regex package |
| context | stdlib | Timeout and cancellation | Standard Go pattern |
| fmt, os, strings | stdlib | General utilities | Standard Go |

**No new dependencies required.** All packages are already in `go.mod`.

**Installation:**
```bash
# No new packages — build directly:
go build ./backend/cmd/migrate-quorum/
```

---

## Architecture Patterns

### Recommended Project Structure
```
backend/cmd/migrate-quorum/
├── main.go          # flag parsing, DB wiring, run() call, summary output
└── migrate_test.go  # unit tests with mock migrationDB interface
```

This matches the two-file structure of `backend/cmd/backfill-embeddings/` (main.go + backfill_test.go).

### Pattern 1: Dual-Connection CLI with Interface Abstraction

**What:** Open two pgx connections (Quorum read-only, Solvr read-write) in `main()`. Pass both to a `migrator` struct that implements all logic. The `migrator` operates against a `migrationDB` interface so tests can use mocks.

**When to use:** Any CLI tool needing two DB connections where logic must be testable without live DBs.

**Example:**
```go
// Source: backend/cmd/backfill-embeddings/main.go (adaptation pattern)

type migrationDB interface {
    // Quorum-side reads
    ListQuorumRooms(ctx context.Context) ([]quorumRoom, error)
    ListQuorumMessages(ctx context.Context, roomID uuid.UUID) ([]quorumMessage, error)

    // Solvr-side writes (all within a passed pgx.Tx)
    InsertRoom(ctx context.Context, tx pgx.Tx, r roomInsert) error
    InsertAgent(ctx context.Context, tx pgx.Tx, a agentInsert) error
    InsertMessages(ctx context.Context, tx pgx.Tx, msgs []messageInsert) error
    UpdateRoomMessageCount(ctx context.Context, tx pgx.Tx, roomID uuid.UUID, count int) error
    CheckSlugConflict(ctx context.Context, slug string) (bool, error)
    FindSolvrUserByEmail(ctx context.Context, email string) (*uuid.UUID, error)
    FindSolvrAgentByID(ctx context.Context, agentID string) (bool, error)
}

type migrator struct {
    quorumPool *pgxpool.Pool  // read-only source
    solvrPool  *db.Pool       // transactional target
    dryRun     bool
}
```

### Pattern 2: Solvr db.Pool Transaction Pattern

**What:** Use `pool.BeginTx(ctx)` to get a `db.Tx` interface. All Solvr writes go through this transaction. Defer `tx.Rollback(ctx)` immediately after `BeginTx` (pgx Rollback is a no-op after Commit).

**When to use:** Any bulk operation that must be all-or-nothing.

**Example:**
```go
// Source: backend/internal/db/pool.go (pool.BeginTx / pool.WithTx patterns)

tx, err := solvrPool.BeginTx(ctx)
if err != nil {
    return fmt.Errorf("begin transaction: %w", err)
}
defer tx.Rollback(ctx) // no-op after Commit; rolls back on error path

// ... all inserts use tx.Exec(ctx, ...) ...

if err := tx.Commit(ctx); err != nil {
    return fmt.Errorf("commit transaction: %w", err)
}
```

### Pattern 3: Bulk Message Insert with ROW_NUMBER() Sequence Assignment

**What:** Do NOT call `MessageRepository.Create()` per-message (its subquery `COALESCE(MAX(sequence_num), 0) + 1` is designed for concurrent single inserts, not bulk). Instead, use a single `INSERT ... SELECT` with `ROW_NUMBER() OVER (PARTITION BY room_id ORDER BY created_at)` directly in SQL.

**When to use:** Bulk insert of ordered messages where sequence_num must be consecutive per room starting from 1.

**Example:**
```sql
-- Source: CONTEXT.md D-07 + messages table schema (000075_create_messages.up.sql)
-- Insert all messages for one room in a single statement.
-- ROW_NUMBER() assigns sequence_num 1, 2, 3, ... ordered by original created_at.

INSERT INTO messages (room_id, author_type, author_id, agent_name, content, content_type, metadata, sequence_num, created_at)
SELECT
    $1::uuid,
    'agent',
    agent_id_map[$2],  -- resolved per agent_name
    agent_name,
    content,
    CASE
        WHEN content ~ '(^#{1,6} |^\*\*|^```|`[^`]+`)' THEN 'markdown'
        ELSE 'text'
    END,
    '{}',
    ROW_NUMBER() OVER (ORDER BY created_at),
    created_at
FROM unnest($3::text[], $4::text[], $5::text[], $6::timestamptz[])
    AS t(agent_name, agent_id, content, created_at)
```

In practice, construct this as a Go-side loop building a pgx batch, using `pgx.Batch` or passing slices via `unnest`. The key constraint is that sequence_num is computed in SQL, not in Go, ensuring consistency.

### Pattern 4: Content-Type Detection

**What:** Pure Go function with compiled regex, no external packages. Called once per message.

**When to use:** All 215 messages during migration.

**Example:**
```go
// Source: CONTEXT.md D-25, verified against known content split: 145 text + 70 markdown
var markdownPattern = regexp.MustCompile(`(?m)(^#{1,6} |\*\*[^*]+\*\*|^` + "```" + `|` + "`[^`]+`" + `)`)

func detectContentType(content string) string {
    if markdownPattern.MatchString(content) {
        return "markdown"
    }
    return "text"
}
```

### Pattern 5: Hardcoded Owner + Agent Mapping

**What:** Since there are exactly 2 human owners and 3 pre-existing agents, use a literal map in Go code rather than a database lookup. This makes the script self-contained and verifiable.

**When to use:** When the mapping set is small, fully enumerated, and verified.

**Example:**
```go
// Source: CONTEXT.md D-18, D-21 — verified against prod data 2026-04-04

// Hardcoded slug -> Solvr user email mapping (verified 2026-04-04)
var slugOwnerEmail = map[string]string{
    "composio-integration":  "felipecavalcantirj@gmail.com",
    "solvr-usage-analysys":  "felipecavalcantirj@gmail.com",  // original slug (pre-fix)
    "ballona-trade-v0":      "macballona@gmail.com",
    "mackjack-ops":          "macballona@gmail.com",
    "jack-mack-msv-trading": "macballona@gmail.com",
}

// Pre-existing Solvr agent IDs for matched Quorum agent names (verified 2026-04-04)
var knownAgentIDs = map[string]string{
    "ClaudiusThePirateEmperor": "agent_ClaudiusThePirateEmperor",
    "Jack":                     "agent_Jack",
    "Mack":                     "agent_Mack",
}
```

### Pattern 6: Dry-Run Reporting

**What:** In dry-run mode, the script queries Quorum, resolves owner IDs and agent mappings in-memory, then prints a summary without opening a Solvr transaction.

**When to use:** All pre-cutover verification runs.

**Example:**
```go
// Source: backend/cmd/backfill-embeddings/main.go (dry-run pattern)
if m.dryRun {
    slog.Info("DRY RUN — no changes written to Solvr DB")
    slog.Info("Would migrate", "rooms", len(rooms), "messages", totalMessages,
        "new_agents", newAgentCount, "skipped_rooms", skippedCount)
    return nil
}
```

### Pattern 7: Quorum DB Connection (Direct pgxpool, Not Solvr db.Pool)

**What:** The Quorum connection is a raw `*pgxpool.Pool` (or single `*pgx.Conn`), not Solvr's `db.Pool` wrapper. The wrapper is not needed for a read-only source connection. Use `pgxpool.New(ctx, os.Getenv("QUORUM_DB_URL"))` directly.

**When to use:** External read-only source database connections.

```go
// Source: pgx/v5 docs, established pattern in backfill-embeddings pgBackfillDB struct
quorumPool, err := pgxpool.New(ctx, os.Getenv("QUORUM_DB_URL"))
if err != nil {
    log.Fatalf("connect to Quorum DB: %v", err)
}
defer quorumPool.Close()
```

### Anti-Patterns to Avoid

- **Using MessageRepository.Create() in a loop:** The `COALESCE(MAX(sequence_num), 0) + 1` subquery is designed for concurrent single inserts. Calling it 215 times will produce correct sequence numbers but is slower and misses the elegance of ROW_NUMBER() in a single statement. Worse, calling it inside a transaction when the room is empty works correctly, but it's not the intended pattern for bulk migration.
- **Preserving Quorum message BIGSERIAL IDs:** Decision D-29 explicitly says do NOT preserve. Solvr's BIGSERIAL sequence auto-assigns new IDs. Attempting to INSERT with explicit IDs breaks the sequence and causes future auto-inserts to conflict.
- **Using golang-migrate for this operation:** This is NOT a schema migration. It is a data migration. It must NOT be a `.sql` file in `backend/migrations/`. It is a standalone Go CLI.
- **Running without stopping Quorum first:** Quorum writes to messages table while migration reads it. New messages could arrive mid-migration, causing an inconsistent count in the summary. D-42 mandates Quorum offline.
- **Storing QUORUM_DB_URL in any file:** D-47 is explicit. Runtime env var only.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Transaction wrapping | Custom BEGIN/COMMIT logic | `pool.BeginTx(ctx)` from `db.Pool` | Already implements `db.Tx` interface with Rollback defer pattern |
| Structured logging | fmt.Printf calls | `log/slog` | Already used across all Solvr CLI tools; JSON output in prod |
| UUID generation | Custom UUID gen | `github.com/google/uuid` | Already in go.mod; UUIDs preserved from Quorum, no generation needed |
| Slug validation | Custom regex | Check against existing `rooms` table | Solvr's rooms table has the slug constraint; let PostgreSQL enforce |
| Content detection | External parser | Pure regex (markdownPattern) | Confirmed against prod split: 145 text + 70 markdown, regex sufficient |

---

## Runtime State Inventory

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | Quorum PostgreSQL: 14 rooms, 302 messages, 16 agent_presence, 1 user at postgres://...148.230.73.44:5444/quorum | Read-only source — no changes to Quorum DB |
| Stored data | Solvr PostgreSQL (local port 5433, prod): rooms/messages/agent_presence tables will be WRITTEN by migration | This is the target of the migration — write via single TX |
| Live service config | Quorum service must be STOPPED before migration runs (D-42) | Stop Quorum process + Docker container BEFORE running script |
| OS-registered state | None — Quorum is a Docker container, not a systemd/launchd service | None |
| Secrets/env vars | QUORUM_DB_URL = runtime env var (never in files); DATABASE_URL = from Solvr .env | Pass QUORUM_DB_URL at command line: `QUORUM_DB_URL="..." go run ...` |
| Build artifacts | New binary: `backend/cmd/migrate-quorum/` — built fresh each run | `go build ./backend/cmd/migrate-quorum/` before cutover |

**Post-migration runtime state changes:**
- Quorum DB: remains as-is (backup until Phase 16)
- Solvr `rooms` table: 5 new rows
- Solvr `messages` table: 215 new rows
- Solvr `agents` table: up to 19 new rows (for unmatched agent names)
- agent_presence: 0 rows added (all expired, D-41)

---

## Common Pitfalls

### Pitfall 1: Quorum rooms.owner_id FK References a Different users Population

**What goes wrong:** Quorum's `rooms.owner_id` is a UUID FK to Quorum's `users.id`. These UUIDs do not exist in Solvr's `users` table. A naive INSERT preserving owner_id will violate the FK constraint.

**Why it happens:** Two independent user tables with the same concept but different UUID populations.

**How to avoid:** Use the hardcoded slug-to-email mapping (D-18). Lookup Solvr user IDs by email at script startup:
```go
var felipeID, marceloID uuid.UUID
pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, "felipecavalcantirj@gmail.com").Scan(&felipeID)
pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, "macballona@gmail.com").Scan(&marceloID)
```
Fail fast if either lookup returns no rows — this is a hard prerequisite.

**Warning signs:** `ERROR: insert or update on table "rooms" violates foreign key constraint "rooms_owner_id_fkey"`

---

### Pitfall 2: Slug Typo Fix Breaks Idempotency

**What goes wrong:** Quorum slug is `solvr-usage-analysys` (typo). D-35 corrects it to `solvr-usage-analysis`. On first run, the room is inserted with slug `solvr-usage-analysis`. On re-run (idempotency), the script reads slug `solvr-usage-analysys` from Quorum, maps it to target slug `solvr-usage-analysis`, and the ON CONFLICT DO NOTHING on `rooms.id` (UUID) handles it correctly — but only if conflict detection is on the UUID (id), not the slug.

**Why it happens:** The slug transformation creates a mismatch between Quorum slug (used to look up the room) and Solvr slug (used for conflict detection).

**How to avoid:** Use `ON CONFLICT (id) DO NOTHING` (conflict on UUID primary key) rather than on slug. The UUID is preserved from Quorum (D-31), making UUID-based idempotency correct. Build the slug-transform map:
```go
var slugTransforms = map[string]string{
    "solvr-usage-analysys": "solvr-usage-analysis",
}
func targetSlug(quorumSlug string) string {
    if s, ok := slugTransforms[quorumSlug]; ok {
        return s
    }
    return quorumSlug
}
```

**Warning signs:** `ERROR: duplicate key value violates unique constraint "rooms_slug_key"` on re-run

---

### Pitfall 3: Solvr rooms Table Has display_name VARCHAR(200), Quorum Has TEXT

**What goes wrong:** Quorum's `display_name` is `TEXT NOT NULL` (unlimited). Solvr's is `VARCHAR(200)`. If any Quorum room has a display_name > 200 chars (unlikely but possible for special chars after cleanup), the INSERT will fail with a truncation error.

**Why it happens:** Schema divergence between source and target.

**How to avoid:** Truncate display_name to 200 chars in the script, log a warning if truncation occurs. Given the 5 rooms are known, this is defensive coding only.

---

### Pitfall 4: Agent ID Length Constraint (VARCHAR(50))

**What goes wrong:** The `agents` table has `id VARCHAR(50) PRIMARY KEY`. Quorum agent names can be long. The format `agent_{quorum_name}` may exceed 50 chars. For example: `agent_ClaudiusThePirateEmperor` = 30 chars (fine). But hypothetically `agent_SomeLongAgentNameWith30Chars` = 39 chars (fine). The pattern `agent_{name}` adds 6 chars of prefix; names up to 44 chars are safe.

**Why it happens:** VARCHAR(50) limit on agents.id is enforced at DB level.

**How to avoid:** Validate agent ID length before INSERT. If an agent name would produce an ID > 50 chars, truncate the name portion: `agent_` + name[:44]. Log a warning. Given the 22 known agent names from the Quorum prod data, verify none exceed 44 chars at script start.

**Warning signs:** `ERROR: value too long for type character varying(50)`

---

### Pitfall 5: MessageRepository.Create() sequence_num Subquery vs Bulk Insert

**What goes wrong:** `MessageRepository.Create()` uses `COALESCE(MAX(sequence_num), 0) + 1 WHERE room_id = $1 AND deleted_at IS NULL` as a subquery within each INSERT. This is designed for concurrent single-message inserts. In a migration context wrapping 215 messages in one TX, calling this 215 times works functionally, but is inefficient and more importantly the subquery reads rows inserted in the same TX, which is correct for sequential inserts but produces sequence numbers 1, 2, 3, ..., N correctly only if inserts are ordered.

**Why it happens:** Temptation to reuse existing repository methods to avoid writing SQL.

**How to avoid:** Do NOT use `MessageRepository.Create()`. Use a raw `tx.Exec()` with ROW_NUMBER() OVER (PARTITION BY room_id ORDER BY created_at) as specified in D-07. This guarantees sequence continuity in a single statement regardless of insertion order.

---

### Pitfall 6: Single Transaction Wraps Both DBs (Not Possible — They Are Separate DBs)

**What goes wrong:** It's tempting to think a single TX can span both the Quorum read and the Solvr write, providing atomic cross-DB safety. This is not possible — PostgreSQL transactions are per-connection and cannot span two separate DB servers without distributed transaction support (XA), which pgx does not support.

**Why it happens:** Misreading D-05 ("single transaction") as covering both databases.

**How to avoid:** D-05 means a single Solvr transaction. The Quorum reads happen outside any transaction (or in a read-only Quorum TX for consistent reads). Rollback on failure only covers the Solvr TX. Quorum DB is never modified. This is correct and sufficient.

---

### Pitfall 7: messages Table content_type CHECK Constraint

**What goes wrong:** `messages.content_type` has `CHECK (content_type IN ('text', 'markdown', 'json'))`. If the regex-based content detection produces any value other than these three, the INSERT fails.

**Why it happens:** The detection function must return exactly one of these three values.

**How to avoid:** The `detectContentType` function must return either `"text"` or `"markdown"` only (D-25 confirmed 0 json messages). Add a unit test that verifies `detectContentType` never returns an unexpected value.

---

### Pitfall 8: Quorum's agent_presence card_json JSONB vs Solvr's 16KB Limit

**What goes wrong:** Quorum `agent_presence.card_json` is `JSONB NOT NULL` with no size limit. Solvr's `agent_presence.card_json` has `CHECK (length(card_json::text) <= 16384)`. If a migrated presence record had card_json > 16KB, the INSERT would fail.

**Why it happens:** Schema divergence; Solvr added a size guard.

**How to avoid:** This is moot for Phase 15 — D-41 skips ALL agent_presence records. DATA-02 is satisfied by not inserting any. Document this constraint in the code comment near the presence-skip logic.

---

### Pitfall 9: Hardcoded Quorum prod connection URL is in CONTEXT.md (not to be committed)

**What goes wrong:** CONTEXT.md (15-CONTEXT.md) contains the literal Quorum prod DB URL under `<specifics>`. If this file is committed, credentials are in git history.

**Why it happens:** Context gathering sessions sometimes capture runtime credentials for convenience.

**How to avoid:** The Quorum prod URL is passed as `QUORUM_DB_URL` env var at runtime (D-47). Never copy it into source files. The CONTEXT.md file is in `.planning/` which is git-tracked — verify `.gitignore` does NOT exclude `.planning/` (currently it doesn't, but check before any commit that touches `15-CONTEXT.md`). The URL in CONTEXT.md is reference documentation, not code. Flag this to user before phase execution.

---

## Code Examples

### Script main() structure (verified from backfill-embeddings/main.go pattern)

```go
// Source: backend/cmd/backfill-embeddings/main.go (pattern)
func main() {
    dryRun := flag.Bool("dry-run", false, "Report what would be migrated without writing")
    flag.Parse()

    quorumURL := os.Getenv("QUORUM_DB_URL")
    if quorumURL == "" {
        log.Fatal("QUORUM_DB_URL is required")
    }

    cfg, err := config.Load()  // reads DATABASE_URL from env
    if err != nil {
        log.Fatalf("load config: %v", err)
    }

    ctx := context.Background()

    quorumPool, err := pgxpool.New(ctx, quorumURL)
    // ... error check ...
    defer quorumPool.Close()

    solvrPool, err := db.NewPool(ctx, cfg.DatabaseURL)
    // ... error check ...
    defer solvrPool.Close()

    m := &migrator{quorumPool: quorumPool, solvrPool: solvrPool, dryRun: *dryRun}
    result, err := m.run(ctx)
    if err != nil {
        log.Fatalf("migration failed: %v", err)
    }
    // print summary
}
```

### Room INSERT with UUID preservation and ON CONFLICT DO NOTHING

```sql
-- Source: 000073_create_rooms.up.sql + CONTEXT.md D-31, D-32, D-33, D-35
INSERT INTO rooms (
    id, slug, display_name, is_private, owner_id,
    token_hash, message_count, created_at, updated_at, last_active_at
) VALUES (
    $1, $2, $3, FALSE, $4,
    $5, 0, $6, $6, $6
) ON CONFLICT (id) DO NOTHING
```

### Agent INSERT for unmatched agents

```sql
-- Source: 000002_create_agents.up.sql + CONTEXT.md D-22, D-23, D-24
INSERT INTO agents (id, display_name, human_id)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO NOTHING
```

### message_count UPDATE after message batch insert

```sql
-- Source: CONTEXT.md D-06
UPDATE rooms
SET message_count = (SELECT COUNT(*) FROM messages WHERE room_id = $1 AND deleted_at IS NULL)
WHERE id = $1
```

### Slug conflict pre-check

```sql
-- Source: CONTEXT.md D-53
SELECT EXISTS(SELECT 1 FROM rooms WHERE slug = $1 AND id != $2)
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Quorum agents.id = VARCHAR(50), display_name = VARCHAR(50) | Same in Solvr agents table | Solvr established pattern | Agent IDs must fit VARCHAR(50) — enforce during insert |
| MessageRepository.Create() for per-message inserts | ROW_NUMBER() OVER for bulk | Phase 15 only | Correct sequence assignment in one SQL statement |
| Quorum token_hash: TEXT | Solvr token_hash: TEXT | Same | No conversion needed, preserve as-is |

**Deprecated/outdated in this context:**
- Quorum's `anonymous_session_id` column: not in Solvr rooms schema — do not migrate
- Quorum's `last_active_at` reset via `UpdateRoomActivity`: during migration, set to `created_at` per D-33

---

## Open Questions

1. **Quorum prod DB accessibility during cutover**
   - What we know: URL is `postgres://quorum:...@148.230.73.44:5444/quorum?sslmode=disable`
   - What's unclear: Whether an SSH tunnel is needed or if the port is directly reachable from macOS. The CONTEXT.md says "Run locally on macOS with SSH tunnels to prod DBs" (D-10).
   - Recommendation: Before cutover, verify direct TCP connectivity: `nc -z 148.230.73.44 5444`. If blocked, set up SSH tunnel: `ssh -L 5444:localhost:5444 user@148.230.73.44`. Planner should include a "verify connectivity" task.

2. **Solvr prod migrations 000073-075 prerequisite (D-46)**
   - What we know: These migrations must be applied to Solvr prod BEFORE the migration script runs. They create the rooms, agent_presence, and messages tables.
   - What's unclear: Whether they've been applied yet. STATE.md says Phase 14 is executing but the requirement MERGE-03 and MERGE-04 are pending.
   - Recommendation: First task in the plan should verify `SELECT table_name FROM information_schema.tables WHERE table_name IN ('rooms', 'agent_presence', 'messages')` returns 3 rows on prod.

3. **Agents in both Marcelo's and Felipe's rooms**
   - What we know: 22 distinct agent names; agents in Felipe's rooms get human_id = Felipe's UUID; agents in Marcelo's rooms get human_id = Marcelo's UUID (D-23).
   - What's unclear: If the same agent name appears in both Felipe's and Marcelo's rooms, which human_id wins? The ON CONFLICT DO NOTHING means the first INSERT wins.
   - Recommendation: Process Felipe's rooms first (insert agents with Felipe's human_id), then Marcelo's rooms (ON CONFLICT DO NOTHING skips agents already inserted). Planner should note this ordering.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | Building migrate-quorum script | yes | go1.25.3 darwin/arm64 | — |
| PostgreSQL (local port 5433) | Integration tests | yes | Docker container running | — |
| PostgreSQL (Quorum prod 148.230.73.44:5444) | Migration execution | unknown | needs verification | SSH tunnel (D-10) |
| PostgreSQL (Solvr prod) | Migration execution | yes | prod server | — |
| pgx/v5 (already in go.mod) | DB connections | yes | v5.7.2 | — |

**Missing dependencies with no fallback:**
- Quorum prod DB connectivity must be verified before cutover (see Open Question 1)
- Solvr prod migrations 000073-075 must be applied before cutover (see Open Question 2)

**Missing dependencies with fallback:**
- None

---

## Validation Architecture

`workflow.nyquist_validation` is not set in `.planning/config.json` — treat as enabled.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | none — standard `go test` |
| Quick run command | `cd backend && go test ./cmd/migrate-quorum/... -v` |
| Full suite command | `cd backend && DATABASE_URL="..." go test ./cmd/migrate-quorum/... -v` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DATA-01 | 5 rooms inserted with correct owner UUIDs | unit (mock) | `go test ./cmd/migrate-quorum/... -run TestMigration_RoomOwnerMapping` | no — Wave 0 |
| DATA-01 | 215 messages inserted with correct author_id and content_type | unit (mock) | `go test ./cmd/migrate-quorum/... -run TestMigration_MessageContentType` | no — Wave 0 |
| DATA-01 | Skipped rooms not in Solvr DB | unit (mock) | `go test ./cmd/migrate-quorum/... -run TestMigration_SkippedRooms` | no — Wave 0 |
| DATA-01 | Idempotency: re-run produces same counts, no duplicates | unit (mock) | `go test ./cmd/migrate-quorum/... -run TestMigration_Idempotent` | no — Wave 0 |
| DATA-02 | Zero agent_presence rows inserted | unit (mock) | `go test ./cmd/migrate-quorum/... -run TestMigration_AgentPresenceSkipped` | no — Wave 0 |
| DATA-03 | sequence_num is 1,2,...,N per room in created_at order | unit (mock) | `go test ./cmd/migrate-quorum/... -run TestMigration_SequenceNumbers` | no — Wave 0 |
| DATA-01 | TX rollback on error: no partial writes to Solvr | unit (mock) | `go test ./cmd/migrate-quorum/... -run TestMigration_TxRollback` | no — Wave 0 |

Integration test (D-08) runs against local Docker PostgreSQL with prod data dump:
```bash
QUORUM_DB_URL="postgres://quorum:pass@localhost:5455/quorum" DATABASE_URL="postgres://..." go test ./cmd/migrate-quorum/... -run TestIntegration_ -v
```

### Sampling Rate
- **Per task commit:** `cd backend && go test ./cmd/migrate-quorum/... -v`
- **Per wave merge:** Same (only one wave for this phase)
- **Phase gate:** All unit tests green + dry-run against prod Quorum shows expected counts

### Wave 0 Gaps
- [ ] `backend/cmd/migrate-quorum/main.go` — the script itself
- [ ] `backend/cmd/migrate-quorum/migrate_test.go` — unit tests with mock migrationDB

---

## Sources

### Primary (HIGH confidence)
- `/Users/fcavalcanti/dev/solvr/.planning/phases/15-data-migration/15-CONTEXT.md` — all 54 decisions
- `/Users/fcavalcanti/dev/quorum/relay/schema.sql` — Quorum source table definitions
- `/Users/fcavalcanti/dev/solvr/backend/migrations/000073_create_rooms.up.sql` — Solvr rooms target schema
- `/Users/fcavalcanti/dev/solvr/backend/migrations/000074_create_agent_presence.up.sql` — Solvr agent_presence target schema
- `/Users/fcavalcanti/dev/solvr/backend/migrations/000075_create_messages.up.sql` — Solvr messages target schema
- `/Users/fcavalcanti/dev/solvr/backend/cmd/backfill-embeddings/main.go` — CLI tool pattern reference
- `/Users/fcavalcanti/dev/solvr/backend/internal/db/pool.go` — db.Pool BeginTx/Tx interface
- `/Users/fcavalcanti/dev/solvr/backend/internal/db/rooms.go` — RoomRepository pgx patterns
- `/Users/fcavalcanti/dev/solvr/backend/internal/db/room_messages.go` — MessageRepository Create sequence_num subquery
- `/Users/fcavalcanti/dev/solvr/backend/internal/db/agents.go` — AgentRepository.Create INSERT pattern
- `/Users/fcavalcanti/dev/solvr/backend/internal/models/room.go` — Room model struct (15 columns)
- `/Users/fcavalcanti/dev/solvr/backend/internal/models/message.go` — Message model struct (11 columns)
- `/Users/fcavalcanti/dev/solvr/backend/internal/models/agent.go` — Agent model struct
- `/Users/fcavalcanti/dev/solvr/backend/go.mod` — dependency versions verified

### Secondary (MEDIUM confidence)
- `.planning/research/PITFALLS.md` — Pitfall 6 (FK to wrong users population) verified and applied to Phase 15 context

### Tertiary (LOW confidence)
- None

---

## Metadata

**Confidence breakdown:**
- Standard Stack: HIGH — all packages already in go.mod, verified versions
- Architecture: HIGH — verified directly against existing CLI tool pattern (backfill-embeddings) and all schema files
- Pitfalls: HIGH — all verified against actual schemas and CONTEXT.md decisions
- Runtime State: HIGH — verified against prod snapshot documented in CONTEXT.md (2026-04-04)

**Research date:** 2026-04-04
**Valid until:** 2026-05-04 (stable — decisions locked, schemas frozen)
