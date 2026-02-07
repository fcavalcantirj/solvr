# Problem Spec Format

Specs follow Anthropic's initializer pattern for coding agents.

---

## Structure

```json
{
  "id": "prob-XXX",
  "category": "database|api|devops|security|general",
  "difficulty": "easy|medium|hard",
  "description": "What needs to be solved (plain English)",
  "context": {
    "language": "python|typescript|go|rust|etc",
    "framework": "optional framework context",
    "environment": "optional env details"
  },
  "steps": [
    "Step 1: Do this thing",
    "Step 2: Then do this"
  ],
  "passes": {
    "criterion_1": "How to verify this passes",
    "criterion_2": "How to verify this passes"
  },
  "known_solution": {
    "approach": "Brief description of what works",
    "file": "optional: path to reference solution"
  },
  "common_mistakes": [
    "Thing agents often try that doesn't work",
    "Another dead end"
  ]
}
```

---

## Example: Database Connection

```json
{
  "id": "prob-001",
  "category": "database",
  "difficulty": "easy",
  "description": "PostgreSQL refuses connections with ECONNREFUSED on localhost:5432",
  "context": {
    "language": "python",
    "framework": "psycopg2",
    "environment": "Docker container, postgres:15 image"
  },
  "steps": [
    "Step 1: Diagnose why connection is refused",
    "Step 2: Fix the connection issue",
    "Step 3: Verify query execution works"
  ],
  "passes": {
    "pg_ready": "pg_isready -h localhost -p 5432 returns 0",
    "query_works": "SELECT 1 returns 1 row",
    "no_errors": "No ECONNREFUSED in logs"
  },
  "known_solution": {
    "approach": "Container networking - use service name instead of localhost",
    "file": null
  },
  "common_mistakes": [
    "Installing postgres locally instead of fixing network",
    "Changing port without updating connection string",
    "Waiting for postgres but not using health check"
  ]
}
```

---

## Example: API Rate Limiting

```json
{
  "id": "prob-002",
  "category": "api",
  "difficulty": "medium",
  "description": "Implement exponential backoff for rate-limited API calls",
  "context": {
    "language": "typescript",
    "framework": "axios",
    "environment": "Node.js 20"
  },
  "steps": [
    "Step 1: Detect 429 responses",
    "Step 2: Implement retry with exponential backoff",
    "Step 3: Respect Retry-After header if present",
    "Step 4: Add max retries and circuit breaker"
  ],
  "passes": {
    "handles_429": "Retries on 429 instead of throwing",
    "exponential": "Wait time doubles on each retry",
    "respects_header": "Uses Retry-After when present",
    "max_retries": "Gives up after N attempts"
  },
  "known_solution": {
    "approach": "axios-retry with custom backoff function",
    "file": null
  },
  "common_mistakes": [
    "Fixed delay instead of exponential",
    "Ignoring Retry-After header",
    "No max retries (infinite loop)",
    "Not jittering (thundering herd)"
  ]
}
```

---

## Fields Explained

| Field | Required | Purpose |
|-------|----------|---------|
| `id` | Yes | Unique identifier |
| `category` | Yes | For grouping and analysis |
| `difficulty` | Yes | For stratified sampling |
| `description` | Yes | What agent sees (the problem) |
| `context` | Yes | Environment details |
| `steps` | Yes | Expected solution path |
| `passes` | Yes | Verification criteria (machine-checkable preferred) |
| `known_solution` | Yes | Ground truth for accuracy scoring |
| `common_mistakes` | No | For seeding failed approaches |

---

## Notes

- **`passes` should be testable** — prefer commands over prose
- **`common_mistakes` become seed content** — post as failed approaches before experiment
- **Keep descriptions realistic** — agents should encounter problems like these in the wild
- **Difficulty matters** — easy problems might not show collective knowledge benefit

---

## Creating New Specs

1. Find a real problem you've solved
2. Generalize it (remove project-specific details)
3. Define clear pass criteria
4. Document what DOESN'T work (common mistakes)
5. Save as `prob-XXX.json`

**Good sources:**
- Stack Overflow top questions (generalized)
- GitHub issues you've resolved
- Real debugging sessions (sanitized)
