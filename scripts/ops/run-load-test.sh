#!/bin/bash
# Run the idx 79 load test against a LOCAL API on a production-shaped copy, then probe
# GET /admin/ops/slo on the rows the run wrote. LAPTOP numbers, never production capacity.
#
#   scripts/ops/run-load-test.sh                 # defaults below
#   TEMPLATE_DB=solvr_window_g2 LOAD_DB=solvr_o_load LOAD_PORT=18650 LOGS=/tmp/solvr-lane-o \
#     STAGES=10:60s,25:60s scripts/ops/run-load-test.sh
#
# Heavy: run it through the shared slot wrapper (/tmp/solvr-heavy.sh) on a shared machine.
# It never reads .env: the API starts under `env -i` with a throwaway JWT secret and operator
# key generated here and never printed, and with no Voyage, Groq, Resend or OAuth keys, so
# nothing leaves the machine. It only ever drops LOAD_DB, by exact name.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TEMPLATE_DB="${TEMPLATE_DB:-solvr_window_g2}"
LOAD_DB="${LOAD_DB:-solvr_o_load}"
# LOAD_PORT, never PORT: a shell that exports PORT for another app must not redirect this run.
LOAD_PORT="${LOAD_PORT:-18650}"
LOGS="${LOGS:-/tmp/solvr-lane-o}"
STAGES="${STAGES:-10:60s,25:60s,50:60s,100:60s,200:60s,400:60s}"
BASELINE="${BASELINE:-30s}"
# Agents spread timeline writes under the 60/min per-IP room write limit: keep
# writes/min = 0.077 x peak RPS x 60 below ~45 per agent.
AGENTS="${AGENTS:-40}"
ROOMS="${ROOMS:-8}"
# The compose database (docker-compose.yml), set from LOAD_PG*, never from an inherited PG*.
export PGHOST="${LOAD_PGHOST:-localhost}" PGPORT="${LOAD_PGPORT:-5435}" PGUSER="${LOAD_PGUSER:-solvr}"
export PGPASSWORD="${LOAD_PGPASSWORD:-solvr_dev}"
DBURL="postgres://${PGUSER}:${PGPASSWORD}@${PGHOST}:${PGPORT}/${LOAD_DB}?sslmode=disable"
mkdir -p "$LOGS"

case "$LOAD_DB" in solvr_o_*) ;; *) echo "LOAD_DB must be a lane-o scratch database (solvr_o_*)" >&2; exit 2 ;; esac

echo "== stop a previous run of this script's API, then require port $LOAD_PORT free"
pkill -f "$LOGS/solvr-api" 2>/dev/null || true
sleep 1
if lsof -nP -iTCP:"$LOAD_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "port $LOAD_PORT is held by another process; refusing to kill it" >&2
  exit 3
fi

echo "== $LOAD_DB from TEMPLATE $TEMPLATE_DB"
psql -d postgres -v ON_ERROR_STOP=1 -qc "DROP DATABASE IF EXISTS \"$LOAD_DB\" WITH (FORCE)"
psql -d postgres -v ON_ERROR_STOP=1 -qc "CREATE DATABASE \"$LOAD_DB\" TEMPLATE \"$TEMPLATE_DB\""
migrate -path "$ROOT/backend/migrations" -database "$DBURL" up > "$LOGS/load-migrate.log" 2>&1
tail -n 1 "$LOGS/load-migrate.log"
psql -d "$LOAD_DB" -Atc "SELECT 'schema version ' || version || CASE WHEN dirty THEN ' (dirty)' ELSE '' END FROM schema_migrations"

echo "== build"
(cd "$ROOT/backend" && go build -o "$LOGS/solvr-api" ./cmd/api && go build -o "$LOGS/solvr-loadtest" ./cmd/loadtest)

JWT="$(openssl rand -hex 32)"
OPKEY="$(openssl rand -hex 24)"
echo "== start API on :$LOAD_PORT (env -i; throwaway secrets)"
env -i PATH="$PATH" DATABASE_URL="$DBURL" JWT_SECRET="$JWT" ADMIN_API_KEY="$OPKEY" PORT="$LOAD_PORT" \
  "$LOGS/solvr-api" > "$LOGS/load-api.log" 2>&1 &
API_PID=$!
trap 'kill $API_PID 2>/dev/null || true; wait $API_PID 2>/dev/null || true' EXIT

echo "== load test (stages $STAGES)"
STARTED="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
(cd "$ROOT/backend" && timeout 1800 "$LOGS/solvr-loadtest" -base-url "http://127.0.0.1:$LOAD_PORT" -stages "$STAGES" -baseline "$BASELINE" -agents "$AGENTS" -rooms "$ROOMS" \
  -mix "$ROOT/docs/ops/load-mix.json" \
  -dataset "$LOAD_DB = TEMPLATE $TEMPLATE_DB (production copy taken 2026-10-02) migrated to head" \
  -out-json "$LOGS/load.json" -out-md "$LOGS/load.md") > "$LOGS/load-harness.log" 2>&1
tail -n 3 "$LOGS/load-harness.log"

echo "== acceptance probe: GET /admin/ops/slo on the rows the run wrote"
sleep 3 # the usage recorder flushes every second
curl -sf --max-time 30 -H "X-Admin-API-Key: $OPKEY" "http://127.0.0.1:$LOAD_PORT/admin/ops/slo" > "$LOGS/load-slo-now.json"
curl -sf --max-time 30 -H "X-Admin-API-Key: $OPKEY" "http://127.0.0.1:$LOAD_PORT/admin/ops/slo?end=2026-10-02T23:52:03Z" \
  > "$LOGS/load-slo-copy-end.json"
jq -r '.data.targets[] | "\(.key): \(.status) measured=\(.measured) samples=\(.samples)"' "$LOGS/load-slo-now.json"
jq -r '"queue: \(.data.queues[0].status) pending=\(.data.queues[0].pending)"' "$LOGS/load-slo-now.json"
echo "-- at the copy's end (2026-10-02T23:52:03Z):"
jq -r '.data.targets[0] | "\(.key): \(.status) measured=\(.measured)"' "$LOGS/load-slo-copy-end.json"
psql -d "$LOAD_DB" -Atc "SELECT 'recorded with duration since $STARTED: ' || count(*) FROM api_request_events WHERE duration_ms IS NOT NULL AND occurred_at >= timestamptz '$STARTED'"

echo "== stop API"
