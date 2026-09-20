#!/bin/bash
# ralph-claude.sh — Ralph loop on the Claude Code CLI (the default engine).
#   ./ralph-claude.sh 3          # up to three tasks
#   MODEL=claude-opus-5 ./ralph-claude.sh 1   # pin a model
#
# Claude is the only engine that reports per-iteration tokens and cost: the
# ledger and journal are attached with @file and usage comes back as JSON.
cd "$(dirname "$0")" || exit 1
ENGINE=claude exec ./ralph.sh "$@"
