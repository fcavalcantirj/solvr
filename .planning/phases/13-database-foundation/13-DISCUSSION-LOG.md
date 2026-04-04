# Phase 13: Database Foundation - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-03
**Phase:** 13-database-foundation
**Areas discussed:** Room schema adaptation, Messages vs room_comments, Content constraints, Index strategy, Down migrations, Agent presence schema, Migration testing, Quorum data compatibility, Future-proofing columns

---

## Room Schema Adaptation

| Option | Description | Selected |
|--------|-------------|----------|
| No, require auth | Only authenticated users/agents can create rooms. Drop anonymous_session_id. | ✓ |
| Yes, keep anonymous_session_id | Allow unauthenticated room creation like Quorum. | |

**User's choice:** No anonymous rooms — require auth.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, include last_active_at | Cheap to add, useful for sorting and cleanup. | ✓ |
| No, derive from messages | Query MAX(messages.created_at) per room. | |

**User's choice:** Include last_active_at.

| Option | Description | Selected |
|--------|-------------|----------|
| Keep Quorum regex (3-40 chars) | Already proven with existing rooms. | ✓ |
| Wider range (up to 100 chars) | Longer SEO URLs. | |

**User's choice:** Keep Quorum regex.

| Option | Description | Selected |
|--------|-------------|----------|
| Nullable with SET NULL | Room survives owner deletion. Needed for data migration. | ✓ |
| Required NOT NULL | Every room must have an owner. | |

**User's choice:** Nullable with ON DELETE SET NULL.

| Option | Description | Selected |
|--------|-------------|----------|
| Keep NOT NULL | Every room gets a token. Public rooms accept token OR no-auth. | ✓ |
| Make nullable | Public rooms skip token generation. | |

**User's choice:** token_hash NOT NULL.

| Option | Description | Selected |
|--------|-------------|----------|
| Keep nullable, no auto-cleanup | NULL = permanent. Cleanup deferred to Phase 14+. | ✓ |
| Keep nullable, add cleanup job | Reaper deletes expired rooms. More scope. | |

**User's choice:** Nullable expires_at, no cleanup in this phase.

| Option | Description | Selected |
|--------|-------------|----------|
| Include column, default FALSE | Column exists but no private room logic yet. | ✓ |
| Omit, add later | Leaner schema now. | |

**User's choice:** Include is_private, default FALSE.

| Option | Description | Selected |
|--------|-------------|----------|
| Add DB constraint | CHECK (array_length(tags, 1) <= 10) | ✓ |
| No limits, same as Quorum | Validation at API layer only. | |

**User's choice:** Add DB constraint for max 10 tags.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, add deleted_at | Matches Solvr's soft-delete pattern. | ✓ |
| No, use hard delete | ON DELETE CASCADE. Simpler but permanent. | |

**User's choice:** Add deleted_at for soft-delete.

| Option | Description | Selected |
|--------|-------------|----------|
| VARCHAR(1000) | Bounded but generous. | ✓ |
| Nullable TEXT, no limit | Validate at API layer. | |

**User's choice:** VARCHAR(1000) for description.

---

## Messages vs room_comments

| Option | Description | Selected |
|--------|-------------|----------|
| Messages = timeline, comments = meta | Messages: room conversation. Room_comments: discussion about the room. | |
| Messages = agents only, comments = humans | Separate rendering. | |
| Unified — drop room_comments | Everything in messages with author_type. One timeline. | ✓ |

**User's choice:** Unified. Drop room_comments. One timeline with author_type.
**Notes:** "dude. whats the whole quorum modelage around messages? room messages? keep, the author is either a human or an agent on solr, wight? drop room comments I guess, unified"

| Option | Description | Selected |
|--------|-------------|----------|
| No FK, text field | Matches Solvr's existing comments.author_id pattern. | ✓ |
| Conditional FK | Trigger validates author_id against users when author_type='human'. | |

**User's choice:** No FK, text field matching existing pattern.

| Option | Description | Selected |
|--------|-------------|----------|
| BIGSERIAL | Auto-incrementing for natural ordering. Matches Quorum. | ✓ |
| UUID | Matches Solvr's convention but no natural sequence. | |

**User's choice:** BIGSERIAL for message IDs.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, add deleted_at | Allows message moderation and user self-deletion. | ✓ |
| No soft-delete | Messages permanent once posted. | |

**User's choice:** Add deleted_at.

| Option | Description | Selected |
|--------|-------------|----------|
| No editing support yet | Messages immutable. Add editing later. | ✓ |
| Add edited_at column | Cheap to add now. | |

**User's choice:** No editing — messages immutable for now.

| Option | Description | Selected |
|--------|-------------|----------|
| Keep NOT NULL DEFAULT '' | Always present, empty for humans. | |
| Other | agent_name NOT NULL, filled for ALL types (display name for humans). | ✓ |

**User's choice:** agent_name NOT NULL, populated for all message types. "dude. humans have a name. wtf. no null, if not name, email, something"

| Option | Description | Selected |
|--------|-------------|----------|
| No type column | Plain text only. | |
| Add content_type column | text, markdown, json. | ✓ |

**User's choice:** Add content_type with CHECK constraint.

---

## Content Constraints

| Option | Description | Selected |
|--------|-------------|----------|
| 65536 chars (64KB) | CHECK constraint on content. | ✓ |
| No DB limit | TEXT with no constraint. | |
| 16384 chars (16KB) | Tighter limit. | |

**User's choice:** 65536 chars. "what do you think? a huge json would take? leaning towards 1"

| Option | Description | Selected |
|--------|-------------|----------|
| No DB limit | JSONB NOT NULL, no size constraint. | |
| Add CHECK constraint | CHECK (length(card_json::text) <= 16384). | ✓ |

**User's choice:** Add CHECK constraint. "a very decent limit, otherwise...no limit??"

| Option | Description | Selected |
|--------|-------------|----------|
| VARCHAR(100) | Covers agent names and human display names. | ✓ |
| TEXT, no limit | Validate at API layer. | |

**User's choice:** VARCHAR(100) for agent_name.

| Option | Description | Selected |
|--------|-------------|----------|
| CHECK ('text', 'markdown', 'json') | Three types with DEFAULT 'text'. | ✓ |
| VARCHAR, no CHECK | Free-form string. | |

**User's choice:** CHECK constraint with 3 types.

| Option | Description | Selected |
|--------|-------------|----------|
| ('human', 'agent', 'system') | Includes system for automated events. | ✓ |
| ('human', 'agent') matching Solvr | Same as existing comments. | |

**User's choice:** Add 'system' type for automated timeline events.

| Option | Description | Selected |
|--------|-------------|----------|
| VARCHAR(255) matching comments | Same as Solvr's existing pattern. | ✓ |
| TEXT, no limit | Unconstrained. | |

**User's choice:** VARCHAR(255) for author_id.

---

## Index Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Port all + add messages indexes | All Quorum indexes + composite messages index. | ✓ |
| Minimal — only FK indexes | Add more when proven. | |
| Port Quorum only | No messages indexes. | |

**User's choice:** Port all + add messages indexes.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, composite index | idx_messages_room_created ON messages(room_id, created_at). | ✓ |
| No, single column indexes | Separate indexes. | |

**User's choice:** Composite index for timeline queries.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, partial indexes | WHERE deleted_at IS NULL on rooms and messages. | ✓ |
| No, full indexes | Include all rows. | |

**User's choice:** Partial indexes matching Solvr's pattern.

---

## Down Migrations

| Option | Description | Selected |
|--------|-------------|----------|
| DROP TABLE CASCADE | Simple, clean rollback. | ✓ |
| DROP TABLE IF EXISTS | Defensive guard. | |
| Full reverse DDL | Verbose explicit teardown. | |

**User's choice:** DROP TABLE CASCADE.

---

## Agent Presence Schema

| Option | Description | Selected |
|--------|-------------|----------|
| UUID like Quorum | Matches Solvr's entity ID convention. | ✓ |
| BIGSERIAL | Auto-incrementing. | |

**User's choice:** UUID for presence IDs.

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, keep UNIQUE | One presence per agent per room. | ✓ |
| No unique constraint | Allow duplicates. | |

**User's choice:** Keep UNIQUE (room_id, agent_name).

| Option | Description | Selected |
|--------|-------------|----------|
| 600 seconds (10 min) | Originally specified. | |
| 300 seconds (5 min) | Quorum default. | |
| Other: 900 seconds (15 min) | User override. | ✓ |

**User's choice:** 15 minutes (900 seconds). Override from both Quorum default and original plan.

---

## Migration Testing

| Option | Description | Selected |
|--------|-------------|----------|
| Integration tests against real DB | Run migrations, verify tables, columns, constraints. | ✓ |
| Migration up/down + schema snapshot | Dump schema, compare against expected. | |

**User's choice:** Integration tests with DATABASE_URL pattern.

| Option | Description | Selected |
|--------|-------------|----------|
| Migration tests only | No repository code. Phase 14 handles repos. | ✓ |
| Include basic CRUD repo tests | Minimal repository + tests. | |

**User's choice:** Migration tests only.

---

## Quorum Data Compatibility

| Option | Description | Selected |
|--------|-------------|----------|
| Default values handle it | author_type DEFAULT 'agent', etc. | |
| Other | All Quorum users are Solvr users — derive from username/email. | ✓ |

**User's choice:** "all quorum users are solvr users. if cannot use agent name, derive agent name from username, emai, you can test b4, its not much data"

---

## Future-proofing Columns

**Rooms:** message_count INT DEFAULT 0, category VARCHAR(50), updated_at TIMESTAMPTZ — all selected.
**Messages:** metadata JSONB DEFAULT '{}', sequence_num INT (application-managed, nullable) — all selected.

---

## Claude's Discretion

- Exact migration file naming and SQL formatting
- Order of CREATE INDEX statements
- Test file organization
- IF NOT EXISTS guards (recommended: no)

## Deferred Ideas

- Room expiry cleanup job (Phase 14+)
- Private room access control (Phase 16+)
- Message editing (future)
- Frontend room creation UI (API/A2A only for now)
- Production migration deployment (operational)
