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
        ((PASSED++))
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        echo "  Expected exit code: ${expected_exit}"
        echo "  Actual exit code: ${actual_exit}"
        echo "  Output: ${output}"
        ((FAILED++))
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
        ((PASSED++))
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        echo "  Expected output to contain: ${expected_content}"
        echo "  Actual output: ${output}"
        ((FAILED++))
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
# Feature completeness tests (docs must mention key features)
# ============================================================================

echo -e "${YELLOW}Feature completeness tests:${NC}"

# Helper: check if a file contains a pattern
test_file_contains() {
    local name="$1" pattern="$2" file="$3"
    echo -n "Testing: ${name}... "
    if grep -qiE "$pattern" "$file" 2>/dev/null; then
        echo -e "${GREEN}PASS${NC}"
        ((PASSED++))
        return 0
    else
        echo -e "${RED}FAIL${NC}"
        echo "  File $file missing pattern: $pattern"
        ((FAILED++))
        return 1
    fi
}

SKILL_MD="${SCRIPT_DIR}/../SKILL.md"
API_MD="${SCRIPT_DIR}/../references/api.md"
EXAMPLES_MD="${SCRIPT_DIR}/../references/examples.md"
SKILL_JSON="${SCRIPT_DIR}/../skill.json"

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
