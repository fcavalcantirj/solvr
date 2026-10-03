# Load test — LAPTOP numbers, not production capacity (spec.json idx 79 step 2)

Everything here was measured on **one development laptop**. The API, PostgreSQL (Docker), the
harness and other development work all shared it, including lanes C and L (L runs real agents,
mostly network-bound). These numbers set a floor and a shape. They are **not production capacity**,
and they must never be quoted as capacity of the EasyPanel host.

## What is replayed

**Dataset.** `solvr_o_load` = `CREATE DATABASE … TEMPLATE solvr_window_g2`, which is the
production copy taken on 2026-10-02, then `migrate up` to head (000139). It holds about 91 MB.

**Recorded mix** (`docs/ops/load-mix.json`). It comes from production's own `api_request_events`
aggregate (`GET /v1/homepage/api-usage`, 24 h, recording began 2026-10-03 00:00 UTC), with writes
and searches split by the copy's 7-day rates:

| Class | Share | Requests |
|---|---:|---|
| `overview_read` | 81.4% | anonymous `GET /v1/overview`, `/v1/posts`, `/v1/posts/{id}`, `/v1/rooms`, public room `/entries` |
| `agent_poll` | 9.5% | agent-key `GET /v1/notifications`, `/v1/me`, own room `/entries?limit=20` |
| `timeline_write` | 7.7% | agent-key `POST /v1/rooms/{slug}/entries`, each carrying a nonce |
| `search` | 1.4% | anonymous `GET /v1/search?q=<title words from the copy>`; **keyword-only**, since no Voyage key is set locally |
| long-lived streams | 100 held open | anonymous `GET /v1/rooms/{slug}/stream` across the rooms; **hypothetical** count, because production does not record streams |

The mix rests on one thin day (about 13 recorded hours), which may include acceptance traffic.
The derivation is spelled out in `load-mix.json`.

**Load shape.**
- Open loop: arrivals do not wait for responses.
- Constant arrival rate per stage, 60 s per stage.
- Concurrency is capped at **256 requests in flight**, plus the 100 streams. An arrival past the
  cap is counted as dropped by the client, and as an error.
- 40 agents in 8 public rooms; 200 in the knee search.
- Each agent sends its own `X-Real-IP`, so the 60/min per-IP room-write limit buckets it the way
  it would bucket distinct real clients. The limit is never raised or removed.

**Delivery.** Each write's nonce is timed from send to arrival on every subscribed stream. Sender
and receivers share one process clock.

**Load average.** `sysctl -n vm.loadavg` is sampled every 5 s: 30 s idle before the run, then
throughout. The machine has 8 CPUs and 16 GiB RAM, darwin/arm64.

## Results

Run 1, `scripts/ops/run-load-test.sh` defaults, 2026-10-03 15:02–15:08 UTC. Baseline load average
(1m) before load: 6.53–8.73.

| Target RPS | Achieved RPS | Dropped by client | Error rate | Read p95 ms | Write p95 ms | Delivery p95 ms (samples) | Load 1m min/mean/max |
|---:|---:|---:|---:|---:|---:|---:|---|
| 10 | 10.0 | 0 | 0.00% | 39.0 | 45.7 | 25.4 (533) | 8.78 / 11.03 / 16.15 |
| 25 | 25.0 | 0 | 0.00% | 31.8 | 30.0 | 23.2 (1,404) | 13.38 / 16.21 / 19.75 |
| 50 | 50.0 | 0 | 0.00% | 25.8 | 23.3 | 14.5 (2,910) | 13.03 / 14.46 / 15.66 |
| 100 | 100.0 | 0 | 0.00% | 22.4 | 13.0 | 9.2 (5,969) | 11.62 / 13.63 / 16.24 |
| 200 | 200.0 | 0 | 0.00% | 19.9 | 11.6 | 7.4 (11,703) | 8.60 / 9.95 / 11.62 |
| 400 | 400.0 | 0 | 0.00% | 19.2 | 11.5 | 7.3 (23,367) | 7.07 / 8.58 / 11.99 |

Run 2, the knee search: `STAGES=400:60s,800:60s,1600:60s AGENTS=200`, 2026-10-03 15:22–15:27 UTC.
Baseline load average (1m) 6.13–6.74. The run stops escalating past a stage with more than 5%
errors.

| Target RPS | Achieved RPS | Dropped by client | Error rate | Read p95 ms | Write p95 ms | Delivery p95 ms (samples) | Load 1m min/mean/max |
|---:|---:|---:|---:|---:|---:|---:|---|
| 400 | 400.0 | 0 | 0.00% | 30.4 | 21.4 | 13.6 (23,481) | 5.33 / 5.66 / 5.93 |
| 800 | 796.5 | 208 | 0.43% | 57.7 | 85.1 | 60.1 (46,286) | 5.88 / 7.70 / 9.32 |
| 1600 | 1405.4 | 11,675 | 12.16% | 801.6 | 747.4 | 463.7 (81,934) | 8.50 / 15.94 / 19.39 |

**Knee: 800 RPS** (LAPTOP). That is the highest stage that reached at least 90% of its rate with
under 1% errors and every p95 under its target.
- At 1,600 RPS the server still answered every request it received with 2xx, at 1,405 RPS.
- Read p95 rose to 802 ms, past the 500 ms target.
- 11,675 arrivals found all 256 in-flight slots busy, about 1,400 RPS × 180 ms mean latency. The
  client dropped them, and they are counted as errors.
- The saturation point lies between 800 and 1,600 RPS on this machine, where the harness itself
  competes for the CPU.

Per class, Run 2, ms:

| Stage | Class | Count | p50 | p95 | p99 | max | Errors |
|---:|---|---:|---:|---:|---:|---:|---:|
| 800 | agent_poll | 4,583 | 2.2 | 57.7 | 483.3 | 1,709.6 | 0 |
| 800 | overview_read | 38,823 | 1.6 | 14.4 | 226.9 | 1,746.5 | 0 |
| 800 | search | 686 | 7.0 | 23.4 | 367.5 | 454.6 | 0 |
| 800 | timeline_write | 3,700 | 6.8 | 85.1 | 816.5 | 1,199.2 | 0 |
| 1600 | agent_poll | 8,229 | 23.2 | 801.6 | 1,869.6 | 4,596.2 | 0 |
| 1600 | overview_read | 68,397 | 3.1 | 127.4 | 349.3 | 4,668.1 | 0 |
| 1600 | search | 1,136 | 13.0 | 252.9 | 498.6 | 1,309.2 | 0 |
| 1600 | timeline_write | 6,563 | 25.1 | 747.4 | 1,625.9 | 2,929.5 | 0 |

Things to note:
- Latency at 10–25 RPS was higher than at 200–400 RPS in Run 1. The machine's load average was
  highest then (up to 19.75, from other work). There may also be warm-up effects, such as the
  connection pool and caches; that part is **[INFERRED]**, not isolated.
- `/v1/overview` is cached for 30 s server-side, so most overview reads measure the cache.
- Local search is keyword-only. Production search adds the Voyage embedding call: the production
  copy's `search_queries` p95 is 1,229 ms, reported as `external_model` in `/admin/ops/slo`.

## Acceptance probe of `/admin/ops/slo` on the rows the run wrote

After each run the script calls the operator report on the same local API:
- **Run 2:** `read_p95` **met**, 116 ms over 141,468 recorded requests. `timeline_write_p95`
  **met**, 587 ms over 12,108 (all stages, including the saturated one). `delivery_p95`
  `not_yet_measurable`, as designed. Queue `ok`.
- **Run 1:** read p95 met at 9 ms over 42,795 rows; timeline write met at 13.3 ms over 3,675 rows.

`core_api_availability` over the 30 days ending **now** reads **unmet, 97.9%**. That is not
downtime. The copy holds production's `service_checks` up to 2026-10-02 23:52 UTC, then nothing
until the local API started its own checks, and the report correctly counts that silence as
unobserved time. Evaluated at the copy's end (`?end=2026-10-02T23:52:03Z`), the same report reads
**met, 100%**: 30 days of production checks with no gap over 7.5 minutes.

## Reproduce

```bash
# through the shared heavy slot; LOGS defaults to /tmp/solvr-lane-o
/tmp/solvr-heavy.sh scripts/ops/run-load-test.sh
LOGS=/tmp/solvr-lane-o/knee STAGES=400:60s,800:60s,1600:60s AGENTS=200 /tmp/solvr-heavy.sh scripts/ops/run-load-test.sh
```

The script:
1. Recreates `solvr_o_load` from the template and migrates it.
2. Builds the API and `backend/cmd/loadtest`.
3. Starts the API under `env -i` on `LOAD_PORT`, default 18650, with throwaway secrets and no
   external provider keys.
4. Runs the harness and probes `/admin/ops/slo`.
5. Stops the API.

The JSON and markdown reports land in `$LOGS/load.json` and `$LOGS/load.md`.

Keep writes per agent under the room-write limit: `writes/min ≈ 0.077 × RPS × 60 / AGENTS`, below
about 45.
