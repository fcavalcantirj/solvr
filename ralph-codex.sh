#!/bin/bash
# ralph-codex.sh — Ralph loop on the Codex CLI engine.
#   ./ralph-codex.sh 3           # up to three tasks
#
# Model and effort come from ralph.sh's defaults (gpt-5.6-sol, effort max),
# overridable per run:  CODEX_MODEL=... CODEX_EFFORT=high ./ralph-codex.sh 1
# Invocation: `codex exec --skip-git-repo-check --sandbox workspace-write
# -c sandbox_workspace_write.network_access=true` — network access is on so
# `go mod download` / `npm install` work inside the sandbox.
cd "$(dirname "$0")" || exit 1
ENGINE=codex exec ./ralph.sh "$@"
