#!/bin/bash
# sync-skill.sh
# Syncs skill files to frontend/public for web serving
#
# Syncs:
# - skill/SKILL.md -> frontend/public/skill.md
# - scripts/install-solvr-skill.sh -> frontend/public/install.sh
# - skill/ folder -> frontend/public/solvr-skill.zip

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PUBLIC_DIR="$REPO_ROOT/frontend/public"

# 1. Sync SKILL.md
SOURCE_SKILL="$REPO_ROOT/skill/SKILL.md"
DEST_SKILL="$PUBLIC_DIR/skill.md"

if [ ! -f "$SOURCE_SKILL" ]; then
    echo "Error: Source file not found: $SOURCE_SKILL"
    exit 1
fi

cp "$SOURCE_SKILL" "$DEST_SKILL"
echo "Synced: skill/SKILL.md -> frontend/public/skill.md"

# 1b. Sync HEARTBEAT.md
SOURCE_HEARTBEAT="$REPO_ROOT/skill/HEARTBEAT.md"
DEST_HEARTBEAT="$PUBLIC_DIR/heartbeat.md"

if [ -f "$SOURCE_HEARTBEAT" ]; then
    cp "$SOURCE_HEARTBEAT" "$DEST_HEARTBEAT"
    echo "Synced: skill/HEARTBEAT.md -> frontend/public/heartbeat.md"
fi

# 2. Sync install script
SOURCE_INSTALL="$REPO_ROOT/scripts/install-solvr-skill.sh"
DEST_INSTALL="$PUBLIC_DIR/install.sh"

if [ -f "$SOURCE_INSTALL" ]; then
    cp "$SOURCE_INSTALL" "$DEST_INSTALL"
    echo "Synced: scripts/install-solvr-skill.sh -> frontend/public/install.sh"
fi

# 3. Generate ZIP of skill folder
DEST_ZIP="$PUBLIC_DIR/solvr-skill.zip"
SKILL_DIR="$REPO_ROOT/skill"

if command -v zip &> /dev/null; then
    cd "$SKILL_DIR"
    zip -r "$DEST_ZIP" \
        SKILL.md \
        HEARTBEAT.md \
        skill.json \
        LICENSE \
        references/api.md \
        references/examples.md \
        scripts/solvr.sh \
        -x "*.DS_Store" \
        2>/dev/null || true
    cd "$REPO_ROOT"
    echo "Created: frontend/public/solvr-skill.zip"
else
    echo "Warning: zip command not found, skipping ZIP creation"
fi

echo "Skill sync complete"
