#!/bin/bash
# ralph-overnight.sh — run the loop unattended and stop at a wall-clock time.
#
#   ./ralph-overnight.sh                  # claude engine, stops at 05:00
#   ./ralph-overnight.sh claude 04:30     # explicit engine + stop time
#   ./ralph-overnight.sh codex            # another engine, still 05:00
#
# Detached, survives closing the terminal:
#   tmux new-session -d -s ralph './ralph-overnight.sh >> ralph-continuous.log 2>&1'
#   tmux attach -t ralph          # watch
#   tmux kill-session -t ralph    # stop early
#
# The stop is CLEAN: the supervisor finishes the task in flight and refuses to start
# another batch at or after the deadline, so no iteration is killed half-done and the
# working tree is never left dirty. It also stops early by itself on RALPH BLOCKED
# (two iterations with no ledger progress) or when the ledger completes.
#
# caffeinate -dimsu keeps the Mac awake for the whole run (display may still sleep;
# that does not pause the loop).
cd "$(dirname "$0")" || exit 1

ENGINE_ARG="${1:-claude}"
STOP_AT="${2:-05:00}"

case "$ENGINE_ARG" in
  claude|codex|opencode|kimi) ;;
  *) echo "Unknown engine '$ENGINE_ARG' (expected claude, codex, opencode, or kimi)"; exit 1 ;;
esac
if ! echo "$STOP_AT" | grep -qE '^([01][0-9]|2[0-3]):[0-5][0-9]$'; then
  echo "Invalid stop time '$STOP_AT' — want HH:MM, e.g. 05:00"
  exit 1
fi

exec caffeinate -dimsu env RALPH_STOP_AT="$STOP_AT" ./ralph-continuous.sh "$ENGINE_ARG"
