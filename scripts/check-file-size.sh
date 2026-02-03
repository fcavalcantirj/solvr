#!/bin/bash
# Check that no code file exceeds 800 lines (Golden Rule #2 from CLAUDE.md)
# Exit code 1 if any file exceeds the limit

set -e

MAX_LINES=800
VIOLATIONS=0

echo "Checking file sizes (max ${MAX_LINES} lines)..."
echo ""

# Find Go files in backend
echo "Checking backend Go files..."
while IFS= read -r -d '' file; do
    lines=$(wc -l < "$file")
    if [ "$lines" -gt "$MAX_LINES" ]; then
        echo "❌ VIOLATION: $file has $lines lines (max: $MAX_LINES)"
        VIOLATIONS=$((VIOLATIONS + 1))
    fi
done < <(find backend -name "*.go" -type f -print0 2>/dev/null)

# Find TypeScript/TSX files in frontend
echo "Checking frontend TypeScript files..."
while IFS= read -r -d '' file; do
    # Skip node_modules
    if [[ "$file" == *"node_modules"* ]]; then
        continue
    fi
    lines=$(wc -l < "$file")
    if [ "$lines" -gt "$MAX_LINES" ]; then
        echo "❌ VIOLATION: $file has $lines lines (max: $MAX_LINES)"
        VIOLATIONS=$((VIOLATIONS + 1))
    fi
done < <(find frontend -name "*.ts" -o -name "*.tsx" -type f -print0 2>/dev/null)

# Summary
echo ""
if [ "$VIOLATIONS" -gt 0 ]; then
    echo "❌ Found $VIOLATIONS file(s) exceeding $MAX_LINES lines."
    echo "Please split large files into smaller modules."
    exit 1
else
    echo "✅ All code files are within the $MAX_LINES line limit."
    exit 0
fi
