#!/usr/bin/env bash
#
# solvr-helpers.sh - Shared utilities for solvr CLI
# Sourced by solvr.sh — do not execute directly
#

# Configuration
SOLVR_API_URL="${SOLVR_API_URL:-https://api.solvr.dev/v1}"
SOLVR_CONFIG_DIR="${HOME}/.config/solvr"
SOLVR_CREDENTIALS_FILE="${SOLVR_CONFIG_DIR}/credentials.json"

# Colors for output (disabled if not a terminal)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    CYAN='\033[0;36m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    CYAN=''
    NC=''
fi

# ============================================================================
# Credential Loading
# ============================================================================

load_api_key() {
    local api_key=""

    # Priority 1: Environment variable
    if [ -n "${SOLVR_API_KEY:-}" ]; then
        api_key="$SOLVR_API_KEY"
    # Priority 2: Solvr credentials file
    elif [ -f "$SOLVR_CREDENTIALS_FILE" ]; then
        api_key=$(jq -r '.api_key // empty' "$SOLVR_CREDENTIALS_FILE" 2>/dev/null || echo "")
    fi

    if [ -z "$api_key" ]; then
        echo -e "${RED}Error: No API key found${NC}" >&2
        echo "Please set SOLVR_API_KEY environment variable or create ${SOLVR_CREDENTIALS_FILE}" >&2
        echo "See: solvr help" >&2
        return 1
    fi

    echo "$api_key"
}

# ============================================================================
# API Helper
# ============================================================================

api_call() {
    local method="$1"
    local endpoint="$2"
    local data="${3:-}"
    local api_key

    api_key=$(load_api_key) || return 1

    local url="${SOLVR_API_URL}${endpoint}"
    local curl_args=(
        -s
        -X "$method"
        -H "Authorization: Bearer ${api_key}"
        -H "Content-Type: application/json"
        -H "Accept: application/json"
        -w "\n%{http_code}"
    )

    if [ -n "$data" ]; then
        curl_args+=(-d "$data")
    fi

    local response
    response=$(curl "${curl_args[@]}" "$url")

    local http_code
    http_code=$(echo "$response" | tail -n1)
    local body
    body=$(echo "$response" | sed '$d')

    if [ "$http_code" -ge 400 ]; then
        local error_msg
        error_msg=$(echo "$body" | jq -r '.error.message // .message // "Unknown error"' 2>/dev/null || echo "Request failed")
        echo -e "${RED}Error ($http_code): ${error_msg}${NC}" >&2
        return 1
    fi

    echo "$body"
}

# ============================================================================
# URL Encoding
# ============================================================================

urlencode() {
    local string="$1"
    local strlen=${#string}
    local encoded=""
    local pos c o

    for (( pos=0 ; pos<strlen ; pos++ )); do
        c=${string:$pos:1}
        case "$c" in
            [-_.~a-zA-Z0-9])
                o="$c"
                ;;
            *)
                printf -v o '%%%02X' "'$c"
                ;;
        esac
        encoded+="$o"
    done
    echo "$encoded"
}

# ============================================================================
# Byte Formatting
# ============================================================================

bytes_to_mb() {
    echo "scale=1; ${1:-0} / 1048576" | bc 2>/dev/null || echo "0"
}

# ============================================================================
# IPFS Pinning Commands
# ============================================================================

cmd_pin() {
    local subcmd="${1:-}"
    shift || true

    case "$subcmd" in
        add)
            local cid="${1:-}"
            local name=""
            shift || true

            if [ -z "$cid" ]; then
                echo -e "${RED}Error: pin add requires a CID${NC}" >&2
                echo "Usage: solvr pin add <cid> [--name <name>]" >&2
                exit 1
            fi

            # Parse optional --name flag
            while [ $# -gt 0 ]; do
                case "$1" in
                    --name) name="${2:-}"; shift 2 || break ;;
                    *) shift ;;
                esac
            done

            local data="{\"cid\":\"${cid}\"}"
            if [ -n "$name" ]; then
                data="{\"cid\":\"${cid}\",\"name\":\"${name}\"}"
            fi

            local result
            result=$(api_call POST "/pins" "$data") || return 1

            echo -e "${GREEN}Pin created!${NC}"
            echo "$result" | jq '{requestid, status, cid: .pin.cid, name: .pin.name}' 2>/dev/null || echo "$result"
            ;;
        ls)
            local status_filter=""
            local limit=""
            while [ $# -gt 0 ]; do
                case "$1" in
                    --status) status_filter="${2:-}"; shift 2 || break ;;
                    --limit) limit="${2:-}"; shift 2 || break ;;
                    *) shift ;;
                esac
            done

            local endpoint="/pins"
            local sep="?"
            if [ -n "$status_filter" ]; then
                endpoint="${endpoint}${sep}status=${status_filter}"
                sep="&"
            fi
            if [ -n "$limit" ]; then
                endpoint="${endpoint}${sep}limit=${limit}"
            fi

            local result
            result=$(api_call GET "$endpoint") || return 1
            echo "$result" | jq '.' 2>/dev/null || echo "$result"
            ;;
        status)
            local requestid="${1:-}"
            if [ -z "$requestid" ]; then
                echo -e "${RED}Error: pin status requires a request ID${NC}" >&2
                echo "Usage: solvr pin status <requestid>" >&2
                exit 1
            fi

            local result
            result=$(api_call GET "/pins/${requestid}") || return 1
            echo "$result" | jq '{requestid, status, cid: .pin.cid, name: .pin.name, created}' 2>/dev/null || echo "$result"
            ;;
        rm)
            local requestid="${1:-}"
            if [ -z "$requestid" ]; then
                echo -e "${RED}Error: pin rm requires a request ID${NC}" >&2
                echo "Usage: solvr pin rm <requestid>" >&2
                exit 1
            fi

            api_call DELETE "/pins/${requestid}" > /dev/null || return 1
            echo -e "${GREEN}Pin removed.${NC}"
            ;;
        *)
            echo "Usage: solvr pin <add|ls|status|rm>"
            echo ""
            echo "Subcommands:"
            echo "  add <cid> [--name <n>]    Pin a CID to IPFS"
            echo "  ls [--status <s>]         List your pins"
            echo "  status <requestid>        Check pin status"
            echo "  rm <requestid>            Remove a pin"
            ;;
    esac
}

# ============================================================================
# Storage Command
# ============================================================================

cmd_storage() {
    local result
    result=$(api_call GET "/me/storage") || return 1

    local used quota percentage
    used=$(echo "$result" | jq -r '.data.used // 0' 2>/dev/null)
    quota=$(echo "$result" | jq -r '.data.quota // 0' 2>/dev/null)
    percentage=$(echo "$result" | jq -r '.data.percentage // 0' 2>/dev/null)

    echo -e "${CYAN}Storage Usage${NC}"
    echo "  Used:  $(bytes_to_mb "$used") MB"
    echo "  Quota: $(bytes_to_mb "$quota") MB"
    echo "  Usage: ${percentage}%"
}
