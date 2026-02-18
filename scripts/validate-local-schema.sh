#!/usr/bin/env bash
set -euo pipefail

# ========================================
# Solvr Local Schema Validation Script
# ========================================
# Compares localhost Docker database schema against production schema documentation
# Reports differences in tables, columns, indexes, and constraints
#
# Prerequisites:
# - Local database running on localhost:5433
# - Production schema backup exists (run backup-prod-db.sh first)
#
# Usage:
#   ./scripts/validate-local-schema.sh

# ---- CONFIG ----
LOCAL_DB_URL="${DATABASE_URL:-postgresql://solvr:solvr_dev@localhost:5433/solvr?sslmode=disable}"
PROD_SCHEMA="./db-backups/schema_latest.sql"
OUTPUT_FILE="./db-backups/schema_diff_$(date +'%Y-%m-%d_%H-%M-%S').txt"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ----------------

echo "🔍 Solvr Schema Validation"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 Local DB:  ${LOCAL_DB_URL}"
echo "📋 Prod Schema: ${PROD_SCHEMA}"
echo ""

# Check if production schema exists
if [[ ! -f "${PROD_SCHEMA}" ]]; then
  echo -e "${RED}❌ ERROR: Production schema not found${NC}"
  echo ""
  echo "Run backup script first:"
  echo "  export SOLVR_DB_PASSWORD='...'"
  echo "  ./scripts/backup-prod-db.sh"
  exit 1
fi

# Extract database connection parts
DB_HOST=$(echo "$LOCAL_DB_URL" | sed -n 's/.*@\([^:]*\):.*/\1/p')
DB_PORT=$(echo "$LOCAL_DB_URL" | sed -n 's/.*:\([0-9]*\)\/.*/\1/p')
DB_NAME=$(echo "$LOCAL_DB_URL" | sed -n 's/.*\/\([^?]*\).*/\1/p')
DB_USER=$(echo "$LOCAL_DB_URL" | sed -n 's/.*:\/\/\([^:]*\):.*/\1/p')
DB_PASS=$(echo "$LOCAL_DB_URL" | sed -n 's/.*:\/\/[^:]*:\([^@]*\)@.*/\1/p')

# Function to run psql queries
run_query() {
  PGPASSWORD="${DB_PASS}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -A -F'|' -c "$1"
}

# Start validation report
exec 3>&1 4>"${OUTPUT_FILE}"

# Also output to stdout and file
log() {
  echo "$@" >&3
  echo "$@" >&4
}

log "# Solvr Schema Validation Report"
log "Generated: $(date)"
log ""
log "Local DB: ${LOCAL_DB_URL}"
log "Production Schema: ${PROD_SCHEMA}"
log ""
log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log ""

# 1. Compare Tables
echo -e "${BLUE}📊 Comparing tables...${NC}"
log "## Tables Comparison"
log ""

PROD_TABLES=$(grep "^CREATE TABLE" "${PROD_SCHEMA}" | sed 's/CREATE TABLE //' | sed 's/ (.*//' | sed 's/public\.//' | sort)
LOCAL_TABLES=$(run_query "SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename;")

log "### Production Tables:"
echo "${PROD_TABLES}" | while read -r table; do
  log "  - ${table}"
done
log ""

log "### Local Tables:"
echo "${LOCAL_TABLES}" | while read -r table; do
  log "  - ${table}"
done
log ""

# Find differences
MISSING_IN_LOCAL=$(comm -23 <(echo "${PROD_TABLES}") <(echo "${LOCAL_TABLES}"))
EXTRA_IN_LOCAL=$(comm -13 <(echo "${PROD_TABLES}") <(echo "${LOCAL_TABLES}"))

if [[ -n "${MISSING_IN_LOCAL}" ]]; then
  echo -e "${RED}❌ Tables missing in LOCAL:${NC}"
  log "### ❌ Tables MISSING in LOCAL:"
  echo "${MISSING_IN_LOCAL}" | while read -r table; do
    echo -e "   ${RED}✗ ${table}${NC}"
    log "  - ${table}"
  done
  log ""
fi

if [[ -n "${EXTRA_IN_LOCAL}" ]]; then
  echo -e "${YELLOW}⚠️  Extra tables in LOCAL (not in production):${NC}"
  log "### ⚠️  Extra tables in LOCAL:"
  echo "${EXTRA_IN_LOCAL}" | while read -r table; do
    echo -e "   ${YELLOW}+ ${table}${NC}"
    log "  - ${table}"
  done
  log ""
fi

# 2. Compare Columns for common tables
echo -e "${BLUE}📋 Comparing columns...${NC}"
log "## Columns Comparison"
log ""

COMMON_TABLES=$(comm -12 <(echo "${PROD_TABLES}") <(echo "${LOCAL_TABLES}"))

while read -r table; do
  [[ -z "${table}" ]] && continue

  # Get production columns
  PROD_COLS=$(grep -A 200 "CREATE TABLE ${table}" "${PROD_SCHEMA}" | \
    sed -n '/CREATE TABLE/,/);/p' | \
    grep -v "CREATE TABLE\|^--\|^)\|^$" | \
    sed 's/^[[:space:]]*//' | \
    sed 's/,$//' | \
    awk '{print $1}' | \
    sort)

  # Get local columns
  LOCAL_COLS=$(run_query "SELECT column_name FROM information_schema.columns WHERE table_name = '${table}' ORDER BY column_name;")

  # Find differences
  MISSING_COLS=$(comm -23 <(echo "${PROD_COLS}") <(echo "${LOCAL_COLS}"))
  EXTRA_COLS=$(comm -13 <(echo "${PROD_COLS}") <(echo "${LOCAL_COLS}"))

  if [[ -n "${MISSING_COLS}" || -n "${EXTRA_COLS}" ]]; then
    echo -e "${YELLOW}⚠️  Table: ${table}${NC}"
    log "### Table: ${table}"

    if [[ -n "${MISSING_COLS}" ]]; then
      echo -e "${RED}   Missing columns in LOCAL:${NC}"
      log "**Missing in LOCAL:**"
      echo "${MISSING_COLS}" | while read -r col; do
        echo -e "   ${RED}✗ ${col}${NC}"
        log "  - ${col}"
      done
    fi

    if [[ -n "${EXTRA_COLS}" ]]; then
      echo -e "${GREEN}   Extra columns in LOCAL:${NC}"
      log "**Extra in LOCAL:**"
      echo "${EXTRA_COLS}" | while read -r col; do
        echo -e "   ${GREEN}+ ${col}${NC}"
        log "  - ${col}"
      done
    fi
    log ""
  fi
done <<< "${COMMON_TABLES}"

# 3. Compare Indexes
echo -e "${BLUE}🔍 Comparing indexes...${NC}"
log "## Indexes Comparison"
log ""

PROD_INDEXES=$(grep "^CREATE.*INDEX" "${PROD_SCHEMA}" | sed 's/CREATE .* INDEX //' | sed 's/ ON .*//' | sed 's/public\.//' | sort)
LOCAL_INDEXES=$(run_query "SELECT indexname FROM pg_indexes WHERE schemaname = 'public' ORDER BY indexname;")

MISSING_INDEXES=$(comm -23 <(echo "${PROD_INDEXES}") <(echo "${LOCAL_INDEXES}"))
EXTRA_INDEXES=$(comm -13 <(echo "${PROD_INDEXES}") <(echo "${LOCAL_INDEXES}"))

if [[ -n "${MISSING_INDEXES}" ]]; then
  echo -e "${RED}❌ Indexes missing in LOCAL:${NC}"
  log "### ❌ Indexes MISSING in LOCAL:"
  echo "${MISSING_INDEXES}" | while read -r idx; do
    echo -e "   ${RED}✗ ${idx}${NC}"
    log "  - ${idx}"
  done
  log ""
fi

if [[ -n "${EXTRA_INDEXES}" ]]; then
  echo -e "${GREEN}✓ Extra indexes in LOCAL:${NC}"
  log "### ✓ Extra indexes in LOCAL:"
  echo "${EXTRA_INDEXES}" | while read -r idx; do
    echo -e "   ${GREEN}+ ${idx}${NC}"
    log "  - ${idx}"
  done
  log ""
fi

# Summary
log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log ""
log "## Summary"
log ""

TOTAL_ISSUES=0

if [[ -n "${MISSING_IN_LOCAL}" ]]; then
  MISSING_COUNT=$(echo "${MISSING_IN_LOCAL}" | wc -l | tr -d ' ')
  log "- Missing tables: ${MISSING_COUNT}"
  TOTAL_ISSUES=$((TOTAL_ISSUES + MISSING_COUNT))
fi

if [[ -n "${MISSING_INDEXES}" ]]; then
  MISSING_IDX_COUNT=$(echo "${MISSING_INDEXES}" | wc -l | tr -d ' ')
  log "- Missing indexes: ${MISSING_IDX_COUNT}"
  TOTAL_ISSUES=$((TOTAL_ISSUES + MISSING_IDX_COUNT))
fi

echo ""
if [[ ${TOTAL_ISSUES} -eq 0 ]]; then
  echo -e "${GREEN}✅ Schema validation passed!${NC}"
  echo -e "${GREEN}   Local database matches production schema.${NC}"
  log "✅ **PASSED** - Local database matches production schema"
else
  echo -e "${RED}❌ Schema validation failed!${NC}"
  echo -e "${RED}   Found ${TOTAL_ISSUES} issues.${NC}"
  log "❌ **FAILED** - Found ${TOTAL_ISSUES} issues"
fi

echo ""
echo -e "${BLUE}📁 Full report saved: ${OUTPUT_FILE}${NC}"
log ""
log "Report saved: ${OUTPUT_FILE}"

exec 3>&- 4>&-

exit ${TOTAL_ISSUES}
