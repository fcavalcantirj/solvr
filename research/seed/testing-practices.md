# Testing Best Practices for Agent Experiments

Best practices for verifying solutions in automated experiments.

---

## Principle: Prefer Machine-Checkable Criteria

**Bad:** "The code should work correctly"
**Good:** "Exit code 0 and output contains 'success'"

**Bad:** "Handle errors gracefully"  
**Good:** "Returns 400 for invalid input, response.error is non-empty string"

---

## Verification Patterns

### 1. Exit Code Check
```bash
command_to_test
if [ $? -eq 0 ]; then echo "PASS"; else echo "FAIL"; fi
```

### 2. Output Contains
```bash
command_to_test 2>&1 | grep -q "expected_string" && echo "PASS" || echo "FAIL"
```

### 3. File Exists
```bash
[ -f expected_file.txt ] && echo "PASS" || echo "FAIL"
```

### 4. JSON Field Check
```bash
result=$(command_to_test)
echo "$result" | jq -e '.status == "success"' > /dev/null && echo "PASS" || echo "FAIL"
```

### 5. HTTP Status
```bash
status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/endpoint)
[ "$status" -eq 200 ] && echo "PASS" || echo "FAIL"
```

### 6. Database State
```bash
result=$(psql -c "SELECT count(*) FROM users WHERE active = true" -t)
[ "$result" -gt 0 ] && echo "PASS" || echo "FAIL"
```

---

## Test Script Template

```bash
#!/bin/bash
# test-solution.sh

PASSES=0
FAILS=0

check() {
  local name="$1"
  local cmd="$2"
  
  if eval "$cmd" > /dev/null 2>&1; then
    echo "✓ $name"
    ((PASSES++))
  else
    echo "✗ $name"
    ((FAILS++))
  fi
}

# Run checks
check "Server starts" "curl -s http://localhost:8080/health | grep -q ok"
check "Returns 200" "[ $(curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/) -eq 200 ]"
check "Response valid" "curl -s http://localhost:8080/ | jq -e '.data' > /dev/null"

# Summary
echo ""
echo "Results: $PASSES passed, $FAILS failed"
[ $FAILS -eq 0 ] && exit 0 || exit 1
```

---

## Common Pitfalls

### Timing Issues
**Problem:** Test runs before server is ready
**Solution:** Add health check wait loop

```bash
wait_for_ready() {
  for i in {1..30}; do
    curl -s http://localhost:8080/health > /dev/null && return 0
    sleep 1
  done
  return 1
}
```

### Environment Differences
**Problem:** Works locally, fails in CI
**Solution:** Use containerized test environment

```bash
docker run --rm -v $(pwd):/app -w /app node:20 npm test
```

### Flaky Tests
**Problem:** Passes sometimes, fails sometimes
**Solution:** Run multiple times, require consistent results

```bash
for i in {1..3}; do
  ./test.sh || exit 1
done
```

---

## Metrics Collection

For experiment analysis, capture:

```bash
# Tokens (from API response headers or logs)
tokens=$(grep "total_tokens" agent.log | tail -1 | jq '.usage.total_tokens')

# Time
start=$(date +%s.%N)
./run-solution.sh
end=$(date +%s.%N)
time=$(echo "$end - $start" | bc)

# Pass rate
./test-solution.sh
pass_rate=$?

# Output JSON for analysis
jq -n \
  --arg tokens "$tokens" \
  --arg time "$time" \
  --arg pass "$pass_rate" \
  '{tokens: $tokens, time_seconds: $time, passed: ($pass == "0")}'
```

---

## Human Scoring Rubric

For subjective quality assessment (0-5):

| Score | Meaning |
|-------|---------|
| 0 | Does not compile/run |
| 1 | Runs but wrong output |
| 2 | Partially correct, major issues |
| 3 | Mostly correct, minor issues |
| 4 | Correct, but not idiomatic |
| 5 | Correct and well-written |

**Blind scoring:** Scorer should not know which condition produced the solution.
