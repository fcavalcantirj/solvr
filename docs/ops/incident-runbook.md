# Runbook — Solvr incidents

What to do when production misbehaves. This is the standing runbook. One-off windows such as the
v1.3 cutover have their own
([`../handovers/2026-10-02-solvr-orchestrator/RUNBOOK-cutover-window.md`](../handovers/2026-10-02-solvr-orchestrator/RUNBOOK-cutover-window.md)),
and this one links to them rather than repeating them.

## Rules

- **bash only.** The owner's login shell is zsh, which splits words differently; type `bash` first.
- **Secrets by name only.** Read `ADMIN_API_KEY`, `SOLVR_DEPLOY_API` and `SOLVR_DEPLOY_WEB` from the
  git-ignored `.env` by name, for example `export $(grep -E '^(ADMIN_API_KEY)=' .env | xargs)`. Never
  print them, never `set -x`, never paste them into a ticket or a chat.
- **No SSH into the production host.** Deploys and rollbacks go through the EasyPanel deploy
  webhooks. Pushing to `main` deploys nothing.
- **Production writes need the owner's explicit yes, one step at a time.** Everything in "Look" is
  read-only.
- Write every report to a file and read its summary. Never stream a whole log.

## 1. Look (read-only, any time)

```bash
curl -s https://api.solvr.dev/health                                   # process up (version string is stale; not a deploy signal)
curl -s https://api.solvr.dev/v1/status | jq '.data.services'           # the public status page: last check per service
curl -s -H "X-Admin-API-Key: $ADMIN_API_KEY" https://api.solvr.dev/admin/ops/slo \
  | jq '.data | {targets: [.targets[] | {key, status, measured, samples, missing}], queues, availability: .availability | {percent, gaps, downtime_seconds}}'
```

`/admin/ops/slo` reports, for the 30 days ending now:

| Field | What it means | Act when |
|---|---|---|
| `core_api_availability` | api and database `service_checks`. Downtime is every outage check's interval plus every silence over 7.5 min. | `unmet`, or `availability.gaps` grew since you last looked |
| `read_p95`, `timeline_write_p95` | server-side p95 from `api_request_events.duration_ms` | `unmet`: section 3 |
| `delivery_p95` | always `not_yet_measurable` in production: nothing records stream delivery | — (load test only) |
| `external_model` | search p95, including the Voyage embedding call | sustained growth: check Voyage before blaming Solvr |
| `queues[0]` (`webhook_deliveries`) | oldest **due** undelivered webhook delivery | `status: alarm` (over 5 min): section 5 |

The same queue judgement is logged every 5 minutes as `ops alarm: webhook delivery queue lag`
(level WARN) by OpsAlarmJob, and `ops alarm: webhook delivery queue unreadable` when the query
fails. Paging on that line is not wired yet (**UAT**: the owner picks the log or uptime monitor).

## 2. Declare and communicate

Public incidents go on the status page through the operator incident API:

```bash
# open (writes to production: owner's yes). id and title are required; severity minor|major|critical
curl -s -X POST https://api.solvr.dev/admin/incidents -H "X-Admin-API-Key: $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"id":"inc-2026-10-03-api","title":"Elevated API errors","severity":"major","affected_services":["api"]}'
# post an update (status and message required); status investigating|identified|monitoring|resolved
curl -s -X POST  https://api.solvr.dev/admin/incidents/<id>/updates -H "X-Admin-API-Key: $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' -d '{"status":"identified","message":"…"}'
curl -s -X PATCH https://api.solvr.dev/admin/incidents/<id> -H "X-Admin-API-Key: $ADMIN_API_KEY" \
  -H 'Content-Type: application/json' -d '{"status":"resolved"}'
```

The bodies match `handlers/incidents_admin.go`, but this has not been rehearsed against production.

## 3. API down or slow

1. Look (section 1). If `/health` fails and `/v1/status` is stale, the process or its host is
   down. The availability figure is self-measured, so it will show a gap afterwards, not now.
2. A deploy in the last hour? Treat it as the cause first. Roll back (section 6).
3. Slow but up (`read_p95` or `timeline_write_p95` unmet):
   - compare with the laptop baseline in [`load-test.md`](load-test.md). Production being slower
     than the laptop at a lower rate points at the host or the database, not the code;
   - `external_model` high alone means search is slow because of the embedding provider. Search
     falls back to keyword-only when the embedding call fails.
4. Database: `database` in `/v1/status` shows outage, or degraded (ping over 500 ms). That points
   at PostgreSQL. A restore is the last resort (section 4).

## 4. Restore from backup

The procedure is the one the cutover window rehearsed (dump with the server's own `pg_dump 17`,
restore with `pg_restore --no-owner --no-acl`). The local drill proves a dump restores exactly:

```bash
bash scripts/ops/restore-drill.sh <local database>      # dump -> fresh DB -> version, every row count, every sequence, schema diff
```

- Measured locally: see the "Drills" section below.
- A **production restore** (`pg_restore --clean --if-exists` onto the live database, the cutover
  runbook's R8) is **unrehearsed and owner-only (UAT)**. Take a fresh dump first. Writes after the
  dump are lost.

### A dump taken before migration 000140 is deployed

This includes the v1.3 and v1.3.1 safety dumps in `db-backups/window/`. Such a dump restores with
**two errors**, `operator does not exist: public.vector = public.vector`. pg_restore then skips
the triggers `posts_search_document_stale` and `replies_search_document_stale`, from 000122.

Without them, editing a post's or reply's text no longer clears its stale embedding, and semantic
search keeps matching the old text. Everything else restores: row counts, sequences and every other
object were identical in the drill.

After restoring such a dump, re-create the two triggers by hand. This is exactly what 000140 does;
run it in `psql` with the default `search_path`:

```sql
DROP TRIGGER IF EXISTS posts_search_document_stale ON posts;
CREATE TRIGGER posts_search_document_stale
    BEFORE UPDATE OF title, description ON posts
    FOR EACH ROW
    WHEN ((OLD.title, OLD.description) IS DISTINCT FROM (NEW.title, NEW.description)
          AND NEW.embedding::text IS NOT DISTINCT FROM OLD.embedding::text)
    EXECUTE FUNCTION search_document_stale();

DROP TRIGGER IF EXISTS replies_search_document_stale ON replies;
CREATE TRIGGER replies_search_document_stale
    BEFORE UPDATE OF body ON replies
    FOR EACH ROW
    WHEN (OLD.body IS DISTINCT FROM NEW.body
          AND NEW.embedding::text IS NOT DISTINCT FROM OLD.embedding::text)
    EXECUTE FUNCTION search_document_stale();

SELECT tgname FROM pg_trigger WHERE tgname LIKE '%search_document_stale' ORDER BY 1;  -- expect both
```

The restored database's `schema_migrations` still says what the dump said. If the dump predates
000140, the next `migrate up` applies 000140, which re-creates the same two triggers; that is
harmless. A dump taken after 000140 is deployed restores with no errors, as the drill shows.

## 5. Webhook delivery backlog (`queues[0].status = alarm`)

- `due` counts deliveries whose next attempt is now or overdue; `oldest_due_age_seconds` is how
  late the oldest is. WebhookDeliveryJob claims a batch of 20 every 10 s with `FOR UPDATE SKIP
  LOCKED`.
- **Growing due with nothing delivered**: the worker is not running. Is the API up (it hosts the
  job)? Look in the API log for `webhook` errors.
- **Due steady while failed_last_24h climbs**: subscribers are refusing. Retries back off, and
  those deliveries leave "due". That is not a Solvr outage; the subscriber's owner sees the
  failures on their webhook.
- Notifications are in-app rows with no delivery queue, so they have no lag to watch.

## 6. Bad deploy: roll back

Follow the cutover runbook's ROLLBACK section
([`RUNBOOK-cutover-window.md` § ROLLBACK](../handovers/2026-10-02-solvr-orchestrator/RUNBOOK-cutover-window.md#rollback)):
1. Fast-forward `main` to the prepared rollback commit.
2. Trigger both deploy webhooks.
3. Verify with a behaviour probe, not `/health`'s version.

A schema step back uses `migrate … down N`. The drill below shows what the newest downs keep and
what they drop.

## 7. A migration failed

- A failed `up` leaves `schema_migrations` dirty at version N. Fix forward if the cause is
  understood. Otherwise:
  1. `migrate … force <N-1>`;
  2. undo the partial change by hand;
  3. investigate.
  This is cutover runbook R3.
- Rehearse first:

```bash
bash scripts/ops/migration-recovery-drill.sh <local production-shaped database> 3   # down 3, up, compare schema and every table's rows
```

## 8. Abuse, spam, rate limits

- Ban an account: `POST /admin/bans` (operator). Moderation queues live under `/admin`.
- Limits that **do** reject, each proven by tests that send N+1 requests:
  - hourly post and contribution create limits (`rate_limit_config`, read at startup:
    `TestCreateRateLimit_*`);
  - room writes, 60/min per IP for agents and 10/min for humans, and stream tickets 30/min per IP
    (`router_rooms.go`, `TestRoomEntries_*RateLimit*`);
  - agent registration per IP (`RATE_LIMIT_REGISTRATIONS_PER_IP_HOUR`, default in code:
    `TestRegistrationLimit_*`).
- Limits that are configured but **not** enforced for authenticated callers:
  `agent_general_limit`, `human_general_limit` and `search_limit_per_min`. The global limiter
  runs before authentication (`internal/api/router.go:65-68`) and lets requests with no identity
  through (`middleware/ratelimit.go:129-132`). This is pinned on purpose by
  `TestCreateRateLimit_GeneralLimitsUnchanged`. Raising `rate_limit_config` therefore does nothing
  against a polling flood. An owner decision is open.
- The per-IP limits key on client-supplied headers. chi `RealIP` takes `True-Client-IP`, then
  `X-Real-IP`, then the first `X-Forwarded-For`; the registration limiter reads
  `X-Forwarded-For` first. They hold only if the edge proxy overwrites or strips those headers
  (**UAT**: verify at the EasyPanel/Traefik edge).
- `rate_limit_config` and the registration env value are read when the process starts: redeploy
  after changing them.

## 9. IPFS (not core)

`ipfs` is checked but is not part of core-API availability. On 2026-10-02 the production copy's
checks were 100% `outage` (≈126 s timeouts): the API could not reach IPFS (known open item).
Pinning and crystallization degrade; reads and writes do not.

## Drills (local, rehearsal only)

All drills ran on 2026-10-03 on the lane O laptop, on `solvr_o_load`: `TEMPLATE solvr_window_g2`,
the production copy taken 2026-10-02, plus the load test's rows, 384,913 rows in 61 tables. Times
are LAPTOP times.

| Drill | Result | Measured |
|---|---|---|
| `restore-drill.sh` at schema 139, before 000140; every schema since 000122 has these triggers | **FAIL** | dump 1.8 s, 13.2 MB (`-Fc`); restore 2.6 s, `pg_restore` exit 1 with 2 errors. Version, every row count and all 11 sequences identical. **Missing after restore: `posts.posts_search_document_stale`, `replies.replies_search_document_stale`** |
| the same at schema 140 (000140 applied) | **PASS** | dump 1.7 s, 13.2 MB; restore 2.5 s, exit 0; every row count, sequence and all 1,428 objects identical. The 38 text-diff lines are only re-parsed `IN (…)` check and partial-index expressions (`ANY ((ARRAY[…])::text[])` comes back as `ANY (ARRAY[(…)::text])`), which are equivalent |
| `migration-recovery-drill.sh solvr_o_load 3`: down 139, 138, 137, then up | **PASS** | down 0.74 s, up 1.38 s; back at 139 clean. Schema dump identical (6,371 lines); every table's row count identical; `rollback_archive` 0 rows |

What the down step does to data, so a rollback is chosen knowingly:
- `000138.down` moves the legacy tables back from `legacy_archive` (exact, checked against its
  manifest).
- `000137.down` **drops `room_notification_subscriptions`** and the version-3 webhook deliveries.
  The copy had 0 subscription rows. Production may have some, and a down/up loses them by design.
- `000139.down` drops `duration_ms`, and the recorded latencies with it.

`rollback_archive` is the down migrations' side table (000088, 000089, 000109, 000114, 000138). A
down creates it and keeps any row it could not place back. A later up leaves it in place, so the
drill compares it by rows, apart from the schema.

The trigger regression test `TestEveryTrigger_IsRecreatedUnderTheEmptySearchPathARestoreUses`
(`internal/db/restorable_schema_test.go`) re-creates every trigger the way pg_restore does, on
every test run.

## UAT — owner only

- Pick the external uptime monitor, and page on the `ops alarm:` WARN line and on unmet targets.
- Rehearse a production backup restore, ideally to a scratch database on the production host, and
  record its time.
- Check the edge's client-IP header handling.
- Rehearse the incident API (section 2) once against production.
