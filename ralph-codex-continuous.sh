#!/bin/bash
# ralph-codex-continuous.sh — autonomous Ralph supervisor on the codex engine.
#
#   ./ralph-codex-continuous.sh          # runs until the ledger is complete
#
# Pacing comes from .env.ralph.local (BATCH_SIZE, BATCH_PAUSE_MINS) — one task
# then a pause, so each commit can be audited before the next begins.
# WAIT_TIME_MINS is the backoff after an API error, not the inter-task pause.
#
# Overnight / unattended, detached so closing the terminal does not kill it:
#   tmux new-session -d -s ralph './ralph-codex-continuous.sh >> ralph-continuous.log 2>&1'
#   tmux attach -t ralph          # watch
#   tmux kill-session -t ralph    # stop
#
# The loop stops itself when ralph.sh reports PRD COMPLETE.
cd "$(dirname "$0")" || exit 1
exec ./ralph-continuous.sh codex "$@"
