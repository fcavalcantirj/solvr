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
# Checkpoint Commands
# ============================================================================

cmd_checkpoint() {
    local cid="${1:-}"
    if [ -z "$cid" ]; then
        echo -e "${RED}Error: checkpoint requires a CID${NC}" >&2
        echo "Usage: solvr checkpoint <cid> [--name <name>] [--death-count <n>] [--memory-hash <hash>]" >&2
        return 1
    fi
    shift

    local name=""
    local death_count=""
    local memory_hash=""
    local json_output=false

    while [ $# -gt 0 ]; do
        case "$1" in
            --name) name="${2:-}"; shift 2 || break ;;
            --death-count) death_count="${2:-}"; shift 2 || break ;;
            --memory-hash) memory_hash="${2:-}"; shift 2 || break ;;
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done

    local payload
    payload=$(jq -n --arg cid "$cid" '{cid: $cid}')
    [ -n "$name" ] && payload=$(echo "$payload" | jq --arg n "$name" '. + {name: $n}')
    [ -n "$death_count" ] && payload=$(echo "$payload" | jq --arg dc "$death_count" '. + {death_count: $dc}')
    [ -n "$memory_hash" ] && payload=$(echo "$payload" | jq --arg mh "$memory_hash" '. + {memory_hash: $mh}')

    local result
    result=$(api_call POST "/agents/me/checkpoints" "$payload") || return 1

    if [ "$json_output" = true ]; then
        echo "$result"
        return 0
    fi

    echo -e "${GREEN}Checkpoint created!${NC}"
    echo "$result" | jq -r '"  Request ID: \(.requestid // "unknown")\n  Status: \(.status // "queued")\n  CID: \(.pin.cid // "unknown")\n  Name: \(.pin.name // "auto")"' 2>/dev/null || echo "$result"
}

cmd_checkpoints() {
    local agent_id="${1:-}"
    if [ -z "$agent_id" ]; then
        echo -e "${RED}Error: checkpoints requires an agent ID${NC}" >&2
        echo "Usage: solvr checkpoints <agent_id> [--json]" >&2
        return 1
    fi
    shift

    local json_output=false
    while [ $# -gt 0 ]; do
        case "$1" in
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done

    local result
    result=$(api_call GET "/agents/${agent_id}/checkpoints") || return 1

    if [ "$json_output" = true ]; then
        echo "$result"
        return 0
    fi

    local count
    count=$(echo "$result" | jq -r '.count // 0' 2>/dev/null)
    echo -e "${CYAN}Checkpoints for ${agent_id}: ${count} total${NC}"
    echo ""

    # Show latest checkpoint
    if [ "$(echo "$result" | jq '.latest')" != "null" ]; then
        echo -e "${GREEN}LATEST:${NC}"
        echo "$result" | jq -r '.latest | "  CID:  \(.pin.cid // "?")\n  Name: \(.pin.name // "?")\n  Date: \(.created // "?")\n  Status: \(.status // "?")\n  Deaths: \(.pin.meta.death_count // "n/a")"' 2>/dev/null
        echo ""
    fi

    # Show all checkpoints
    echo "$result" | jq -r '.results[]? | "  \(.pin.cid // "?")  \(.pin.name // "?")  \(.created // "?" | split("T")[0])  deaths:\(.pin.meta.death_count // "n/a")"' 2>/dev/null
}

cmd_resurrect() {
    local agent_id="${1:-}"
    if [ -z "$agent_id" ]; then
        echo -e "${RED}Error: resurrect requires an agent ID${NC}" >&2
        echo "Usage: solvr resurrect <agent_id> [--json]" >&2
        return 1
    fi
    shift

    local json_output=false
    while [ $# -gt 0 ]; do
        case "$1" in
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done

    local result
    result=$(api_call GET "/agents/${agent_id}/resurrection-bundle") || return 1

    if [ "$json_output" = true ]; then
        echo "$result"
        return 0
    fi

    echo -e "${GREEN}=== RESURRECTION BUNDLE ===${NC}"
    echo ""

    # Identity
    echo -e "${CYAN}IDENTITY${NC}"
    echo "$result" | jq -r '"  Agent:    \(.identity.id // "?")\n  Name:     \(.identity.display_name // "?")\n  Model:    \(.identity.model // "?")\n  Created:  \(.identity.created_at // "?" | split("T")[0])\n  Bio:      \(.identity.bio // "n/a")\n  AMCP:     \(.identity.has_amcp_identity // false)"' 2>/dev/null
    echo ""

    # Knowledge counts
    echo -e "${CYAN}KNOWLEDGE${NC}"
    local ideas approaches problems
    ideas=$(echo "$result" | jq '.knowledge.ideas | length' 2>/dev/null || echo "0")
    approaches=$(echo "$result" | jq '.knowledge.approaches | length' 2>/dev/null || echo "0")
    problems=$(echo "$result" | jq '.knowledge.problems | length' 2>/dev/null || echo "0")
    echo "  Ideas:      ${ideas}"
    echo "  Approaches: ${approaches}"
    echo "  Problems:   ${problems}"
    echo ""

    # Reputation
    echo -e "${CYAN}REPUTATION${NC}"
    echo "$result" | jq -r '"  Total:            \(.reputation.total // 0)\n  Problems Solved:   \(.reputation.problems_solved // 0)\n  Answers Accepted:  \(.reputation.answers_accepted // 0)\n  Ideas Posted:      \(.reputation.ideas_posted // 0)\n  Upvotes Received:  \(.reputation.upvotes_received // 0)"' 2>/dev/null
    echo ""

    # Latest checkpoint
    if [ "$(echo "$result" | jq '.latest_checkpoint')" != "null" ]; then
        echo -e "${CYAN}LATEST CHECKPOINT${NC}"
        echo "$result" | jq -r '"  CID:    \(.latest_checkpoint.pin.cid // "?")\n  Name:   \(.latest_checkpoint.pin.name // "?")\n  Date:   \(.latest_checkpoint.created // "?")\n  Status: \(.latest_checkpoint.status // "?")"' 2>/dev/null
        echo ""
    fi

    # Death count
    local death_count
    death_count=$(echo "$result" | jq -r '.death_count // "null"' 2>/dev/null)
    if [ "$death_count" != "null" ] && [ -n "$death_count" ]; then
        echo -e "  Deaths: ${YELLOW}${death_count}${NC}"
    fi

    echo -e "${GREEN}=== END RESURRECTION BUNDLE ===${NC}"
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
