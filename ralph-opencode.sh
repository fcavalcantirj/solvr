#!/bin/bash
# ralph-opencode.sh — Ralph loop on the OpenCode Zen engine.
#   ./ralph-opencode.sh 3        # up to three tasks
#
# Credentials live in opencode's own auth store (`opencode auth login`), not in
# an env var. OPENCODE_MODEL / OPENCODE_VARIANT / OPENCODE_EXTRA_FLAGS override
# the model, reasoning effort and verbosity — set them here or in .env.ralph.local.
cd "$(dirname "$0")" || exit 1
ENGINE=opencode exec ./ralph.sh "$@"
