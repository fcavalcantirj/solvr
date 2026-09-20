#!/bin/bash
# ralph-kimi.sh — Ralph loop on the Kimi Code CLI (~/.kimi-code/bin/kimi).
#   ./ralph-kimi.sh 2                                     # up to two tasks
#   KIMI_MODEL=openrouter/z-ai/glm-5.3 ./ralph-kimi.sh 1  # another alias
#
# Model aliases are the keys of [models."..."] in ~/.kimi-code/config.toml
# (`kimi provider list` shows the providers). Default in ralph.sh:
# openrouter/poolside/laguna-s-2.1:free (free tier, 1000 requests/day).
# KIMI_EXTRA_FLAGS appends raw flags. Transient provider errors are retried
# forever by resuming the session (backoff 60 s doubling to 300 s); the batch
# stops only when the daily free quota is reached (error text says so, or 6
# retries in a row die within 20 s). KIMI_RETRIES>0 caps the attempts.
# Both may also live in .env.ralph.local.
cd "$(dirname "$0")" || exit 1
ENGINE=kimi exec ./ralph.sh "$@"
