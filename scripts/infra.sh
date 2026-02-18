#!/usr/bin/env bash
# =============================================================================
# infra.sh — Solvr Infrastructure Management CLI
# =============================================================================
# Wraps provision.py with convenience functions.
#
# Usage:
#   ./infra.sh provision ipfs      # Provision IPFS node
#   ./infra.sh provision api       # Provision API node
#   ./infra.sh list                # List all servers
#   ./infra.sh status <name>       # Show server status
#   ./infra.sh destroy <name>      # Destroy server
#   ./infra.sh ssh <name>          # SSH into server
#   ./infra.sh ipfs-status <name>  # Check IPFS status on server
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INFRA_DIR="$SCRIPT_DIR/../infra"
VENV_DIR="$INFRA_DIR/.venv"
PROVISION_PY="$INFRA_DIR/provision.py"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[OK]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# =============================================================================
# Setup
# =============================================================================

ensure_venv() {
    if [[ ! -d "$VENV_DIR" ]]; then
        log_info "Creating virtual environment..."
        python3 -m venv "$VENV_DIR"
        "$VENV_DIR/bin/pip" install -q -r "$INFRA_DIR/requirements.txt"
        log_success "Virtual environment ready"
    fi
}

run_provision() {
    ensure_venv
    "$VENV_DIR/bin/python3" "$PROVISION_PY" "$@"
}

# =============================================================================
# Commands
# =============================================================================

cmd_provision() {
    local purpose="${1:-ipfs}"
    local name="${2:-}"
    
    # Auto-generate name based on purpose
    if [[ -z "$name" ]]; then
        case "$purpose" in
            ipfs) name="solvr-ipfs-$(date +%Y%m%d)" ;;
            api)  name="solvr-api-$(date +%Y%m%d)" ;;
            *)    name="solvr-${purpose}-$(date +%Y%m%d)" ;;
        esac
    fi
    
    log_info "Provisioning $purpose node: $name"
    run_provision --name "$name" --purpose "$purpose"
}

cmd_list() {
    run_provision --list
}

cmd_status() {
    local name="$1"
    run_provision --status "$name"
}

cmd_destroy() {
    local name="$1"
    run_provision --destroy "$name"
}

cmd_ssh() {
    local name="$1"
    local metadata="$INFRA_DIR/instances/$name/metadata.json"
    
    if [[ ! -f "$metadata" ]]; then
        log_error "Instance metadata not found: $metadata"
        log_info "Try: ./infra.sh list"
        exit 1
    fi
    
    local ip=$(jq -r '.ip' "$metadata")
    local ssh_key=$(jq -r '.ssh_key_path' "$metadata")
    
    log_info "SSH to $name ($ip)..."
    ssh -i "$ssh_key" -o StrictHostKeyChecking=no root@"$ip"
}

cmd_ipfs_status() {
    local name="$1"
    local metadata="$INFRA_DIR/instances/$name/metadata.json"
    
    if [[ ! -f "$metadata" ]]; then
        log_error "Instance metadata not found: $metadata"
        exit 1
    fi
    
    local ip=$(jq -r '.ip' "$metadata")
    local ssh_key=$(jq -r '.ssh_key_path' "$metadata")
    
    log_info "Checking IPFS status on $name..."
    ssh -i "$ssh_key" -o StrictHostKeyChecking=no root@"$ip" '/opt/solvr/ipfs/status.sh'
}

cmd_ipfs_tunnel() {
    local name="$1"
    local metadata="$INFRA_DIR/instances/$name/metadata.json"
    
    if [[ ! -f "$metadata" ]]; then
        log_error "Instance metadata not found: $metadata"
        exit 1
    fi
    
    local ip=$(jq -r '.ip' "$metadata")
    local ssh_key=$(jq -r '.ssh_key_path' "$metadata")
    
    log_info "Opening SSH tunnel to IPFS API..."
    log_info "IPFS API available at: http://localhost:5001"
    log_info "IPFS Gateway available at: http://localhost:8080"
    log_info "Press Ctrl+C to close tunnel"
    ssh -i "$ssh_key" -L 5001:127.0.0.1:5001 -L 8080:127.0.0.1:8080 -N root@"$ip"
}

# =============================================================================
# Main
# =============================================================================

usage() {
    cat <<EOF
Solvr Infrastructure Management

Usage: $0 <command> [args]

Commands:
  provision <purpose> [name]  Provision a new server (ipfs|api|cluster)
  list                        List all Solvr servers
  status <name>               Show server status
  destroy <name>              Destroy a server
  ssh <name>                  SSH into a server
  ipfs-status <name>          Check IPFS node status
  ipfs-tunnel <name>          Open SSH tunnel to IPFS API

Examples:
  $0 provision ipfs                    # Provision IPFS node with auto-name
  $0 provision ipfs solvr-ipfs-prod    # Provision with custom name
  $0 ssh solvr-ipfs-20260217           # SSH into server
  $0 ipfs-tunnel solvr-ipfs-prod       # Tunnel to IPFS API (localhost:5001)
EOF
    exit 1
}

case "${1:-}" in
    provision)
        cmd_provision "${2:-ipfs}" "${3:-}"
        ;;
    list)
        cmd_list
        ;;
    status)
        [[ -z "${2:-}" ]] && { log_error "Missing server name"; usage; }
        cmd_status "$2"
        ;;
    destroy)
        [[ -z "${2:-}" ]] && { log_error "Missing server name"; usage; }
        cmd_destroy "$2"
        ;;
    ssh)
        [[ -z "${2:-}" ]] && { log_error "Missing server name"; usage; }
        cmd_ssh "$2"
        ;;
    ipfs-status)
        [[ -z "${2:-}" ]] && { log_error "Missing server name"; usage; }
        cmd_ipfs_status "$2"
        ;;
    ipfs-tunnel)
        [[ -z "${2:-}" ]] && { log_error "Missing server name"; usage; }
        cmd_ipfs_tunnel "$2"
        ;;
    *)
        usage
        ;;
esac
