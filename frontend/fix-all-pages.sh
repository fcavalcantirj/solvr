#!/bin/bash
# Comprehensive fix for all pages that import Header

cd /Users/fcavalcanti/dev/solvr/frontend

# Find all page.tsx files that import Header
find app -name "page.tsx" -type f | while read file; do
  if grep -q 'from "@/components/header"' "$file" || grep -q "from '@/components/header'" "$file"; then
    # Check if it already has dynamic export
    if ! grep -q "export const dynamic" "$file"; then
      # Check if it has "use client"
      if grep -q '"use client"' "$file"; then
        # Add dynamic export after "use client"
        perl -i -p0e 's/"use client";/"use client";\n\n\/\/ Force dynamic rendering - this page imports Header which uses client-side state\nexport const dynamic = '\''force-dynamic'\'';/s' "$file"
        echo "✓ Added dynamic to: $file"
      else
        # Add both "use client" and dynamic at the top
        perl -i -pe 'print "\"use client\";\n\n// Force dynamic rendering - this page imports Header which uses client-side state\nexport const dynamic = '\''force-dynamic'\'';\n\n" if $. == 1' "$file"
        echo "✓ Added use client + dynamic to: $file"
      fi

      # Remove metadata export if exists (client components can't have it)
      if grep -q "export const metadata" "$file"; then
        perl -i -p0e 's/export const metadata = \{[^}]*\};//gs' "$file"
        echo "   → Also removed metadata export"
      fi
    fi
  fi
done

echo "Done! All pages fixed."
