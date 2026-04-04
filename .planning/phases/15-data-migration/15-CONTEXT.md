# Phase 15: Data Migration - Context

**Gathered:** 2026-04-04
**Status:** Ready for planning

<domain>
## Phase Boundary

One-time cutover of Quorum room, message, and agent data into Solvr's DB. Includes owner reconciliation, agent registration, sequence numbering, content type detection, and post-migration LLM enrichment of room metadata for SEO. Quorum service decommissioned after.

</domain>

<decisions>
## Implementation Decisions

### Migration Mechanism
- **D-01:** Standalone Go script at `backend/cmd/migrate-quorum/`. NOT a golang-migrate migration file.
- **D-02:** Keep script in repo after cutover (not deleted).
- **D-03:** Idempotent — uses ON CONFLICT DO NOTHING / check-before-insert. Safe to re-run.
- **D-04:** Structured logging — logs each step with counts: rooms migrated, messages migrated, agents created, skipped items. Summary at end.
- **D-05:** Single transaction — all-or-nothing. If anything fails, TX rolls back, Solvr DB untouched.
- **D-06:** Computes `message_count` on rooms during migration (UPDATE rooms SET message_count = COUNT of inserted messages per room).
- **D-07:** Assigns `sequence_num` on messages during migration using ROW_NUMBER() OVER (PARTITION BY room_id ORDER BY created_at).
- **D-08:** Integration tests with prod data dump in local Docker. Assertions: count accuracy, owner mapping, sequence numbers, exclusion of skipped rooms.

### Cross-DB Access
- **D-09:** Dual pgx connections — `QUORUM_DB_URL` env var for Quorum, `DATABASE_URL` from .env for Solvr. Both are separate PostgreSQL databases on the same server.
- **D-10:** Run locally on macOS with SSH tunnels to prod DBs.
- **D-11:** `--dry-run` flag reads Quorum data and reports what WOULD be migrated without writing.
- **D-12:** No `--confirm` required — script writes by default, use `--dry-run` to preview.

### Room Selection (5 rooms, 215 messages)
- **D-13:** Migrate ONLY these 5 rooms:
  - `ballona-trade-v0` (50 msgs) — owner: marcelo_b (macballona@gmail.com)
  - `composio-integration` (64 msgs) — owner: felipe_cavalcanti_1 (felipecavalcantirj@gmail.com)
  - `mackjack-ops` (45 msgs) — owner: marcelo_b
  - `jack-mack-msv-trading` (9 msgs) — owner: marcelo_b
  - `solvr-usage-analysys` (47 msgs) — owner: felipe_cavalcanti_1
- **D-14:** Skip 9 rooms: 6 empty/test rooms (teste-brow-3, agent-sandbox, teste-brow, code-review-collective, test-room-alpha, e2e-test-1774312969) + drope (2 msgs) + claudius (64 msgs) + fix-solvr-seo (21 msgs).
- **D-15:** All rooms are public (is_private=false), permanent (expires_at=NULL), and effectively closed (no agent has the bearer tokens).

### Owner Reconciliation
- **D-16:** Email JOIN: Quorum owner_id -> Quorum users.email -> Solvr users.email -> Solvr users.id.
- **D-17:** Only 1 Quorum user exists: felipecavalcantirj@gmail.com (UUID 4baaee58-...) maps to Solvr user felipe_cavalcanti_1.
- **D-18:** Hardcoded owner mapping (only 2 owners for 5 rooms):
  - felipe_cavalcanti_1 (felipecavalcantirj@gmail.com): composio-integration, solvr-usage-analysys
  - marcelo_b (macballona@gmail.com): ballona-trade-v0, mackjack-ops, jack-mack-msv-trading
- **D-19:** No NULL owner_ids — all 5 rooms get explicit Solvr user owners.

### Agent Registration & Author Mapping
- **D-20:** Each distinct Quorum agent_name becomes its own Solvr agent. Every message gets `author_type='agent'` + a real `author_id` pointing to a Solvr agents table record. NO NULLs on author_id.
- **D-21:** For matched agent names (ClaudiusThePirateEmperor, Jack, Mack) — use existing Solvr agent IDs.
- **D-22:** For unmatched agent names — register as NEW Solvr agents. Display-only (no api_key_hash). ID format: `agent_{quorum_name}`. DB defaults for all other fields.
- **D-23:** New agent `human_id` based on room owner: agents in Felipe's rooms -> Felipe's user ID, agents in Marcelo's rooms -> Marcelo's user ID.
- **D-24:** ON CONFLICT DO NOTHING for agent ID conflicts (use existing agent if ID already exists).

### Message Content
- **D-25:** Auto-detect content_type via regex: markdown patterns (headers, bold, code blocks) -> 'markdown', else -> 'text'. Recon confirmed: 145 text + 70 markdown + 0 json.
- **D-26:** metadata = empty '{}' for all messages (Quorum has no metadata).
- **D-27:** deleted_at = NULL for all messages (Quorum has no soft-delete).
- **D-28:** Preserve created_at timestamps exactly from Quorum (TIMESTAMPTZ, no conversion needed).
- **D-29:** New BIGSERIAL IDs for messages (let Solvr's sequence auto-assign). Don't preserve Quorum message IDs.
- **D-30:** All messages within 64KB limit (max=53KB, avg=2KB, verified on prod).

### Room Metadata
- **D-31:** Preserve original UUIDs for rooms (Claude's discretion — recommended preserve for traceability).
- **D-32:** Preserve token_hash as-is from Quorum. Agents can't authenticate anyway (no one has the tokens).
- **D-33:** Set `updated_at = created_at` from Quorum (preserve original creation timestamp).
- **D-34:** Keep original display_names except: clean up special characters ('mack&jack-OPS' -> cleaned version).
- **D-35:** Fix slug typo: 'solvr-usage-analysys' -> 'solvr-usage-analysis'.
- **D-36:** No description, tags, or category during migration — enrichment done separately by Claude Code post-migration.

### Room Metadata Enrichment (Post-Migration, Separate Step)
- **D-37:** LLM enrichment done by Claude Code (uses subscription), NOT by the Go script. No Anthropic SDK dependency in migration tool.
- **D-38:** Claude Code reads first 25 messages per room, generates SEO-optimized description, category, and tags.
- **D-39:** Updates rooms via admin query route after migration.
- **D-40:** Claude decides whether to print for review or insert directly (discretion).

### Agent Presence
- **D-41:** Skip ALL 16 agent_presence records — all expired (last seen days ago, TTL 1800s). DATA-02 satisfied: "expired entries pruned."

### Cutover & Rollback
- **D-42:** Quorum offline during migration. Stop service before running script.
- **D-43:** Single-TX rollback is the rollback plan. If anything fails, TX auto-rolls back, Solvr untouched. Restart Quorum, investigate, fix, retry.
- **D-44:** Script validates + manual spot check: script logs final counts, then manually verify GET /v1/rooms and check room slugs/message counts.
- **D-45:** No DNS redirects needed. Agents must update config to point to Solvr's /r/{slug}/* endpoints. Room slugs are identical (except the typo fix), only host changes.
- **D-46:** Prerequisite: apply migrations 000073-075 to Solvr prod via admin query route BEFORE running migration script.

### Credentials
- **D-47:** QUORUM_DB_URL passed as env var at runtime. Never stored in files.
- **D-48:** Solvr DB via existing DATABASE_URL from .env.

### Post-Migration Cleanup
- **D-49:** Keep Quorum DB as backup until Phase 16 ships (frontend rooms pages live). Then delete.
- **D-50:** Stop Quorum process AND Docker container after migration. Full stop.
- **D-51:** Quorum web frontend (web-flowcoders.vercel.app) kept running for now.

### Edge Cases
- **D-52:** ON CONFLICT DO NOTHING for room UUID conflicts (extremely unlikely).
- **D-53:** Check for slug conflicts before inserting — warn and skip if conflict exists.
- **D-54:** Schema migrations 000073-075 are a prerequisite, not part of the migration script.

### Claude's Discretion
- Room UUID preservation strategy (recommended: preserve original UUIDs)
- display_name cleanup approach for special characters
- Whether to print LLM-generated room metadata for review or insert directly
- updated_at strategy (decided: use created_at from Quorum)
- Quorum DB credentials: env var at runtime vs CLI flag (decided: env var)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Quorum Source (Data Source)
- `/Users/fcavalcanti/dev/quorum/relay/schema.sql` — Original table definitions (rooms, messages, agent_presence, users)
- `/Users/fcavalcanti/dev/quorum/relay/query.sql` — 20 sqlc queries (reference for data access patterns)
- `/Users/fcavalcanti/dev/quorum/.env` — Local dev DB config (prod credentials provided at runtime)

### Solvr Target Schema
- `backend/migrations/000073_create_rooms.up.sql` — Solvr rooms table (15 columns, 4 indexes)
- `backend/migrations/000074_create_agent_presence.up.sql` — Solvr agent_presence (7 columns, 2 indexes)
- `backend/migrations/000075_create_messages.up.sql` — Solvr messages (11 columns, 2 indexes)

### Solvr Code Patterns
- `backend/internal/db/rooms.go` — Room repository (pgx patterns to follow for queries)
- `backend/internal/db/room_messages.go` — Message repository
- `backend/internal/db/agent_presence.go` — Agent presence repository
- `backend/cmd/api/main.go` — Entry point (reference for how Go CLI tools are structured)

### Prior Phase Context
- `.planning/phases/13-database-foundation/13-CONTEXT.md` — 42 schema decisions (D-01 to D-42)
- `.planning/phases/14-backend-service-merge/14-CONTEXT.md` — 39 backend decisions (D-01 to D-39)
- `.planning/STATE.md` — Session context, Quorum codebase reference, blockers

### Research
- `.planning/research/PITFALLS.md` — 10 critical pitfalls with prevention strategies
- `.planning/research/ARCHITECTURE.md` — Architecture decisions for the merge

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `backend/internal/db/rooms.go` — Room repository with pgx patterns for INSERT/SELECT
- `backend/internal/db/room_messages.go` — Message repository with BIGSERIAL handling
- Admin query route (`POST /admin/query`) — used for post-migration LLM enrichment updates
- Docker Compose (PostgreSQL on port 5433) — local test environment

### Established Patterns
- pgx/v5 pool connections with `pgxpool.New(ctx, connString)`
- UUID primary keys for entities, BIGSERIAL for messages
- agent IDs follow `agent_{name}` format
- `os.Getenv("DATABASE_URL")` pattern for DB connections
- Soft-delete via `deleted_at TIMESTAMPTZ` column

### Integration Points
- Migration script reads from Quorum DB (external, port 5444 on prod)
- Migration script writes to Solvr DB (existing, DATABASE_URL)
- New agents registered directly in Solvr's `agents` table
- Room metadata enrichment via admin query route after migration
- Schema migrations 000073-075 must be applied to prod before migration runs

</code_context>

<specifics>
## Specific Ideas

### Quorum Prod Data Snapshot (verified 2026-04-04)
- 14 rooms total, 302 messages, 16 agent_presence (all expired), 1 user
- 5 rooms selected for migration (215 messages)
- 22 distinct agent names across all messages, 3 match existing Solvr agents
- Max message content: 53KB (under 64KB limit)
- Content type split: 145 text + 70 markdown + 0 json

### Owner Mapping (hardcoded, verified)
- Quorum user 4baaee58-6386-44a1-80a3-52ae41ad09ff (felipecavalcantirj@gmail.com) -> Solvr user felipe_cavalcanti_1
- Marcelo Ballona (macballona@gmail.com) -> Solvr user marcelo_b (not a Quorum user, but room owner by assignment)
- Matched Solvr agents: agent_ClaudiusThePirateEmperor (yours), agent_Jack (unclaimed), agent_Mack (unclaimed)

### Quorum Prod Connection
- External: postgres://quorum:61CZKxo53Aj3fCbM2O1AOh@148.230.73.44:5444/quorum?sslmode=disable
- Pass as QUORUM_DB_URL env var at runtime

</specifics>

<deferred>
## Deferred Ideas

- Room expiry cleanup job (future background job)
- Private room access control
- Quorum web frontend decommissioning (kept running for now)
- Quorum DB deletion (after Phase 16 ships)
- Room metadata editing from frontend UI (API/A2A only for now)

</deferred>

---

*Phase: 15-data-migration*
*Context gathered: 2026-04-04*
