# Cutover window: per-gate shell setup (bash). Every gate block starts with:
#   export WINDOW_ENV=<env file> FROZEN_SHA=<sha> WINDOW_DIR=<clean worktree at FROZEN_SHA>
#   source "$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window/window-env.sh"
# It never prints a secret, the database URL or a password; it prints names and non-secret settings only.
: "${WINDOW_ENV:?set WINDOW_ENV}" "${FROZEN_SHA:?set FROZEN_SHA}" "${WINDOW_DIR:?set WINDOW_DIR}"
RB="$WINDOW_DIR/docs/handovers/2026-10-02-solvr-orchestrator/window"
source "$RB/load-env.sh"
load_env_file "$WINDOW_ENV" SOLVR_DB_HOST SOLVR_DB_PORT SOLVR_DB_USER SOLVR_DB_PASSWORD SOLVR_DB_NAME \
  SOLVR_DEPLOY_API SOLVR_DEPLOY_WEB ADMIN_API_KEY \
  SOLVR_API_BASE SOLVR_WEB_BASE BACKUP_DIR RESTORE_TEST_DB LOCAL_PG || return 1
: "${SOLVR_API_BASE:=https://api.solvr.dev}"
: "${SOLVR_WEB_BASE:=https://solvr.dev}"
: "${BACKUP_DIR:=/Users/fcavalcanti/dev/solvr/db-backups/window}"
: "${RESTORE_TEST_DB:=solvr_window_g2}"
: "${LOCAL_PG:=postgres://solvr:solvr_dev@localhost:5435}"
export SOLVR_API_BASE SOLVR_WEB_BASE BACKUP_DIR RESTORE_TEST_DB LOCAL_PG
BIN="$BACKUP_DIR/bin"
mkdir -p "$BACKUP_DIR" "$BIN"
# TLS: psql (libpq) and the cutover (pgx) accept prefer; migrate (lib/pq) accepts only require or
# disable, so the mode is derived once from the server itself.
if [ -z "${SOLVR_DB_SSLMODE:-}" ]; then
  _ssl=$(docker exec -e PGPASSWORD="$SOLVR_DB_PASSWORD" -e PGSSLMODE=prefer -e PGCONNECT_TIMEOUT=10 solvr-postgres \
    psql -h "$SOLVR_DB_HOST" -p "$SOLVR_DB_PORT" -U "$SOLVR_DB_USER" -d "$SOLVR_DB_NAME" -XAtc 'SHOW ssl') || return 1
  if [ "$_ssl" = on ]; then SOLVR_DB_SSLMODE=require; else SOLVR_DB_SSLMODE=disable; fi
fi
export SOLVR_DB_SSLMODE
PSQL=(docker exec -i -e PGPASSWORD="$SOLVR_DB_PASSWORD" -e PGSSLMODE="$SOLVR_DB_SSLMODE" -e PGCONNECT_TIMEOUT=10
      solvr-postgres psql -X -h "$SOLVR_DB_HOST" -p "$SOLVR_DB_PORT" -U "$SOLVR_DB_USER" -d "$SOLVR_DB_NAME" -v ON_ERROR_STOP=1)
PSQL_RO=(docker exec -i -e PGPASSWORD="$SOLVR_DB_PASSWORD" -e PGSSLMODE="$SOLVR_DB_SSLMODE" -e PGCONNECT_TIMEOUT=10
      -e PGOPTIONS="-c default_transaction_read_only=on"
      solvr-postgres psql -X -h "$SOLVR_DB_HOST" -p "$SOLVR_DB_PORT" -U "$SOLVR_DB_USER" -d "$SOLVR_DB_NAME" -v ON_ERROR_STOP=1)
PGDUMP=(docker exec -e PGPASSWORD="$SOLVR_DB_PASSWORD" -e PGSSLMODE="$SOLVR_DB_SSLMODE" solvr-postgres
      pg_dump -h "$SOLVR_DB_HOST" -p "$SOLVR_DB_PORT" -U "$SOLVR_DB_USER" -d "$SOLVR_DB_NAME" --format=custom --no-owner --no-privileges)
LPSQL=(docker exec -i solvr-postgres psql -X -U solvr -v ON_ERROR_STOP=1)
# The URL migrate and the cutover take; the password is percent-encoded. It is never echoed; it is
# visible in `ps` while migrate or the cutover runs (single-user Mac: accepted).
DBURL="postgres://$(jq -rn --arg v "$SOLVR_DB_USER" '$v|@uri'):$(jq -rn --arg v "$SOLVR_DB_PASSWORD" '$v|@uri')@${SOLVR_DB_HOST}:${SOLVR_DB_PORT}/$(jq -rn --arg v "$SOLVR_DB_NAME" '$v|@uri')?sslmode=${SOLVR_DB_SSLMODE}"
RTDBURL="${LOCAL_PG}/${RESTORE_TEST_DB}?sslmode=disable"
cd "$WINDOW_DIR" || return 1
[ "$(git rev-parse HEAD)" = "$(git rev-parse "$FROZEN_SHA")" ] || { echo "WINDOW_DIR is not at FROZEN_SHA" >&2; return 1; }
echo "window: sha=$(git rev-parse --short HEAD) db=$SOLVR_DB_NAME sslmode=$SOLVR_DB_SSLMODE api=$SOLVR_API_BASE web=$SOLVR_WEB_BASE backups=$BACKUP_DIR"
