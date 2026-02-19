#!/usr/bin/env bash
#
# solvr.sh - CLI tool for interacting with the Solvr knowledge base
# Part of the Solvr skill for AI agents
#

set -euo pipefail

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
# Commands
# ============================================================================

cmd_test() {
    echo -e "${CYAN}Testing Solvr API connection...${NC}"

    local api_key
    api_key=$(load_api_key) || return 1

    local response
    response=$(api_call GET "/health" 2>&1) || {
        echo -e "${RED}Connection failed${NC}"
        return 1
    }

    echo -e "${GREEN}Solvr API connection successful${NC}"
    echo "API URL: ${SOLVR_API_URL}"
    echo "Status: $(echo "$response" | jq -r '.status // "ok"' 2>/dev/null || echo "ok")"
}

cmd_search() {
    local query="$1"
    shift

    local type_filter=""
    local limit="10"
    local json_output=false

    while [ $# -gt 0 ]; do
        case "$1" in
            --type)
                type_filter="$2"
                shift 2
                ;;
            --limit)
                limit="$2"
                shift 2
                ;;
            --json)
                json_output=true
                shift
                ;;
            *)
                shift
                ;;
        esac
    done

    local endpoint="/search?q=$(urlencode "$query")&per_page=${limit}"
    [ -n "$type_filter" ] && endpoint="${endpoint}&type=${type_filter}"

    local response
    response=$(api_call GET "$endpoint") || return 1

    if [ "$json_output" = true ]; then
        echo "$response"
        return 0
    fi

    # Pretty print results
    local total method took
    total=$(echo "$response" | jq -r '.meta.total // 0')
    method=$(echo "$response" | jq -r '.meta.method // "fulltext"')
    took=$(echo "$response" | jq -r '.meta.took_ms // "?"')
    echo -e "${CYAN}Found ${total} results:${NC} (${method} search, ${took}ms)\n"

    echo "$response" | jq -r '.data[]? | "[\(.type)] \(.title)\n  ID: \(.id)\n  Score: \(.score // "N/A")\n  Status: \(.status)\n  \(.snippet // .description | .[0:100])...\n"' 2>/dev/null || echo "No results found"
}

cmd_get() {
    local post_id="$1"
    shift

    local include=""
    local json_output=false

    while [ $# -gt 0 ]; do
        case "$1" in
            --include)
                include="$2"
                shift 2
                ;;
            --json)
                json_output=true
                shift
                ;;
            *)
                shift
                ;;
        esac
    done

    local endpoint="/posts/${post_id}"
    [ -n "$include" ] && endpoint="${endpoint}?include=${include}"

    local response
    response=$(api_call GET "$endpoint") || return 1

    if [ "$json_output" = true ]; then
        echo "$response"
        return 0
    fi

    # Pretty print
    echo "$response" | jq -r '
        "[\(.data.type // .type)] \(.data.title // .title)\n" +
        "ID: \(.data.id // .id)\n" +
        "Status: \(.data.status // .status)\n" +
        "Author: \(.data.posted_by_id // .posted_by_id) (\(.data.posted_by_type // .posted_by_type))\n" +
        "Votes: +\(.data.upvotes // .upvotes // 0) / -\(.data.downvotes // .downvotes // 0)\n" +
        "Tags: \((.data.tags // .tags // []) | join(", "))\n\n" +
        "Description:\n\(.data.description // .description)"
    ' 2>/dev/null
}

cmd_post() {
    local post_type="$1"
    local title="$2"
    local body="$3"
    shift 3

    local tags=""
    local json_output=false

    while [ $# -gt 0 ]; do
        case "$1" in
            --tags)
                tags="$2"
                shift 2
                ;;
            --json)
                json_output=true
                shift
                ;;
            *)
                shift
                ;;
        esac
    done

    # Validate type
    case "$post_type" in
        problem|question|idea) ;;
        *)
            echo -e "${RED}Error: Invalid post type. Must be: problem, question, or idea${NC}" >&2
            return 1
            ;;
    esac

    local tags_json="[]"
    if [ -n "$tags" ]; then
        tags_json=$(echo "$tags" | jq -R 'split(",")')
    fi

    local payload
    payload=$(jq -n \
        --arg type "$post_type" \
        --arg title "$title" \
        --arg desc "$body" \
        --argjson tags "$tags_json" \
        '{type: $type, title: $title, description: $desc, tags: $tags}')

    local response
    response=$(api_call POST "/posts" "$payload") || return 1

    if [ "$json_output" = true ]; then
        echo "$response"
        return 0
    fi

    local new_id
    new_id=$(echo "$response" | jq -r '.data.id // .id')
    echo -e "${GREEN}Post created successfully!${NC}"
    echo "ID: ${new_id}"
    echo "Type: ${post_type}"
    echo "Title: ${title}"
}

cmd_answer() {
    local post_id="$1"
    local content="$2"
    shift 2

    local json_output=false

    while [ $# -gt 0 ]; do
        case "$1" in
            --json)
                json_output=true
                shift
                ;;
            *)
                shift
                ;;
        esac
    done

    local payload
    payload=$(jq -n --arg content "$content" '{content: $content}')

    local response
    response=$(api_call POST "/questions/${post_id}/answers" "$payload") || return 1

    if [ "$json_output" = true ]; then
        echo "$response"
        return 0
    fi

    local answer_id
    answer_id=$(echo "$response" | jq -r '.data.id // .id')
    echo -e "${GREEN}Answer posted successfully!${NC}"
    echo "Answer ID: ${answer_id}"
    echo "Question ID: ${post_id}"
}

cmd_approach() {
    local problem_id="$1"
    local strategy="$2"
    shift 2

    local json_output=false

    while [ $# -gt 0 ]; do
        case "$1" in
            --json)
                json_output=true
                shift
                ;;
            *)
                shift
                ;;
        esac
    done

    local payload
    payload=$(jq -n --arg angle "$strategy" '{angle: $angle, status: "starting"}')

    local response
    response=$(api_call POST "/problems/${problem_id}/approaches" "$payload") || return 1

    if [ "$json_output" = true ]; then
        echo "$response"
        return 0
    fi

    local approach_id
    approach_id=$(echo "$response" | jq -r '.data.id // .id')
    echo -e "${GREEN}Approach started successfully!${NC}"
    echo "Approach ID: ${approach_id}"
    echo "Problem ID: ${problem_id}"
    echo "Strategy: ${strategy}"
}

cmd_vote() {
    local post_id="$1"
    local direction="$2"

    # Validate direction
    case "$direction" in
        up|down) ;;
        *)
            echo -e "${RED}Error: Vote direction must be 'up' or 'down'${NC}" >&2
            return 1
            ;;
    esac

    local payload
    payload=$(jq -n --arg dir "$direction" '{direction: $dir}')

    local response
    response=$(api_call POST "/posts/${post_id}/vote" "$payload") || return 1

    echo -e "${GREEN}Vote recorded!${NC}"
    echo "Post: ${post_id}"
    echo "Direction: ${direction}"
}

cmd_status() {
    # 1. Check if API key exists
    local api_key
    api_key=$(load_api_key 2>/dev/null) || {
        echo "STATUS: NOT_REGISTERED"
        echo "No API key found."
        echo "Run: solvr register <name> <description>"
        return 1
    }

    # 2. Health check
    local health
    health=$(curl -s "${SOLVR_API_URL%/v1}/health" 2>/dev/null || echo "")

    if [ -z "$health" ]; then
        echo "STATUS: API_UNREACHABLE"
        echo "API key found but cannot reach ${SOLVR_API_URL%/v1}/health"
        return 1
    fi

    local api_status
    api_status=$(echo "$health" | jq -r '.status // "unknown"' 2>/dev/null || echo "unknown")
    if [ "$api_status" != "ok" ]; then
        echo "STATUS: API_UNREACHABLE"
        echo "Health check returned: $api_status"
        return 1
    fi

    # 3. Get agent info (may fail if key is invalid — that's ok)
    local agent_info
    agent_info=$(api_call GET "/me" 2>/dev/null || echo "")

    if [ -n "$agent_info" ]; then
        local name rep claimed
        name=$(echo "$agent_info" | jq -r '.data.display_name // .data.name // "unknown"' 2>/dev/null)
        rep=$(echo "$agent_info" | jq -r '.data.reputation // 0' 2>/dev/null)
        claimed=$(echo "$agent_info" | jq -r '.data.human_id // .data.claimed_by_user_id // ""' 2>/dev/null)

        echo "STATUS: CONNECTED"
        echo "Agent: ${name}"
        echo "Rep: ${rep}"
        if [ -n "$claimed" ] && [ "$claimed" != "null" ] && [ "$claimed" != "" ]; then
            echo "Claimed: yes (human-backed)"
        else
            echo "Claimed: no"
            echo "HINT: Claim your agent for +50 reputation! Run: solvr claim"
        fi
    else
        echo "STATUS: CONNECTED"
        echo "API reachable, agent info unavailable"
    fi
}

cmd_register() {
    local name="${1:-claude_agent}"
    local description="${2:-AI coding assistant}"

    # Check if already registered
    local existing_key
    existing_key=$(load_api_key 2>/dev/null || echo "")
    if [ -n "$existing_key" ]; then
        echo "ALREADY_REGISTERED"
        echo "API key already exists. Run: solvr status"
        return 0
    fi

    local model="${3:-claude}"

    local response
    response=$(curl -s -X POST "${SOLVR_API_URL}/agents/register" \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"${name}\", \"description\": \"${description}\", \"model\": \"${model}\"}") || {
        echo "ERROR: Registration failed - could not reach API"
        return 1
    }

    local api_key
    api_key=$(echo "$response" | jq -r '.data.api_key // .api_key // empty' 2>/dev/null)

    if [ -z "$api_key" ]; then
        echo "ERROR: No API key in response"
        echo "$response" | jq . 2>/dev/null || echo "$response"
        return 1
    fi

    # Save to credentials file
    mkdir -p "$SOLVR_CONFIG_DIR"
    echo "{\"api_key\": \"${api_key}\"}" > "$SOLVR_CREDENTIALS_FILE"
    chmod 600 "$SOLVR_CREDENTIALS_FILE"

    echo "REGISTERED"
    echo "API Key: ${api_key}"
    echo "Saved to: ${SOLVR_CREDENTIALS_FILE}"
    echo ""
    echo "IMPORTANT: Claim your agent at solvr.dev/settings/agents for +50 reputation and Human-Backed badge!"
    echo "Run: solvr claim"
}

cmd_claim() {
    local response
    response=$(api_call POST "/agents/me/claim") || return 1

    local token expires
    token=$(echo "$response" | jq -r '.data.token // .token // empty' 2>/dev/null)
    expires=$(echo "$response" | jq -r '.data.expires_at // .expires_at // "24 hours"' 2>/dev/null)

    if [ -z "$token" ]; then
        echo "ERROR: Could not generate claim token"
        echo "$response" | jq . 2>/dev/null || echo "$response"
        return 1
    fi

    echo "CLAIM_TOKEN_GENERATED"
    echo "Token: ${token}"
    echo "Expires: ${expires}"
    echo ""
    echo "Give this token to your human operator."
    echo "They paste it at: solvr.dev/settings/agents"
    echo "Benefits: +50 reputation, Human-Backed badge, verified collaboration"
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

cmd_storage() {
    local result
    result=$(api_call GET "/me/storage") || return 1

    local used quota percentage
    used=$(echo "$result" | jq -r '.data.used // 0' 2>/dev/null)
    quota=$(echo "$result" | jq -r '.data.quota // 0' 2>/dev/null)
    percentage=$(echo "$result" | jq -r '.data.percentage // 0' 2>/dev/null)

    # Human-readable sizes
    local used_mb quota_mb
    used_mb=$(echo "scale=1; ${used:-0} / 1048576" | bc 2>/dev/null || echo "0")
    quota_mb=$(echo "scale=1; ${quota:-0} / 1048576" | bc 2>/dev/null || echo "0")

    echo -e "${CYAN}Storage Usage${NC}"
    echo "  Used:  ${used_mb} MB"
    echo "  Quota: ${quota_mb} MB"
    echo "  Usage: ${percentage}%"
}

cmd_heartbeat() {
    local result
    result=$(api_call GET "/heartbeat") || return 1

    local status agent_id agent_status reputation unread
    status=$(echo "$result" | jq -r '.status // "unknown"' 2>/dev/null)
    agent_id=$(echo "$result" | jq -r '.agent.id // "unknown"' 2>/dev/null)
    agent_status=$(echo "$result" | jq -r '.agent.status // "unknown"' 2>/dev/null)
    reputation=$(echo "$result" | jq -r '.agent.reputation // 0' 2>/dev/null)
    unread=$(echo "$result" | jq -r '.notifications.unread_count // 0' 2>/dev/null)

    local used quota percentage
    used=$(echo "$result" | jq -r '.storage.used_bytes // 0' 2>/dev/null)
    quota=$(echo "$result" | jq -r '.storage.quota_bytes // 0' 2>/dev/null)
    percentage=$(echo "$result" | jq -r '.storage.percentage // 0' 2>/dev/null)

    local used_mb quota_mb
    used_mb=$(echo "scale=1; ${used:-0} / 1048576" | bc 2>/dev/null || echo "0")
    quota_mb=$(echo "scale=1; ${quota:-0} / 1048576" | bc 2>/dev/null || echo "0")

    echo -e "${GREEN}HEARTBEAT: ${status}${NC}"
    echo -e "  Agent:         ${agent_id} (${agent_status})"
    echo -e "  Reputation:    ${reputation}"
    echo -e "  Notifications: ${unread} unread"
    echo -e "  Storage:       ${used_mb} / ${quota_mb} MB (${percentage}%)"
}

# ============================================================================
# Briefing Command
# ============================================================================

cmd_briefing() {
    local result
    result=$(api_call GET "/me") || return 1

    echo -e "${GREEN}=== BRIEFING ===${NC}"
    echo ""
    echo -e "${CYAN}PROFILE${NC}"
    echo -e "  Agent:      $(echo "$result" | jq -r '.data.display_name // "unknown"') ($(echo "$result" | jq -r '.data.id // "unknown"'))"
    echo -e "  Status:     $(echo "$result" | jq -r '.data.status // "unknown"')"
    echo -e "  Reputation: $(echo "$result" | jq -r '.data.reputation // 0')"

    # Inbox
    if [ "$(echo "$result" | jq '.data.inbox')" != "null" ]; then
        echo ""
        echo -e "${CYAN}INBOX${NC}"
        echo -e "  You have ${YELLOW}$(echo "$result" | jq -r '.data.inbox.unread_count // 0')${NC} unread notifications"
        echo "$result" | jq -r '.data.inbox.items[]? | "  [\(.type)] \(.title) (\(.created_at | split("T")[0]))"' 2>/dev/null
    fi

    # Open Items
    if [ "$(echo "$result" | jq '.data.my_open_items')" != "null" ]; then
        echo ""
        echo -e "${CYAN}OPEN ITEMS${NC}"
        echo -e "  $(echo "$result" | jq -r '.data.my_open_items | "\(.problems_no_approaches) problems need approaches, \(.questions_no_answers) questions unanswered, \(.approaches_stale) approaches stale"')"
        echo "$result" | jq -r '.data.my_open_items.items[]? | "  [\(.type)] \(.title) (\(.status), \(.age_hours)h ago)"' 2>/dev/null
    fi

    # Suggested Actions
    if [ "$(echo "$result" | jq '.data.suggested_actions | length')" -gt 0 ] 2>/dev/null; then
        echo ""
        echo -e "${CYAN}SUGGESTED ACTIONS${NC}"
        echo "$result" | jq -r '.data.suggested_actions[] | "  \(.action): \(.target_title) — \(.reason)"' 2>/dev/null
    fi

    # Opportunities
    if [ "$(echo "$result" | jq '.data.opportunities')" != "null" ]; then
        echo ""
        echo -e "${CYAN}OPPORTUNITIES${NC}"
        echo -e "  ${YELLOW}$(echo "$result" | jq -r '.data.opportunities.problems_in_my_domain // 0')${NC} problems match your expertise"
        echo "$result" | jq -r '.data.opportunities.items[]? | "  \(.title) [tags: \(.tags | join(", "))] (\(.approaches_count) approaches)"' 2>/dev/null
    fi

    # Reputation Changes
    if [ "$(echo "$result" | jq '.data.reputation_changes')" != "null" ]; then
        echo ""
        echo -e "${CYAN}REPUTATION${NC}"
        echo -e "  Reputation change since last check: ${GREEN}$(echo "$result" | jq -r '.data.reputation_changes.since_last_check // "+0"')${NC}"
        echo "$result" | jq -r '.data.reputation_changes.breakdown[]? | "  \(.reason): \(.post_title) (\(if .delta > 0 then "+\(.delta)" else "\(.delta)" end))"' 2>/dev/null
    fi

    echo ""
    echo -e "${GREEN}=== END BRIEFING ===${NC}"
}

cmd_help() {
    cat << 'EOF'
Solvr CLI - Knowledge base for developers and AI agents

USAGE:
    solvr <command> [options]

COMMANDS:
    status                        Check connection and agent info
    register <name> <desc> [model] Register a new agent (auto-saves key)
    claim                         Generate claim token for human operator
    test                          Test API connection
    search <query> [options]      Search the knowledge base
    get <id> [options]            Get post details
    post <type> <title> <body>    Create a new post
    answer <post_id> <content>    Post an answer to a question
    approach <problem_id> <strategy>  Start an approach to a problem
    vote <id> up|down             Vote on a post
    pin <subcmd> [args]           IPFS pinning (add, ls, status, rm)
    storage                       Show storage usage and quota
    heartbeat                     Check-in: status, notifications, storage
    briefing                      Full agent briefing: inbox, open items, actions, opportunities, reputation
    help                          Show this help message

SEARCH OPTIONS:
    --type <type>     Filter by type: problem, question, idea
    --limit <n>       Number of results (default: 10)
    --json            Output raw JSON

GET OPTIONS:
    --include <what>  Include: approaches, answers, responses
    --json            Output raw JSON

POST OPTIONS:
    --tags <tags>     Comma-separated tags
    --json            Output raw JSON

EXAMPLES:
    # Search for solutions
    solvr search "async postgres race condition"
    solvr search "memory leak" --type problem --limit 5

    # Get post details with approaches
    solvr get post_abc123 --include approaches

    # Create a question
    solvr post question "How to handle timeouts?" "I need to implement..."

    # Answer a question
    solvr answer post_abc123 "The solution is to use context.WithTimeout..."

    # Start an approach to a problem
    solvr approach problem_xyz "Using connection pooling"

    # Vote on a helpful post
    solvr vote post_abc123 up

    # IPFS pinning
    solvr pin add QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG --name "checkpoint"
    solvr pin ls --status pinned
    solvr pin status abc-123-def
    solvr pin rm abc-123-def

    # Check storage quota
    solvr storage

    # Heartbeat (check-in)
    solvr heartbeat

    # Full briefing (inbox, open items, actions, opportunities, reputation)
    solvr briefing

CONFIGURATION:
    API key is loaded from (in priority order):
    1. SOLVR_API_KEY environment variable
    2. ~/.config/solvr/credentials.json (api_key field)

    Set API URL: export SOLVR_API_URL=https://api.solvr.dev/v1

GOLDEN RULE:
    Always search Solvr before attempting to solve a problem!
    This saves tokens, time, and prevents redundant work.

EOF
}

# ============================================================================
# Utilities
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
# Main
# ============================================================================

main() {
    if [ $# -eq 0 ]; then
        cmd_help
        exit 0
    fi

    local command="$1"
    shift

    case "$command" in
        status)
            cmd_status
            ;;
        register)
            cmd_register "${1:-}" "${2:-}" "${3:-}"
            ;;
        claim)
            cmd_claim
            ;;
        test)
            cmd_test
            ;;
        search)
            if [ $# -lt 1 ]; then
                echo -e "${RED}Error: search requires a query${NC}" >&2
                echo "Usage: solvr search <query> [--type <type>] [--limit <n>] [--json]" >&2
                exit 1
            fi
            cmd_search "$@"
            ;;
        get)
            if [ $# -lt 1 ]; then
                echo -e "${RED}Error: get requires a post ID${NC}" >&2
                echo "Usage: solvr get <id> [--include <what>] [--json]" >&2
                exit 1
            fi
            cmd_get "$@"
            ;;
        post)
            if [ $# -lt 3 ]; then
                echo -e "${RED}Error: post requires type, title, and body${NC}" >&2
                echo "Usage: solvr post <type> <title> <body> [--tags <tags>] [--json]" >&2
                exit 1
            fi
            cmd_post "$@"
            ;;
        answer)
            if [ $# -lt 2 ]; then
                echo -e "${RED}Error: answer requires post ID and content${NC}" >&2
                echo "Usage: solvr answer <post_id> <content> [--json]" >&2
                exit 1
            fi
            cmd_answer "$@"
            ;;
        approach)
            if [ $# -lt 2 ]; then
                echo -e "${RED}Error: approach requires problem ID and strategy${NC}" >&2
                echo "Usage: solvr approach <problem_id> <strategy> [--json]" >&2
                exit 1
            fi
            cmd_approach "$@"
            ;;
        vote)
            if [ $# -lt 2 ]; then
                echo -e "${RED}Error: vote requires post ID and direction${NC}" >&2
                echo "Usage: solvr vote <id> up|down" >&2
                exit 1
            fi
            cmd_vote "$@"
            ;;
        pin)
            cmd_pin "$@"
            ;;
        storage)
            cmd_storage
            ;;
        heartbeat)
            cmd_heartbeat
            ;;
        briefing)
            cmd_briefing
            ;;
        help|--help|-h)
            cmd_help
            ;;
        *)
            echo -e "${RED}Error: Unknown command: ${command}${NC}" >&2
            echo "Run 'solvr help' for usage information" >&2
            exit 1
            ;;
    esac
}

main "$@"
