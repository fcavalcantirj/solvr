#!/bin/bash
# Backup restoration drill (spec.json idx 79 step 5), LOCAL only: dump a database with the
# server's own pg_dump (inside the solvr-postgres container, as the cutover runbook's G2 does),
# restore it into a fresh scratch database, and prove the copy is exact — schema version,
# every table's row count, every sequence's value, and the schema dump itself.
#
#   scripts/ops/restore-drill.sh solvr_o_load
#
# A production restore is the owner's (UAT): this drill is the rehearsal, never that.
# It creates and drops only solvr_o_restore_<time>, by exact name.
set -euo pipefail

SRC="${1:?usage: restore-drill.sh <source database>}"
LOGS="${LOGS:-/tmp/solvr-lane-o}"
CONTAINER="${CONTAINER:-solvr-postgres}"
export PGHOST="${LOAD_PGHOST:-localhost}" PGPORT="${LOAD_PGPORT:-5435}" PGUSER="${LOAD_PGUSER:-solvr}"
export PGPASSWORD="${LOAD_PGPASSWORD:-solvr_dev}"
STAMP="$(date +%H%M%S)"
DST="solvr_o_restore_${STAMP}"
DUMP="/tmp/${DST}.dump"
OUT="$LOGS/restore-drill-${STAMP}"
mkdir -p "$OUT"
REPORT="$OUT/report.txt"
fail=0
say() { echo "$*" | tee -a "$REPORT"; }
check() { if [ "$2" = "$3" ]; then say "PASS $1"; else say "FAIL $1: source=$2 restored=$3"; fail=1; fi; }
now_ms() { python3 -c 'import time; print(int(time.time()*1000))'; }

COUNTS_SQL="SELECT table_schema || '.' || table_name || ' ' ||
  (xpath('/row/c/text()', query_to_xml(format('SELECT count(*) AS c FROM %I.%I', table_schema, table_name), false, true, '')))[1]::text
  FROM information_schema.tables
 WHERE table_type = 'BASE TABLE' AND table_schema NOT IN ('pg_catalog', 'information_schema')
 ORDER BY 1"
SEQ_SQL="SELECT schemaname || '.' || sequencename || ' ' || coalesce(last_value::text, 'null') FROM pg_sequences ORDER BY 1"
VERSION_SQL="SELECT version || ' dirty=' || dirty FROM schema_migrations"
# Every object by name (and column type): what the comparison judges. The text of a schema dump
# also changes when PostgreSQL re-parses an expression it restores (an IN (...) check on a varchar
# column comes back as an equivalent ANY (ARRAY[...]) tree), so the text diff is reported, not judged.
INVENTORY_SQL="SELECT kind || ' ' || name FROM (
  SELECT 'relation:' || c.relkind::text AS kind, n.nspname || '.' || c.relname AS name
    FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
   WHERE c.relkind IN ('r','p','v','m','S','i') AND n.nspname NOT IN ('pg_catalog','information_schema','pg_toast')
  UNION ALL SELECT 'column', table_schema || '.' || table_name || '.' || column_name || ' ' || udt_name
    FROM information_schema.columns WHERE table_schema NOT IN ('pg_catalog','information_schema')
  UNION ALL SELECT 'constraint:' || contype::text, conrelid::regclass::text || '.' || conname
    FROM pg_constraint WHERE connamespace NOT IN ('pg_catalog'::regnamespace, 'information_schema'::regnamespace)
  UNION ALL SELECT 'trigger', tgrelid::regclass::text || '.' || tgname FROM pg_trigger WHERE NOT tgisinternal
  UNION ALL SELECT 'function', p.oid::regprocedure::text
    FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema')
  UNION ALL SELECT 'extension', extname || ' ' || extversion FROM pg_extension
) x ORDER BY 1"
schema_dump() { docker exec "$CONTAINER" pg_dump -U "$PGUSER" -s -d "$1" | grep -v '^\\\(un\)\?restrict '; }

cleanup() {
  psql -d postgres -qc "DROP DATABASE IF EXISTS \"$DST\" WITH (FORCE)" >/dev/null 2>&1 || true
  docker exec "$CONTAINER" rm -f "$DUMP" >/dev/null 2>&1 || true
}
trap cleanup EXIT

say "restore drill: $SRC -> $DST ($(date -u +%Y-%m-%dT%H:%M:%SZ))"
t0=$(now_ms)
docker exec "$CONTAINER" pg_dump -U "$PGUSER" -Fc -d "$SRC" -f "$DUMP"
t1=$(now_ms)
docker cp -q "$CONTAINER:$DUMP" "$OUT/source.dump"
say "dump: $(( (t1 - t0) )) ms, $(wc -c < "$OUT/source.dump" | tr -d ' ') bytes, sha256 $(shasum -a 256 "$OUT/source.dump" | cut -d' ' -f1)"
say "dump TOC: $(docker exec "$CONTAINER" pg_restore -l "$DUMP" | grep -c 'TABLE DATA') TABLE DATA entries"

psql -d postgres -v ON_ERROR_STOP=1 -qc "CREATE DATABASE \"$DST\""
t2=$(now_ms)
# No --exit-on-error: a restore that skips an object must still be compared in full, so the
# report shows everything that did and did not come back. Every pg_restore error is a FAIL.
rc=0
docker exec "$CONTAINER" pg_restore -U "$PGUSER" -d "$DST" --no-owner --no-acl "$DUMP" > "$OUT/pg_restore.log" 2>&1 || rc=$?
t3=$(now_ms)
say "restore: $(( (t3 - t2) )) ms, pg_restore exit $rc"
if [ "$rc" -ne 0 ] || grep -q '^pg_restore: error' "$OUT/pg_restore.log"; then
  fail=1
  { grep -A3 '^pg_restore: error' "$OUT/pg_restore.log" || true; } | { grep -E '^pg_restore: error|^Command was' || true; } \
    | cut -c1-220 | sed 's/^/FAIL /' | tee -a "$REPORT"
fi

check "schema_migrations" "$(psql -d "$SRC" -Atc "$VERSION_SQL")" "$(psql -d "$DST" -Atc "$VERSION_SQL")"

psql -d "$SRC" -Atc "$COUNTS_SQL" > "$OUT/counts-source.txt"
psql -d "$DST" -Atc "$COUNTS_SQL" > "$OUT/counts-restored.txt"
say "tables: $(wc -l < "$OUT/counts-source.txt" | tr -d ' ') in source, $(awk '{s+=$2} END {print s}' "$OUT/counts-source.txt") rows"
if diff "$OUT/counts-source.txt" "$OUT/counts-restored.txt" > "$OUT/counts.diff"; then
  say "PASS every table's row count is identical"
else
  say "FAIL row counts differ (see $OUT/counts.diff)"; fail=1
fi

psql -d "$SRC" -Atc "$SEQ_SQL" > "$OUT/seq-source.txt"
psql -d "$DST" -Atc "$SEQ_SQL" > "$OUT/seq-restored.txt"
if diff "$OUT/seq-source.txt" "$OUT/seq-restored.txt" > "$OUT/seq.diff"; then
  say "PASS every sequence value is identical ($(wc -l < "$OUT/seq-source.txt" | tr -d ' ') sequences)"
else
  say "FAIL sequence values differ (see $OUT/seq.diff)"; fail=1
fi

psql -d "$SRC" -Atc "$INVENTORY_SQL" > "$OUT/inventory-source.txt"
psql -d "$DST" -Atc "$INVENTORY_SQL" > "$OUT/inventory-restored.txt"
if diff "$OUT/inventory-source.txt" "$OUT/inventory-restored.txt" > "$OUT/inventory.diff"; then
  say "PASS every object came back ($(wc -l < "$OUT/inventory-source.txt" | tr -d ' ') relations, columns, constraints, triggers, functions, extensions)"
else
  fail=1
  { grep '^<' "$OUT/inventory.diff" || true; } | sed 's/^< /FAIL missing after restore: /' | tee -a "$REPORT"
  { grep '^>' "$OUT/inventory.diff" || true; } | sed 's/^> /FAIL only in the restore: /' | tee -a "$REPORT"
fi

schema_dump "$SRC" > "$OUT/schema-source.sql"
schema_dump "$DST" > "$OUT/schema-restored.sql"
if diff "$OUT/schema-source.sql" "$OUT/schema-restored.sql" > "$OUT/schema.diff"; then
  say "INFO the schema dump text is identical ($(wc -l < "$OUT/schema-source.sql" | tr -d ' ') lines)"
else
  say "INFO the schema dump text differs in $(grep -c '^[<>]' "$OUT/schema.diff") lines (re-parsed expressions and anything missing above; $OUT/schema.diff)"
fi
rm -f "$OUT/source.dump"

if [ "$fail" -eq 0 ]; then say "RESULT: PASS"; else say "RESULT: FAIL"; fi
exit "$fail"
