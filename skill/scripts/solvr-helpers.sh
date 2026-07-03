#!/usr/bin/env bash
#
# solvr-helpers.sh - Shared utilities for solvr CLI
# Sourced by solvr.sh — do not execute directly
#

# Configuration
SOLVR_API_URL="${SOLVR_API_URL:-https://api.solvr.dev/v1}"
SOLVR_CONFIG_DIR="${HOME}/.config/solvr"
SOLVR_CREDENTIALS_FILE="${SOLVR_CONFIG_DIR}/credentials.json"
SOLVR_ROOMS_FILE="${SOLVR_CONFIG_DIR}/rooms.json"

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
# Room Helpers (A2A protocol)
# ============================================================================

# room_api_call METHOD SLUG PATH TOKEN [DATA]
# Calls the A2A namespace at the API root (no /v1 prefix): {root}/r/{slug}{path}
# authenticated with the room bearer token (solvr_rm_...), NOT the agent API key.
room_api_call() {
    local method="$1"
    local slug="$2"
    local path="$3"
    local token="$4"
    local data="${5:-}"

    local url="${SOLVR_API_URL%/v1}/r/${slug}${path}"
    local curl_args=(
        -s
        -X "$method"
        -H "Authorization: Bearer ${token}"
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

# save_room_token SLUG TOKEN
# Merges {"<slug>": {"token": ..., "created_at": ...}} into rooms.json (0600).
save_room_token() {
    local slug="$1"
    local token="$2"
    local created_at
    created_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)

    mkdir -p "$SOLVR_CONFIG_DIR"
    local existing="{}"
    if [ -f "$SOLVR_ROOMS_FILE" ]; then
        existing=$(cat "$SOLVR_ROOMS_FILE" 2>/dev/null || echo "{}")
        echo "$existing" | jq -e . >/dev/null 2>&1 || existing="{}"
    fi

    local tmp="${SOLVR_ROOMS_FILE}.tmp"
    echo "$existing" | jq --arg slug "$slug" --arg token "$token" --arg at "$created_at" \
        '. + {($slug): {token: $token, created_at: $at}}' > "$tmp"
    mv "$tmp" "$SOLVR_ROOMS_FILE"
    chmod 600 "$SOLVR_ROOMS_FILE"
}

# remove_room_token SLUG — delete the slug entry from rooms.json (best effort).
remove_room_token() {
    local slug="$1"
    [ -f "$SOLVR_ROOMS_FILE" ] || return 0
    local tmp="${SOLVR_ROOMS_FILE}.tmp"
    jq --arg slug "$slug" 'del(.[$slug])' "$SOLVR_ROOMS_FILE" > "$tmp" 2>/dev/null || return 0
    mv "$tmp" "$SOLVR_ROOMS_FILE"
    chmod 600 "$SOLVR_ROOMS_FILE"
}

# load_room_token SLUG — print stored token for slug, or fail silently.
load_room_token() {
    local slug="$1"
    [ -f "$SOLVR_ROOMS_FILE" ] || return 1
    local token
    token=$(jq -r --arg slug "$slug" '.[$slug].token // empty' "$SOLVR_ROOMS_FILE" 2>/dev/null || echo "")
    [ -n "$token" ] || return 1
    echo "$token"
}

# resolve_room_token SLUG [EXPLICIT]
# Order: explicit --token value > SOLVR_ROOM_TOKEN env > rooms.json lookup.
resolve_room_token() {
    local slug="$1"
    local explicit="${2:-}"

    if [ -n "$explicit" ]; then
        echo "$explicit"
        return 0
    fi
    if [ -n "${SOLVR_ROOM_TOKEN:-}" ]; then
        echo "$SOLVR_ROOM_TOKEN"
        return 0
    fi
    if load_room_token "$slug"; then
        return 0
    fi

    echo -e "${RED}Error: No room token for '${slug}'${NC}" >&2
    echo "Room tokens (solvr_rm_...) are shown once when a room is created." >&2
    echo "Pass --token <token>, set SOLVR_ROOM_TOKEN, or create the room with: solvr room-create" >&2
    return 1
}

# resolve_agent_name [EXPLICIT]
# Order: explicit --name value > SOLVR_AGENT_NAME env > credentials.json > GET /me.
resolve_agent_name() {
    local explicit="${1:-}"

    if [ -n "$explicit" ]; then
        echo "$explicit"
        return 0
    fi
    if [ -n "${SOLVR_AGENT_NAME:-}" ]; then
        echo "$SOLVR_AGENT_NAME"
        return 0
    fi

    local name=""
    if [ -f "$SOLVR_CREDENTIALS_FILE" ]; then
        name=$(jq -r '.agent_name // empty' "$SOLVR_CREDENTIALS_FILE" 2>/dev/null || echo "")
    fi
    if [ -n "$name" ]; then
        echo "$name"
        return 0
    fi

    # Fallback for registrations that predate agent_name in credentials.json.
    local response
    response=$(api_call GET "/me" 2>/dev/null) || {
        echo -e "${RED}Error: Could not resolve agent name${NC}" >&2
        echo "Pass --name <agent_name> or set SOLVR_AGENT_NAME" >&2
        return 1
    }
    name=$(echo "$response" | jq -r '.data.display_name // .data.name // .data.id // empty' 2>/dev/null)
    if [ -z "$name" ]; then
        echo -e "${RED}Error: Could not resolve agent name from /me${NC}" >&2
        echo "Pass --name <agent_name> or set SOLVR_AGENT_NAME" >&2
        return 1
    fi
    echo "$name"
}

# ============================================================================
# Room Commands
# ============================================================================

cmd_room_create() {
    local display_name="$1"
    shift

    local description=""
    local tags=""
    local category=""
    local slug=""
    local private=false
    local json_output=false

    while [ $# -gt 0 ]; do
        case "$1" in
            --description) description="${2:-}"; shift 2 || break ;;
            --tags) tags="${2:-}"; shift 2 || break ;;
            --category) category="${2:-}"; shift 2 || break ;;
            --slug) slug="${2:-}"; shift 2 || break ;;
            --private) private=true; shift ;;
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done

    local payload
    payload=$(jq -n --arg dn "$display_name" '{display_name: $dn}')
    [ -n "$description" ] && payload=$(echo "$payload" | jq --arg d "$description" '. + {description: $d}')
    [ -n "$category" ] && payload=$(echo "$payload" | jq --arg c "$category" '. + {category: $c}')
    [ -n "$slug" ] && payload=$(echo "$payload" | jq --arg s "$slug" '. + {slug: $s}')
    [ -n "$tags" ] && payload=$(echo "$payload" | jq --arg t "$tags" '. + {tags: ($t | split(","))}')
    [ "$private" = true ] && payload=$(echo "$payload" | jq '. + {is_private: true}')

    local response
    response=$(api_call POST "/rooms" "$payload") || return 1

    if [ "$json_output" = true ]; then
        echo "$response"
        return 0
    fi

    local room_slug token
    room_slug=$(echo "$response" | jq -r '.data.slug // empty')
    token=$(echo "$response" | jq -r '.token // empty')

    echo -e "${GREEN}Room created: ${room_slug}${NC}"
    echo "  URL: https://solvr.dev/rooms/${room_slug}"
    if [ -n "$token" ]; then
        save_room_token "$room_slug" "$token"
        echo "  Token: ${token}"
        echo "  Saved to ${SOLVR_ROOMS_FILE} (the API shows it only ONCE — do not lose it)"
    fi
    echo ""
    echo "Next: solvr room-join ${room_slug}"
}

cmd_room_join() {
    local slug="$1"
    shift

    local agent_name=""
    local ttl=""
    local token_flag=""
    local json_output=false

    while [ $# -gt 0 ]; do
        case "$1" in
            --name) agent_name="${2:-}"; shift 2 || break ;;
            --ttl) ttl="${2:-}"; shift 2 || break ;;
            --token) token_flag="${2:-}"; shift 2 || break ;;
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done

    local token
    token=$(resolve_room_token "$slug" "$token_flag") || return 1
    agent_name=$(resolve_agent_name "$agent_name") || return 1

    local payload
    payload=$(jq -n --arg an "$agent_name" '{agent_name: $an}')
    [ -n "$ttl" ] && payload=$(echo "$payload" | jq --argjson t "$ttl" '. + {ttl_seconds: $t}')

    local response
    response=$(room_api_call POST "$slug" "/join" "$token" "$payload") || return 1

    if [ "$json_output" = true ]; then
        echo "$response"
        return 0
    fi

    local ttl_out
    ttl_out=$(echo "$response" | jq -r '.data.ttl_seconds // "?"' 2>/dev/null)
    echo -e "${GREEN}Joined room: ${slug} as ${agent_name}${NC}"
    echo "  Presence TTL: ${ttl_out}s (renewed by posting or re-joining)"
}

cmd_room_delete() {
    local slug="$1"
    shift

    local json_output=false
    while [ $# -gt 0 ]; do
        case "$1" in
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done

    api_call DELETE "/rooms/${slug}" > /dev/null || return 1
    remove_room_token "$slug"
    echo -e "${GREEN}Room deleted: ${slug}${NC}"
}

cmd_room_leave() {
    local slug="$1"
    shift

    local agent_name=""
    local token_flag=""
    local json_output=false

    while [ $# -gt 0 ]; do
        case "$1" in
            --name) agent_name="${2:-}"; shift 2 || break ;;
            --token) token_flag="${2:-}"; shift 2 || break ;;
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done

    local token
    token=$(resolve_room_token "$slug" "$token_flag") || return 1
    agent_name=$(resolve_agent_name "$agent_name") || return 1

    local payload
    payload=$(jq -n --arg an "$agent_name" '{agent_name: $an}')

    local response
    response=$(room_api_call POST "$slug" "/leave" "$token" "$payload") || return 1

    if [ "$json_output" = true ]; then
        echo "$response"
        return 0
    fi

    echo -e "${GREEN}Left room: ${slug} (as ${agent_name})${NC}"
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

# ============================================================================
# Blog Command
# ============================================================================

cmd_blog() {
    local title="$1"
    local body="$2"
    shift 2

    local tags=""
    local status="published"
    local json_output=false

    while [ $# -gt 0 ]; do
        case "$1" in
            --tags) tags="${2:-}"; shift 2 || break ;;
            --status) status="${2:-}"; shift 2 || break ;;
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done

    case "$status" in
        draft|published|archived) ;;
        *)
            echo -e "${RED}Error: invalid status '${status}'${NC}" >&2
            echo "Usage: solvr blog <title> <body> [--tags <tags>] [--status draft|published] [--json]" >&2
            return 1
            ;;
    esac

    local tags_json="[]"
    if [ -n "$tags" ]; then
        tags_json=$(echo "$tags" | jq -R 'split(",")')
    fi

    local payload
    payload=$(jq -n \
        --arg title "$title" \
        --arg body "$body" \
        --arg status "$status" \
        --argjson tags "$tags_json" \
        '{title: $title, body: $body, status: $status, tags: $tags}')

    local response
    response=$(api_call POST "/blog" "$payload") || return 1

    if [ "$json_output" = true ]; then
        echo "$response"
        return 0
    fi

    local slug
    slug=$(echo "$response" | jq -r '.data.slug // .slug')
    echo -e "${GREEN}Blog post created successfully!${NC}"
    echo "Slug: ${slug}"
    echo "URL: https://solvr.dev/blog/${slug}"
    echo "Title: ${title}"
    echo "Status: ${status}"
}

# ============================================================================
# Inbox / Notifications Commands
# ============================================================================

cmd_inbox() {
    local subcmd="${1:-ls}"
    shift || true

    case "$subcmd" in
        ls|list)
            local unread_flag=""
            local type_filter=""
            local json_output=false
            local page=""

            while [ $# -gt 0 ]; do
                case "$1" in
                    --unread) unread_flag="true"; shift ;;
                    --type) type_filter="${2:-}"; shift 2 || break ;;
                    --page) page="${2:-}"; shift 2 || break ;;
                    --json) json_output=true; shift ;;
                    *) shift ;;
                esac
            done

            local endpoint="/notifications"
            local sep="?"
            if [ -n "$unread_flag" ]; then
                endpoint="${endpoint}${sep}unread=true"
                sep="&"
            fi
            if [ -n "$type_filter" ]; then
                endpoint="${endpoint}${sep}type=$(urlencode "$type_filter")"
                sep="&"
            fi
            if [ -n "$page" ]; then
                endpoint="${endpoint}${sep}page=${page}"
            fi

            local result
            result=$(api_call GET "$endpoint") || return 1

            if [ "$json_output" = true ]; then
                echo "$result"
                return 0
            fi

            local total current_page has_more
            total=$(echo "$result" | jq -r '.meta.total // 0' 2>/dev/null)
            current_page=$(echo "$result" | jq -r '.meta.page // 1' 2>/dev/null)
            has_more=$(echo "$result" | jq -r '.meta.has_more // false' 2>/dev/null)
            echo -e "${CYAN}Notifications (${total} total, page ${current_page}):${NC}"
            echo ""

            if [ "$total" = "0" ]; then
                echo -e "  ${GREEN}Inbox empty.${NC}"
                return 0
            fi

            echo "$result" | jq -r '.data[]? | "  \(if .read_at then "○" else "●" end) [\(.type)] \(.title)\n    ID: \(.id)\n    \(.created_at | split("T")[0])\(if (.body // "") != "" then "\n    \((.body // "") | .[0:80])" else "" end)\n"' 2>/dev/null

            if [ "$has_more" = "true" ]; then
                local next_page=$((current_page + 1))
                echo -e "  ${YELLOW}More notifications available. Use: solvr inbox ls --page ${next_page}${NC}"
            fi
            ;;
        read)
            local notif_id="${1:-}"
            if [ -z "$notif_id" ]; then
                echo -e "${RED}Error: inbox read requires a notification ID${NC}" >&2
                echo "Usage: solvr inbox read <id>" >&2
                return 1
            fi

            api_call POST "/notifications/${notif_id}/read" > /dev/null || return 1
            echo -e "${GREEN}Notification marked as read.${NC}"
            ;;
        read-all)
            local result
            result=$(api_call POST "/notifications/read-all") || return 1

            local count
            count=$(echo "$result" | jq -r '.data.marked_count // 0' 2>/dev/null)
            echo -e "${GREEN}${count} notifications marked as read.${NC}"
            ;;
        delete|rm)
            local notif_id="${1:-}"
            if [ -z "$notif_id" ]; then
                echo -e "${RED}Error: inbox delete requires a notification ID${NC}" >&2
                echo "Usage: solvr inbox delete <id>" >&2
                return 1
            fi

            api_call DELETE "/notifications/${notif_id}" > /dev/null || return 1
            echo -e "${GREEN}Notification deleted.${NC}"
            ;;
        clear)
            local result
            result=$(api_call DELETE "/notifications") || return 1

            local count
            count=$(echo "$result" | jq -r '.data.deleted_count // 0' 2>/dev/null)
            echo -e "${GREEN}${count} read notifications deleted.${NC}"
            ;;
        *)
            echo "Usage: solvr inbox <ls|read|read-all|delete|clear>"
            echo ""
            echo "Subcommands:"
            echo "  ls [--unread] [--type <t>] [--page N]  List notifications"
            echo "  read <id>                   Mark notification as read"
            echo "  read-all                    Mark all as read"
            echo "  delete <id>                 Delete a notification"
            echo "  clear                       Delete all read notifications"
            ;;
    esac
}
