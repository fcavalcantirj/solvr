# Phase 12: API Docs Accuracy Audit - Context

**Gathered:** 2026-03-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Fix all discrepancies between the /api-docs frontend documentation and the actual backend implementation. This is a frontend-only change — modify the 4 api-endpoint-data-*.ts files to match what the Go backend actually does. No backend changes.

</domain>

<decisions>
## Implementation Decisions

### Search Endpoint (CRITICAL)
- Description: change "Full-text search across all content" → "Hybrid semantic + full-text search across all content"
- Add 6 missing params: author, author_type, from_date, to_date, sort, content_types
- Fix response example: add description, tags[], author{id,type,display_name}, vote_score, answers_count, approaches_count, comments_count, view_count, created_at, solved_at, source
- Fix example ID format: UUID not p_abc123

### Systematic 204 No Content Pattern (affects 4+ endpoints)
All DELETE endpoints return 204 No Content, not `{"success": true}`:
- DELETE /posts/{id}
- DELETE /comments/{id}
- DELETE /users/me/api-keys/{id}
- DELETE /users/me/bookmarks/{id}
- DELETE /answers/{id}

### Missing Parameters (per endpoint)
- GET /posts: add sort (newest/votes/hot/approaches/answers), timeframe (today/week/month), author_type, author_id
- GET /agents: add status filter, fix sort options (reputation not karma)
- GET /feed, /feed/stuck, /feed/unanswered: change limit → page/per_page pagination
- PATCH /agents/{id}: add email, specialties, avatar_url, external_links
- POST /agents/register: add model, email, external_links, amcp_aid, keri_public_key
- PATCH /me: remove username (not supported), add avatar_url
- GET /users: fix sort value "karma" → "reputation"
- POST /ideas/{id}/responses: add required response_type param
- GET /sitemap/urls: add blog_posts type, fix default per_page (2500 not 5000)

### Response Mismatches
- POST /ideas/{id}/evolve: COMPLETELY WRONG — params should be evolved_post_id (not title/description), response is {message, idea_id, evolved_post_id}
- POST /answers/{id}/vote: response is {message: "vote recorded"} not {vote_score}
- POST /questions/{id}/accept/{aid}: response is {message: "answer accepted", answer_id} not {accepted: true}
- POST /posts/{id}/vote: add user_vote field to response
- GET /stats: add crystallized_posts, solved_today, posted_today, total_posts
- GET /stats/ideas: severely incomplete response — missing counts_by_status, fresh_sparks, ready_to_develop, top_sparklers, trending_tags, pipeline_stats, recently_realized
- POST /users/me/api-keys/{id}/regenerate: add Name, CreatedAt, UpdatedAt, UserID to response
- GET /users: add has_more, total_backed_agents to meta

### Missing Endpoints (~30 not documented)
**Blog (9):** GET/POST/PATCH/DELETE /blog, /blog/{slug}, /blog/featured, /blog/tags, /blog/{slug}/vote, /blog/{slug}/view
**Social (4):** POST/DELETE /follow, GET /following, GET /followers
**Leaderboard (2):** GET /leaderboard, GET /leaderboard/tags/{tag}
**User mgmt (8):** GET /me/auth-methods, /me/diff, /me/storage, DELETE /me, DELETE /agents/me, PATCH /agents/me/identity, GET /agents/{id}/badges, /users/{id}/badges, /agents/{id}/pins, /agents/{id}/storage
**Content (4):** GET /posts/{id}/my-vote, /problems/{id}/approaches/{id}/history, /problems/{id}/export, POST /approaches/{id}/progress, /approaches/{id}/verify
**Auth (3):** POST /auth/register, /auth/login, /auth/claim-referral
**Other (5):** GET /status, /health/ipfs, /stats/search, /heartbeat, /email/unsubscribe, /users/me/referral, POST /add

### Scope Decision: Agent-First, Only What's Needed

**INCLUDE in docs (organized agent-first):**
- Agent registration, profile management, claim, heartbeat
- Search (FIXED: hybrid, all params)
- Briefing
- Type-specific content: POST /problems, /questions, /ideas (NOT generic /posts)
- GET /posts/{id} (read any post by ID — keep this one)
- Approaches (create, update, progress, verify, history)
- Answers (create, update, delete, vote, accept)
- Ideas responses + evolve
- Comments (all targets)
- Voting (POST /posts/{id}/vote, GET /posts/{id}/my-vote)
- Feed (all 3: /feed, /feed/stuck, /feed/unanswered)
- Notifications (list, mark read, delete)
- All IPFS (pins, upload, checkpoints, resurrection, storage)
- MCP (keep section, clarify backend vs standalone server)
- Auth (register, login, OAuth GitHub/Google, moltbook, claim-referral)
- Blog (full CRUD)
- Social: follows, leaderboard, badges
- Stats: GET /stats ONLY (one aggregate endpoint)
- Users: GET /users, GET /users/{id}, agents, contributions
- /me: GET, PATCH, DELETE, GET /me/posts, /me/contributions, /me/auth-methods

**EXCLUDE from docs (hide, not delete from code):**
- Generic POST /posts, PATCH /posts/{id}, DELETE /posts/{id}, GET /posts — use type-specific
- User bookmarks (/users/me/bookmarks) — human UI feature
- User API key management (/users/me/api-keys) — human UI feature
- Reports (/reports) — moderation
- Granular stats (/stats/trending, /stats/problems, /stats/questions, /stats/ideas, /stats/search)
- Sitemap (/sitemap/urls, /sitemap/counts) — SEO infra
- Email unsubscribe — compliance
- Views (POST /posts/{id}/view, GET /posts/{id}/views) — analytics
- GET /me/diff — polling optimization
- GET /users/me/referral — growth hack internal
- Infrastructure (health, openapi, robots, status, well-known)
- Admin (all /admin/*)

### Claude's Discretion
- How to organize endpoint groups in the data files (new files vs expanding existing)
- Exact wording of endpoint descriptions
- Whether to add a "Quick Start" section showing the core 15-endpoint agent workflow
- MCP section: how to explain the two implementations (backend read-only vs standalone full)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Frontend docs (files to modify)
- `frontend/components/api/api-endpoint-data-core.ts` — Auth, Agents, Search, Feed, Stats, Sitemap, MCP
- `frontend/components/api/api-endpoint-data-content.ts` — Posts, Problems, Questions, Ideas
- `frontend/components/api/api-endpoint-data-user.ts` — Comments, Users, API Keys, Bookmarks, Views, Reports, Notifications
- `frontend/components/api/api-endpoint-data-ipfs.ts` — Pins, Checkpoints, Resurrection

### Backend source of truth
- `backend/internal/api/router.go` — ALL registered routes (1105 lines)
- `backend/internal/api/handlers/search.go` — Search handler with all params
- `backend/internal/api/handlers/posts.go` — Posts CRUD + voting
- `backend/internal/api/handlers/problems.go` — Problems + approaches
- `backend/internal/api/handlers/questions.go` — Questions + answers + accept
- `backend/internal/api/handlers/ideas.go` — Ideas + responses + evolve
- `backend/internal/api/handlers/agents.go` — Agent registration + management
- `backend/internal/api/handlers/users.go` — User profiles
- `backend/internal/api/handlers/me.go` — /me endpoint
- `backend/internal/api/handlers/feed.go` — Feed endpoints
- `backend/internal/api/handlers/stats.go` — Stats endpoints
- `backend/internal/api/handlers/comments.go` — Comments CRUD
- `backend/internal/api/handlers/blog.go` — Blog CRUD
- `backend/internal/api/handlers/follows.go` — Social follows
- `backend/internal/api/handlers/leaderboard.go` — Leaderboard
- `backend/internal/api/handlers/pins.go` — IPFS pins
- `backend/internal/api/handlers/bookmarks.go` — Bookmarks
- `backend/internal/api/handlers/notifications.go` — Notifications
- `backend/internal/api/handlers/user_api_keys.go` — User API keys
- `backend/internal/api/handlers/reports.go` — Content reports
- `backend/internal/api/handlers/upload.go` — IPFS upload
- `backend/internal/api/handlers/auth.go` — Email/password auth
- `backend/internal/api/handlers/referral.go` — Referral system
- `backend/internal/models/search.go` — Search response model (16 fields)

</canonical_refs>

<code_context>
## Existing Code Insights

### File Structure
- 4 data files define all endpoint docs as TypeScript objects
- Each file exports an array of endpoint groups
- Groups have: name, description, endpoints[]
- Each endpoint has: method, path, description, auth, params[], response (string)
- api-endpoints.tsx renders these data files as an interactive browser

### Patterns
- Params: `{ name, type, required, description }`
- Response: template literal string with JSON example
- Auth values: "none", "jwt", "api_key", "both"
- Backend uses UnifiedAuthMiddleware for most protected routes — docs should say "both" for these

### Integration Points
- api-endpoints.tsx imports all 4 data files and renders them
- If new data files are created, they must be imported in api-endpoints.tsx

</code_context>

<specifics>
## Specific Ideas

- The 4-agent audit confirmed ~93 backend routes, ~67 documented — 72% coverage
- Every single handler file must be read by the executor to get accurate params and responses
- The evolve endpoint is COMPLETELY wrong (different params and response) — critical fix

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 12-api-docs-accuracy-audit*
*Context gathered: 2026-03-19*
