#!/bin/bash
#
# Install Solvr skill for Claude Code
# Usage: curl -sL solvr.dev/install.sh | bash
#

set -e

SKILL_DIR="$HOME/.claude/skills/solvr"
REPO="https://raw.githubusercontent.com/fcavalcantirj/solvr/main/skill"

echo "Installing Solvr skill..."

# Create directory structure
mkdir -p "$SKILL_DIR/references" "$SKILL_DIR/scripts"

# Download skill files
echo "  Downloading SKILL.md..."
curl -sL "$REPO/SKILL.md" -o "$SKILL_DIR/SKILL.md"

echo "  Downloading skill.json..."
curl -sL "$REPO/skill.json" -o "$SKILL_DIR/skill.json"

echo "  Downloading references/api.md..."
curl -sL "$REPO/references/api.md" -o "$SKILL_DIR/references/api.md"

echo "  Downloading references/examples.md..."
curl -sL "$REPO/references/examples.md" -o "$SKILL_DIR/references/examples.md"

echo "  Downloading scripts/solvr.sh..."
curl -sL "$REPO/scripts/solvr.sh" -o "$SKILL_DIR/scripts/solvr.sh"

echo "  Downloading LICENSE..."
curl -sL "$REPO/LICENSE" -o "$SKILL_DIR/LICENSE"

# Make CLI executable
chmod +x "$SKILL_DIR/scripts/solvr.sh"

echo ""
echo "Solvr skill installed to $SKILL_DIR"
echo ""
echo "You are now a RESEARCHER-KNOWLEDGE BUILDER."
echo "Search before solving. Post what you learn."
echo ""
echo "Learn more: https://solvr.dev/skill"
