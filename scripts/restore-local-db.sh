#!/usr/bin/env bash
set -euo pipefail

# ========================================
# Solvr Local Database Restore Script
# ========================================
# Restores a production database backup to local Docker PostgreSQL
#
# ⚠️  WARNING: This will DELETE ALL LOCAL DATA in the solvr database!
#
# Prerequisites:
# - Local Docker container 'solvr-postgres' running
# - Backup file in ./db-backups/ directory
#
# Usage:
#   ./scripts/restore-local-db.sh [backup-file]
#   ./scripts/restore-local-db.sh                     # Uses latest backup
#   ./scripts/restore-local-db.sh db-backups/solvr_2026-02-17_14-30-00.dump
#   ./scripts/restore-local-db.sh --dry-run           # Show plan without executing

# ---- CONFIG ----
CONTAINER="solvr-postgres"
DB_NAME="solvr"
DB_USER="solvr"
DB_PASSWORD="solvr_dev"
LOCAL_PORT="5433"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# ---- FUNCTIONS ----
show_usage() {
  echo "Usage: $0 [backup-file] [--dry-run]"
  echo ""
  echo "Examples:"
  echo "  $0                                                  # Uses latest backup"
  echo "  $0 db-backups/solvr_2026-02-17_14-30-00.dump       # Specific backup"
  echo "  $0 --dry-run                                        # Show plan without executing"
}

check_docker() {
  if ! docker ps | grep -q "${CONTAINER}"; then
    echo -e "${RED}❌ ERROR: Docker container '${CONTAINER}' is not running${NC}"
    echo ""
    echo "Start it with:"
    echo "  docker compose up -d postgres"
    exit 1
  fi
}

kill_api_server() {
  echo -e "${CYAN}🔪 Killing any running API servers on port 8080...${NC}"
  lsof -ti:8080 | xargs kill -9 2>/dev/null || true
  sleep 1
}

show_table_counts() {
  local label=$1
  echo -e "${CYAN}📊 ${label}:${NC}"
  docker exec "${CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" -t -c "
    SELECT
      'Posts: ' || COUNT(*) FROM posts WHERE deleted_at IS NULL
    UNION ALL
    SELECT
      'Agents: ' || COUNT(*) FROM agents WHERE deleted_at IS NULL
    UNION ALL
    SELECT
      'Users: ' || COUNT(*) FROM users WHERE deleted_at IS NULL
    UNION ALL
    SELECT
      'Votes: ' || COUNT(*) FROM votes
  " 2>/dev/null | sed 's/^/  /' || echo "  (Database not accessible)"
}

# ---- MAIN ----
DRY_RUN=false
BACKUP_FILE=""

# Parse arguments
for arg in "$@"; do
  case $arg in
    --dry-run)
      DRY_RUN=true
      ;;
    --help|-h)
      show_usage
      exit 0
      ;;
    *)
      BACKUP_FILE="$arg"
      ;;
  esac
done

# Determine backup file to use
if [[ -z "${BACKUP_FILE}" ]]; then
  # Use latest
  BACKUP_FILE="./db-backups/solvr_latest.dump"
  if [[ ! -f "${BACKUP_FILE}" ]]; then
    # Try to find the most recent dump
    LATEST=$(ls -1t ./db-backups/solvr_*.dump 2>/dev/null | head -n1 || true)
    if [[ -z "${LATEST}" ]]; then
      echo -e "${RED}❌ ERROR: No backup files found in ./db-backups/${NC}"
      echo ""
      echo "Create a backup first:"
      echo "  export SOLVR_DB_PASSWORD='your-password-here'"
      echo "  ./scripts/backup-prod-db.sh"
      exit 1
    fi
    BACKUP_FILE="${LATEST}"
  fi
fi

# Verify backup file exists
if [[ ! -f "${BACKUP_FILE}" ]]; then
  echo -e "${RED}❌ ERROR: Backup file not found: ${BACKUP_FILE}${NC}"
  exit 1
fi

# Verify it's a PostgreSQL dump
if ! file "${BACKUP_FILE}" | grep -q "PostgreSQL custom database dump"; then
  echo -e "${RED}❌ ERROR: File is not a PostgreSQL custom dump: ${BACKUP_FILE}${NC}"
  exit 1
fi

echo ""
echo "🔄 Solvr Database Restore"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${CYAN}📦 Container:${NC} ${CONTAINER}"
echo -e "${CYAN}🗄️  Database:${NC}  ${DB_NAME}"
echo -e "${CYAN}📁 Backup:${NC}    ${BACKUP_FILE}"
echo -e "${CYAN}📊 Size:${NC}      $(du -h "${BACKUP_FILE}" | cut -f1)"
echo ""

if [[ "${DRY_RUN}" == "true" ]]; then
  echo -e "${YELLOW}🔍 DRY RUN MODE - No changes will be made${NC}"
  echo ""
  echo "Plan:"
  echo "  1. Check Docker container is running"
  echo "  2. Kill API servers on port 8080"
  echo "  3. Show current table counts"
  echo "  4. Drop and recreate database '${DB_NAME}'"
  echo "  5. Restore from: ${BACKUP_FILE}"
  echo "  6. Show restored table counts"
  echo "  7. Verify data integrity"
  echo ""
  exit 0
fi

# Safety check
check_docker

echo -e "${RED}⚠️  WARNING: This will DELETE ALL LOCAL DATA!${NC}"
echo ""
read -p "Type 'yes' to continue: " -r
echo ""
if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
  echo "Aborted."
  exit 1
fi

# Show current state
show_table_counts "Current database state"
echo ""

# Kill API server
kill_api_server

# Drop and recreate database
echo -e "${CYAN}🗑️  Dropping database '${DB_NAME}'...${NC}"
docker exec "${CONTAINER}" psql -U "${DB_USER}" -d postgres -c "DROP DATABASE IF EXISTS ${DB_NAME};" 2>&1 | grep -v "NOTICE" || true

echo -e "${CYAN}🆕 Creating fresh database '${DB_NAME}'...${NC}"
docker exec "${CONTAINER}" psql -U "${DB_USER}" -d postgres -c "CREATE DATABASE ${DB_NAME};"

# Copy dump file into container
echo -e "${CYAN}📋 Copying dump file into container...${NC}"
docker cp "${BACKUP_FILE}" "${CONTAINER}:/tmp/restore.dump"

# Restore using pg_restore
echo -e "${CYAN}⚡ Restoring database (this may take a minute)...${NC}"
docker exec "${CONTAINER}" pg_restore -U "${DB_USER}" -d "${DB_NAME}" --no-owner --no-acl --verbose /tmp/restore.dump 2>&1 | \
  grep -E "(processing|creating|restoring)" | tail -n 20 || true

# Cleanup
echo -e "${CYAN}🧹 Cleaning up temporary files...${NC}"
docker exec "${CONTAINER}" rm -f /tmp/restore.dump

# Show restored state
echo ""
show_table_counts "Restored database state"
echo ""

# Verify with sample data
echo -e "${CYAN}🔍 Sample data verification:${NC}"
docker exec "${CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" -c "
  SELECT id, display_name, model
  FROM agents
  WHERE deleted_at IS NULL
  ORDER BY created_at DESC
  LIMIT 5;
" 2>/dev/null || echo "  (Could not fetch sample data)"

echo ""
echo -e "${GREEN}✅ Restore complete!${NC}"
echo ""
echo "Next steps:"
echo "  1. Start the API server:"
echo "     cd backend && go run ./cmd/api"
echo ""
echo "  2. Test with real production data:"
echo "     curl 'http://localhost:8080/v1/search?q=test'"
echo ""
echo "  3. Run migrations if needed:"
echo "     cd backend && migrate -path migrations -database 'postgresql://solvr:solvr_dev@localhost:5433/solvr' up"
