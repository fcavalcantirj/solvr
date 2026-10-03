#!/bin/bash
# Migration recovery drill (spec.json idx 79 step 5), LOCAL only: on a TEMPLATE copy of a
# production-shaped database, take the newest migrations down and back up with the migrate
# CLI the runbooks use, and prove what comes back: the schema dump must be identical, and
# every table's row count is compared — a change is listed, never hidden (some downs drop
# data by design; they are named in the report).
#
#   scripts/ops/migration-recovery-drill.sh solvr_o_load 3
#
# It creates and drops only solvr_o_migr_<time>, by exact name.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SRC="${1:?usage: migration-recovery-drill.sh <source database> [steps]}"
STEPS="${2:-3}"
LOGS="${LOGS:-/tmp/solvr-lane-o}"
CONTAINER="${CONTAINER:-solvr-postgres}"
export PGHOST="${LOAD_PGHOST:-localhost}" PGPORT="${LOAD_PGPORT:-5435}" PGUSER="${LOAD_PGUSER:-solvr}"
export PGPASSWORD="${LOAD_PGPASSWORD:-solvr_dev}"
STAMP="$(date +%H%M%S)"
DB="solvr_o_migr_${STAMP}"
URL="postgres://${PGUSER}:${PGPASSWORD}@${PGHOST}:${PGPORT}/${DB}?sslmode=disable"
OUT="$LOGS/migration-drill-${STAMP}"
mkdir -p "$OUT"
REPORT="$OUT/report.txt"
fail=0
say() { echo "$*" | tee -a "$REPORT"; }
now_ms() { python3 -c 'import time; print(int(time.time()*1000))'; }

COUNTS_SQL="SELECT table_schema || '.' || table_name || ' ' ||
  (xpath('/row/c/text()', query_to_xml(format('SELECT count(*) AS c FROM %I.%I', table_schema, table_name), false, true, '')))[1]::text
  FROM information_schema.tables
 WHERE table_type = 'BASE TABLE' AND table_schema NOT IN ('pg_catalog', 'information_schema')
 ORDER BY 1"
VERSION_SQL="SELECT version || ' dirty=' || dirty FROM schema_migrations"
# rollback_archive is the down migrations' side table (the convention of 000088, 000089, 000109,
# 000114 and 000138: CREATE TABLE IF NOT EXISTS, kept so a row a rollback could not place back is
# never lost). A down creates it and a later up keeps it, by design: it is compared apart, by rows.
schema_dump() { docker exec "$CONTAINER" pg_dump -U "$PGUSER" -s -d "$1" -T public.rollback_archive -T public.rollback_archive_id_seq \
  | grep -v '^\\\(un\)\?restrict '; }
counts() { psql -d "$1" -Atc "$COUNTS_SQL" | grep -v '^public\.rollback_archive ' || true; }

trap 'psql -d postgres -qc "DROP DATABASE IF EXISTS \"$DB\" WITH (FORCE)" >/dev/null 2>&1 || true' EXIT

say "migration recovery drill: $SRC -> $DB, down $STEPS then up ($(date -u +%Y-%m-%dT%H:%M:%SZ))"
psql -d postgres -v ON_ERROR_STOP=1 -qc "CREATE DATABASE \"$DB\" TEMPLATE \"$SRC\""
before_version="$(psql -d "$DB" -Atc "$VERSION_SQL")"
say "before: $before_version"
schema_dump "$DB" > "$OUT/schema-before.sql"
counts "$DB" > "$OUT/counts-before.txt"

t0=$(now_ms)
migrate -path "$ROOT/backend/migrations" -database "$URL" down "$STEPS" > "$OUT/down.log" 2>&1
t1=$(now_ms)
say "down $STEPS: $(( t1 - t0 )) ms -> $(psql -d "$DB" -Atc "$VERSION_SQL")"
sed 's/^/  /' "$OUT/down.log" | tee -a "$REPORT"
counts "$DB" > "$OUT/counts-down.txt"

t2=$(now_ms)
migrate -path "$ROOT/backend/migrations" -database "$URL" up > "$OUT/up.log" 2>&1
t3=$(now_ms)
after_version="$(psql -d "$DB" -Atc "$VERSION_SQL")"
say "up: $(( t3 - t2 )) ms -> $after_version"
sed 's/^/  /' "$OUT/up.log" | tee -a "$REPORT"

if [ "$before_version" = "$after_version" ]; then say "PASS back at $after_version"; else say "FAIL version $before_version -> $after_version"; fail=1; fi

schema_dump "$DB" > "$OUT/schema-after.sql"
if diff "$OUT/schema-before.sql" "$OUT/schema-after.sql" > "$OUT/schema.diff"; then
  say "PASS the schema after down+up is identical to before ($(wc -l < "$OUT/schema-before.sql" | tr -d ' ') lines)"
else
  say "FAIL the schema changed across down+up ($(grep -c '^[<>]' "$OUT/schema.diff") differing lines, see $OUT/schema.diff)"; fail=1
fi

counts "$DB" > "$OUT/counts-after.txt"
if diff "$OUT/counts-before.txt" "$OUT/counts-after.txt" > "$OUT/counts.diff"; then
  say "PASS every table's row count is identical after down+up"
else
  say "CHANGED row counts across down+up (data a down migration drops by design, or a loss):"
  join -a1 -a2 -e missing -o 0,1.2,2.2 "$OUT/counts-before.txt" "$OUT/counts-after.txt" \
    | awk '$2 != $3 {print "  " $1 ": " $2 " -> " $3}' | tee -a "$REPORT"
fi
archived="$(psql -d "$DB" -Atc "SELECT CASE WHEN to_regclass('public.rollback_archive') IS NULL THEN 'absent' ELSE (SELECT count(*)::text FROM public.rollback_archive) END")"
if [ "$archived" = "absent" ] || [ "$archived" = "0" ]; then
  say "PASS rollback_archive: $archived rows (no row was set aside by the down migrations)"
else
  say "CHANGED rollback_archive holds $archived rows the down migrations could not place back"
fi
say "tables whose rows the down step changed (before -> after down):"
join -a1 -a2 -e missing -o 0,1.2,2.2 "$OUT/counts-before.txt" "$OUT/counts-down.txt" \
  | awk '$2 != $3 {print "  " $1 ": " $2 " -> " $3}' | tee -a "$REPORT"

if [ "$fail" -eq 0 ]; then say "RESULT: PASS"; else say "RESULT: FAIL"; fi
exit "$fail"
