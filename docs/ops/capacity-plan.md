# Capacity plan — a planning model, not a forecast (spec.json idx 79 step 3)

The spec's planning case is 1,000,000 monthly active participants, 10% daily activity and 20
requests per daily participant. **These are assumptions (hypothetical), not forecasts.** Measured
polling, stream and burst costs are added separately, and the peak is compared with the measured
laptop capacity from [`load-test.md`](load-test.md).

- Code: `backend/internal/ops/capacity.go` (`CapacityPlan`). Tests in `capacity_test.go` pin
  1,000,000 × 0.10 × 20 = **2,000,000 requests/day ≈ 23.15 RPS**.
- Render: `cd backend && go run ./cmd/ops-model -inputs ../docs/ops/model-inputs.example.json`.

## The plan (example inputs)

| Input or result | Value | Unit | Source |
|---|---:|---|---|
| Monthly active participants | 1,000,000 | participants | hypothetical |
| Daily active share | 0.1 | ratio | hypothetical |
| Requests per daily participant | 20 | requests | hypothetical |
| Daily active participants | 100,000 | participants | derived |
| **Base requests per day** | **2,000,000** | requests/day | derived |
| Base mean RPS | 23.1481 | req/s | derived |
| Polling share of daily participants | 0.2 | ratio | hypothetical |
| Poll interval | 30 | s | hypothetical |
| Polling hours per day | 1 | h | hypothetical |
| Polling requests per day | 2,400,000 | requests/day | derived |
| Total requests per day | 4,400,000 | requests/day | derived |
| Mean RPS | 50.9259 | req/s | derived |
| Burst factor (peak / mean) | 10.1 | x | observed |
| Peak RPS | 514.3519 | req/s | derived |
| Stream share of daily participants | 0.05 | ratio | hypothetical |
| Stream hours per day | 1 | h | hypothetical |
| Stream heartbeat interval | 30 | s | observed |
| Mean concurrent streams | 208.3333 | streams | derived |
| Peak concurrent streams | 2,104.1667 | streams | derived |
| Heartbeat frames per second | 6.9444 | frames/s | derived |
| Measured capacity (load test) | 800 | req/s | observed |
| Peak / measured capacity | 0.6429 | x | derived |

Where the observed inputs come from:
- **Burst factor 10.1**: production's busiest hour (489 calls) over its mean hour (1,160 / 24) on
  2026-10-03. That is one thin day, measured on very low traffic. Treat it as a pessimistic
  placeholder, not a property of the product.
- **Stream heartbeat 30 s**: read from `handlers/rooms_sse.go`.
- **Measured capacity 800 RPS**: the **LAPTOP** knee, with the API, database and harness on one
  machine.

## What it says, and what it does not

- Even the polling-inflated planning case peaks around 514 RPS. That is 0.64 of the laptop knee,
  on the recorded mix.
- It does **not** say production holds 1,000,000 MAP. The production host is a different machine.
  The SSE limit is a global 1,000 connections per process (`rooms_sse.go`), below the 2,104 peak
  streams above. One instance cannot serve that stream count, whatever its RPS.
- The polling term is as large as the base term with these hypothetical inputs. Measure real
  agent poll intervals before trusting either.

## Before increasing an acquisition channel

1. Replace each hypothetical input with an observed value. `/admin/growth/participants` holds
   MAP and the daily share. `api_request_events` holds polling per agent and the burst over a
   real month.
2. Re-run the load test on the production host class, not the laptop (UAT).
3. Raise the SSE connection cap, or plan a second instance, before peak concurrent streams
   approach 1,000.
