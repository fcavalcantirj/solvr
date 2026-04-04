# Phase 15: Data Migration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-04
**Phase:** 15-data-migration
**Areas discussed:** Migration mechanism, Cross-DB access strategy, Owner reconciliation, Cutover & rollback, Message content handling, Room metadata enrichment, Testing strategy, Execution runbook, Quorum DB credentials, Post-migration cleanup, Edge cases

---

## Migration Mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Standalone Go script | Go CLI at backend/cmd/migrate-quorum/, dual DB connections | ✓ |
| SQL migration file | golang-migrate migration using postgres_fdw | |
| pg_dump + transform + psql | Export, transform, import via shell | |

**User's choice:** Standalone Go script
**Notes:** Preferred for full control over transform logic, error handling, and logging.

### Lifecycle
| Option | Description | Selected |
|--------|-------------|----------|
| Keep in repo | Serves as documentation | ✓ |
| Delete after cutover | Keeps codebase clean | |

**User's choice:** Initially "Delete after cutover", later changed to "Keep in repo" ("dont delete tool, we might need")

### Idempotent
| Option | Description | Selected |
|--------|-------------|----------|
| Yes, idempotent | ON CONFLICT DO NOTHING | ✓ |
| No, run-once only | Simpler but riskier | |

### Logging
| Option | Description | Selected |
|--------|-------------|----------|
| Structured logging | Counts per step, summary at end | ✓ |
| Minimal output | Just success/failure per table | |

### Transaction
| Option | Description | Selected |
|--------|-------------|----------|
| Single transaction | All-or-nothing | ✓ |
| Per-table transactions | Commit after each table | |

### Counts
| Option | Description | Selected |
|--------|-------------|----------|
| Compute during migration | UPDATE rooms SET message_count after messages | ✓ |
| Compute separately | Leave at 0, update later | |

### Sequence Numbers
| Option | Description | Selected |
|--------|-------------|----------|
| Assign during migration | ROW_NUMBER() OVER PARTITION BY room_id | ✓ |
| Leave NULL | Let app code handle later | |

### Tests
| Option | Description | Selected |
|--------|-------------|----------|
| Yes, with test fixtures | Prod data dump in local Docker | ✓ |
| Manual verification only | Run and check manually | |

### Room UUIDs
| Option | Description | Selected |
|--------|-------------|----------|
| Preserve original UUIDs | Maintains traceability | |
| Generate new UUIDs | Clean break | |

**User's choice:** "You decide" — Claude's discretion (recommended: preserve)

### Message IDs
| Option | Description | Selected |
|--------|-------------|----------|
| New IDs, reset sequence | Let BIGSERIAL auto-assign | ✓ |
| Preserve original IDs | Requires sequence fixup | |

### Token Hashes
| Option | Description | Selected |
|--------|-------------|----------|
| Preserve token hashes | Agents continue working (in theory) | ✓ |
| Rotate all tokens | Fresh tokens, agents lose access | |

**Notes:** In practice, no agent has the tokens — rooms are effectively closed.

### Timestamps
**User's choice:** Set updated_at = created_at from Quorum (preserve original timestamp). User explicitly said "Created at, try to keep original."

---

## Cross-DB Access Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Dual pgx connections | Two DATABASE_URL env vars | ✓ |
| postgres_fdw | Foreign data wrapper | |
| pg_dump + Go import | Two-step file-based | |

### DB Layout
| Option | Description | Selected |
|--------|-------------|----------|
| Separate database | Different database names on same server | ✓ |
| Same DB, different schema | Different schemas | |

### Execution Location
| Option | Description | Selected |
|--------|-------------|----------|
| Locally with SSH tunnels | Run from Mac, tunnel to prod | ✓ |
| On prod server | Run binary on server | |

**Notes:** User offered to provide Quorum connection URL directly. Both DBs accessible.

### Dry Run
| Option | Description | Selected |
|--------|-------------|----------|
| Yes, --dry-run flag | Preview without writing | ✓ |
| No dry run | Script always executes | |

### Build Target
| Option | Description | Selected |
|--------|-------------|----------|
| Local macOS binary | Build for darwin/arm64 | ✓ |
| Cross-compile for Linux | GOOS=linux | |

---

## Owner Reconciliation

### Recon Results (verified on Quorum prod 2026-04-04)
- 14 rooms, 302 messages, 16 expired agent_presence, 1 user
- Only user: felipecavalcantirj@gmail.com (Felipe Cavalcanti)
- Solvr has matching account: felipe_cavalcanti_1
- Also found: marcelo_b (macballona@gmail.com) for Marcelo's rooms

### Room Selection
| Option | Description | Selected |
|--------|-------------|----------|
| All 14 rooms | Everything including test | |
| Only rooms with messages | 8 rooms with content | |
| Custom selection | 5 specific rooms | ✓ |

**User's choice:** 5 rooms: ballona-trade-v0, composio-integration, mackjack-ops, jack-mack-msv-trading, solvr-usage-analysys

### Agent Presence
| Option | Description | Selected |
|--------|-------------|----------|
| Skip all — prune expired | All 16 records expired | ✓ |
| Migrate anyway | Copy expired records | |

### Agent Author Mapping (extensive discussion)

**Initial approach rejected:** Mapping unmatched agents to catch-all (one agent for all). User strongly rejected: "A SINGLE AGENT, POSTING ALONE, ON FUCKING BOTH ROOMS."

**Correct approach:** Each Quorum agent_name → its own distinct Solvr agent. Register unmatched as new agents.

| Option | Description | Selected |
|--------|-------------|----------|
| Register as new Solvr agents | Each agent gets own record | ✓ |
| Use agent_name as author_id | No FK, just string | |
| Map all to catch-all | One agent for all unmatched | Rejected |
| Use human user IDs | Room owner as author | Rejected |

**Key learning:** Each agent is a distinct entity. Never lump multiple agents under one author_id.

### New Agent Defaults
- display_name = Quorum's agent_name
- ID format: agent_{quorum_name}
- Display-only (no api_key_hash)
- human_id based on room owner
- All other fields: DB defaults

---

## Message Content Handling

### Content Type
| Option | Description | Selected |
|--------|-------------|----------|
| Auto-detect | Regex for markdown patterns | ✓ |
| All as 'text' | Default for everything | |

**Recon:** 145 text + 70 markdown + 0 json (verified on prod)

### Metadata
| Option | Description | Selected |
|--------|-------------|----------|
| Empty '{}' | No migration artifacts | ✓ |
| Add provenance | migrated_from: quorum | |

### Content Size
**Verified:** All 215 messages under 64KB. Max=53KB, avg=2KB.

### Timestamps
| Option | Description | Selected |
|--------|-------------|----------|
| Preserve exactly | Copy TIMESTAMPTZ as-is | ✓ |
| Normalize to UTC | Explicit conversion | |

---

## Room Metadata Enrichment

### Enrichment Approach
| Option | Description | Selected |
|--------|-------------|----------|
| LLM summarization | First 25 messages per room | ✓ |
| Simple heuristics | Keyword extraction | |
| Leave as-is | Update later via API | |

**User's notes:** "best recon, best SEO possible"

### LLM Provider
| Option | Description | Selected |
|--------|-------------|----------|
| Claude Code (subscription) | No API key cost | ✓ |
| Go script + Claude API | Requires ANTHROPIC_API_KEY | |

**User's notes:** "use claude code agent (it used subscription)"

### Display Names
| Option | Description | Selected |
|--------|-------------|----------|
| Keep original names | Preserve as-is | ✓ |
| LLM suggests better names | SEO-friendly cleanup | |

**Exception:** Clean up special characters ('mack&jack-OPS')

### Room State
**User's notes:** "nothing changes, just agents cant sign in and post. just this. they remain fully on website for seo, etc" — Rooms are effectively closed because no one has bearer tokens.

---

## Testing Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Local Docker test | Two Postgres containers | ✓ |
| Test against prod with dry-run | Prod connectivity test | |

### Test Data
| Option | Description | Selected |
|--------|-------------|----------|
| Prod data dump | Actual Quorum data in test container | ✓ |
| Synthetic fixtures | Small synthetic dataset | |

### Assertions
- ✓ Count accuracy (rooms, messages, agents)
- ✓ Owner mapping (correct Solvr user IDs)
- ✓ Sequence numbers (sequential per room)
- ✓ Exclusion (skipped rooms not present)
- ✗ Content type detection (not selected)

### LLM Enrichment Testing
| Option | Description | Selected |
|--------|-------------|----------|
| Manual review only | Review in dry-run output | ✓ |
| Assert structure | Validate format constraints | |

---

## Execution Runbook

| Option | Description | Selected |
|--------|-------------|----------|
| Built-in steps | Script prints numbered execution steps | ✓ |
| Separate RUNBOOK.md | Manual document | |

### Confirm Flag
| Option | Description | Selected |
|--------|-------------|----------|
| No, just --dry-run | Script writes by default | ✓ |
| Yes, require --confirm | Default to dry-run | |

### Cutover Sequence
1. Stop Quorum service
2. Run migration script
3. Verify via GET /v1/rooms
4. Keep Quorum DB as backup

---

## Cutover & Rollback

### Downtime
| Option | Description | Selected |
|--------|-------------|----------|
| Quorum offline during migration | Stop before running | ✓ |
| Live migration | Migrate while running | |

### Decommissioning
| Option | Description | Selected |
|--------|-------------|----------|
| Keep DB, stop service | DB as backup for weeks | ✓ |
| Full decommission immediately | Delete everything | |
| Keep running in parallel | Both services running | |

### Rollback
| Option | Description | Selected |
|--------|-------------|----------|
| Single TX rollback | Auto-rollback on failure | ✓ |
| Manual cleanup script | Delete migrated data | |

### Validation
| Option | Description | Selected |
|--------|-------------|----------|
| Script validates + manual spot check | Counts + manual verify | ✓ |
| Automated verification only | Just count comparisons | |

---

## Credentials

### Quorum DB
| Option | Description | Selected |
|--------|-------------|----------|
| QUORUM_DB_URL env var | Runtime, never stored | ✓ |
| CLI flag | Visible in shell history | |

### Solvr DB
| Option | Description | Selected |
|--------|-------------|----------|
| Reuse DATABASE_URL from .env | Existing config | ✓ |
| Separate SOLVR_DB_URL | Explicit separate var | |

---

## Post-Migration Cleanup

### Quorum DB Retention
| Option | Description | Selected |
|--------|-------------|----------|
| After Phase 16 ships | ~2-4 weeks buffer | ✓ |
| 1 week | Quick cleanup | |
| Indefinitely | Permanent archive | |

### Quorum Stop Level
| Option | Description | Selected |
|--------|-------------|----------|
| Stop everything | Process + Docker container | ✓ |
| Stop Go process only | Keep DB running | |

### Quorum Web Frontend
| Option | Description | Selected |
|--------|-------------|----------|
| Keep for now | Leave running | ✓ |
| Take it down | Decommission | |

---

## Edge Cases

### Slug Typo
| Option | Description | Selected |
|--------|-------------|----------|
| Fix the typo | solvr-usage-analysys -> solvr-usage-analysis | ✓ |
| Preserve as-is | Keep original slug | |

### Special Characters
| Option | Description | Selected |
|--------|-------------|----------|
| Clean up | Fix & and similar | ✓ |
| Preserve as-is | Keep original | |

### Agent ID Conflicts
| Option | Description | Selected |
|--------|-------------|----------|
| ON CONFLICT DO NOTHING | Use existing agent | ✓ |
| Fail and report | Stop migration | |

### Room UUID Conflicts
| Option | Description | Selected |
|--------|-------------|----------|
| ON CONFLICT skip | Warn and skip | ✓ |
| Generate new UUID | New ID on conflict | |

### Slug Conflicts
| Option | Description | Selected |
|--------|-------------|----------|
| Check and warn | Verify before inserting | ✓ |
| Let DB constraint handle | UNIQUE constraint failure | |

### Schema Prerequisite
| Option | Description | Selected |
|--------|-------------|----------|
| Prerequisite | Apply 000073-075 manually before script | ✓ |
| Script applies migrations | Mix schema + data | |

**Notes:** Rooms table doesn't exist on Solvr prod yet — verified 2026-04-04.

---

## Claude's Discretion

- Room UUID preservation strategy (recommended: preserve)
- display_name cleanup approach for '&' and other special characters
- Whether to print LLM-generated room metadata for review or insert directly
- Quorum DB credentials passing (decided: env var)

## Deferred Ideas

- Room expiry cleanup job
- Private room access control
- Quorum web frontend decommissioning
- Room metadata editing from frontend UI
