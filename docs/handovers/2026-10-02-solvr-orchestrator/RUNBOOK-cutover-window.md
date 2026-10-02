# Runbook — Solvr v1.3 cutover window

Production runs `c03734ae` at schema 84 (no `schema_migrations`). This window moves it to the frozen v1.3 commit:
schema 135, the knowledge cutover, and both services redeployed. Rehearsed end to end on a local stand-in
(lane D, D6c); the measured numbers are in each step and in "Dress rehearsal results".

## Rules of the window

- **One gate at a time.** Each block below is run by itself, by the orchestrator, after Felipe's explicit "yes"
  for that gate. A yes for one gate never carries to the next. There is no run-all script, on purpose.
- **Production and the rehearsal run the same commands.** Only three shell values and the env file differ
  (table below). If a command has to be edited to work, that is a runbook defect: fix it here first.
- **bash only.** Felipe's login shell is zsh, which splits words differently; type `bash` first.
- **Secrets by name only.** The env file is never `source`d: `window/load-env.sh` reads only named keys and
  never evaluates a value. Nothing here prints a password or the database URL. `$DBURL` is visible in `ps`
  while `migrate` or the cutover runs (single-user Mac: accepted). Never `set -x`.
- **Between G1 and G6 nobody clicks Deploy in EasyPanel.** After G1, GitHub `main` is v1.3; v1.3 against
  schema 84 is broken (rooms read dropped columns; contribution counters read 0).
- Test-output rule: every report goes to a file under `$BACKUP_DIR`; read summaries, never stream whole logs.

## The three shell values and the env file

| | Production | Rehearsal (dress) |
|---|---|---|
| `WINDOW_ENV` | `/Users/fcavalcanti/dev/solvr/.env` (never edited) | `/tmp/solvr-lane-d/d6/standin.env` |
| `FROZEN_SHA` | DECIDED 2026-10-02 (owner): the lane D freeze head (D7), recorded in the D7 report and the journal | the D7 freeze head |
| `WINDOW_DIR` | `/Users/fcavalcanti/dev/solvr-window` | `/Users/fcavalcanti/dev/solvr-lanes/lane-d-window` |

Keys read from `WINDOW_ENV` (names only): `SOLVR_DB_HOST SOLVR_DB_PORT SOLVR_DB_USER SOLVR_DB_PASSWORD
SOLVR_DB_NAME SOLVR_DEPLOY_API SOLVR_DEPLOY_WEB ADMIN_API_KEY`, plus optional overrides that production's
`.env` does not have, so the defaults apply: `SOLVR_API_BASE` (https://api.solvr.dev), `SOLVR_WEB_BASE`
(https://solvr.dev), `BACKUP_DIR` (`/Users/fcavalcanti/dev/solvr/db-backups/window`, git-ignored),
`RESTORE_TEST_DB` (`solvr_window_g2`), `LOCAL_PG` (the local `solvr-postgres` container on 5435).
`SOLVR_DB_SSLMODE` is derived from the server (`SHOW ssl`): `require` when on, else `disable`
(`migrate` uses lib/pq, which accepts only those two).

**Prelude: every gate block starts with these two lines** (the values from the table):

```bash
export WINDOW_ENV=/Users/fcavalcanti/dev/solvr/.env FROZEN_SHA=<frozen sha> WINDOW_DIR=/Users/fcavalcanti/dev/solvr-window
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
```

It prints `loaded: <names>` and one `window:` line (sha, database name, sslmode, API and web bases, backup
dir), and defines `PSQL` (read-write), `PSQL_RO` (`default_transaction_read_only=on`), `PGDUMP`, `LPSQL`
(local container), `DBURL` (production, percent-encoded password), `RTDBURL` (the local restore-test copy),
`RB` (this runbook's `window/` folder) and `BIN`. It refuses to run when `WINDOW_DIR` is not at
`FROZEN_SHA`.

## Support files (`window/`)

`load-env.sh` (non-evaluating loader), `window-env.sh` (prelude), `markers.sql` (P1), `sessions.sql` (G3),
`legacy-state.sql` (G3 snapshot and rollback check), `reconcile.sql`, `contrib-breakdown.sql` and
`trigger-probe.sql` (G5), `cutover-summary.jq`, `compare-legacy.jq`, `probes.sh` (G6, R6),
`secret-scan.pl` (G1), `xacts.sql` (rehearsal round-trip counter).

---

## PRE-0 — window worktree (no gate; local)

Measured (dress): 0.8 s.

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude (first line only: the worktree does not exist yet)
git -C /Users/fcavalcanti/dev/solvr worktree add --detach "$WINDOW_DIR" "$FROZEN_SHA"
git -C "$WINDOW_DIR" log --oneline -1
```

Expect the frozen commit. Abort if the worktree exists at another commit (remove it, never reuse it).

## PRE-1 — both images build at FROZEN_SHA, the cutover binary, its version guard (no gate; local)

Measured (dress): 95.8 s in all: api image 11.9 s (warm layer cache), web image 80.4 s, cutover build and guard test about 3.5 s.

A failed image build is a **window blocker**: EasyPanel deploys images built from these exact Dockerfiles.

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
/usr/bin/time -p docker build -t "solvr-window/api:$FROZEN_SHA" "$WINDOW_DIR/backend" > "$BACKUP_DIR/pre1-build-api.log" 2>&1; echo "api build exit $?"; tail -n 3 "$BACKUP_DIR/pre1-build-api.log"
/usr/bin/time -p docker build --build-arg NEXT_PUBLIC_API_URL="$SOLVR_API_BASE" -t "solvr-window/web:$FROZEN_SHA" "$WINDOW_DIR/frontend" > "$BACKUP_DIR/pre1-build-web.log" 2>&1; echo "web build exit $?"; tail -n 3 "$BACKUP_DIR/pre1-build-web.log"
(cd "$WINDOW_DIR/backend" && go build -o "$BIN/cutover" ./cmd/cutover && go test -count=1 ./cmd/cutover/ > "$BACKUP_DIR/pre1-cutover-test.log" 2>&1; echo "cutover test exit $?"; tail -n 2 "$BACKUP_DIR/pre1-cutover-test.log")
ls "$WINDOW_DIR"/backend/migrations/*.up.sql | tail -n 1
grep -o 'expect-version", [0-9]*' "$WINDOW_DIR/backend/cmd/cutover/main.go"
```

Expect: both builds exit 0; `cutover test exit 0` (`TestParseOptions_ExpectsTheHighestMigrationByDefault`
pins the default to the newest migration); the newest file is `000135_…` and the default is `135` (or both
the same newer number). Abort on any failure.

## PRE-2 — the rollback commit exists and is exactly c03734ae's tree on top of FROZEN_SHA (no gate; local)

DECIDED 2026-10-02 (owner): rollback route (c), a tree-restore commit. It is built at the freeze, with git
plumbing and no checkout, and stays local until R6:
`R=$(git commit-tree 'c03734ae^{tree}' -p "$FROZEN_SHA" -m "revert(release): restore the c03734ae tree (v1.3 rollback; parent = frozen $FROZEN_SHA)")`,
then `git branch rollback/v1.3-to-c03734ae "$R"`.

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
git -C /Users/fcavalcanti/dev/solvr rev-parse --verify rollback/v1.3-to-c03734ae
[ "$(git -C /Users/fcavalcanti/dev/solvr rev-parse 'rollback/v1.3-to-c03734ae^')" = "$(git -C /Users/fcavalcanti/dev/solvr rev-parse "$FROZEN_SHA")" ] && echo "parent = FROZEN_SHA" || echo "PARENT MISMATCH"
git -C /Users/fcavalcanti/dev/solvr diff --quiet c03734ae rollback/v1.3-to-c03734ae && echo "tree = c03734ae" || echo "TREE MISMATCH"
```

Expect the rollback commit's SHA, `parent = FROZEN_SHA`, `tree = c03734ae`. Abort on either mismatch:
rebuild the branch with the two commands above (`git branch -f`), then run PRE-2 again.

## P1 — read-only checks on production (gate P1)

Measured (dress): 1.0 s, 14 transactions; local RTT through the container route 5–26 ms.

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
"${PSQL_RO[@]}" -At < "$RB/markers.sql" > "$BACKUP_DIR/p1-markers.json"; jq -c . "$BACKUP_DIR/p1-markers.json"
"${PSQL_RO[@]}" -At < "$RB/legacy-state.sql" > "$BACKUP_DIR/p1-legacy-state.json"
jq -c '{counts, legacy_public_contributions, tombstones: [.tombstones[] | .kind + ":" + .name + "@" + .deleted_at], trigger: .trigger.exists}' "$BACKUP_DIR/p1-legacy-state.json"
printf '\\timing on\nSELECT 1;\nSELECT 1;\nSELECT 1;\nSELECT 1;\nSELECT 1;\n' | "${PSQL_RO[@]}" -At | grep '^Time'
```

Expect:
- `ssl` on or off (this decides `SOLVR_DB_SSLMODE`); `server_version` 17.x; `collation` datcollversion
  2.41 vs actual 2.36 (the P5 mismatch) — or equal, in which case P5 (d)/(e) are optional;
- `schema_migrations_present: false`; every `markers_84_present` true; every `markers_85_plus_present`
  false, including `tags` and `post_tags` (production never had them);
- `rate_limit_config` with its 8 keys; `other_sessions` lists the running API's connections;
- tombstones: 6 users and 5 agents, the purge subset (4 users, 3 agents) at
  `2026-09-29 22:23:53.494759+00`; trigger `true`;
- the five `Time:` lines are the round-trip time (RTT) used in the write-pause estimate.

Abort if any 85+ marker is true, `schema_migrations` exists, or a tombstone is missing. **STOP — report to
Felipe.**

## G1 — push the frozen SHA (gate G1)

Measured (dress): scan 39.6 s (fetch, 314 commits, 152 masked rows, the same 18 unhinted rows as D4); push not rehearsed.

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
git -C /Users/fcavalcanti/dev/solvr fetch origin
git -C /Users/fcavalcanti/dev/solvr rev-list --count origin/main.."$FROZEN_SHA"
git -C /Users/fcavalcanti/dev/solvr log -p origin/main.."$FROZEN_SHA" | perl "$RB/secret-scan.pl" > "$BACKUP_DIR/g1-scan.tsv"
wc -l < "$BACKUP_DIR/g1-scan.tsv"
awk -F'\t' '$7=="-" && $8=="-" {print $1, $2, $3, $5, $6}' "$BACKUP_DIR/g1-scan.tsv"
```

The scan prints masked tokens only. Expect every unhinted row to be one already reviewed (lane D, D4/D5:
fixtures, the GitHub docs example token, journal placeholders, the local OpenAPI example room token) and no
new one. Abort on any unreviewed row: look at it (masked) before anything is pushed.
**STOP — Felipe's yes to push.**

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
git -C /Users/fcavalcanti/dev/solvr push origin "$FROZEN_SHA:refs/heads/main"
git -C /Users/fcavalcanti/dev/solvr ls-remote origin refs/heads/main
```

Expect a fast-forward and `ls-remote` = `FROZEN_SHA`. Abort on a rejected push: never force.
The rollback branch is not pushed here (route (c): it reaches GitHub only in R6, as a fast-forward of `main`).

**From here until G6: nobody clicks Deploy in EasyPanel.**

## G2 — fresh dump and restore-test (gate G2: reads production; writes only locally)

Measured (dress): 60.3 s: dump and restore about 5 s, `up` 2.6 s, three cutover passes about 45 s, reconciliation under 1 s.

The 2026-09-29 route: `pg_dump` 17 inside the `solvr-postgres` container (the host's pg_dump is 18).

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
NAME=g2-$(date +%Y%m%d-%H%M%S)
"${PGDUMP[@]}" -f "/tmp/$NAME.dump"; echo "dump exit $?"
docker cp "solvr-postgres:/tmp/$NAME.dump" "$BACKUP_DIR/$NAME.dump"
shasum -a 256 "$BACKUP_DIR/$NAME.dump" | tee "$BACKUP_DIR/$NAME.dump.sha256"
docker exec solvr-postgres pg_restore --list "/tmp/$NAME.dump" | grep -c 'TABLE DATA'
docker exec solvr-postgres createdb -U solvr "$RESTORE_TEST_DB"
docker exec solvr-postgres pg_restore -U solvr -d "$RESTORE_TEST_DB" --no-owner --no-acl "/tmp/$NAME.dump"; echo "restore exit $?"
docker exec solvr-postgres rm -f "/tmp/$NAME.dump"
migrate -path backend/migrations -database "$RTDBURL" force 84
migrate -path backend/migrations -database "$RTDBURL" up 2>&1 | tail -n 2
"${LPSQL[@]}" -d "$RESTORE_TEST_DB" -Atc 'SELECT version, dirty FROM schema_migrations'
"$BIN/cutover" --database-url "$RTDBURL" --dry-run --report "$BACKUP_DIR/g2-dry.json" > "$BACKUP_DIR/g2-dry.log" 2>&1; echo "dry exit $?"
"$BIN/cutover" --database-url "$RTDBURL" --confirm-prod --report "$BACKUP_DIR/g2-apply.json" > "$BACKUP_DIR/g2-apply.log" 2>&1; echo "apply exit $?"
"$BIN/cutover" --database-url "$RTDBURL" --confirm-prod --report "$BACKUP_DIR/g2-second.json" > "$BACKUP_DIR/g2-second.log" 2>&1; echo "second exit $?"
for r in dry apply second; do jq -c -f "$RB/cutover-summary.jq" "$BACKUP_DIR/g2-$r.json"; done
"${LPSQL[@]}" -d "$RESTORE_TEST_DB" -At < "$RB/reconcile.sql" > "$BACKUP_DIR/g2-reconcile.json"
jq -c '{schema, replies_by_legacy_type, orphans: (.orphans | length), drift, cutover_ledger}' "$BACKUP_DIR/g2-reconcile.json"
```

Expect: `TABLE DATA` 42; restore exit 0; `135|f`; dry/apply/second exit 0; second `all_zero: true`;
drift all 0 except `search_document_drift` (rows awaiting embeddings). On the 2026-09-29 data the apply
created 883 replies and 135 progress notes, with 22 orphan system comments on hard-deleted posts; today's
numbers differ by the writes since then and are the G5 reference. Abort on any non-zero exit, a
`post_exceptions` > 0, an unexplained orphan, or `all_zero` false on the second pass. **STOP — report.**

## G3 — write pause: stop solvr-api (gate G3)

Measured (dress): the stop (simulated with `docker stop`) 0.3 s; the block 1.8 s including the safety dump.

**Felipe, in EasyPanel:** `solvr-api` → Stop. Note the time: the write pause starts here.
`solvr-web` keeps running and shows errors until G6 (DECIDED 2026-10-02 (owner): solvr-web keeps running during the pause).

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
curl -s -o /dev/null -w 'api /health %{http_code}\n' --max-time 10 "$SOLVR_API_BASE/health"
"${PSQL_RO[@]}" < "$RB/sessions.sql"
NAME=g3-safety-$(date +%Y%m%d-%H%M%S)
"${PGDUMP[@]}" -f "/tmp/$NAME.dump"; echo "dump exit $?"
docker cp "solvr-postgres:/tmp/$NAME.dump" "$BACKUP_DIR/$NAME.dump" && docker exec solvr-postgres rm -f "/tmp/$NAME.dump"
shasum -a 256 "$BACKUP_DIR/$NAME.dump" | tee "$BACKUP_DIR/$NAME.dump.sha256"
"${PSQL_RO[@]}" -At < "$RB/legacy-state.sql" > "$BACKUP_DIR/g3-legacy-state.json"
jq -c '{counts, legacy_public_contributions, tombstones: (.tombstones | length), trigger: .trigger.exists}' "$BACKUP_DIR/g3-legacy-state.json"
```

Expect `/health` not 200 (502/503 behind the proxy; `000` when nothing listens); `sessions.sql` shows no
API session (only short-lived sessions of these commands, which it excludes for itself); dump exit 0.
The safety dump is mandatory (DECIDED 2026-10-02 (owner): yes): it is the exact pre-cutover state and what makes
a lossless restore possible (ROLLBACK R8). Abort if the API still answers 200 or still holds sessions, or if the
dump does not exit 0. **STOP — Felipe's yes for P5.**

## P5 — collation repair inside the write pause (gate P5; `RUNBOOK-collation.md`)

Measured (dress): (a)+(b) 0.4 s; (d)+(e) 1.6 s (production on the 2026-09-29 pre-purge data: REINDEX DATABASE 3.7 s).

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
"${PSQL_RO[@]}" -c "SELECT datname, datcollversion, pg_database_collation_actual_version(oid) FROM pg_database"
"${PSQL_RO[@]}" < "$WINDOW_DIR/docs/handovers/2026-09-29-solvr-anti-abuse/collation-dup-precheck.sql"
```

Expect (a) the database's `datcollversion` 2.41 against 2.36; (b) `NOTICE: duplicate pre-check: 34 unique
indexes checked, 0 duplicate key groups`. **Any `WARNING: DUPLICATES`: STOP** (what to keep is Felipe's call).
Optional (c), its own yes, a write (`CREATE EXTENSION amcheck`):
`"${PSQL[@]}" < "$WINDOW_DIR/docs/handovers/2026-09-29-solvr-anti-abuse/collation-amcheck.sql"` — expect `0 failed`.
**STOP — Felipe's yes for (d)+(e).**

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
"${PSQL[@]}" -v db="$SOLVR_DB_NAME" <<< 'REINDEX DATABASE :"db";'
"${PSQL[@]}" -v db="$SOLVR_DB_NAME" <<< 'ALTER DATABASE :"db" REFRESH COLLATION VERSION;'
"${PSQL_RO[@]}" -c "SELECT datname, datcollversion, pg_database_collation_actual_version(oid) FROM pg_database"
```

Expect `REINDEX`, then `NOTICE: changing version from 2.41 to 2.36` (or `version has not changed`), then no
mismatch for the database. Production measured on 2026-09-29 data: REINDEX DATABASE 3.7 s.

## G4 — schema 84 → head (gate G4; includes P6: 000114 ban list and trigger)

Measured (dress): 3.0 s (`up` 2.54 s for 51 files), 192 transactions.

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
"${PSQL[@]}" -Atc 'SELECT 1'
migrate -path backend/migrations -database "$DBURL" force 84; rc=$?; echo "force exit $rc"
[ "$rc" = 0 ] && { /usr/bin/time -p migrate -path backend/migrations -database "$DBURL" up > "$BACKUP_DIR/g4-up.log" 2>&1; echo "up exit $?"; tail -n 4 "$BACKUP_DIR/g4-up.log"; }
"${PSQL_RO[@]}" -Atc 'SELECT version, dirty FROM schema_migrations'
```

`force 84` creates `schema_migrations` on production: the first schema write. Expect `force exit 0`, 51
lines `NNN/u …`, `up exit 0`, `135|f`. **Abort on any non-zero exit or `dirty = t`: go to ROLLBACK, entry
R-G4.** Do not re-run `up` blindly.

## G5 — the knowledge cutover (gate G5)

Measured (dress): dry run 8.1 s (1,831 transactions); apply, second pass, reconciliation and trigger probe 43.5 s (10,941 transactions). The 200-query search sample is most of it (see the write-pause estimate).

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
"$BIN/cutover" --database-url "$DBURL" --dry-run --report "$BACKUP_DIR/g5-dry.json" > "$BACKUP_DIR/g5-dry.log" 2>&1; echo "dry exit $?"
jq -c -f "$RB/cutover-summary.jq" "$BACKUP_DIR/g5-dry.json"; jq -c -f "$RB/cutover-summary.jq" "$BACKUP_DIR/g2-dry.json"
```

The search sample stays at its default, 200 queries, on every pass (DECIDED 2026-10-02 (owner); no flag).
Expect the dry run to match G2's dry run, apart from writes made after the G2 dump. **STOP — Felipe's yes to apply.**

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
"$BIN/cutover" --database-url "$DBURL" --confirm-prod --report "$BACKUP_DIR/g5-apply.json" > "$BACKUP_DIR/g5-apply.log" 2>&1; rc=$?; echo "apply exit $rc"
[ "$rc" = 0 ] && { "$BIN/cutover" --database-url "$DBURL" --confirm-prod --report "$BACKUP_DIR/g5-second.json" > "$BACKUP_DIR/g5-second.log" 2>&1; echo "second exit $?"; }
for r in apply second; do jq -c -f "$RB/cutover-summary.jq" "$BACKUP_DIR/g5-$r.json"; done
"${PSQL_RO[@]}" -At < "$RB/reconcile.sql" > "$BACKUP_DIR/g5-reconcile.json"
jq -c '{schema, legacy, replies_by_legacy_type, orphans: (.orphans | length), unexplained: ([.orphans[] | select(.reason | test("UNEXPLAINED"))] | length), drift, cutover_ledger}' "$BACKUP_DIR/g5-reconcile.json"
"${PSQL_RO[@]}" -At < "$RB/contrib-breakdown.sql" > "$BACKUP_DIR/g5-contrib.json"
jq -c '{new_count, legacy_count_from_tables, legacy_counted_without_reply, delta: (.by_legacy_type | map_values(.delta))}' "$BACKUP_DIR/g5-contrib.json"
"${PSQL[@]}" -q < "$RB/trigger-probe.sql" 2>&1 | grep 'trigger probe'
```

Expect: both exit 0; the second pass `all_zero: true`; `unexplained: 0`; drift 0 except
`search_document_drift`; `legacy_counted_without_reply: 0`, and `new_count` = legacy count + the per-type
deltas (on the 2026-09-29 data: 305 − 26 − 3 + 16 + 126 = 418, the recount Felipe accepted on 2026-09-29);
the trigger probe refuses both tombstoned emails (`REFUSED P0001 account suspended`) and accepts the control.
The probe's inserts are inside `BEGIN … ROLLBACK`: nothing is kept. Abort on a failed pass, a non-zero
`all_zero`, or an unexplained orphan: ROLLBACK, entry R-G5.

## G6 — deploy both services (gate G6)

Measured (dress): 5.9 s with images already built locally; in production EasyPanel builds first (API 1–2 min, web 2–4 min, from memory). One `curl: (7) … Couldn't connect` line before the API is up is expected.

**Before the yes:** Felipe ticks the "Pre-G6 EasyPanel env checklist" below, for both services.

Trigger both webhooks **back to back**: old web against the new API is broken, and so is new web against a
stopped API.

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
curl -fsS -X POST "$SOLVR_DEPLOY_API"; echo; curl -fsS -X POST "$SOLVR_DEPLOY_WEB"; echo
curl -fsS -o /dev/null -w 'api /v1/overview %{http_code}\n' --connect-timeout 5 --max-time 10 --retry 90 --retry-delay 5 --retry-all-errors --retry-max-time 900 "$SOLVR_API_BASE/v1/overview"
for i in $(seq 90); do n=$(curl -s --max-time 10 "$SOLVR_WEB_BASE/skill.md?cb=$(date +%s)$RANDOM" | grep -c 'Connect your agents so they collaborate in a shared room'); echo "$(date +%T) web skill.md v1.3 marker: $n"; [ "$n" -ge 1 ] && break; sleep 10; done
bash "$RB/probes.sh" new "$BACKUP_DIR/g6-probes"
```

Expect `Deploying...` twice; `api /v1/overview 200` (old code answers 404, so 200 means v1.3 serves); the
`skill.md` marker 1 (that sentence is in v1.3's `skill.md` and not in `c03734ae`'s); `probes failed: 0` with
`total_contributions` equal to G5's `new_count`. **The window ends only when both completion probes pass.**
Worst case from memory: API build 1–2 min, web 2–4 min after the trigger. Abort (ROLLBACK, entry R-G6) when
a probe still fails after 15 minutes or `probes.sh` reports a failure.

## G7 — UAT with Felipe (gate G7)

Not rehearsed (human UAT).

Against production, read-only unless Felipe acts himself:
1. Home: live overview (rooms, searches, statistics), the agent-connect prompt; navigation Rooms, Posts, Docs,
   plus DATA (/data) and SKILL (/skill) as top-level header links on desktop and mobile (idx 95).
2. Posts: a migrated problem shows its approaches as replies with provenance; a migrated question shows its
   answers; moderator verdicts are not counted as contributions.
3. Legacy URLs (`/problems/<id>`, `/questions/<id>`, `/ideas/<id>`) redirect to `/posts/<id>`.
4. Search: the five probe queries return results; a reply match surfaces its post.
5. Rooms: ids, slugs and URLs unchanged; transcripts readable; a public room's entries load.
6. Agent journey (idx 94): paste a starter prompt into one agent, get a room link and join prompts, join a
   second agent, watch them exchange; no human account, plugin or MCP server needed.
7. Agents that held a shared room token (`solvr_rm_…`) must rejoin through the handshake (owner decision
   2026-09-30; shared tokens retired at cutover).
8. Sign-in with Google and GitHub; a tombstoned account stays refused.
9. `/v1/stats` contributions equal G5's `new_count`.
Failure on any item that blocks users: Felipe decides between a fix-forward and ROLLBACK (entry R-G7).

## P7 / P8 — IPFS unpin and gc (each its own gate; after G6; `RUNBOOK-ipfs-unpin.md`)

Measured (dress): dry run 0.3 s (`{"would_unpin":87}`); the real run and gc were not rehearsed (no IPFS node; irreversible).

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
CSV="$WINDOW_DIR/docs/handovers/2026-09-29-solvr-anti-abuse/ipfs-unpin-cids.csv"
tail -n +2 "$CSV" | cut -d, -f3 | jq -R . | jq -s '{cids: ., dry_run: true}'  > "$BACKUP_DIR/p7-unpin-dry.json"
tail -n +2 "$CSV" | cut -d, -f3 | jq -R . | jq -s '{cids: ., dry_run: false}' > "$BACKUP_DIR/p7-unpin-run.json"
jq '.cids | length' "$BACKUP_DIR/p7-unpin-dry.json"
curl -s -X POST "$SOLVR_API_BASE/admin/ipfs/unpin" -H "X-Admin-API-Key: $ADMIN_API_KEY" -H "Content-Type: application/json" --data @"$BACKUP_DIR/p7-unpin-dry.json" | tee "$BACKUP_DIR/p7-unpin-dry-report.json" | jq -c .data.summary
```

Expect 87 and `{"would_unpin": 87}`. Any `in_use_*`, `invalid` or `error`: STOP. **STOP — Felipe's yes (P7).**

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
curl -s -X POST "$SOLVR_API_BASE/admin/ipfs/unpin" -H "X-Admin-API-Key: $ADMIN_API_KEY" -H "Content-Type: application/json" --data @"$BACKUP_DIR/p7-unpin-run.json" | tee "$BACKUP_DIR/p7-unpin-report.json" | jq -c .data.summary
```

Expect `unpinned` (or `not_pinned`) for every CID. **STOP — Felipe's separate yes (P8, irreversible).**

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
curl -s -X POST "$SOLVR_API_BASE/admin/ipfs/gc" -H "X-Admin-API-Key: $ADMIN_API_KEY" --max-time 700 | jq -c .
```

Expect `{"data": {"removed": N}}`. The admin key is in `curl`'s argv (visible in `ps`; accepted).

---

## ROLLBACK

Felipe's call at any entry point. Each step is its own block, as above.

| Entry | When | Start at |
|---|---|---|
| R-G4 | `migrate up` failed or left `dirty = t` | R1, then R3, R4 |
| R-G5 | a cutover pass failed or reconciliation is unexplained | R1, R2, R4 |
| R-G6 | a completion probe or `probes.sh` failed | R1, R2, R4 |
| R-G7 | UAT failed and Felipe chose rollback | R1, R2, R4 |

**R1 — stop serving.** Felipe, in EasyPanel: stop `solvr-api` (and `solvr-web` if v1.3 web is deployed). Measured (dress, simulated): 0.2 s.

**R2 — pre-rollback dump** (keeps every post-cutover write, beyond what `rollback_archive` keeps): Measured (dress): 2.0 s.

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
NAME=r2-prerollback-$(date +%Y%m%d-%H%M%S)
"${PGDUMP[@]}" -f "/tmp/$NAME.dump"; echo "dump exit $?"
docker cp "solvr-postgres:/tmp/$NAME.dump" "$BACKUP_DIR/$NAME.dump" && docker exec solvr-postgres rm -f "/tmp/$NAME.dump"
"${PSQL_RO[@]}" -Atc 'SELECT version, dirty FROM schema_migrations'
```

**R3 — only for a dirty schema (R-G4).** Every migration file 000085–000135, up and down, is plain
single-transaction SQL (table below), and `migrate` v4.19.1 runs each file as one simple-protocol batch
(`x-multi-statement` off), which PostgreSQL executes as one implicit transaction. So a failed file left
nothing behind, and `dirty = t` only marks the version:
- after an **up** failure at version N (`N|t`): the database is at N−1 →
  `migrate -path backend/migrations -database "$DBURL" force <N-1>`;
- after a **down** failure while going down (`T|t`, T = the version below the failed file F): the database is
  still at F → `migrate -path backend/migrations -database "$DBURL" force <F>`.

**R4 — schema back to 84:** Measured (dress): 1.0 s (`goto 84` 0.67 s, 51 files).

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
/usr/bin/time -p migrate -path backend/migrations -database "$DBURL" goto 84 > "$BACKUP_DIR/r4-goto84.log" 2>&1; echo "goto exit $?"; tail -n 4 "$BACKUP_DIR/r4-goto84.log"
"${PSQL_RO[@]}" -Atc 'SELECT version, dirty FROM schema_migrations'
```

Expect `goto exit 0`, 51 lines `NNN/d …` (from 135), `84|f`. `schema_migrations`, `rollback_archive` and
`cutover_ledger` stay (old code ignores them). Abort on a failure: R3, then R4 again.

**R5 — compare with the G3 snapshot:** Measured (dress): 0.5 s.

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
"${PSQL_RO[@]}" -At < "$RB/legacy-state.sql" > "$BACKUP_DIR/r5-legacy-state.json"
jq -n --slurpfile a "$BACKUP_DIR/g3-legacy-state.json" --slurpfile b "$BACKUP_DIR/r5-legacy-state.json" -f "$RB/compare-legacy.jq" | jq -c .
"${PSQL_RO[@]}" -Atc "SELECT source_table, reason, count(*) FROM rollback_archive GROUP BY 1, 2 ORDER BY 1, 2"
"${PSQL[@]}" -q < "$RB/trigger-probe.sql" 2>&1 | grep 'trigger probe'
```

Expect only the kept post-cutover writes in `counts_changed` (each new post +1 as `idea`, each new room entry
+1 in `messages`); tombstones, trigger and room ids/slugs/owners equal; `legacy_public_contributions` equal.
Measured losses (lane D, D5, rehearsed): replies and child replies created after the cutover and votes on
replies leave the live tables and remain only in `rollback_archive`; posts are kept, relabeled `idea`;
room entries are kept; **0 of 78 shared room tokens stay valid** (000098.down fills random hashes);
per-agent room tokens keep their hashes; the ban list is archived and its table dropped; the
`users_refuse_tombstoned_email` trigger stays with its original body.

**R6 — redeploy old code, both services. DECIDED 2026-10-02 (owner): option (c), the tree-restore commit
`rollback/v1.3-to-c03734ae` built at the freeze (PRE-2).** The option table is kept for the record.

| Option | How | Risks |
|---|---|---|
| (a) Branch at c03734ae | Before the window (G1): push `rollback-c03734ae`. In rollback: EasyPanel → `solvr-api` and `solvr-web` → source branch `rollback-c03734ae` → Deploy. | Needs EasyPanel's per-service branch setting (verify it exists before the window). `main` still holds v1.3: any later Deploy click or webhook on `main` puts v1.3 back on schema 84. The branch setting must be switched back afterwards. |
| (b) Force-push main | `git push --force-with-lease=main:"$FROZEN_SHA" origin c03734ae:refs/heads/main`, then both webhooks. | Rewrites the public `main` history; local `main` (and the build loop) diverge from GitHub; re-shipping v1.3 needs another force-push. |
| (c) Tree-restore commit | In a clean worktree at `FROZEN_SHA`: restore the `c03734ae` tree as one new commit, push it to `main` (fast-forward), both webhooks. | One very large commit; history is kept and no force is needed; re-shipping v1.3 later means reverting that commit. Build is the same tree as c03734ae. |
| (d) EasyPanel previous image | EasyPanel's redeploy/rollback of the previous deployment, if it keeps one. | [UNVERIFIED] that EasyPanel keeps and offers the previous image; must be checked in the panel before the window. No git change; fastest. `main` still holds v1.3 (same risk as (a)). |

Procedure for (c), three blocks.

R6a — preflight:

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
git -C /Users/fcavalcanti/dev/solvr fetch origin
[ "$(git -C /Users/fcavalcanti/dev/solvr rev-parse origin/main)" = "$(git -C /Users/fcavalcanti/dev/solvr rev-parse "$FROZEN_SHA")" ] && echo "origin/main = FROZEN_SHA" || echo "ORIGIN/MAIN IS NOT FROZEN_SHA"
[ "$(git -C /Users/fcavalcanti/dev/solvr rev-parse 'rollback/v1.3-to-c03734ae^')" = "$(git -C /Users/fcavalcanti/dev/solvr rev-parse "$FROZEN_SHA")" ] && echo "parent = FROZEN_SHA" || echo "PARENT MISMATCH"
git -C /Users/fcavalcanti/dev/solvr diff --quiet c03734ae rollback/v1.3-to-c03734ae && echo "tree = c03734ae" || echo "TREE MISMATCH"
```

Expect the three positive lines. If `origin/main` is not `FROZEN_SHA`, someone pushed after G1: STOP (never
force); Felipe decides. **STOP — Felipe's yes to push the rollback commit.**

R6b — push it (a plain fast-forward of `main`: it fails safely if `origin/main` is not `FROZEN_SHA`):

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
git -C /Users/fcavalcanti/dev/solvr push origin rollback/v1.3-to-c03734ae:main
git -C /Users/fcavalcanti/dev/solvr ls-remote origin refs/heads/main
git -C /Users/fcavalcanti/dev/solvr rev-parse rollback/v1.3-to-c03734ae
```

Expect the push to succeed and the two SHAs to be equal. A rejected push: STOP, never force.

R6c — both webhooks back to back, then the old-code completion probes and smoke probes:

```bash
export WINDOW_ENV=... FROZEN_SHA=... WINDOW_DIR=...   # the prelude
source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
curl -fsS -X POST "$SOLVR_DEPLOY_API"; echo; curl -fsS -X POST "$SOLVR_DEPLOY_WEB"; echo
for i in $(seq 90); do c=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "$SOLVR_API_BASE/v1/overview"); echo "$(date +%T) api /v1/overview $c (old code: 404)"; [ "$c" = 404 ] && break; sleep 10; done
for i in $(seq 90); do c=$(curl -s -o "$BACKUP_DIR/r6-skill.md" -w '%{http_code}' --max-time 10 "$SOLVR_WEB_BASE/skill.md?cb=$(date +%s)$RANDOM"); n=$(grep -c 'Connect your agents so they collaborate in a shared room' "$BACKUP_DIR/r6-skill.md"); echo "$(date +%T) web skill.md $c, v1.3 marker $n (old web: 200, 0)"; [ "$c" = 200 ] && [ "$n" = 0 ] && break; sleep 10; done
bash "$RB/probes.sh" old "$BACKUP_DIR/r6-probes"
```

Expect `Deploying...` twice, `/v1/overview 404`, web `200` with marker `0`, `probes failed: 0`, and
`total_contributions` = G3's `legacy_public_contributions`. Rollback ends when both completion probes pass.

**R7 — tell agents.** Shared room tokens are invalid after the rollback; per-agent room tokens still work.

**R8 — last resort, Felipe's call, UNREHEARSED:** restore the G3 safety dump over production
(`pg_restore --clean --if-exists --no-owner --no-acl` through the same container route). It discards every
write since G3. Not rehearsed in D6.

### Migration transaction safety, 000085–000135

Audit of every up and down file for statements that run outside, or end, the implicit transaction:
`CONCURRENTLY`, `BEGIN;`, `COMMIT;`, `ROLLBACK;`, `START TRANSACTION`, `SAVEPOINT`, `VACUUM`,
`ALTER TYPE … ADD VALUE`, `ALTER SYSTEM`, `CREATE/DROP DATABASE`, `REINDEX`, `CALL`, `dblink`, `pg_sleep`.

| Files | Result |
|---|---|
| 000085–000135, up and down (102 files) | **none** in any statement: every file is atomic; "force, then goto 84" is safe for all of them |
| comment-only matches (not statements) | `000121.up`, `000131.up`, `000132.down` ("call" in prose); `000122`, `000125`–`000128`, `000130` up ("REINDEX" in prose) |

---

## Pre-G6 EasyPanel env checklist

D6a, `c03734ae` → frozen v1.3, names only. Felipe ticks each line in EasyPanel before the G6 yes.
Only three names are new; none was removed; both Dockerfiles are unchanged. What changed is what several
unchanged names now gate.

**solvr-api**

| ✓ | Name | Needed? | Absent or invalid at v1.3 |
|---|---|---|---|
| [ ] | `DATABASE_URL` | yes | The process does **not** exit: it logs a warning and serves `/health` 200 with no `/v1`, no rooms, no jobs (`/health/ready` 503). The G6 probe `/v1/overview` catches it. The new room relay holds a `LISTEN` connection: use a direct or session-mode connection, not a transaction pooler [UNVERIFIED for EasyPanel]. |
| [ ] | `JWT_SECRET` | yes, ≥ 32 chars, **unchanged value** | Missing or short: same degraded mode as above. v1.3 also encrypts stored claim tokens and webhook secrets and signs room stream tickets with it; a new value logs everyone out and breaks unsubscribe links and stored claim links. |
| [ ] | `FRONTEND_URL` | yes | v1.3 OAuth redirects to `FRONTEND_URL/auth/callback?code=…` (a one-time code; old code sent `?token=`). Default `http://localhost:3000`: logins would land on localhost. |
| [ ] | `ALLOWED_ORIGINS` | if set | If set it **replaces** the defaults (`http://localhost:3000`, `https://solvr.dev`, `https://www.solvr.dev`); the browser now POSTs `/v1/auth/oauth/exchange` cross-origin, so the site origin must be in it. |
| [ ] | `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `GITHUB_REDIRECT_URI`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REDIRECT_URI` | yes, unchanged | No code default for the redirect URIs; errors now redirect to `FRONTEND_URL/auth/callback?error=…`. |
| [ ] | `GROQ_API_KEY` | yes | **Changed impact.** The typed create routes now answer 410, so every new public post comes through `POST /v1/posts` or a room publish and starts `pending_review`; without the key it never leaves it (no new post ever goes public). An invalid key fails silently (no flag). Replies are not moderated. Also the translation job. `GROQ_MODEL`, `TRANSLATION_MODEL`, `TRANSLATION_BATCH_SIZE`, `TRANSLATION_DELAY_MS` optional. |
| [ ] | `VOYAGE_API_KEY` | yes | Without it the new 5-minute search-document job does not start: the cutover's pending vectors (251 on the 2026-09-29 data) and every new post or reply stay without one, and search silently runs keyword-only. |
| [ ] | `EMBEDDING_PROVIDER` | must not be `ollama` | `ollama` makes the process exit at startup (crash loop); anything else means Voyage. |
| [ ] | `IPFS_API_URL` | yes | Default `http://localhost:5001` does not reach the node from the container: pins, uploads, crystallization and the new `/admin/ipfs/unpin` and `/admin/ipfs/gc` (P7/P8) fail. |
| [ ] | `ADMIN_API_KEY` | yes | Every `/admin/*` route (P7/P8, bans, query, broadcast) answers 503 without it. |
| [ ] | `RESEND_API_KEY`, `FROM_EMAIL` | optional | Only the admin broadcast sends email (503 without the key). |
| [ ] | `DESTRUCTIVE_QUERIES` | leave unset | Only `true` lets `/admin/query` write. |
| [ ] | `PORT` | 8080 | Must match EasyPanel's target port (`EXPOSE 8080`); an unusable port exits. |
| [ ] | `HOMEPAGE_EXAMPLE_ROOM_SLUG` | new, optional | Default `tictactoe-human-vs-computer-20260920`; a missing room gives an illustrative example (still 200). |
| [ ] | `HOMEPAGE_PREVIEW_ROOM_SLUGS` | new, optional | Comma list, first 3; empty means the example room only; unloadable slugs are skipped. |
| [ ] | `SEARCH_QUERY_BLOCKLIST` | new, optional | Adds terms hidden from the homepage "top searches" (built-ins always apply). |
| [ ] | EasyPanel health check | recommendation | If it probes `/health`, a degraded API (bad `DATABASE_URL`/`JWT_SECRET`) passes it; `/health/ready` answers 503 without a database. |

Read but without effect (safe to leave as they are): `RATE_LIMIT_AGENT_GENERAL`, `RATE_LIMIT_AGENT_SEARCH`,
`RATE_LIMIT_HUMAN_GENERAL` (the real limits are in the `rate_limit_config` table; the startup log prints the
env values misleadingly), `SENTRY_DSN`, `LOG_LEVEL`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`,
`LLM_PROVIDER`, `LLM_API_KEY`, `LLM_MODEL`, `APP_URL`, `API_URL`, `APP_ENV`, `JWT_EXPIRY`,
`REFRESH_TOKEN_EXPIRY`, `OLLAMA_BASE_URL`, `MAX_UPLOAD_SIZE_BYTES` (falls back to 100 MB),
`SEARCH_CONFIDENCE_THRESHOLD` (falls back to 0.85). Not read by solvr-api at all: `QUORUM_DB_URL`,
`TEST_GROQ_API_KEY` (separate command-line tools). `cmd/cutover` reads no env (`--database-url` only).

**solvr-web**

| ✓ | Name | Needed? | Notes |
|---|---|---|---|
| [ ] | `NEXT_PUBLIC_API_URL` (Docker build arg) | build time only | Baked in at `next build`; if EasyPanel passes it, it must be `https://api.solvr.dev`; the Dockerfile default is the same. Changing it at runtime does nothing. |
| [ ] | `NEXT_PUBLIC_GA_ID` | no | Not a Docker build `ARG`, so the code's fallback ID is always used. |
| [ ] | `SOLVR_API_KEY` | no | Only text inside an SDK sample; never read. |

Login spans both services (the code exchange): old web with the new API, or the reverse, cannot log in.
That is one more reason G6 triggers both webhooks back to back.

## Dress rehearsal results (D6c)

Lane D, 2026-10-02, on a local stand-in. The stand-in was born through the runbook's own dump route:
`solvr_rehearsal_purge` (post-purge production copy, schema 84, no `schema_migrations`) was dumped and
restored into `solvr_lane_d_dress`, owned by a dedicated role whose password holds `# & ? = $ ' " @ : / % +`
and a space. "Production" ran the `c03734ae` **API image** behind a stub of the two deploy webhooks; G6 and R6
"deployed" the v1.3 and `c03734ae` images through that stub. Every block was extracted from the committed
runbook and run unchanged except the prelude values; four defects were found and fixed here first:

1. PRE-0 used `WINDOW_DIR`/`FROZEN_SHA` without setting them (found before running).
2. G4 ran `up` even after a failed `force`, and G5 ran the second pass after a failed apply (found reading
   G2's output): both are now chained.
3. The cutover logs every sampled search (3,597 lines, 465 KB per G2) to the terminal: each run now writes
   its own log under `$BACKUP_DIR`.
4. Continuation blocks after a STOP had no prelude, so a fresh shell had an empty `PSQL`
   (`-v: command not found`, P5 d/e): every block now starts with the prelude.

| Step | Result | Time |
|---|---|---|
| PRE-0, PRE-1 | worktree; both images built; cutover guard test ok; default 135 = newest migration | 0.8 s; 95.8 s |
| P1 | all 84 markers present, all 85+ absent (incl. `tags`), 8 rate limits, 11 tombstones | 1.0 s |
| G1 | scan only: 152 masked rows, the 18 unhinted = D4's reviewed set; no push | 39.6 s |
| G2 | dump, 42 tables, restore; `135\|f`; 883 replies + 135 progress notes; second pass all zero | 60.3 s |
| G3 | `/health` 000, 0 API sessions, safety dump, snapshot | 0.3 s stop + 1.8 s |
| P5 | 0 duplicate groups; REINDEX + REFRESH (no local mismatch) | 0.4 s + 1.6 s |
| G4 | `force 84`, 51 files `up`, `135\|f` | 3.0 s |
| G5 | dry = G2; apply 883 + 135; second all zero; 0 unexplained orphans; 418 = 305 − 26 − 3 + 16 + 126; trigger refuses both tombstoned emails | 8.1 s + 43.5 s |
| G6 | both webhooks `Deploying...`; API image `/v1/overview` 200; web image `skill.md` marker 1; `probes.sh new` 13/13, contributions 418 | 5.9 s |
| P7 | dry run `{"would_unpin":87}` (real run and gc not rehearsed) | 0.3 s |
| R1–R6 | 5 post-cutover writes, then: pre-rollback dump; `goto 84` 51 files `84\|f`; only the kept writes changed (post +1 as `idea`, messages +1); 2 replies, 1 vote, 1 post, 12 bans archived; tombstones, trigger, rooms equal; 0/78 shared tokens, 44/44 agent tokens; `c03734ae` image `probes.sh old` 9/9, contributions 305, 0 5xx | 0.2 + 2.0 + 1.0 + 0.5 + 1.1 + 0.6 s |

Hygiene: after the rehearsal, no file under the rehearsal folder (128 files) and neither container log held
the stand-in password, raw or percent-encoded (the stand-in env file itself excepted).

## Write-pause estimate

The pause runs from the G3 stop to the moment both G6 completion probes pass.

| Part | Dress | Production |
|---|---|---|
| G3 stop + checks + safety dump | 2.1 s | + the dump's transfer time (a few MB of post-purge data) |
| P5 (a)–(e) | 2.0 s | REINDEX measured 3.7 s on the larger pre-purge data |
| G4 | 3.0 s | + 192 transactions × RTT |
| G5 (dry, apply, second, checks) | 51.6 s | + 12,772 transactions × RTT |
| G6 | 5.9 s | EasyPanel builds: API 1–2 min, web 2–4 min, in parallel; worst case 4 min |
| Felipe's yes at G3→P5, P5 (d), G4, G5 apply, G6 | — | about 1 min each (estimate) |

Machine time is about 65 s locally. Network: about 13,000 transactions at the RTT measured in P1. At 20 ms
that adds about 4.5 min, at 50 ms about 11 min. Roughly 9,000 of the G5 transactions are the 200-query
search sample, run before and after each pass (DECIDED 2026-10-02 (owner): 200 on every pass).
**Estimate: about 10–15 minutes at 20–30 ms RTT; worst case about 25 minutes** (slow RTT, 4-minute web build,
slow approvals). Recompute with P1's RTT before G3.
