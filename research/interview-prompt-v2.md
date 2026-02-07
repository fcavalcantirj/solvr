# Interview Prompt: HTTP Server Feature Spec

Copy this entire prompt into a fresh Claude Code session to generate a detailed spec file.

---

## PROMPT START

You are a technical interviewer helping me create a detailed **feature specification** for a coding task. 

The task is to build an **HTTP server**. You will help me define the features it needs, following the Anthropic harness format.

**Reference:** https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents

Each feature should be a JSON object:
```json
{
  "category": "functional|performance|error-handling|security",
  "description": "What this feature does",
  "steps": [
    "Step 1 to implement",
    "Step 2 to implement",
    "Verification step"
  ],
  "passes": false
}
```

After defining features, we'll also define:
1. **Build verification** - How to know all features work
2. **Benchmark tasks** - Stress test after build is complete
3. **Human scoring criteria** - How human rates the result

---

**Interview me about:**

### Part 1: The Server Features

1. **Language/Framework**
   - What language?
   - Framework or stdlib only?
   - Forbidden dependencies?

2. **Core Features** (ask one by one)
   - What endpoints does it need?
   - What should each return?
   - Any authentication?
   - Any rate limiting?
   - Error handling requirements?

3. **For each feature**, generate a spec object with:
   - Category
   - Description
   - Implementation + verification steps
   - passes: false

### Part 2: Build Verification

4. **How do we verify the build is complete?**
   - What command runs all checks?
   - What does success look like?

### Part 3: Benchmark (post-build)

5. **Stress test parameters**
   - What tool? (wrk, hey, ab)
   - Concurrent connections?
   - Duration?
   - Thresholds to pass?
     - Min requests/sec?
     - Max p99 latency?
     - Max error rate?

### Part 4: Human Scoring

6. **What should human evaluate?**
   - Code quality (1-5)?
   - Documentation (1-5)?
   - Error handling (1-5)?
   - Overall (1-5)?

---

**Output format:**

After interview, generate a complete spec file:

```json
{
  "id": "http-server-001",
  "name": "HTTP Server",
  "language": "...",
  "framework": "...",
  
  "features": [
    { "category": "...", "description": "...", "steps": [...], "passes": false },
    { "category": "...", "description": "...", "steps": [...], "passes": false }
  ],
  
  "build_verification": {
    "command": "...",
    "expected": "..."
  },
  
  "benchmark": {
    "tool": "...",
    "command": "...",
    "thresholds": {
      "requests_per_sec_min": ...,
      "p99_latency_max_ms": ...,
      "error_rate_max_percent": ...
    }
  },
  
  "human_scoring": {
    "criteria": [
      { "name": "Code Quality", "weight": 0.3 },
      { "name": "Documentation", "weight": 0.2 },
      { "name": "Error Handling", "weight": 0.2 },
      { "name": "Overall", "weight": 0.3 }
    ],
    "pass_threshold": 3.5
  },
  
  "metrics_to_collect": [
    "tokens_used",
    "time_to_completion",
    "api_cost",
    "attempts_count",
    "human_score"
  ]
}
```

---

Start by asking the first question about language/framework.

## PROMPT END

---

## Usage

1. Create fresh repo
2. Open Claude Code with no prior context
3. Paste this entire prompt
4. Answer the interview questions
5. Claude generates the spec file
6. Save as `specs/http-server.json`

---

## After Spec is Created

Run the experiment:
1. **Baseline**: Fresh context, no Solvr, build from spec
2. **Solvr-enabled**: Fresh context + Solvr MCP, build from spec
3. **For each**: Record tokens, time, cost, human score
4. **Compare**: Analyze differences
