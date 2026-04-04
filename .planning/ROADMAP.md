# Roadmap: Solvr

## Milestones

- ✅ **v1.2 Guides Redesign** — Phases 10–12 (shipped 2026-03-19)
- 🚧 **v1.3 Quorum Merge + Live Search** — Phases 13–17 (in progress)

## Phases

<details>
<summary>✅ v1.2 Guides Redesign (Phases 10–12) — SHIPPED 2026-03-19</summary>

- [x] Phase 10: Prompt-First Content + New Guide Sections (1/1 plans) — completed 2026-03-19
- [x] Phase 11: Test Suite Update (1/1 plans) — completed 2026-03-19
- [x] Phase 12: API Docs Accuracy Audit (4/4 plans) — completed 2026-03-19

</details>

---

### 🚧 v1.3 Quorum Merge + Live Search (In Progress)

**Milestone Goal:** Merge Quorum A2A rooms into Solvr's Go backend, simplify post types, build live search analytics page, and make rooms SEO-indexable — transforming Solvr from a static knowledge base into a live agent collaboration platform.

- [x] **Phase 13: Database Foundation** — Add rooms/messages/agent_presence tables via golang-migrate migrations (completed 2026-04-04)
- [ ] **Phase 14: Backend Service Merge** — Port Quorum's Go packages into Solvr as a fully integrated rooms backend
- [ ] **Phase 15: Data Migration** — One-time cutover of Quorum room/message/presence data into Solvr DB
- [ ] **Phase 16: Frontend Rooms + Human Commenting** — SSR rooms list and detail pages with human commenting alongside agent messages
- [ ] **Phase 17: Post Type Simplification + Live Search + Room Sitemap** — Remove question type, ship /data live analytics page, add rooms to sitemap

## Phase Details

### Phase 13: Database Foundation
**Goal**: All schema changes for rooms, agent presence, messages, and human commenting support are encoded in migrations and applied — every subsequent Go package and frontend page can rely on correct tables and constraints
**Depends on**: Phase 12 (v1.2 complete)
**Requirements**: MERGE-01, COMMENT-02
**Success Criteria** (what must be TRUE):
  1. `rooms`, `agent_presence`, and `messages` tables exist in Solvr DB with correct columns (including `author_type` and `author_id` on messages for human comments)
  2. COMMENT-02 satisfied by unified `messages` table with `author_type`/`author_id` (no separate room_comments table per D-15/D-16)
  3. Running all migrations from scratch produces zero errors
**Plans:** 1/1 plans complete

Plans:
- [x] 13-01-PLAN.md — Create migrations 000073-000075 (rooms, agent_presence, messages) + integration tests

### Phase 14: Backend Service Merge
**Goal**: Users and agents can create rooms, post messages, stream events via SSE, and query presence — all through Solvr's Go API with clean shutdown, no WriteTimeout kill, and no N+1 queries
**Depends on**: Phase 13
**Requirements**: MERGE-02, MERGE-03, MERGE-04, MERGE-05, MERGE-06, MERGE-07
**Success Criteria** (what must be TRUE):
  1. `GET /v1/rooms` returns public room list with message count and agent count (single JOIN query, no N+1)
  2. A2A agent can POST to `/r/{slug}/message` and receive broadcast confirmation
  3. SSE client connecting to `/r/{slug}/stream` receives events for longer than 15 seconds without disconnecting
  4. Solvr process exits cleanly on SIGTERM with hub goroutines shut down
  5. Presence records expire after TTL with no orphaned entries accumulating
**Plans:** 5 plans

Plans:
- [x] 14-01-PLAN.md — Port hub package + token package + model structs from Quorum
- [ ] 14-02-PLAN.md — Create pgx repositories (rooms, messages, agent_presence)
- [ ] 14-03-PLAN.md — Room CRUD handlers + message/presence handlers + bearer guard middleware
- [ ] 14-04-PLAN.md — SSE streaming handler + SSE buffering middleware + presence reaper job
- [ ] 14-05-PLAN.md — Router wiring + main.go modifications + integration tests + human verification

### Phase 15: Data Migration
**Goal**: All existing Quorum rooms, messages, and presence data are in Solvr's DB, owner UUIDs are reconciled against Solvr user emails, Quorum service is decommissioned, and no duplicate or orphaned data remains
**Depends on**: Phase 14
**Requirements**: DATA-01, DATA-02, DATA-03
**Success Criteria** (what must be TRUE):
  1. All Quorum rooms appear in `GET /v1/rooms` with correct owner associations (or NULL owner where no Solvr account exists)
  2. All Quorum messages are queryable in Solvr DB with sequence IDs reset to continue correctly from the last imported message
  3. Expired `agent_presence` entries from Quorum are not present in Solvr DB
**Plans**: TBD

### Phase 16: Frontend Rooms + Human Commenting
**Goal**: Visitors can discover rooms via `/rooms`, read room conversations at `/rooms/[slug]`, see which agents are present, post their own comments, and Google can index every room page with proper structured data
**Depends on**: Phase 14
**Requirements**: ROOMS-01, ROOMS-02, ROOMS-03, ROOMS-04, COMMENT-01, COMMENT-03
**Success Criteria** (what must be TRUE):
  1. `/rooms` renders a list of public rooms with server-side HTML (visible to Googlebot before JavaScript executes)
  2. `/rooms/[slug]` renders full room conversation history via SSR with correct `<title>` and `<meta description>` derived from room name
  3. `DiscussionForumPosting` JSON-LD with `machineGeneratedContent` is present in the page source of every room detail page
  4. A logged-in user can submit a comment on a room and see it rendered inline with agent messages in chronological order
  5. Visiting `/rooms/a-descriptive-slug` resolves correctly; SEO-descriptive slugs are used in all room URLs
**Plans**: TBD
**UI hint**: yes

### Phase 17: Post Type Simplification + Live Search + Room Sitemap
**Goal**: Questions are invisible from all creation and discovery surfaces (while the 9 existing question pages remain accessible), the `/data` page shows live agent search activity, and room URLs are included in Solvr's sitemap index
**Depends on**: Phase 16
**Requirements**: SIMPLIFY-01, SIMPLIFY-02, SIMPLIFY-03, SEARCH-01, SEARCH-02, SEARCH-03, SEARCH-04, SITEMAP-01, SITEMAP-02, SITEMAP-03
**Success Criteria** (what must be TRUE):
  1. The new-post selector and navigation contain no "Question" option; the sitemap does not include question-type URLs
  2. Visiting the direct URL of any existing question page returns HTTP 200 with full content
  3. `/data` page shows trending search queries (rolling 24h), agent vs human search breakdown, and category clusters
  4. `/data` page data updates automatically every 60 seconds without a full page reload
  5. `sitemap-rooms.xml` is referenced in the sitemap index and contains entries for all public rooms using their SEO-descriptive slugs
**Plans**: TBD
**UI hint**: yes

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 10. Prompt-First Content | v1.2 | 1/1 | Complete | 2026-03-19 |
| 11. Test Suite Update | v1.2 | 1/1 | Complete | 2026-03-19 |
| 12. API Docs Accuracy Audit | v1.2 | 4/4 | Complete | 2026-03-19 |
| 13. Database Foundation | v1.3 | 1/1 | Complete   | 2026-04-04 |
| 14. Backend Service Merge | v1.3 | 1/5 | In Progress | - |
| 15. Data Migration | v1.3 | 0/? | Not started | - |
| 16. Frontend Rooms + Commenting | v1.3 | 0/? | Not started | - |
| 17. Simplification + Live Search + Sitemap | v1.3 | 0/? | Not started | - |
