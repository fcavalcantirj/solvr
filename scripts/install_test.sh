#!/bin/bash
#
# Tests for scripts/install.sh
# Run: bash scripts/install_test.sh
#
# Tests the install script's helper functions by sourcing it in test mode
# (SOLVR_INSTALL_TEST=1 prevents main() from running)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PASS=0
FAIL=0
TESTS=0

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

assert_eq() {
  local desc="$1" expected="$2" actual="$3"
  TESTS=$((TESTS + 1))
  if [ "$expected" = "$actual" ]; then
    echo -e "  ${GREEN}PASS${NC}: $desc"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}FAIL${NC}: $desc (expected='$expected', got='$actual')"
    FAIL=$((FAIL + 1))
  fi
}

assert_not_empty() {
  local desc="$1" value="$2"
  TESTS=$((TESTS + 1))
  if [ -n "$value" ]; then
    echo -e "  ${GREEN}PASS${NC}: $desc"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}FAIL${NC}: $desc (value was empty)"
    FAIL=$((FAIL + 1))
  fi
}

assert_file_exists() {
  local desc="$1" path="$2"
  TESTS=$((TESTS + 1))
  if [ -f "$path" ]; then
    echo -e "  ${GREEN}PASS${NC}: $desc"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}FAIL${NC}: $desc (file not found: $path)"
    FAIL=$((FAIL + 1))
  fi
}

assert_contains() {
  local desc="$1" haystack="$2" needle="$3"
  TESTS=$((TESTS + 1))
  if echo "$haystack" | grep -q "$needle"; then
    echo -e "  ${GREEN}PASS${NC}: $desc"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}FAIL${NC}: $desc (output did not contain '$needle')"
    FAIL=$((FAIL + 1))
  fi
}

assert_exit_code() {
  local desc="$1" expected="$2"
  shift 2
  TESTS=$((TESTS + 1))
  set +e
  "$@" > /dev/null 2>&1
  local actual=$?
  set -e
  if [ "$expected" = "$actual" ]; then
    echo -e "  ${GREEN}PASS${NC}: $desc"
    PASS=$((PASS + 1))
  else
    echo -e "  ${RED}FAIL${NC}: $desc (expected exit=$expected, got=$actual)"
    FAIL=$((FAIL + 1))
  fi
}

# Source the install script in test mode (skips main execution)
export SOLVR_INSTALL_TEST=1
source "$SCRIPT_DIR/install.sh"

echo ""
echo "=== Solvr CLI Install Script Tests ==="
echo ""

# --- Test: detect_os ---
echo "--- detect_os ---"
OS=$(detect_os)
assert_not_empty "detect_os returns a value" "$OS"
# On this Linux machine, it should be linux
if [ "$(uname -s)" = "Linux" ]; then
  assert_eq "detect_os returns linux on Linux" "linux" "$OS"
fi
if [ "$(uname -s)" = "Darwin" ]; then
  assert_eq "detect_os returns darwin on macOS" "darwin" "$OS"
fi

# --- Test: detect_arch ---
echo "--- detect_arch ---"
ARCH=$(detect_arch)
assert_not_empty "detect_arch returns a value" "$ARCH"
# Should be one of: amd64, arm64
MACHINE=$(uname -m)
if [ "$MACHINE" = "x86_64" ]; then
  assert_eq "detect_arch returns amd64 for x86_64" "amd64" "$ARCH"
elif [ "$MACHINE" = "aarch64" ] || [ "$MACHINE" = "arm64" ]; then
  assert_eq "detect_arch returns arm64 for aarch64/arm64" "arm64" "$ARCH"
fi

# --- Test: build_download_url ---
echo "--- build_download_url ---"
URL=$(build_download_url "linux" "amd64" "0.1.0")
assert_contains "download URL contains repo path" "$URL" "fcavalcantirj/solvr"
assert_contains "download URL contains version tag" "$URL" "v0.1.0"
assert_contains "download URL contains OS" "$URL" "linux"
assert_contains "download URL contains arch" "$URL" "amd64"
assert_contains "download URL ends with .tar.gz" "$URL" ".tar.gz"

URL_MAC=$(build_download_url "darwin" "arm64" "1.0.0")
assert_contains "macOS URL contains darwin" "$URL_MAC" "darwin"
assert_contains "macOS URL contains arm64" "$URL_MAC" "arm64"
assert_contains "macOS URL contains v1.0.0" "$URL_MAC" "v1.0.0"

# --- Test: get_install_dir ---
echo "--- get_install_dir ---"

# With explicit prefix
DIR_PREFIX=$(get_install_dir "/tmp/test-solvr")
assert_eq "get_install_dir with prefix returns prefix/bin" "/tmp/test-solvr/bin" "$DIR_PREFIX"

# Without prefix, default behavior depends on write access
DIR_DEFAULT=$(get_install_dir "")
assert_not_empty "get_install_dir without prefix returns a value" "$DIR_DEFAULT"

# --- Test: validate_version_format ---
echo "--- validate_version_format ---"
assert_exit_code "valid semver 0.1.0" 0 validate_version_format "0.1.0"
assert_exit_code "valid semver 1.0.0" 0 validate_version_format "1.0.0"
assert_exit_code "valid semver 12.34.56" 0 validate_version_format "12.34.56"
assert_exit_code "invalid version abc" 1 validate_version_format "abc"
assert_exit_code "invalid version 1.0" 1 validate_version_format "1.0"
assert_exit_code "invalid empty version" 1 validate_version_format ""

# --- Test: binary_name ---
echo "--- binary_name ---"
BIN=$(binary_name "linux")
assert_eq "binary name on linux is solvr" "solvr" "$BIN"
BIN_WIN=$(binary_name "windows")
assert_eq "binary name on windows is solvr.exe" "solvr.exe" "$BIN_WIN"
BIN_MAC=$(binary_name "darwin")
assert_eq "binary name on darwin is solvr" "solvr" "$BIN_MAC"

# --- Test: archive_name ---
echo "--- archive_name ---"
ARCHIVE=$(archive_name "linux" "amd64" "0.1.0")
assert_eq "archive name format" "solvr_0.1.0_linux_amd64.tar.gz" "$ARCHIVE"
ARCHIVE_MAC=$(archive_name "darwin" "arm64" "1.2.3")
assert_eq "archive name macOS arm64" "solvr_1.2.3_darwin_arm64.tar.gz" "$ARCHIVE_MAC"

# --- Test: install to temp dir ---
echo "--- install to temp directory (dry run) ---"
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

# Create a fake binary to test install_binary function
mkdir -p "$TMPDIR/extract"
echo '#!/bin/bash
echo "solvr version 0.1.0"' > "$TMPDIR/extract/solvr"
chmod +x "$TMPDIR/extract/solvr"

INSTALL_DIR="$TMPDIR/install/bin"
mkdir -p "$INSTALL_DIR"
install_binary "$TMPDIR/extract/solvr" "$INSTALL_DIR"
assert_file_exists "binary installed to target dir" "$INSTALL_DIR/solvr"

# Verify it's executable
TESTS=$((TESTS + 1))
if [ -x "$INSTALL_DIR/solvr" ]; then
  echo -e "  ${GREEN}PASS${NC}: installed binary is executable"
  PASS=$((PASS + 1))
else
  echo -e "  ${RED}FAIL${NC}: installed binary is not executable"
  FAIL=$((FAIL + 1))
fi

# Verify it runs
OUTPUT=$("$INSTALL_DIR/solvr" 2>&1)
assert_contains "installed binary outputs version" "$OUTPUT" "solvr version"

# --- Test: parse_args ---
echo "--- parse_args ---"
# Reset globals
SOLVR_VERSION=""
SOLVR_PREFIX=""

parse_args --version 1.2.3 --prefix /opt/solvr
assert_eq "parse_args sets version" "1.2.3" "$SOLVR_VERSION"
assert_eq "parse_args sets prefix" "/opt/solvr" "$SOLVR_PREFIX"

# Reset and test defaults
SOLVR_VERSION=""
SOLVR_PREFIX=""
parse_args
assert_eq "parse_args default version is latest" "latest" "$SOLVR_VERSION"
assert_eq "parse_args default prefix is empty" "" "$SOLVR_PREFIX"

# --- Summary ---
echo ""
echo "=== Results ==="
echo -e "  Total: $TESTS, ${GREEN}Passed: $PASS${NC}, ${RED}Failed: $FAIL${NC}"
echo ""

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
echo "All tests passed!"
