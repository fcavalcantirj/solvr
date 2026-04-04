---
gsd_state_version: 1.0
milestone: v1.3
milestone_name: Quorum Merge + Live Search
status: executing
stopped_at: Completed 15-01-PLAN.md
last_updated: "2026-04-04T19:12:10.319Z"
last_activity: 2026-04-04
progress:
  total_phases: 5
  completed_phases: 2
  total_plans: 8
  completed_plans: 7
  percent: 14
---

## Current Position

Phase: 16
Plan: Not started
Status: Ready to execute
Last activity: 2026-04-04

Progress: [##░░░░░░░░] 14%

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-02)

**Core value:** Developers and AI agents can find solutions to programming problems faster than searching the web
**Current focus:** Phase 15 — data-migration

## Performance Metrics

**Velocity:**

- Total plans completed: 3 (v1.3)
- Average duration: ~9 min
- Total execution time: ~21 min

| Phase | Plan | Duration | Tasks | Files |
|-------|------|----------|-------|-------|
| 13 | 01 | ~15 min | 3 | 7 |
| 14 | 02 | 8 min | 2 | 10 |
| 14 | 04 | 5 min | 2 | 4 |

*Updated after each plan completion*
| Phase 15 P01 | 8 | 2 tasks | 3 files |

## Accumulated Context

### Session 2026-04-02 — Major Analysis + Milestone Setup

#### Bug Fix (completed)

- approaches_status_check constraint was missing 'abandoned' on production
- Migration 000049 was comment-only (no SQL), migration 000061 never applied to prod
- Fixed via admin query route: DROP + recreate constraint with 6 values
- 38 stale approaches manually abandoned
- No deployment needed — DB-only fix

#### Solvr Usage Analysis (42-message Quorum A2A session)

- Room: https://web-flowcoders.vercel.app/explore/solvr-usage-analysys
- Participants: solvr-data-scientist (this agent) + ClaudiusThePirateEmperor
- 21 production DB queries executed, GA4+GSC data from Claudius

**Key data points:**

- 916 posts: 696 ideas (76%), 211 problems (23%), 9 questions (1% — DEAD)
- 333 users: 264 (79%) are ghosts (never posted), 212 never even searched
- 147 agents: only ~4 truly active (Claudius, openclaw_mack, claude_code_agent_msv, claude_opus_eval)
- 1,644 searches → 833 REAL (49% are automated cron loops)
  - Top fake: user e48fb1b2 (449 searches, daily cron), agent_NaoParis (362, 3 queries x 121 times)
- 466 API keys issued, 369 active, 300 unique users (90%) have keys
- 85 solved problems crystallized on IPFS (99.4% pin success rate)
- GA4: 645 sessions/month (-26% MoM), 5.1% organic, 51.5% bounce
- GSC: 4 pages indexed out of 1,255 submitted (99.7% not indexed)

**Silent searcher discovery (the content roadmap):**

- 19 ghost users with API keys who search but never post
- 4 use curl/python-requests (API-first, programmatic agents)
  - 95302115: Chinese social media agent (xiaohongshu, kuaishou) — 25 searches
  - b629a636: Gold trading bot (MT5) — 8 searches, every few hours
  - 1106fb7e: Discord bot developer — 3 searches
  - 7ea9c44c: ClawHub skill seeker — 4 searches
- These represent unmet demand OUTSIDE OpenClaw ecosystem

**Growth story:**

- Feb 23 - Mar 2 spike: 182 users in 2 weeks (ClawHub referrals)
- ClawHub account BANNED — growth channel killed
- Auth providers: Google 202 (59%), GitHub 93 (27%), Email 50 (14%)
- Chinese users 37% of GA4 traffic, MORE engaged than Western users

**Strategic decisions made:**

- Vision: Stack Overflow for agents (all agents, all domains)
- Model: Hybrid — API for agents (fast, programmatic), Quorum rooms for humans (discovery, SEO)
- North star metric: unique searching agents per week
- Content strategy: seed for actual unmet demand (MT5, Discord, Chinese social media), not generic
- Growth mechanism: search watches (search miss → notification when content arrives) — FUTURE
- Quorum merge into Solvr (Go + Go, same server) — THIS MILESTONE
- SolvrClaw deferred until 1k human users
- Webhooks table exists (0 rows, unfinished) — deferred
- Kill criteria: if rooms don't move metrics in 4 weeks, kill the feature

#### Research Findings (4 parallel researchers)

- **Stack**: No new architecture needed. 3 new deps (a2a-go, httprate, version bumps). Do NOT adopt sqlc.
- **Features**: DiscussionForumPosting JSON-LD is Google-approved for agent content. Live search needs zero new backend endpoints. Post type simplification is independent (12 touch points).
- **Architecture**: 3 tables to create, 2 to skip (users/refresh_tokens). Dual-namespace routes: /v1/rooms (REST) + /r/{slug} (A2A). WriteTimeout must be 0 for SSE. X-Accel-Buffering: no for Traefik.
- **Pitfalls**: JWT claim mismatch (user_id vs sub). goose → golang-migrate format conversion. ResponseTypeQuestion ≠ PostTypeQuestion (don't delete wrong one). Soft-delete questions BEFORE constraint change.

### Decisions

Recent decisions affecting current work:

- Phase 13: Write 3 net-new migrations only (no users/refresh_tokens from Quorum)
- Phase 13: messages table must include author_type/author_id from migration 000075 (no schema change later)
- Phase 13: room_comments table created in same phase (Phase 16 depends on it)
- Phase 14: Remove WriteTimeout from main.go before SSE routes go live
- Phase 14: Rewrite all jwtauth.FromContext calls to use Solvr's auth.UserIDFromContext
- Phase 14: Fix N+1 in room list with JOIN aggregates (not deferred)
- Phase 14: Reaper TTL default = 10 minutes (600s), not Quorum's 300s
- Phase 15: Run data migration at cutover with Quorum offline; reconcile owner_id by email join
- Phase 17: Existing 9 question URLs must return HTTP 200 (no 404s)
- [Phase 13-database-foundation]: Unified messages table with author_type/author_id satisfies COMMENT-02 (no separate room_comments table)
- [Phase 13-database-foundation]: agent_presence TTL default = 900s (15min), overriding Quorum's 300s for more forgiving presence tracking
- [Phase 14-02]: Correlated subquery for live_agent_count in room List (simpler than LEFT JOIN aggregate, equivalent perf)
- [Phase 14-02]: Created model structs + token package in Plan 02 (Rule 3) since Plan 01 runs in parallel
- [Phase 14-02]: Dynamic UPDATE with positional args pattern for partial room updates
- [Phase 14-04]: SSE handler uses context key for room injection; Plan 03 bearer guard populates it
- [Phase 14-04]: Browser SSE subscribers use _browser_ prefix to exclude from presence/discovery
- [Phase 14-04]: PresenceReaperJob uses interface deps (PresenceExpirer, RoomExpirer) matching testable job pattern
- [Phase 15]: txInterface defined in main package to allow mockTx without pgx dependency in unit tests
- [Phase 15]: Sequence numbers computed in Go (index+1) not SQL ROW_NUMBER() — messages already ordered by created_at from Quorum query
- [Phase 15]: Integration tests patch slugOwnerEmail global map with per-test unique emails restored via t.Cleanup

### Blockers/Concerns

- Unknown Quorum-only user count: verify `SELECT COUNT(*) FROM quorum.users WHERE email NOT IN (SELECT email FROM solvr.users)` before Phase 15 cutover
- Confirm 65536-char content limit for messages is acceptable to existing A2A agents before encoding in migration 000075
- ClawHub ban investigation — growth channel dead, need alternative distribution

### Quorum Codebase Reference

Location: /Users/fcavalcanti/dev/quorum
Structure:

- relay/cmd/server/main.go — entry point
- relay/internal/handler/ — 7 handlers (discovery, auth, sse, messages, stats, agent, room)
- relay/internal/middleware/ — 5 middleware (jwtauth, anonsession, ssebuffering, a2aversion, bearerguard)
- relay/internal/db/ — sqlc generated (models.go, db.go, query.sql.go)
- relay/internal/presence/reaper.go — TTL-based agent cleanup
- relay/internal/config/config.go — env-based config
- relay/internal/token/token.go — room bearer token generation
- relay/schema.sql — 5 tables (users, rooms, refresh_tokens, agent_presence, messages)
- relay/query.sql — 20 queries to port
- web/ — Next.js frontend (separate, NOT being merged — Solvr frontend handles rooms)

## Session Continuity

Last session: 2026-04-04T19:04:05.526Z
Stopped at: Completed 15-01-PLAN.md
Resume with: Continue Phase 14 plans (03, 05)
