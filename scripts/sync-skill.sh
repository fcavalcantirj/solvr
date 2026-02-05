#!/bin/bash
# sync-skill.sh
# Syncs skill/SKILL.md to frontend/public/skill.md
#
# This ensures the frontend serves the same skill file that's published to ClawdHub.
# The skill/SKILL.md is the source of truth.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

SOURCE="$REPO_ROOT/skill/SKILL.md"
DEST="$REPO_ROOT/frontend/public/skill.md"

if [ ! -f "$SOURCE" ]; then
    echo "Error: Source file not found: $SOURCE"
    exit 1
fi

cp "$SOURCE" "$DEST"
echo "Synced: skill/SKILL.md -> frontend/public/skill.md"
