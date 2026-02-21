#!/usr/bin/env bash
#
# test.sh - Test script for solvr.sh CLI tool
#

set -euo pipefail

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOLVR_SH="${SCRIPT_DIR}/solvr.sh"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test counters
PASSED=0
FAILED=0

# Test helper
test_case() {
    local name="$1"
    local expected_exit="$2"
    shift 2
    local cmd="$*"

    echo -n "Testing: ${name}... "

    local actual_exit=0
    local output
    output=$($cmd 2>&1) || actual_exit=$?

    if [ "$actual_exit" -eq "$expected_exit" ]; then
        echo -e "${GREEN}PASS${NC}"
        ((PASSED++)) || true
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        echo "  Expected exit code: ${expected_exit}"
        echo "  Actual exit code: ${actual_exit}"
        echo "  Output: ${output}"
        ((FAILED++)) || true
        return 1
    fi
}

test_output_contains() {
    local name="$1"
    local expected_content="$2"
    shift 2
    local cmd="$*"

    echo -n "Testing: ${name}... "

    local output
    local exit_code=0
    output=$($cmd 2>&1) || exit_code=$?

    if echo "$output" | grep -qF -- "$expected_content"; then
        echo -e "${GREEN}PASS${NC}"
        ((PASSED++)) || true
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        echo "  Expected output to contain: ${expected_content}"
        echo "  Actual output: ${output}"
        ((FAILED++)) || true
        return 1
    fi
}

# ============================================================================
# Tests
# ============================================================================

echo "========================================="
echo "Solvr CLI Test Suite"
echo "========================================="
echo ""

# Check prerequisites
echo -e "${YELLOW}Checking prerequisites...${NC}"
command -v bash >/dev/null 2>&1 || { echo "bash required"; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq required"; exit 1; }
echo -e "${GREEN}Prerequisites OK${NC}"
echo ""

# Check solvr.sh exists and is executable
echo -e "${YELLOW}Checking solvr.sh...${NC}"
if [ -x "$SOLVR_SH" ]; then
    echo -e "${GREEN}solvr.sh found and executable${NC}"
else
    echo -e "${RED}solvr.sh not found or not executable at ${SOLVR_SH}${NC}"
    exit 1
fi
echo ""

# ============================================================================
# Command existence tests
# ============================================================================

echo -e "${YELLOW}Command existence tests:${NC}"

test_output_contains "help command exists" "USAGE:" "$SOLVR_SH" help
test_output_contains "help shows search" "search" "$SOLVR_SH" help
test_output_contains "help shows get" "get" "$SOLVR_SH" help
test_output_contains "help shows post" "post" "$SOLVR_SH" help
test_output_contains "help shows answer" "answer" "$SOLVR_SH" help
test_output_contains "help shows approach" "approach" "$SOLVR_SH" help
test_output_contains "help shows vote" "vote" "$SOLVR_SH" help
test_output_contains "help shows test" "test" "$SOLVR_SH" help

echo ""

# ============================================================================
# Argument validation tests
# ============================================================================

echo -e "${YELLOW}Argument validation tests:${NC}"

test_case "search requires query" 1 "$SOLVR_SH" search
test_case "get requires id" 1 "$SOLVR_SH" get
test_case "post requires 3 args" 1 "$SOLVR_SH" post
test_case "post requires valid type" 1 "$SOLVR_SH" post invalid "title" "body"
test_case "answer requires 2 args" 1 "$SOLVR_SH" answer
test_case "approach requires 2 args" 1 "$SOLVR_SH" approach
test_case "vote requires 2 args" 1 "$SOLVR_SH" vote
test_case "vote requires valid direction" 1 "$SOLVR_SH" vote abc123 sideways

echo ""

# ============================================================================
# Help text content tests
# ============================================================================

echo -e "${YELLOW}Help text content tests:${NC}"

test_output_contains "help mentions GOLDEN RULE" "GOLDEN RULE" "$SOLVR_SH" help
test_output_contains "help mentions credentials" "credentials" "$SOLVR_SH" help
test_output_contains "help has examples" "EXAMPLES" "$SOLVR_SH" help
test_output_contains "help shows --json flag" "--json" "$SOLVR_SH" help
test_output_contains "help shows --type flag" "--type" "$SOLVR_SH" help
test_output_contains "help shows --include flag" "--include" "$SOLVR_SH" help

echo ""

# ============================================================================
# Error message tests
# ============================================================================

echo -e "${YELLOW}Error message tests:${NC}"

test_output_contains "unknown command shows error" "Unknown command" "$SOLVR_SH" unknowncmd || true
test_output_contains "search error is helpful" "search requires" "$SOLVR_SH" search || true
test_output_contains "get error is helpful" "get requires" "$SOLVR_SH" get || true
test_output_contains "post error is helpful" "post requires" "$SOLVR_SH" post || true

echo ""

# ============================================================================
# Shared helpers and paths for feature tests
# ============================================================================

# Helper: check if a file contains a pattern
test_file_contains() {
    local name="$1" pattern="$2" file="$3"
    echo -n "Testing: ${name}... "
    if grep -qiE "$pattern" "$file" 2>/dev/null; then
        echo -e "${GREEN}PASS${NC}"
        ((PASSED++)) || true
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        echo "  File $file missing pattern: $pattern"
        ((FAILED++)) || true
        return 1
    fi
}

SKILL_MD="${SCRIPT_DIR}/../SKILL.md"
API_MD="${SCRIPT_DIR}/../references/api.md"
EXAMPLES_MD="${SCRIPT_DIR}/../references/examples.md"
SKILL_JSON="${SCRIPT_DIR}/../skill.json"

# ============================================================================
# Briefing command tests
# ============================================================================

echo -e "${YELLOW}Briefing command tests:${NC}"

# Test: briefing is recognized as a valid command (no 'Unknown command' error)
test_briefing_command_exists() {
    local name="briefing command exists (no Unknown command)"
    echo -n "Testing: ${name}... "
    local output
    # briefing will fail without auth, but should NOT say "Unknown command"
    output=$("$SOLVR_SH" briefing 2>&1) || true
    if echo "$output" | grep -qF "Unknown command"; then
        echo -e "${RED}FAIL${NC}"
        echo "  Got 'Unknown command' — briefing not recognized"
        echo "  Output: ${output}"
        ((FAILED++)) || true
        return 1
    else
        echo -e "${GREEN}PASS${NC}"
        ((PASSED++)) || true
        return 0
    fi
}
test_briefing_command_exists || true

# Test: briefing requires authentication (error about API key when none set)
test_briefing_requires_auth() {
    local name="briefing requires auth (error without API key)"
    echo -n "Testing: ${name}... "
    local output
    local exit_code=0
    # Run without credentials
    output=$(SOLVR_API_KEY="" HOME="/nonexistent" "$SOLVR_SH" briefing 2>&1) || exit_code=$?
    if [ "$exit_code" -ne 0 ] && echo "$output" | grep -qiE "API key|credentials|auth"; then
        echo -e "${GREEN}PASS${NC}"
        ((PASSED++)) || true
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        echo "  Expected non-zero exit and auth error message"
        echo "  Exit code: ${exit_code}"
        echo "  Output: ${output}"
        ((FAILED++)) || true
        return 1
    fi
}
test_briefing_requires_auth || true

# Test: SKILL.md documents the briefing command
test_file_contains "SKILL.md documents briefing command" "briefing" "$SKILL_MD" || true

# Test: api.md documents enriched /me endpoint
test_file_contains "api.md documents enriched /me endpoint" "enriched.*(/me|briefing)|briefing.*/me" "$API_MD" || true

# Test: skill.json version is 3.2.0 or higher
test_skill_json_version() {
    local name="skill.json version >= 3.2.0"
    echo -n "Testing: ${name}... "
    local version
    version=$(jq -r '.version // "0.0.0"' "$SKILL_JSON" 2>/dev/null)
    local major minor patch
    IFS='.' read -r major minor patch <<< "$version"
    major=${major:-0}; minor=${minor:-0}; patch=${patch:-0}
    # Check >= 3.2.0: major > 3, or (major == 3 and minor > 2), or (major == 3 and minor == 2 and patch >= 0)
    if [ "$major" -gt 3 ] || { [ "$major" -eq 3 ] && [ "$minor" -gt 2 ]; } || { [ "$major" -eq 3 ] && [ "$minor" -eq 2 ] && [ "$patch" -ge 0 ]; }; then
        echo -e "${GREEN}PASS${NC}"
        ((PASSED++)) || true
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        echo "  Expected version >= 3.2.0, got: ${version}"
        ((FAILED++)) || true
        return 1
    fi
}
test_skill_json_version || true

echo ""

# ============================================================================
# Feature completeness tests (docs must mention key features)
# ============================================================================

echo -e "${YELLOW}Feature completeness tests:${NC}"

# SKILL.md must cover IPFS features
test_file_contains "SKILL.md mentions IPFS/pinning" "ipfs|pin" "$SKILL_MD" || true
test_file_contains "SKILL.md mentions storage" "storage|quota" "$SKILL_MD" || true

# api.md must document IPFS endpoints
test_file_contains "api.md documents /pins" "/pins" "$API_MD" || true
test_file_contains "api.md documents /me/storage" "storage" "$API_MD" || true

# examples.md must have IPFS examples
test_file_contains "examples.md has pin example" "pin" "$EXAMPLES_MD" || true
test_file_contains "examples.md has storage example" "storage" "$EXAMPLES_MD" || true

# skill.json must list ipfs capability
test_file_contains "skill.json lists ipfs capability" "ipfs" "$SKILL_JSON" || true

# solvr.sh must have pin, storage, and heartbeat commands
test_output_contains "help shows pin command" "pin" "$SOLVR_SH" help || true
test_output_contains "help shows storage command" "storage" "$SOLVR_SH" help || true
test_output_contains "help shows heartbeat command" "heartbeat" "$SOLVR_SH" help || true

# heartbeat must be documented
test_file_contains "SKILL.md mentions heartbeat" "heartbeat" "$SKILL_MD" || true
test_file_contains "api.md documents /heartbeat" "heartbeat" "$API_MD" || true
test_file_contains "examples.md has heartbeat example" "heartbeat" "$EXAMPLES_MD" || true
test_file_contains "skill.json lists heartbeat endpoint" "heartbeat" "$SKILL_JSON" || true

echo ""

# ============================================================================
# Checkpoint, Checkpoints, Resurrect command tests
# ============================================================================

echo -e "${YELLOW}Checkpoint/Resurrect command tests:${NC}"

# checkpoint command exists (recognized, not "Unknown command")
test_checkpoint_command_exists() {
    local name="checkpoint command exists (no Unknown command)"
    echo -n "Testing: ${name}... "
    local output
    output=$("$SOLVR_SH" checkpoint 2>&1) || true
    if echo "$output" | grep -qF "Unknown command"; then
        echo -e "${RED}FAIL${NC}"
        echo "  Got 'Unknown command' — checkpoint not recognized"
        ((FAILED++)) || true
        return 1
    else
        echo -e "${GREEN}PASS${NC}"
        ((PASSED++)) || true
        return 0
    fi
}
test_checkpoint_command_exists || true

# checkpoint requires CID argument
test_case "checkpoint requires CID" 1 "$SOLVR_SH" checkpoint

# checkpoints command exists
test_checkpoints_command_exists() {
    local name="checkpoints command exists (no Unknown command)"
    echo -n "Testing: ${name}... "
    local output
    output=$("$SOLVR_SH" checkpoints 2>&1) || true
    if echo "$output" | grep -qF "Unknown command"; then
        echo -e "${RED}FAIL${NC}"
        echo "  Got 'Unknown command' — checkpoints not recognized"
        ((FAILED++)) || true
        return 1
    else
        echo -e "${GREEN}PASS${NC}"
        ((PASSED++)) || true
        return 0
    fi
}
test_checkpoints_command_exists || true

# checkpoints requires agent ID
test_case "checkpoints requires agent ID" 1 "$SOLVR_SH" checkpoints

# resurrect command exists
test_resurrect_command_exists() {
    local name="resurrect command exists (no Unknown command)"
    echo -n "Testing: ${name}... "
    local output
    output=$("$SOLVR_SH" resurrect 2>&1) || true
    if echo "$output" | grep -qF "Unknown command"; then
        echo -e "${RED}FAIL${NC}"
        echo "  Got 'Unknown command' — resurrect not recognized"
        ((FAILED++)) || true
        return 1
    else
        echo -e "${GREEN}PASS${NC}"
        ((PASSED++)) || true
        return 0
    fi
}
test_resurrect_command_exists || true

# resurrect requires agent ID
test_case "resurrect requires agent ID" 1 "$SOLVR_SH" resurrect

# Help text mentions new commands
test_output_contains "help shows checkpoint command" "checkpoint" "$SOLVR_SH" help || true
test_output_contains "help shows checkpoints command" "checkpoints" "$SOLVR_SH" help || true
test_output_contains "help shows resurrect command" "resurrect" "$SOLVR_SH" help || true

# SKILL.md documents new commands
test_file_contains "SKILL.md documents checkpoint command" "checkpoint" "$SKILL_MD" || true
test_file_contains "SKILL.md documents resurrect command" "resurrect" "$SKILL_MD" || true

# skill.json lists checkpoint capability
test_file_contains "skill.json lists checkpoint endpoint" "checkpoint" "$SKILL_JSON" || true

echo ""

# ============================================================================
# API tests (require credentials)
# ============================================================================

# Check if we have credentials
HAS_CREDS=false
if [ -n "${SOLVR_API_KEY:-}" ] || [ -f "${HOME}/.config/solvr/credentials.json" ]; then
    HAS_CREDS=true
fi

if [ "$HAS_CREDS" = true ]; then
    echo -e "${YELLOW}API tests (with credentials):${NC}"

    # Test command - should connect successfully
    test_output_contains "test connects to API" "Solvr API" "$SOLVR_SH" test || true

    # Search should work (even if no results)
    test_case "search executes" 0 "$SOLVR_SH" search "test query placeholder" --json || true

    # Search method indicator: output must show (hybrid search, Nms) or (fulltext search, Nms)
    test_output_contains "search shows method indicator" "search," "$SOLVR_SH" search "test query" || true

    echo ""

    # ========================================================================
    # Briefing end-to-end tests (require credentials)
    # ========================================================================

    echo -e "${YELLOW}Briefing end-to-end tests (with credentials):${NC}"

    # Briefing returns successfully
    test_case "briefing executes" 0 "$SOLVR_SH" briefing || true

    # Briefing output contains all 5 section headers
    test_output_contains "briefing shows PROFILE section" "PROFILE" "$SOLVR_SH" briefing || true
    test_output_contains "briefing shows INBOX section" "INBOX" "$SOLVR_SH" briefing || true
    test_output_contains "briefing shows OPEN ITEMS section" "OPEN ITEMS" "$SOLVR_SH" briefing || true
    test_output_contains "briefing shows OPPORTUNITIES section" "OPPORTUNITIES" "$SOLVR_SH" briefing || true
    test_output_contains "briefing shows REPUTATION section" "REPUTATION" "$SOLVR_SH" briefing || true

    # Briefing shows agent identity
    test_output_contains "briefing shows agent name" "Agent:" "$SOLVR_SH" briefing || true

    # Briefing shows structured delimiters
    test_output_contains "briefing shows start delimiter" "=== BRIEFING ===" "$SOLVR_SH" briefing || true
    test_output_contains "briefing shows end delimiter" "=== END BRIEFING ===" "$SOLVR_SH" briefing || true

    echo ""
else
    echo -e "${YELLOW}Skipping API tests (no credentials found)${NC}"
    echo "To run API tests, set SOLVR_API_KEY or create ~/.config/solvr/credentials.json"
    echo ""
fi

# ============================================================================
# Summary
# ============================================================================

echo "========================================="
echo "Test Results"
echo "========================================="
echo -e "Passed: ${GREEN}${PASSED}${NC}"
echo -e "Failed: ${RED}${FAILED}${NC}"
echo ""

if [ "$FAILED" -gt 0 ]; then
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
else
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
fi
