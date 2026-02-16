#!/bin/bash
# Script to add dynamic export to all pages that import Header
# This fixes the Next.js 15 build error with useState in Header component

pages=(
  "app/privacy/page.tsx"
  "app/skill/page.tsx"
  "app/agents/[id]/page.tsx"
  "app/agents/page.tsx"
  "app/problems/[id]/page.tsx"
  "app/problems/page.tsx"
  "app/mcp/page.tsx"
  "app/leaderboard/page.tsx"
  "app/docs/guides/page.tsx"
  "app/how-it-works/page.tsx"
  "app/feed/page.tsx"
  "app/users/[id]/page.tsx"
  "app/users/page.tsx"
  "app/ideas/[id]/page.tsx"
  "app/ideas/page.tsx"
  "app/api-docs/page.tsx"
  "app/page.tsx"
  "app/questions/[id]/page.tsx"
  "app/questions/page.tsx"
)

for page in "${pages[@]}"; do
  file="/Users/fcavalcanti/dev/solvr/frontend/$page"

  # Check if file exists and doesn't already have dynamic export
  if [ -f "$file" ] && ! grep -q "export const dynamic" "$file"; then
    # Add dynamic export after "use client" directive
    sed -i '' '/^"use client";$/a\
\
// Force dynamic rendering - this page imports Header which uses client-side state\
export const dynamic = '\''force-dynamic'\'';
' "$file"
    echo "✓ Fixed: $page"
  else
    echo "→ Skipped: $page (already has dynamic export or file not found)"
  fi
done

echo "Done!"
