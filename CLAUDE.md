# Solvr Project Guidelines

## Overview

**Solvr** is a knowledge base for developers and AI agents. The Stack Overflow for the AI age.

- **Backend:** Go (API)
- **Frontend:** Next.js (React)
- **Database:** PostgreSQL
- **Full spec:** `SPEC.md` (~2800 lines, 19 parts)

### Production URLs
- **Frontend:** https://solvr.dev
- **API:** https://api.solvr.dev
- **Google OAuth callback:** https://api.solvr.dev/v1/auth/google/callback

---

## Golden Rules

### 1. TDD First — 80%+ Test Coverage

**ALWAYS follow Test-Driven Development.**

1. **RED**: Write a failing test that describes expected behavior
2. **GREEN**: Write minimum code to make test pass
3. **REFACTOR**: Clean up while keeping tests green

```bash
# Backend
cd backend && go test ./... -cover

# Frontend
cd frontend && npm test -- --coverage
```

**Coverage requirement: 80% minimum for both backend and frontend.**

### 2. File Size Limit — ~900 Lines Max

**No single code file should exceed ~900 lines.**

- If a file grows beyond ~900 lines, split it into modules
- Documentation files (`.md`) are exempt
- Check before committing: `wc -l backend/**/*.go frontend/**/*.ts frontend/**/*.tsx`

**Why:** Large files cause context explosion with AI assistants. Keep files focused.

### 3. API Is Smart, Client Is Dumb

**100% of business logic lives in the API. The frontend is a dumb terminal.**

The frontend must NEVER:
- Validate data (API validates)
- Transform data (API formats response)
- Make business decisions
- Calculate anything domain-specific

The frontend ONLY:
- Calls API endpoints
- Displays what API returns
- Sends user input to API
- Shows loading/error states

### 4. API-First Design

**Design API endpoints before implementation.**

1. Define the endpoint in SPEC.md
2. Write API tests
3. Implement the endpoint
4. Then build frontend to consume it

### 5. v0 = No Backwards Compatibility

**During v0 (pre-1.0), we can break things freely.**

- No need to maintain backwards compat
- Move fast, fix design mistakes immediately
- Document breaking changes in commits

### 6. No Stubs or In-Memory Implementations in Production Code

**NEVER use in-memory/stub implementations when database is available.**

- If a feature needs database storage, implement the REAL database repository
- No "temporary" in-memory repos that "will be replaced later"
- No comments like "For now, use in-memory until DB is added"
- If you can't implement DB storage, DON'T implement the feature

**Why:** In-memory data is lost on every deploy. This causes data loss bugs that are hard to detect until production data disappears.

**Check before committing:**
- Search for `InMemory` in production code paths
- Ensure all repositories use `db.New*Repository(pool)` not `NewInMemory*Repository()`

### 7. Use Solvr — Search Before Solving, Document When Interesting

**This IS the Solvr project. Dogfood it.**

#### BEFORE solving any problem:
```bash
# Search Solvr for existing solutions
curl -s "https://api.solvr.dev/v1/search?q=YOUR_PROBLEM_KEYWORDS" | jq '.data[0:3]'
```

- Check for existing solutions → use them
- Check for failed approaches → avoid them
- Don't reinvent wheels

#### WHEN you hit a wall AND overcome it:
**This is the interesting stuff. Document it.**

```bash
# 1. Create a problem (if novel)
curl -X POST "https://api.solvr.dev/v1/posts" \
  -H "Authorization: Bearer $SOLVR_API_KEY" \
  -d '{
    "type": "problem",
    "title": "Exact error or issue description",
    "description": "What happened, context, symptoms",
    "tags": ["solvr", "backend", "relevant-tag"]
  }'

# 2. Add your approach (what you tried)
curl -X POST "https://api.solvr.dev/v1/problems/{problem_id}/approaches" \
  -H "Authorization: Bearer $SOLVR_API_KEY" \
  -d '{
    "angle": "Brief description of approach",
    "method": "Detailed steps you took"
  }'

# 3. Update status (succeeded or failed)
curl -X PATCH "https://api.solvr.dev/v1/approaches/{approach_id}" \
  -H "Authorization: Bearer $SOLVR_API_KEY" \
  -d '{"status": "succeeded"}'  # or "failed"
```

#### WHEN NOT to post:
- Trivial tasks that worked first try
- Standard implementation (no wall hit)
- Already documented in Solvr

#### WHEN TO post:
- Hit a wall, tried multiple approaches, finally succeeded ✅
- Discovered a non-obvious solution
- Found a bug/gotcha others should know about
- Approach failed in an interesting way (saves others time)

**Key:** Quality > quantity. Only post what helps others.

---

## Tech Stack Commands

### Backend (Go)

```bash
cd backend

# Run all tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Run specific package tests
go test ./internal/api/...

# Build
go build ./cmd/api

# Run
go run ./cmd/api

# Lint
golangci-lint run
```

### Frontend (Next.js)

```bash
cd frontend

# Install deps
npm install

# Run dev server
npm run dev

# Run tests
npm test

# Run tests with coverage
npm test -- --coverage

# Build
npm run build

# Lint
npm run lint

# Type check
npm run typecheck
```

**IMPORTANT: Use Vitest, NOT Jest**

The frontend uses **Vitest** for testing. When writing tests:
- Use `vi.mock()` not `jest.mock()`
- Use `vi.fn()` not `jest.fn()`
- Use `vi.mocked()` not `jest.Mocked`
- Import from `'vitest'`: `import { describe, it, expect, vi, beforeEach } from 'vitest'`

### Database

```bash
# Start PostgreSQL (Docker)
docker compose up -d

# Run migrations
cd backend
migrate -path migrations -database "$DATABASE_URL" up

# Rollback
migrate -path migrations -database "$DATABASE_URL" down 1

# Create new migration
migrate create -ext sql -dir migrations -seq <name>
```

---

## File Structure

```
solvr/
├── SPEC.md                    # Full specification (READ THIS)
├── CLAUDE.md                  # This file
├── specs/
│   ├── prd-v1.json            # All requirements (testable)
│   └── progress.txt           # Current progress notes
├── backend/
│   ├── cmd/api/main.go        # Entry point
│   ├── internal/
│   │   ├── api/               # HTTP handlers
│   │   ├── auth/              # Auth logic
│   │   ├── db/                # Database layer
│   │   ├── models/            # Data models
│   │   └── services/          # Business logic
│   ├── migrations/            # SQL migrations
│   └── go.mod
├── frontend/
│   ├── app/                   # Next.js pages
│   ├── components/            # React components
│   ├── lib/                   # Utilities
│   └── package.json
├── mcp-server/                # MCP server for Claude Code etc.
├── cli/                       # CLI tool
└── docker-compose.yml
```

---

## Workflow

### Working on a Requirement

1. **Read `specs/prd-v1.json`** — find next `"passes": false` requirement
2. **Read relevant section in `SPEC.md`** — understand the full context
3. **Write tests first** — TDD approach
4. **Implement** — make tests pass
5. **Verify:**
   - `go test ./...` passes
   - `npm test` passes
   - No file exceeds ~900 lines
6. **Update `specs/prd-v1.json`** — set `"passes": true`
7. **Update `specs/progress.txt`** — note what you did
8. **Commit and push**

### Commit Message Format

```
feat(api): implement search endpoint

- Add GET /v1/search with full-text search
- Add query params: q, type, tags, status, sort
- Add response with snippets and scores
- Tests: 12 new, all passing
```

Prefixes: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`

---

## API Versioning

All endpoints use `/v1/` prefix:

```
GET  /v1/search
GET  /v1/posts
POST /v1/posts
GET  /v1/agents/:id
...
```

---

## Admin Query Route (Production Database)

For running migrations or raw SQL on production, use the admin query endpoint.

**Key location:** `.env` file (git-ignored)

```bash
# Load the key
source .env

# Run a SELECT query
curl -X POST https://api.solvr.dev/admin/query \
  -H "Content-Type: application/json" \
  -H "X-Admin-API-Key: $ADMIN_API_KEY" \
  -d '{"query": "SELECT COUNT(*) FROM agents;"}'

# Run a migration (requires DESTRUCTIVE_QUERIES=true on server)
curl -X POST https://api.solvr.dev/admin/query \
  -H "Content-Type: application/json" \
  -H "X-Admin-API-Key: $ADMIN_API_KEY" \
  -d '{"query": "ALTER TABLE agents ADD COLUMN IF NOT EXISTS model VARCHAR(100);"}'
```

**Handler:** `backend/internal/api/handlers/admin.go`

---

## Email Broadcasts

Send emails to users via the admin broadcast system. Backend handles branded template wrapping, HMAC unsubscribe links, `List-Unsubscribe` headers, and 150ms rate limiting between sends.

**Skill:** `~/.claude/skills/solvr/scripts/solvr-admin.sh`

**Template variables:** `{name}`, `{referral_code}`, `{referral_link}` — substituted per-recipient by the backend.

```bash
# Load admin key
export $(grep ADMIN_API_KEY .env | xargs)

# Preview recipients without sending (dry-run)
bash ~/.claude/skills/solvr/scripts/solvr-admin.sh email dry-run \
  --subject "Subject line" \
  --body-html "<p>Hey {name}, check {referral_link}</p>"

# Send to ALL active users (branded template + unsubscribe added automatically)
bash ~/.claude/skills/solvr/scripts/solvr-admin.sh email send \
  --subject "Subject line" \
  --body-html "<p>Hey {name}, your content here</p>"

# Send to ONE user
bash ~/.claude/skills/solvr/scripts/solvr-admin.sh email send \
  --to "user@example.com" \
  --subject "Subject line" \
  --body-html "<p>Hey {name}, your content here</p>"

# View past broadcasts
bash ~/.claude/skills/solvr/scripts/solvr-admin.sh email history
```

**Segmented sends:** The broadcast endpoint sends to all users or one user (`--to`). For different emails per segment, loop with `--to` per user. See `/tmp/solvr_segmented_send.sh` for a working example that queries user segments from the DB and sends per-segment templates.

**Key files:**
- **Skill script:** `~/.claude/skills/solvr/scripts/solvr-admin.sh`
- **Handler:** `backend/internal/api/handlers/admin.go` (`BroadcastEmail`)
- **Template wrapper:** `backend/internal/emailutil/template.go` (`WrapInBrandedTemplate`)
- **Unsubscribe:** `backend/internal/api/handlers/unsubscribe.go` (HMAC-signed one-click)
- **Broadcast log:** `backend/internal/db/email_broadcast.go` (audit trail)
- **API endpoint:** `POST /admin/email/broadcast` (requires `X-Admin-API-Key`, needs `RESEND_API_KEY` on server)

**Deduplication:** Same subject within 24h is blocked unless `force: true` is set in the JSON payload.

---

## Deployment Constraints

- Frontend uses `output: 'standalone'` in `next.config.mjs` for Docker deployment
- **DO NOT use `generateSitemaps()`** — it creates dynamic routes that break with standalone mode (caused production 404)
- Current sitemap (`frontend/app/sitemap.ts`) is a single flat file fetching all URLs via `GET /v1/sitemap/urls`
- Sitemap protocol limit: 50,000 URLs per file. Current count: ~100

---

## Roadmap

### Sitemap sharding (when approaching 50k URLs)
- Backend pagination API is already built: `GET /v1/sitemap/urls?type=posts&page=1&per_page=2500`
- Backend counts API is already built: `GET /v1/sitemap/counts`
- Frontend types (`SitemapUrlsParams`, `APISitemapCountsResponse`) are already in `lib/api-types.ts`
- When needed, replace single `sitemap.ts` with a route handler (`app/sitemap.xml/route.ts`) that generates a sitemap index + individual sitemap files — this approach works with `output: 'standalone'`
- Alternative: generate static XML files at build time via a script writing to `/public`

---

## Important Notes

1. **Read SPEC.md** — it has everything: data model, API endpoints, schemas, security rules
2. **One task at a time** — don't try to implement multiple requirements at once
3. **Tests prove completion** — if tests pass, requirement is done
4. **Ask if unclear** — better to clarify than assume wrong

---

## Progress Tracking

Check current progress:
```bash
./progress.sh
```

Run the harness:
```bash
# Single iteration
./ralph.sh 1

# Multiple iterations
./ralph.sh 5

# Continuous (batches of 3, pauses between)
./ralph-continuous.sh
```
