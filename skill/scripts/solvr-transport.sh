#!/usr/bin/env bash
# Shared transport policy for solvr.sh. Bash 3.2 compatible; no address pinning or retries.

solvr_positive_seconds() {
    case "$1" in ''|*[!0-9]*) return 1 ;; esac
    [ "$1" -gt 0 ] 2>/dev/null
}

# solvr_curl request|stream ARGS... — ordinary requests have a total deadline;
# streams have only a connection deadline. The caller reports a curl failure.
solvr_curl() {
    local mode="$1"; shift
    local connect="${SOLVR_CONNECT_TIMEOUT:-5}" request="${SOLVR_REQUEST_TIMEOUT:-30}"
    solvr_positive_seconds "$connect" || return 2
    if [ "$mode" = request ]; then
        solvr_positive_seconds "$request" || return 2
        curl --connect-timeout "$connect" --max-time "$request" "$@"
    else
        curl --connect-timeout "$connect" "$@"
    fi
}

# solvr_stream ARGS... — preserve raw SSE, including heartbeat comments. A quiet
# room stays open as long as its heartbeats arrive. Every exit reaps curl.
solvr_stream() {
    local idle="${SOLVR_STREAM_IDLE_TIMEOUT:-90}"
    if ! solvr_positive_seconds "$idle" || ! solvr_positive_seconds "${SOLVR_CONNECT_TIMEOUT:-5}"; then
        solvr_report_error "" "" "connection and stream idle deadlines must be positive integers"
        return 1
    fi
    local stream_dir stream_pid="" stream_code=0 line="" read_code=0 read_started=0
    stream_dir=$(mktemp -d) || return 1
    solvr_stream_cleanup() {
        if [ -n "$stream_pid" ]; then
            kill "$stream_pid" 2>/dev/null || true
            wait "$stream_pid" 2>/dev/null || true
        fi
        rm -rf "$stream_dir"
    }
    trap solvr_stream_cleanup EXIT
    trap 'exit 130' INT
    trap 'exit 143' TERM
    mkfifo "$stream_dir/body"
    # exec keeps the captured PID attached to curl itself, so cancellation can reap it.
    (exec curl --connect-timeout "${SOLVR_CONNECT_TIMEOUT:-5}" "$@") > "$stream_dir/body" 2>/dev/null &
    stream_pid=$!
    while true; do
        line=""
        read_started=$SECONDS
        if IFS= read -r -t "$idle" line; then
            printf '%s\n' "$line"
        else
            read_code=$?
            # Bash 3.2 returns 1 for both EOF and a read timeout. A timeout
            # consumed the whole idle budget while curl is still running.
            if [ "$read_code" -gt 128 ] || { [ "$((SECONDS - read_started))" -ge "$idle" ] && kill -0 "$stream_pid" 2>/dev/null; }; then
                stream_code=1
                solvr_report_error "" "" "room stream received no complete line for ${idle} seconds; reconnect with the last event id"
            elif [ -n "$line" ]; then
                printf '%s' "$line"
            fi
            break
        fi
    done < "$stream_dir/body"
    if [ "$stream_code" -eq 0 ]; then
        if wait "$stream_pid"; then
            stream_pid=""
        else
            stream_code=$?
            stream_pid=""
            solvr_report_error "" "" "room stream transport failed (curl exit ${stream_code}); reconnect with the last event id"
            stream_code=1
        fi
    fi
    solvr_stream_cleanup
    trap - EXIT INT TERM
    return "$stream_code"
}
