# Connectivity incident handoff

Checkpoint: **2026-10-23 02:36 UTC** (2026-10-22 23:36 in São Paulo).
These are observed session results, not ongoing monitoring. Revalidate before acting.

## Current outcome and remaining work

The API and frontend fixes are deployed. The user reports the webpage feels fast. Latest API
status is operational; five fresh normal-DNS health requests all returned 200 in 0.16–0.70 s.
All five selected a recovered IPv6 address. The external routing issue has **partly recovered**:

| Advertised API address | Latest affected-network result |
|---|---|
| `104.21.51.232` | HTTP 200, about 0.15 s |
| `172.67.190.162` | TCP connection timed out in three checks, about 5 s each; no TLS/HTTP response |
| `2606:4700:3034::6815:33e8` | HTTP 200, about 0.14 s |
| `2606:4700:3034::ac43:bea2` | HTTP 200, about 0.15 s |

The latest address-by-address recheck covered the API. Do not present it as a fresh complete
frontend/browser verification. Earlier checks of both hostnames found the same advertised
addresses and failure pattern. No current full-recovery claim is justified by five fast requests.

**DNS was never changed.** Both apex and API remained Cloudflare-proxied. The operator exported
the DNS records and staged the API A record as DNS-only with TTL 300, but never saved it. There
were no AAAA records in the exported zone; Cloudflare supplied the advertised IPv6 addresses.
The existing origin destination was unchanged in the form. Leave unrelated records alone.

The user authorized investigation, Hostinger access, fixes/deployments and a temporary bypass
plan. At the final browser save, the computer-use policy separately required action-time
confirmation for materially reducing edge protection, even with prior approval. An explicit
confirmation was requested and not received. Later messages about speed and saving context
were not that confirmation. The original incident goal was marked blocked on this remaining
decision, not complete. Do not interpret closing/reopening the session as approval or a reason
to redeploy. Re-evaluate the need for a bypass now that IPv6 has recovered, and follow the
current user's authorization and applicable tool rules. Do not blindly save an old editor.

## Root cause and introduction history

Initial supplied evidence, **2026-10-22 22:39–22:45 UTC**, showed only one of four Cloudflare
destinations reachable from Claro Brazil (AS28573). Both hostnames suffered delayed fresh
connections. The same destinations worked from Hostinger in São Paulo. Repeated checks
confirmed address-specific failure before TLS; normal clients waited and fell back to a working
destination. A fast browser can reuse a working connection; that explanation was an inference,
not a browser transport trace.

The origin, API and database were healthy. The evidence locates the fault on the affected
client-to-Cloudflare path. It does not establish whether Claro, a transit operator or Cloudflare
loses/filters traffic. Missing traceroute responses do not prove the dropping operator. Exact
outage onset and the external change that introduced it are unknown; do not blame an app commit.

Two application issues worsened handling of the incident:

- `ea980f69` (2026-10-01): canonical shell watcher hid curl failures and could return success
  after a network error. Legacy requests/streams also lacked adequate connection deadlines.
- `d18c79fb` (2026-10-03): unconditional Cloudflare client-IP header trust was unsafe for direct
  ingress. A bypass needed correct visitor attribution before it could preserve rate-limit buckets.

At 00:29 UTC, normal API probes returned 9/10 HTTP 200, with a median about 2.8 s. The failed
sample was curl exit 28 after its **5-second connection limit**, with no completed TCP/TLS or
HTTP response. That diagnostic sample does not establish a 10% production failure rate. By
02:36 UTC both IPv6 destinations worked and normal requests were fast, while one IPv4 still failed.

## What shipped

| Component | Production revision / version |
|---|---|
| API | `b251f95ad97e43bb49ebabfe5db9bb8a08bad45a`, PR [#12](https://github.com/fcavalcantirj/solvr/pull/12) |
| Web | `47b44aa1bb46b1b0d38eea5acb584093a3fa105d`, PR [#11](https://github.com/fcavalcantirj/solvr/pull/11), web 0.4.21 |
| Agent skill | 4.0.1, published installer/manifest/docs/ZIP; local existing installation upgraded |
| Release journal | v1.3.24 entry in `progress.txt`; checkpoint commit `297a3861` |

No database migration was needed. API env `TRUSTED_PROXY_CIDRS=10.11.0.0/16` was added at
2026-10-22 23:58:56 UTC; the other 22 existing values were preserved and compared by content.
The API port remains unpublished. Local skill credential files were compared and unchanged.

Client requests now have connect 5 s / ordinary total 30 s deadlines. SSE uses a connect
deadline and a 90 s idle deadline refreshed by incoming lines, including heartbeat comments,
rather than a maximum lifetime. Failures propagate; cancellation reaps curl and FIFO resources.
Overrides: `SOLVR_CONNECT_TIMEOUT`, `SOLVR_REQUEST_TIMEOUT`, `SOLVR_STREAM_IDLE_TIMEOUT`.
The Bash 3.2 timeout/EOF distinction is covered. No address pinning or retries were added.

**Important production discovery:** EasyPanel sets Traefik `forwardedHeaders.insecure=true`
through container environment variables. Inspecting only service args/command missed this.
The first release assumed sanitized `X-Real-IP`; live verification caught the spoof before DNS
changed. PR #12 corrects it: verify the configured socket proxy, then use only the last
`X-Forwarded-For` address Traefik appends from its actual peer. Earlier addresses and caller
`X-Real-IP` are ignored. Only a verified Cloudflare peer may supply a single valid
`CF-Connecting-IP`. Invalid/duplicate applicable headers fail closed to verified addresses.
Live direct/Cloudflare attribution agreed and CF, proxy and duplicate forwarding spoofs were
ignored. Global proxy configuration and unrelated services were not changed.

## Validation and its limits

- Both PRs passed all four CI jobs: backend, frontend, agent skill and file-size checks.
- Local frontend production build/typecheck, middleware race tests and changed-package Go lint
  passed. Existing shell checks: 286; new transport regressions: 11. The follow-up includes a
  real reverse-proxy regression that failed before the fix and passed afterwards. Database-backed
  registration rate-limit checks passed. Served skill/install/ZIP bytes matched the release.
- The database-backed local API package was not wholly green: `TestRoomRoutes_ListRooms`
  expected a freshly created unlisted room in public discovery. The identical failure was
  reproduced on unchanged base `1a9dfaa5`. Other full-suite DB failures were one timing bound
  and three referral-code fixture constraints, outside the changed packages. Do not claim the
  complete database-backed suite passed merely because CI was green.
- An initial full API test run used an incompatible custom JWT secret and produced many
  invalid 401 failures. The corrected API run omitted `JWT_SECRET`, matching fixture defaults,
  and used `IPFS_API_URL=http://127.0.0.1:1` for expected offline behavior. If repeating integration
  checks, use an isolated local migrated scratch DB and an explicit clean environment; never
  source production `.env` into test commands. Saved logs were read by tail/failing assertions.
- Direct-origin validation passed 10/10 per hostname, p95 0.57 s site / 0.17 s API; 18/18 key
  pages and sitemaps; 19/19 homepage JS/CSS assets; validated HTTPS and HTTP-to-HTTPS redirects;
  two public SSE heartbeat comments over 65 s. Earlier transient TCP delays and one asset
  failure were followed by the complete passing check. These were direct-origin controls,
  not evidence that normal-DNS routing had recovered.
- At 00:25 UTC, 81 recorded API requests since 00:18 had zero server errors. This is a bounded
  historical observation, not a current uptime guarantee. Latest 02:36 status was operational.

## Active-room count follow-up

Independent production SQL and API checks at 00:22 UTC agreed: one room / 116 non-system
messages over 24 h; six rooms over 7 d; 33 rooms over 30 d. The hero intentionally stays on
**last 24 h** even when the lower sections select 7 d or 30 d. Those sections correctly returned
1 / 6 / 33. The count was not hardcoded or stale legacy storage: `messages` is a view over
canonical `room_entries` of kind `message`. No private room names or content were needed.

Relevant code: `backend/internal/db/homepage_rooms.go`,
`backend/internal/api/handlers/homepage_hero.go`, and migration `000094`'s messages view.

## Durable evidence and safe resumption

Private incident artifacts were copied from `/tmp/solvr-connectivity-release/` to
**`../solvr-ops/incidents/2026-10-23-connectivity/`**, outside this repo, with owner-only
permissions. Required snapshot/evidence bytes were compared after copying. `/tmp` and an open
browser are conveniences only; neither is required to recover session context.

- `api-current.json`: latest 02:36 measurements, including two rechecks of the failing IPv4.
- `original-report.txt`: the original user-supplied report, preserved from the session attachment.
- `recovery-claro.json`, `recovery-hostinger.json`: earlier 00:29 comparative probes.
- `provider-evidence.md`, `provider-*-current.json`: prepared provider report and 00:25-era
  comparison. The report predates IPv6 recovery; update it from `api-current.json` before sending.
  **No provider ticket or message was sent.**
- `api-before.json`, `web-before.json`, `dns-before.txt`: private rollback snapshots. Panel
  JSON contains the entire env and deployment-hook secrets: never print, commit or upload it.
- `verify-ingress.py`, `ingress-result.json`: marked live spoof probe and passing result.
- `origin-*.json`, `room-overview-check.json`: origin controls and aggregate counter evidence.
- `installed-skill-before/`: backup of the previous local skill installation.

Operator helpers are in `../solvr-ops/api-tools/`: `ep.sh` filters EasyPanel results, and
`ep_env_add.sh` preserves existing env text, checks duplicates, and verifies saved content.
`updateAppEnv` replaces the complete env; do not send a guessed subset. For revision checks,
filter `.commit.sha` only. Never print whole panel responses, `.env`, service env values or hook
URLs. Operator credentials remain in the git-ignored root `.env` (names documented in
`CLAUDE.md`). Deployments used the existing `SOLVR_DEPLOY_API` / `SOLVR_DEPLOY_WEB` hooks;
SSH was used read-only. The `solvr-prod` SSH alias points at a different provider, not the
Hostinger origin; use verified private operator details rather than that alias.

GitHub's default account only has read access to this repo. The existing owner credential is
`fcavalcantirj`; use scoped child-process credentials when needed, without changing the default
account or putting tokens in command arguments/output. Do not rely on `/tmp/solvr-owner-gh.py`
surviving; its durable private copy is `owner-gh.py` in the evidence directory. It obtains the
existing credential using `gh auth token --user fcavalcantirj`, rather than containing a token.
Some saved probe helpers retain `/tmp/solvr-connectivity-release/` output paths; recreate that
directory or adjust the paths before reuse. Read the helpers and revalidate targets first.

Next useful check, already authorized read-only:

```sh
python3 scripts/ops/probe-connectivity.py --attempts 10 > /tmp/solvr-connectivity-recheck.json
```

Run the same probe from Hostinger and compare per-address TCP/TLS plus normal-DNS timings.
`/health` is the API health endpoint; `/v1/health` is not. A fresh curl process avoids mistaking
connection reuse for recovered routes. Before closing the external routing incident, require
every currently advertised destination to pass three checks five minutes apart from both
networks, plus page/assets/API/SSE behavior. If recovery holds, a DNS bypass may be unnecessary.
For any later bypass, follow the [runbook](../ops/incident-runbook.md#regional-connection-timeouts-2026-10-22),
re-export/re-read current DNS, use existing destinations and TTL 300, preserve unrelated DNS,
TLS and firewall settings, and verify ten ordinary-DNS successes per host/network with p95
below one second. Recheck current policy/confirmation requirements at the final action.

Separate open bug: `/blog/tags` can hit dynamic `/blog/[slug]`, receive a tag array from
`/v1/blog/tags`, then dereference `raw.author.display_name`; the SSR path dates to `c46f1d29`
(2026-02-28). It was observed but not fixed or claimed as the cause of pre-TLS failures.

Keep existing unrelated untracked folders under `docs/handovers/2026-10-04-seo-360-recon/`
(`evidence`, `harness`, `patches`, `preview-options`) untouched. Do not stage or delete them.
