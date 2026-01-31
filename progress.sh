#!/bin/bash

# Count passed and total requirements in PRD

prd_file="specs/prd-v1.json"

if [ ! -f "$prd_file" ]; then
  echo "0/0 (0%) - PRD not found"
  exit 0
fi

# Count total (lines with "passes":)
total=$(grep -c '"passes"' "$prd_file" | tr -d '\n')

# Count passed (lines with "passes": true)
passed=$(grep -c '"passes": true' "$prd_file" | tr -d '\n')

# Handle empty/zero cases
total=${total:-0}
passed=${passed:-0}

if [ "$total" -eq 0 ]; then
  echo "0/0 (0%)"
else
  percent=$((passed * 100 / total))
  echo "${passed}/${total} (${percent}%)"
fi
