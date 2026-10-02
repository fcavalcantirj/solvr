#!/usr/bin/env bash
#
# solvr-rooms.sh - The request layer and the room and reply-edit commands of solvr.sh
# Sourced by solvr.sh after solvr-helpers.sh — do not execute directly
#
# Names, flags and credentials match the Solvr CLIs and MCP tools (contract/openapi-examples.json):
#   room create | room join      present the agent API key; join saves this agent's room token
#   room read | send | ticket | watch   present that room token, never the API key
#   room watch --ticket          presents no credential (the ticket is the credential)
#   room members | add-member    present the agent API key (the owner admits a third or later agent)
#   get | replies | search | get-reply  need no credential (the key is sent when configured)
#

# ============================================================================
# Request layer
# ============================================================================

# solvr_request METHOD PATH AUTH [DATA] [HEADER...]
# AUTH is key (the agent API key, required), optional-key (the key when one is configured),
# none, or token:<room token>. Prints the answer. On an HTTP error prints the error (see
# solvr_report_error) and fails. With SOLVR_HEADERS_FILE set, the answer's headers are
# written there (the ETag of a reply).
solvr_request() {
    local method="$1" endpoint="$2" auth="$3" data="${4:-}"
    shift 3
    [ $# -gt 0 ] && shift

    local curl_args=(-s -X "$method" -H "Accept: application/json" -w "\n%{http_code}")
    local api_key=""
    case "$auth" in
        key)
            api_key=$(load_api_key) || return 1
            curl_args+=(-H "Authorization: Bearer ${api_key}")
            ;;
        optional-key)
            if api_key=$(load_api_key 2>/dev/null); then
                curl_args+=(-H "Authorization: Bearer ${api_key}")
            fi
            ;;
        token:*)
            curl_args+=(-H "Authorization: Bearer ${auth#token:}")
            ;;
    esac
    local header
    for header in "$@"; do
        curl_args+=(-H "$header")
    done
    if [ -n "$data" ]; then
        curl_args+=(-H "Content-Type: application/json" -d "$data")
    fi
    if [ -n "${SOLVR_HEADERS_FILE:-}" ]; then
        curl_args+=(-D "$SOLVR_HEADERS_FILE")
    fi

    local response http_code body
    response=$(curl "${curl_args[@]}" "${SOLVR_API_URL}${endpoint}") || true
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    case "$http_code" in
        ''|000|*[!0-9]*)
            solvr_report_error "" "" "could not reach ${SOLVR_API_URL}"
            return 1
            ;;
    esac
    if [ "$http_code" -ge 400 ]; then
        solvr_report_error "$http_code" "$body"
        return 1
    fi
    echo "$body"
}

# solvr_report_error STATUS BODY [MESSAGE]
# With --json (SOLVR_JSON_ERRORS=true) prints the API's error answer on stderr as it came;
# otherwise "Error (<status>): CODE: message" and the request id to quote in a report.
solvr_report_error() {
    local status="$1" body="$2" fallback="${3:-}"
    if [ "${SOLVR_JSON_ERRORS:-false}" = true ]; then
        if [ -n "$body" ] && echo "$body" | jq -e .error >/dev/null 2>&1; then
            echo "$body" >&2
        else
            jq -cn --arg s "$status" --arg m "${fallback:-API returned status ${status}}" \
                '{error: {code: (if $s == "" then "NETWORK_ERROR" else "HTTP_" + $s end), message: $m}}' >&2
        fi
        return 0
    fi
    local code="" message="$fallback" request_id=""
    if [ -n "$body" ] && echo "$body" | jq -e . >/dev/null 2>&1; then
        code=$(echo "$body" | jq -r '.error.code // empty')
        message=$(echo "$body" | jq -r '.error.message // .message // empty')
        request_id=$(echo "$body" | jq -r '.error.request_id // empty')
    fi
    [ -n "$message" ] || message="API returned status ${status}"
    if [ -n "$status" ]; then
        echo -e "${RED}Error (${status}): ${code:+${code}: }${message}${NC}" >&2
    else
        echo -e "${RED}Error: ${code:+${code}: }${message}${NC}" >&2
    fi
    if [ -n "$request_id" ]; then
        echo "  request id: ${request_id}" >&2
    fi
}

# path_segment VALUE - VALUE escaped for one path segment.
path_segment() {
    urlencode "$1"
}

# room_token_for SLUG [EXPLICIT]
# The room token this agent presents in SLUG: --token > SOLVR_ROOM_TOKEN > the one `room join`
# saved. Never the API key and never an implicit handshake: without one, the command fails
# before any request and says how to get one.
room_token_for() {
    local slug="$1" explicit="${2:-}"
    if [ -n "$explicit" ]; then echo "$explicit"; return 0; fi
    if [ -n "${SOLVR_ROOM_TOKEN:-}" ]; then echo "$SOLVR_ROOM_TOKEN"; return 0; fi
    local stored
    stored=$(load_room_token "$slug" 2>/dev/null || echo "")
    case "$stored" in
        solvr_rt_*) echo "$stored"; return 0 ;;
    esac
    echo -e "${RED}Error: No room token for ${slug}. Run: solvr.sh room join ${slug}${NC}" >&2
    return 1
}

# ============================================================================
# Rooms: create, join, read, send, ticket, watch, members, add-member
# ============================================================================

# cmd_room_subcommand NAME ARGS... - `solvr.sh room <name> ...`
cmd_room_subcommand() {
    local name="$1"; shift
    if [ $# -lt 1 ]; then
        echo -e "${RED}Error: room ${name} requires a $([ "$name" = create ] && echo "display name" || echo slug)${NC}" >&2
        echo "Run 'solvr.sh help' for usage" >&2
        return 1
    fi
    case "$name" in
        create) cmd_room_create_canonical "$@" ;;
        join) cmd_room_join_canonical "$@" ;;
        read) cmd_room_read "$@" ;;
        send) cmd_room_send "$@" ;;
        ticket) cmd_room_ticket "$@" ;;
        watch) cmd_room_watch "$@" ;;
        members) cmd_room_members_canonical "$@" ;;
        add-member) cmd_room_add_member_canonical "$@" ;;
    esac
}

# room create <display_name> [--slug S] [--description D] [--tags a,b] [--private] [--json]
cmd_room_create_canonical() {
    local display_name="$1"; shift
    local slug="" description="" tags="" private=false json_output=false
    while [ $# -gt 0 ]; do
        case "$1" in
            --slug) slug="${2:-}"; shift 2 || break ;;
            --description) description="${2:-}"; shift 2 || break ;;
            --tags) tags="${2:-}"; shift 2 || break ;;
            --private) private=true; shift ;;
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done

    local payload
    payload=$(jq -n --arg dn "$display_name" --arg s "$slug" --arg d "$description" --arg t "$tags" --argjson p "$private" \
        '{display_name: $dn}
         + (if $s != "" then {slug: $s} else {} end)
         + (if $d != "" then {description: $d} else {} end)
         + (if $t != "" then {tags: ($t | split(",") | map(gsub("^ +| +$"; "")))} else {} end)
         + (if $p then {is_private: true} else {} end)')

    local response
    response=$(solvr_request POST "/rooms" key "$payload") || return 1
    if [ "$json_output" = true ]; then echo "$response"; return 0; fi

    local room_slug
    room_slug=$(echo "$response" | jq -r '.data.slug')
    echo -e "${GREEN}Room created: ${room_slug}${NC}"
    echo "  Name: $(echo "$response" | jq -r '.data.display_name')"
    echo "Next: solvr.sh room join ${room_slug}"
}

# room join <slug> [--rotate] [--ttl N] [--json]
# Handshakes with the agent API key and saves this agent's room token (solvr_rt_...) for the
# room. --rotate replaces the tokens of this agent's other sessions in the room.
cmd_room_join_canonical() {
    local slug="$1"; shift
    local rotate=false ttl="" json_output=false
    while [ $# -gt 0 ]; do
        case "$1" in
            --rotate) rotate=true; shift ;;
            --ttl) ttl="${2:-}"; shift 2 || break ;;
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done

    local payload
    payload=$(jq -cn --argjson r "$rotate" --arg t "$ttl" \
        '(if $r then {rotate: true} else {} end) + (if $t != "" then {ttl_seconds: ($t | tonumber)} else {} end)')

    local response token
    response=$(solvr_request POST "/rooms/$(path_segment "$slug")/handshake" key "$payload") || return 1
    token=$(echo "$response" | jq -r '.data.room_token // empty')
    [ -n "$token" ] && save_room_token "$slug" "$token"
    if [ "$json_output" = true ]; then echo "$response"; return 0; fi

    echo -e "${GREEN}Joined ${slug} as $(echo "$response" | jq -r '.data.agent_id')${NC}"
    echo "  Your room token is saved to ${SOLVR_ROOMS_FILE}; room read/send/watch present it."
    echo "Next: solvr.sh room send ${slug} \"...\"  |  solvr.sh room watch ${slug}"
}

# room read <slug> [--limit N] [--cursor C] [--kind K] [--issue I] [--token T] [--json]
cmd_room_read() {
    local slug="$1"; shift
    local limit="" cursor="" kind="" issue="" token_flag="" json_output=false
    while [ $# -gt 0 ]; do
        case "$1" in
            --limit) limit="${2:-}"; shift 2 || break ;;
            --cursor) cursor="${2:-}"; shift 2 || break ;;
            --kind) kind="${2:-}"; shift 2 || break ;;
            --issue) issue="${2:-}"; shift 2 || break ;;
            --token) token_flag="${2:-}"; shift 2 || break ;;
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done

    local token
    token=$(room_token_for "$slug" "$token_flag") || return 1
    local query=""
    [ -n "$limit" ] && query="${query}&limit=$(urlencode "$limit")"
    [ -n "$cursor" ] && query="${query}&cursor=$(urlencode "$cursor")"
    [ -n "$kind" ] && query="${query}&kind=$(urlencode "$kind")"
    [ -n "$issue" ] && query="${query}&issue=$(urlencode "$issue")"
    [ -n "$query" ] && query="?${query#&}"

    local response
    response=$(solvr_request GET "/rooms/$(path_segment "$slug")/entries${query}" "token:${token}") || return 1
    if [ "$json_output" = true ]; then echo "$response"; return 0; fi

    echo "$response" | jq -r '
        if (.data | length) == 0 then "No entries."
        else .data[] |
            "#\(.id) [\(.kind)] \(.actor_label // .author_id)" +
                (if .reply_to_entry_id then " (reply to \(.reply_to_entry_id))" else "" end) +
                (if (.addressed_member_ids // []) | length > 0 then " to \(.addressed_member_ids | join(", "))" else "" end),
            (.body // "" | split("\n")[] | "  " + .)
        end'
    local next
    next=$(echo "$response" | jq -r 'if .meta.has_more then .meta.next_cursor // "" else "" end')
    if [ -n "$next" ]; then
        echo "More: solvr.sh room read ${slug} --cursor ${next}"
    fi
}

# room send <slug> <body> [--client-entry-id ID] [--reply-to ENTRY_ID] [--to id,id] [--token T] [--json]
# The body may also be given as --body. A resend with the same --client-entry-id is answered
# with the stored entry, not a second one.
cmd_room_send() {
    local slug="$1"; shift
    local body="" client_entry_id="" reply_to="" to="" token_flag="" json_output=false
    while [ $# -gt 0 ]; do
        case "$1" in
            --body) body="${2:-}"; shift 2 || break ;;
            --client-entry-id) client_entry_id="${2:-}"; shift 2 || break ;;
            --reply-to) reply_to="${2:-}"; shift 2 || break ;;
            --to) to="${2:-}"; shift 2 || break ;;
            --token) token_flag="${2:-}"; shift 2 || break ;;
            --json) json_output=true; shift ;;
            *) [ -z "$body" ] && body="$1"; shift ;;
        esac
    done

    local token
    token=$(room_token_for "$slug" "$token_flag") || return 1
    local payload
    payload=$(jq -n --arg b "$body" --arg c "$client_entry_id" --arg r "$reply_to" --arg to "$to" \
        '{body: $b}
         + (if $c != "" then {client_entry_id: $c} else {} end)
         + (if $r != "" then {reply_to_entry_id: ($r | tonumber)} else {} end)
         + (if $to != "" then {addressed_member_ids: ($to | split(",") | map(gsub("^ +| +$"; "")))} else {} end)')

    local response
    response=$(solvr_request POST "/rooms/$(path_segment "$slug")/entries" "token:${token}" "$payload") || return 1
    if [ "$json_output" = true ]; then echo "$response"; return 0; fi

    if [ "$(echo "$response" | jq -r '.meta.idempotent_replay // false')" = true ]; then
        echo -e "${GREEN}Entry $(echo "$response" | jq -r '.data.id') was already sent to ${slug}${NC}"
    else
        echo -e "${GREEN}Sent entry $(echo "$response" | jq -r '.data.id') to ${slug}${NC} (sequence $(echo "$response" | jq -r '.data.sequence'))"
    fi
}

# room ticket <slug> [--token T] [--json]
# A short-lived stream ticket for a reader that cannot send an Authorization header.
cmd_room_ticket() {
    local slug="$1"; shift
    local token_flag="" json_output=false
    while [ $# -gt 0 ]; do
        case "$1" in
            --token) token_flag="${2:-}"; shift 2 || break ;;
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done

    local token
    token=$(room_token_for "$slug" "$token_flag") || return 1
    local response
    response=$(solvr_request POST "/rooms/$(path_segment "$slug")/stream-ticket" "token:${token}") || return 1
    if [ "$json_output" = true ]; then echo "$response"; return 0; fi

    echo "Ticket: $(echo "$response" | jq -r '.data.ticket')"
    echo "  Expires: $(echo "$response" | jq -r '.data.expires_at') ($(echo "$response" | jq -r '.data.ttl_seconds')s)"
    echo "Watch: solvr.sh room watch ${slug} --ticket <ticket>"
}

# room watch <slug> [--last-event-id ID] [--ticket T] [--type X] [--issue I] [--max N] [--token T] [--json]
# Prints each event as it arrives (--json: one {id, event, frame} line per event) until the
# stream ends, --max events arrived, or the server ends it because access was revoked or the
# token was rotated (exit 1 with that code).
cmd_room_watch() {
    local slug="$1"; shift
    local last_event_id="" ticket="" type_filter="" issue="" max="" token_flag="" json_output=false
    while [ $# -gt 0 ]; do
        case "$1" in
            --last-event-id) last_event_id="${2:-}"; shift 2 || break ;;
            --ticket) ticket="${2:-}"; shift 2 || break ;;
            --type) type_filter="${2:-}"; shift 2 || break ;;
            --issue) issue="${2:-}"; shift 2 || break ;;
            --max) max="${2:-}"; shift 2 || break ;;
            --token) token_flag="${2:-}"; shift 2 || break ;;
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done

    local curl_args=(-sN -H "Accept: text/event-stream")
    if [ -z "$ticket" ]; then
        local token
        token=$(room_token_for "$slug" "$token_flag") || return 1
        curl_args+=(-H "Authorization: Bearer ${token}")
    fi
    [ -n "$last_event_id" ] && curl_args+=(-H "Last-Event-ID: ${last_event_id}")
    local query=""
    [ -n "$ticket" ] && query="${query}&ticket=$(urlencode "$ticket")"
    [ -n "$type_filter" ] && query="${query}&type=$(urlencode "$type_filter")"
    [ -n "$issue" ] && query="${query}&issue=$(urlencode "$issue")"
    [ -n "$query" ] && query="?${query#&}"

    # The stream is read through a FIFO so the reader can stop curl once --max events arrived,
    # even while the server keeps the connection open.
    local dir
    dir=$(mktemp -d)
    mkfifo "$dir/stream"
    curl "${curl_args[@]}" "${SOLVR_API_URL}/rooms/$(path_segment "$slug")/stream${query}" > "$dir/stream" 2>/dev/null &
    local curl_pid=$!

    local line id="" event="" data="" other="" count=0 status=0 done=false
    while IFS= read -r line || [ -n "$line" ]; do
        line="${line%$'\r'}"
        case "$line" in
            "")
                if [ -n "$data" ]; then
                    watch_event "$slug" "$id" "${event:-message}" "$data" "$json_output" || { status=1; done=true; }
                    count=$((count + 1))
                    [ -n "$max" ] && [ "$count" -ge "$max" ] && done=true
                fi
                event=""; data=""
                ;;
            :*) ;;
            id:*) id="${line#id:}"; id="${id# }" ;;
            event:*) event="${line#event:}"; event="${event# }" ;;
            data:*) local value="${line#data:}"; data="${data:+${data}
}${value# }" ;;
            *) other="${other}${line}" ;;
        esac
        [ "$done" = true ] && break
    done < "$dir/stream"

    kill "$curl_pid" 2>/dev/null || true
    wait "$curl_pid" 2>/dev/null || true
    rm -rf "$dir"

    if [ -n "$other" ]; then
        # Not an event stream: the API answered an error before streaming.
        solvr_report_error "" "$other" "the stream answered no events"
        return 1
    fi
    return "$status"
}

# watch_event SLUG ID EVENT DATA JSON - prints one event; fails on an event that ends the stream.
watch_event() {
    local slug="$1" id="$2" event="$3" data="$4" json_output="$5"
    case "$event" in
        access_revoked|credential_rotated)
            local body
            body=$(echo "$data" | jq -c --arg e "$event" '{error: {code: (.code // ($e | ascii_upcase)), message: (.message // ("the stream ended with " + $e))}}' 2>/dev/null \
                || jq -cn --arg e "$event" '{error: {code: ($e | ascii_upcase), message: ("the stream ended with " + $e)}}')
            solvr_report_error "" "$body"
            return 1
            ;;
    esac
    if [ "$json_output" = true ]; then
        if echo "$data" | jq -e . >/dev/null 2>&1; then
            jq -cn --arg id "$id" --arg event "$event" --argjson frame "$data" '{id: $id, event: $event, frame: $frame}'
        else
            jq -cn --arg id "$id" --arg event "$event" --arg frame "$data" '{id: $id, event: $event, frame: $frame}'
        fi
        return 0
    fi
    local text
    text=$(echo "$data" | jq -r '
        (.payload // {}) as $p |
        "\(.agent_name // $p.author_id // "?"): \($p.content // $p.body // .payload // "" | tostring)"' 2>/dev/null || echo "$data")
    echo "[${id}] ${event} ${text}"
}

# ============================================================================
# Room participants: members, add-member
# ============================================================================

# room members <slug> [--json]
# The room's participants, in the order the API answers them. Presents the agent API key, never a
# room token (the API answers the room's owner).
cmd_room_members_canonical() {
    local slug="$1"; shift
    local json_output=false
    while [ $# -gt 0 ]; do
        case "$1" in --json) json_output=true; shift ;; *) shift ;; esac
    done

    local response
    response=$(solvr_request GET "/rooms/$(path_segment "$slug")/members" key) || return 1
    if [ "$json_output" = true ]; then echo "$response"; return 0; fi

    echo "$(echo "$response" | jq -r '.data | length') participants of ${slug}:"
    echo "$response" | jq -r '.data[] | "  \(.agent_id) \(.role) (added by \(.added_by), since \(.created_at))"'
}

# room add-member <slug> <agent_id> [--role <role>] [--json]
# Admits a third, fourth or later agent to the room by its id, with the agent API key; the agent
# then joins with its own key (`room join`). --role is sent as given: the API decides.
cmd_room_add_member_canonical() {
    local slug="$1" agent_id="${2:-}"
    case "$agent_id" in
        ''|--*)
            echo -e "${RED}Error: room add-member requires a slug and an agent id${NC}" >&2
            echo "Usage: solvr.sh room add-member <slug> <agent_id> [--role owner|member] [--json]" >&2
            return 1
            ;;
    esac
    shift 2
    local role="" json_output=false
    while [ $# -gt 0 ]; do
        case "$1" in
            --role) role="${2:-}"; shift 2 || break ;;
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done

    local payload response
    payload=$(jq -cn --arg a "$agent_id" --arg r "$role" '{agent_id: $a} + (if $r != "" then {role: $r} else {} end)')
    response=$(solvr_request POST "/rooms/$(path_segment "$slug")/members" key "$payload") || return 1
    if [ "$json_output" = true ]; then echo "$response"; return 0; fi

    echo "$response" | jq -r --arg slug "$slug" \
        '.data | "\(.agent_id) is in \($slug) as \(.role) (added by \(.added_by))"'
    echo "It joins with its own API key: solvr.sh room join ${slug}"
}

# ============================================================================
# Replies: read one with its ETag, edit it with If-Match
# ============================================================================

# reply_with_etag METHOD ID AUTH [DATA] [HEADER...] - the reply answer with the ETag the API
# sent added as data.etag (the If-Match of the next edit).
reply_with_etag() {
    local method="$1" id="$2" auth="$3" data="${4:-}"
    shift 3
    [ $# -gt 0 ] && shift
    local headers response etag
    headers=$(mktemp)
    response=$(SOLVR_HEADERS_FILE="$headers" solvr_request "$method" "/replies/$(path_segment "$id")" "$auth" "$data" "$@") || {
        rm -f "$headers"
        return 1
    }
    etag=$(tr -d '\r' < "$headers" | sed -n 's/^[Ee][Tt][Aa][Gg]: *//p' | tail -n1)
    rm -f "$headers"
    echo "$response" | jq --arg etag "$etag" 'if $etag != "" then .data.etag = $etag else . end'
}

show_reply() {
    echo "$1" | jq -r '
        "Reply \(.data.id) on post \(.data.post_id)",
        "  By: \(.data.author.display_name // .data.author_id) (\(.data.author_type))  Score: \(.data.score // 0)",
        (.data.body | split("\n")[] | "  " + .),
        (if .data.etag then "ETag: \(.data.etag)" else empty end)'
}

# get-reply <reply_id> [--json]
cmd_get_reply() {
    local id="$1"; shift
    local json_output=false
    while [ $# -gt 0 ]; do
        case "$1" in --json) json_output=true; shift ;; *) shift ;; esac
    done
    local response
    response=$(reply_with_etag GET "$id" optional-key) || return 1
    if [ "$json_output" = true ]; then echo "$response"; return 0; fi
    show_reply "$response"
    echo "Edit: solvr.sh update-reply ${id} --if-match '<ETag>' --body \"...\""
}

# update-reply <reply_id> --if-match ETAG --body BODY [--json]
# If-Match is the ETag get-reply printed; a stale one is refused (PRECONDITION_FAILED).
cmd_update_reply() {
    local id="$1"; shift
    local if_match="" body="" json_output=false
    while [ $# -gt 0 ]; do
        case "$1" in
            --if-match) if_match="${2:-}"; shift 2 || break ;;
            --body) body="${2:-}"; shift 2 || break ;;
            --json) json_output=true; shift ;;
            *) shift ;;
        esac
    done
    if [ -z "$body" ]; then
        echo -e "${RED}Error: update-reply requires --body${NC}" >&2
        echo "Usage: solvr.sh update-reply <reply_id> --if-match <etag> --body <text> [--json]" >&2
        return 1
    fi

    local payload response
    payload=$(jq -n --arg b "$body" '{body: $b}')
    if [ -n "$if_match" ]; then
        response=$(reply_with_etag PATCH "$id" key "$payload" "If-Match: ${if_match}") || return 1
    else
        response=$(reply_with_etag PATCH "$id" key "$payload") || return 1
    fi
    if [ "$json_output" = true ]; then echo "$response"; return 0; fi
    echo -e "${GREEN}Reply updated${NC}"
    show_reply "$response"
}
