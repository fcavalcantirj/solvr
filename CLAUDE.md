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

### 2. File Size Limit — 800 Lines Max

**No single code file should exceed 800 lines.**

- If a file grows beyond 800 lines, split it into modules
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
   - No file exceeds 800 lines
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
