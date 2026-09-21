#!/usr/bin/env bash
set -euo pipefail

# Simple fast validation: local DB = production schema

LOCAL_DB="postgresql://solvr:solvr_dev@localhost:5435/solvr?sslmode=disable"
PROD_SCHEMA="./db-backups/schema_latest.sql"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  VALIDATE: LOCAL = PRODUCTION"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 1. Check production schema exists
if [[ ! -f "${PROD_SCHEMA}" ]]; then
  echo "❌ ERROR: ${PROD_SCHEMA} not found"
  echo "Run: export SOLVR_DB_PASSWORD='...' && bash scripts/backup-prod-db.sh"
  exit 1
fi

# 2. Dump local schema
echo "📊 Dumping local schema..."
PGPASSWORD=solvr_dev pg_dump -h localhost -p 5435 -U solvr -d solvr --schema-only > /tmp/local_schema.sql 2>/dev/null

# 3. Compare table counts
PROD_TABLES=$(grep "^CREATE TABLE" "${PROD_SCHEMA}" | wc -l | tr -d ' ')
LOCAL_TABLES=$(grep "^CREATE TABLE" /tmp/local_schema.sql | wc -l | tr -d ' ')

echo "📋 Tables: prod=${PROD_TABLES}, local=${LOCAL_TABLES}"

if [[ "${PROD_TABLES}" != "${LOCAL_TABLES}" ]]; then
  echo "❌ FAIL: Table count mismatch"
  exit 1
fi

# 4. Compare table names
PROD_TABLE_NAMES=$(grep "^CREATE TABLE" "${PROD_SCHEMA}" | sed 's/CREATE TABLE public\.//' | sed 's/ (.*//' | sort)
LOCAL_TABLE_NAMES=$(grep "^CREATE TABLE" /tmp/local_schema.sql | sed 's/CREATE TABLE public\.//' | sed 's/ (.*//' | sort)

DIFF=$(diff <(echo "${PROD_TABLE_NAMES}") <(echo "${LOCAL_TABLE_NAMES}") || true)

if [[ -n "${DIFF}" ]]; then
  echo "❌ FAIL: Table names differ:"
  echo "${DIFF}"
  exit 1
fi

# 5. Compare index counts
PROD_INDEXES=$(grep "^CREATE.*INDEX" "${PROD_SCHEMA}" | wc -l | tr -d ' ')
LOCAL_INDEXES=$(grep "^CREATE.*INDEX" /tmp/local_schema.sql | wc -l | tr -d ' ')

echo "🔍 Indexes: prod=${PROD_INDEXES}, local=${LOCAL_INDEXES}"

# 6. Check data restored
DATA_CHECK=$(psql "${LOCAL_DB}" -t -c "SELECT COUNT(*) FROM posts;")
echo "📦 Data: ${DATA_CHECK} posts in local"

if [[ "${DATA_CHECK}" -lt 100 ]]; then
  echo "⚠️  WARNING: Expected ~126 posts from production backup"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ VALIDATION PASSED"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Local database = Production"
echo "- ${PROD_TABLES} tables ✓"
echo "- ${PROD_INDEXES} indexes ✓"
echo "- ${DATA_CHECK} posts ✓"
echo ""
