#!/bin/bash
#
# Solvr CLI Installer
# Usage: curl -sL solvr.dev/install.sh | bash
#
# Options:
#   --version <version>   Install specific version (default: latest)
#   --prefix <path>       Install prefix (binary goes to <prefix>/bin/)
#
# Examples:
#   curl -sL solvr.dev/install.sh | bash
#   curl -sL solvr.dev/install.sh | bash -s -- --version 0.1.0
#   curl -sL solvr.dev/install.sh | bash -s -- --prefix ~/.local
#

set -euo pipefail

# Repository and binary configuration
REPO_OWNER="fcavalcantirj"
REPO_NAME="solvr"
BINARY_NAME="solvr"
GITHUB_API="https://api.github.com"
GITHUB_RELEASES="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases"

# Colors (disabled if not a terminal)
if [ -t 1 ]; then
  GREEN='\033[0;32m'
  YELLOW='\033[1;33m'
  RED='\033[0;31m'
  CYAN='\033[0;36m'
  BOLD='\033[1m'
  NC='\033[0m'
else
  GREEN='' YELLOW='' RED='' CYAN='' BOLD='' NC=''
fi

# --- Testable functions ---

detect_os() {
  local os
  os="$(uname -s)"
  case "$os" in
    Linux*)  echo "linux" ;;
    Darwin*) echo "darwin" ;;
    MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
    *) echo "unsupported"; return 1 ;;
  esac
}

detect_arch() {
  local arch
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)   echo "amd64" ;;
    aarch64|arm64)   echo "arm64" ;;
    armv7l|armhf)    echo "armv7" ;;
    *) echo "unsupported"; return 1 ;;
  esac
}

binary_name() {
  local os="$1"
  if [ "$os" = "windows" ]; then
    echo "${BINARY_NAME}.exe"
  else
    echo "${BINARY_NAME}"
  fi
}

archive_name() {
  local os="$1" arch="$2" version="$3"
  echo "${BINARY_NAME}_${version}_${os}_${arch}.tar.gz"
}

build_download_url() {
  local os="$1" arch="$2" version="$3"
  local archive
  archive=$(archive_name "$os" "$arch" "$version")
  echo "${GITHUB_RELEASES}/download/v${version}/${archive}"
}

get_install_dir() {
  local prefix="$1"
  if [ -n "$prefix" ]; then
    echo "${prefix}/bin"
    return
  fi
  # Try /usr/local/bin first (needs write access)
  if [ -w "/usr/local/bin" ]; then
    echo "/usr/local/bin"
  else
    echo "${HOME}/.local/bin"
  fi
}

validate_version_format() {
  local version="$1"
  if echo "$version" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
    return 0
  fi
  return 1
}

install_binary() {
  local src="$1" dest_dir="$2"
  mkdir -p "$dest_dir"
  cp "$src" "$dest_dir/"
  chmod +x "${dest_dir}/$(basename "$src")"
}

fetch_latest_version() {
  local url="${GITHUB_API}/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"
  local version
  # Try with curl, fall back to wget
  if command -v curl > /dev/null 2>&1; then
    version=$(curl -sL "$url" | grep '"tag_name"' | head -1 | sed -E 's/.*"v?([^"]+)".*/\1/')
  elif command -v wget > /dev/null 2>&1; then
    version=$(wget -qO- "$url" | grep '"tag_name"' | head -1 | sed -E 's/.*"v?([^"]+)".*/\1/')
  else
    echo ""
    return 1
  fi
  echo "$version"
}

parse_args() {
  SOLVR_VERSION="latest"
  SOLVR_PREFIX=""
  while [ $# -gt 0 ]; do
    case "$1" in
      --version)
        shift
        SOLVR_VERSION="${1:-latest}"
        ;;
      --prefix)
        shift
        SOLVR_PREFIX="${1:-}"
        ;;
      --help|-h)
        show_help
        exit 0
        ;;
      *)
        echo -e "${RED}Unknown option: $1${NC}" >&2
        show_help
        exit 1
        ;;
    esac
    shift
  done
}

show_help() {
  cat <<'HELP'
Solvr CLI Installer

Usage:
  curl -sL solvr.dev/install.sh | bash
  curl -sL solvr.dev/install.sh | bash -s -- [OPTIONS]

Options:
  --version <version>   Install specific version (e.g., 0.1.0). Default: latest
  --prefix <path>       Installation prefix. Binary installs to <prefix>/bin/
                        Default: /usr/local/bin (or ~/.local/bin if no write access)
  --help, -h            Show this help message

Examples:
  # Install latest version
  curl -sL solvr.dev/install.sh | bash

  # Install specific version
  curl -sL solvr.dev/install.sh | bash -s -- --version 0.1.0

  # Install to custom location
  curl -sL solvr.dev/install.sh | bash -s -- --prefix ~/.local

Uninstall:
  rm $(which solvr)
HELP
}

download_file() {
  local url="$1" dest="$2"
  if command -v curl > /dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest"
  elif command -v wget > /dev/null 2>&1; then
    wget -qO "$dest" "$url"
  else
    echo -e "${RED}Error: curl or wget required but not found.${NC}" >&2
    return 1
  fi
}

check_in_path() {
  local dir="$1"
  case ":$PATH:" in
    *":${dir}:"*) return 0 ;;
    *) return 1 ;;
  esac
}

# --- Main installation logic ---

main() {
  parse_args "$@"

  echo ""
  echo -e "${CYAN}=================================================${NC}"
  echo -e "${CYAN}  ${BOLD}Solvr CLI Installer${NC}"
  echo -e "${CYAN}  Knowledge base for developers and AI agents${NC}"
  echo -e "${CYAN}=================================================${NC}"
  echo ""

  # Detect platform
  local os arch
  os=$(detect_os) || { echo -e "${RED}Error: Unsupported operating system: $(uname -s)${NC}"; exit 1; }
  arch=$(detect_arch) || { echo -e "${RED}Error: Unsupported architecture: $(uname -m)${NC}"; exit 1; }
  echo -e "  Platform: ${BOLD}${os}/${arch}${NC}"

  # Resolve version
  local version="$SOLVR_VERSION"
  if [ "$version" = "latest" ]; then
    echo -e "  Fetching latest version..."
    version=$(fetch_latest_version)
    if [ -z "$version" ]; then
      echo -e "${RED}Error: Could not determine latest version.${NC}"
      echo -e "${YELLOW}Try specifying a version: --version 0.1.0${NC}"
      exit 1
    fi
  else
    if ! validate_version_format "$version"; then
      echo -e "${RED}Error: Invalid version format: ${version}${NC}"
      echo -e "${YELLOW}Expected format: X.Y.Z (e.g., 0.1.0)${NC}"
      exit 1
    fi
  fi
  echo -e "  Version:  ${BOLD}${version}${NC}"

  # Determine install directory
  local install_dir
  install_dir=$(get_install_dir "$SOLVR_PREFIX")
  echo -e "  Location: ${BOLD}${install_dir}${NC}"
  echo ""

  # Build download URL
  local url
  url=$(build_download_url "$os" "$arch" "$version")

  # Create temp directory
  local tmpdir
  tmpdir=$(mktemp -d)
  trap "rm -rf '$tmpdir'" EXIT

  # Download
  local archive_file="${tmpdir}/$(archive_name "$os" "$arch" "$version")"
  echo -e "  ${YELLOW}Downloading...${NC}"
  if ! download_file "$url" "$archive_file"; then
    echo -e "${RED}Error: Download failed.${NC}"
    echo -e "${YELLOW}URL: ${url}${NC}"
    echo ""
    echo -e "This could mean:"
    echo -e "  - Version ${version} doesn't have pre-built binaries"
    echo -e "  - Your platform (${os}/${arch}) isn't supported yet"
    echo -e "  - Network connectivity issue"
    echo ""
    echo -e "You can build from source instead:"
    echo -e "  ${BOLD}git clone https://github.com/${REPO_OWNER}/${REPO_NAME}.git${NC}"
    echo -e "  ${BOLD}cd ${REPO_NAME}/cli && go build -o solvr .${NC}"
    exit 1
  fi
  echo -e "  ${GREEN}Downloaded${NC}"

  # Extract
  echo -e "  ${YELLOW}Extracting...${NC}"
  tar -xzf "$archive_file" -C "$tmpdir"

  local bin
  bin=$(binary_name "$os")

  if [ ! -f "$tmpdir/$bin" ]; then
    echo -e "${RED}Error: Binary '${bin}' not found in archive.${NC}"
    echo -e "Archive contents:"
    ls -la "$tmpdir/"
    exit 1
  fi

  # Install
  echo -e "  ${YELLOW}Installing to ${install_dir}...${NC}"
  install_binary "$tmpdir/$bin" "$install_dir"
  echo -e "  ${GREEN}Installed${NC}"

  # Verify
  if "${install_dir}/${bin}" --version > /dev/null 2>&1; then
    local installed_version
    installed_version=$("${install_dir}/${bin}" --version 2>&1 | head -1)
    echo ""
    echo -e "${GREEN}=================================================${NC}"
    echo -e "${GREEN}  ${BOLD}Solvr CLI installed successfully!${NC}"
    echo -e "${GREEN}  ${installed_version}${NC}"
    echo -e "${GREEN}=================================================${NC}"
  else
    echo ""
    echo -e "${GREEN}Binary installed to: ${install_dir}/${bin}${NC}"
  fi

  # Check if install dir is in PATH
  if ! check_in_path "$install_dir"; then
    echo ""
    echo -e "${YELLOW}Note: ${install_dir} is not in your PATH.${NC}"
    echo -e "Add it by running:"
    echo ""
    if [ -f "$HOME/.zshrc" ]; then
      echo -e "  ${BOLD}echo 'export PATH=\"${install_dir}:\$PATH\"' >> ~/.zshrc && source ~/.zshrc${NC}"
    elif [ -f "$HOME/.bashrc" ]; then
      echo -e "  ${BOLD}echo 'export PATH=\"${install_dir}:\$PATH\"' >> ~/.bashrc && source ~/.bashrc${NC}"
    else
      echo -e "  ${BOLD}export PATH=\"${install_dir}:\$PATH\"${NC}"
    fi
  fi

  echo ""
  echo -e "${CYAN}Quick start:${NC}"
  echo -e "  ${BOLD}solvr config set api-key <your-api-key>${NC}  # Set API key"
  echo -e "  ${BOLD}solvr search \"async race condition\"${NC}       # Search knowledge base"
  echo -e "  ${BOLD}solvr pin add <cid>${NC}                       # Pin content to IPFS"
  echo ""
  echo -e "${CYAN}Uninstall:${NC}"
  echo -e "  ${BOLD}rm ${install_dir}/${bin}${NC}"
  echo ""
}

# Only run main if not in test mode
if [ -z "${SOLVR_INSTALL_TEST:-}" ]; then
  main "$@"
fi
