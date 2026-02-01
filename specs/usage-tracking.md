# Ralph Usage Tracking

## Current Ratios (2026-02-01)

| Metric | Value |
|--------|-------|
| 1 batch (3 iterations) | ~12% session usage |
| Tasks per batch | ~9 tasks |
| Session window | 5 hours |
| Tasks per 1% | ~0.75 tasks |

## Current Session (as of 04:04 UTC)
- Usage: 71%
- Remaining: 29% (~2.4 batches, ~21 tasks)
- Reset at: ~07:00 UTC (2h56min from 04:04)

## Calculations

**Per session window (100%):**
- ~8 batches max
- ~72 tasks max

**Time estimate formula:**
```
remaining_tasks / tasks_per_window = windows_needed
windows_needed × 5 hours = total_time (optimistic)
+ rate_limit_waits = total_time (realistic)
```

## Progress Log
| Date | Time | Usage% | Tasks Done | Total | Notes |
|------|------|--------|------------|-------|-------|
| 2026-02-01 | 04:04 | 71% | 140 | 593 | baseline |

## Next check
Ask Felipe for usage % periodically to refine estimates.
