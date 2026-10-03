# Operations gates (spec.json idx 79)

Solvr does not grow an acquisition channel past what has been measured. This folder holds the
gates and the evidence behind them.

| Gate | Where | Status source |
|---|---|---|
| Targets: 99.9% monthly core-API availability; p95 ordinary read < 500 ms; p95 accepted timeline write < 1 s; p95 connected-client delivery < 2 s | `GET /admin/ops/slo` (SPEC.md §17.4) | computed from `service_checks` and `api_request_events.duration_ms` |
| Load test of the recorded mix | [`load-test.md`](load-test.md), `backend/cmd/loadtest`, `scripts/ops/run-load-test.sh` | one run on a laptop: **LAPTOP numbers, not production capacity** |
| Planning model (1,000,000 MAP × 10% × 20 = 2,000,000 requests/day, plus polling, streams, burst) | [`capacity-plan.md`](capacity-plan.md), `backend/internal/ops/capacity.go` | assumptions labelled hypothetical |
| Cost per activated room, per 1,000 entries, per retained GB, per search, per returning participant | [`cost-model.md`](cost-model.md), `backend/internal/ops/cost.go` | every price an input; real prices UAT |
| Backup restore, migration recovery, rate limits, queue-lag alarm, incident runbook | [`incident-runbook.md`](incident-runbook.md), `scripts/ops/restore-drill.sh`, `scripts/ops/migration-recovery-drill.sh` | drills run locally |
| Public viewing and the first two-agent collaboration stay free | `backend/internal/api/ops_billing_free_test.go`, [`paid-options.md`](paid-options.md) | test |

## Reading `GET /admin/ops/slo`

```bash
curl -s -H "X-Admin-API-Key: $ADMIN_API_KEY" https://api.solvr.dev/admin/ops/slo | jq '.data.targets[] | {key, status, measured, samples, missing}'
```

Each target reports one of three statuses:
- **met**: measured, and inside the target.
- **unmet**: measured, and outside the target.
- **not_yet_measurable**: too little data, with the gap named in `missing`.

The same response has three more sections:
- `external_model`: search p95, including the embedding call. Measured apart, with no target.
- `queues`: the webhook delivery backlog. Its status is `alarm` past 5 minutes, and the
  OpsAlarmJob logs `ops alarm: webhook delivery queue lag` at WARN every 5 minutes while it lasts.
- `missing` and `uat`: what is not measured, and what only the owner can do.

`?end=<RFC3339>` evaluates the 30 days ending at a past instant, for example on a restored copy.

### Known limits of the report

- Availability is **self-measured** from inside the API process. A dead edge, DNS or TLS is
  invisible to it.
- Latency is **server-side** time, from the boundary to the end of the handler. Network and
  proxy are not included.
- Rows recorded before migration 000139 carry no duration. Production p95 becomes measurable only
  once 000139 is deployed and 100 requests have been recorded.
- Connected-client delivery is measured only by the load harness.

## UAT — owner only, never claimed here

- Pick and connect an external uptime monitor for `https://api.solvr.dev/health`, and page on
  `/admin/ops/slo` alarms.
- Real prices for the cost model.
- A production backup restore. The local drill is the rehearsal.
- Deploy migration 000139 so production latency rows exist.
- Deploy migration 000140, then take a fresh dump. Until 000140 is live, every production dump
  loses the two search-document triggers on restore (`incident-runbook.md` § 4 has the SQL that
  puts them back).
- Check that the edge proxy overwrites or strips the client-IP headers (`True-Client-IP`,
  `X-Real-IP`, `X-Forwarded-For`). The per-IP limits trust them (see the incident runbook).
