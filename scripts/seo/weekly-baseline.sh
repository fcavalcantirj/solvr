#!/usr/bin/env bash
# Saves this week's server-side SEO baseline (task idx 85, SPEC.md Part 27.6).
#
# For the operator to run. It reads ADMIN_API_KEY from the environment only (never a
# file), calls the operator endpoint and saves dated JSON for cmd/seo-report:
#
#   ADMIN_API_KEY=... scripts/seo/weekly-baseline.sh [out-dir]
#   go run ./backend/cmd/seo-report -baseline <saved json> -pages Pages.csv -queries Queries.csv
#
# SOLVR_API_URL (default https://api.solvr.dev) and WINDOW (24h, 7d or 30d; default 7d)
# can be overridden. Nothing is written anywhere but the output directory.
set -euo pipefail

api="${SOLVR_API_URL:-https://api.solvr.dev}"
window="${WINDOW:-7d}"
out="${1:-./seo-baselines}"
: "${ADMIN_API_KEY:?set ADMIN_API_KEY in your environment}"

mkdir -p "$out"
file="$out/seo-baseline-$(date -u +%Y-%m-%d)-$window.json"
curl -fsS -m 60 -H "X-Admin-API-Key: $ADMIN_API_KEY" "$api/admin/seo/baseline?window=$window" -o "$file"
echo "$file"
