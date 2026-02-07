# Interview Prompt: HTTP Server Benchmark Spec

Copy this entire prompt into a fresh Claude Code session to generate a detailed spec file.

---

## PROMPT START

You are a technical interviewer helping me create a detailed specification for a coding task. The task is to build an **HTTP server that passes a stress test benchmark**.

Your job is to interview me to understand exactly what I want, then produce a structured JSON spec file following this format:

```json
{
  "id": "http-benchmark-001",
  "name": "HTTP Server Stress Test",
  "category": "systems",
  "description": "Build an HTTP server that handles concurrent load",
  "language": "...",
  "requirements": [...],
  "endpoints": [...],
  "benchmark": {
    "tool": "wrk|hey|ab",
    "duration": "...",
    "concurrency": "...",
    "thresholds": {
      "requests_per_sec_min": ...,
      "p99_latency_max_ms": ...,
      "error_rate_max_percent": ...
    }
  },
  "verification": {
    "commands": [...],
    "expected_outputs": [...]
  },
  "steps": [...],
  "passes": false
}
```

**Interview me about:**

1. **Language/Framework**
   - What language should this be written in?
   - Any framework constraints (stdlib only? specific framework?)
   - Any forbidden dependencies?

2. **Endpoints**
   - What endpoints should the server have?
   - What should each endpoint return?
   - Any specific response format requirements?

3. **Benchmark Parameters**
   - What benchmark tool should we use? (wrk, hey, ab)
   - How many concurrent connections?
   - How long should the benchmark run?
   - What are the minimum acceptable metrics?
     - Requests per second?
     - P99 latency threshold?
     - Maximum error rate?

4. **Constraints**
   - Any memory limits?
   - Any CPU constraints?
   - Must it work on specific OS?

5. **Verification**
   - How do we verify correctness beyond the benchmark?
   - Any unit tests required?
   - Any specific edge cases to handle?

Ask me these questions one at a time. After I answer all questions, generate the complete JSON spec file.

Start by asking the first question.

## PROMPT END

---

## Usage

1. Create fresh repo on your machine
2. Open Claude Code with no prior context
3. Paste this entire prompt
4. Answer the interview questions
5. Claude generates the spec file
6. Save as `specs/http-benchmark.json`

---

## Notes

- The interview approach ensures no assumptions
- Each answer becomes part of the spec
- The resulting spec is YOUR requirements, not Claude's guesses
