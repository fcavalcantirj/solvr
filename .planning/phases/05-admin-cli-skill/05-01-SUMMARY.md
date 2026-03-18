---
phase: 5
plan: "01"
title: "Admin CLI Skill (solvr-admin.sh)"
status: complete
executed: 2026-03-17
---

# Summary: Plan 05-01 — Admin CLI Skill

## What Was Built

Two files created:

1. `/Users/fcavalcanti/.claude/skills/solvr/scripts/solvr-admin.sh` (404 lines)
2. `/Users/fcavalcanti/.claude/skills/solvr-admin/skill.json`

## Task 1: solvr-admin.sh

Bash script following the exact solvr.sh patterns: `set -euo pipefail`, terminal-aware color codes, `load_admin_key()`, `api_call()`, subcommand dispatch via `case`, `jq` for JSON.

Key differences from solvr.sh:
- Uses `ADMIN_API_KEY` env var (not `SOLVR_API_KEY`)
- Auth header: `X-Admin-API-Key` (not `Authorization: Bearer`)
- Base URL: `https://api.solvr.dev` (no `/v1/` prefix)
- Credentials file: `~/.config/solvr/admin-credentials.json` (field: `admin_api_key`)

Commands implemented:
- `email send --subject <s> --body <b> [--body-html <h>] [--json]` → POST /admin/email/broadcast
- `email dry-run --subject <s> --body <b> [--body-html <h>] [--json]` → POST /admin/email/broadcast with dry_run:true
- `email history [--json]` → GET /admin/email/history
- `help` → prints usage

## Task 2: skill.json

Created `/Users/fcavalcanti/.claude/skills/solvr-admin/skill.json` with:
- `name: "solvr-admin"`, `category: "admin"`
- `authentication.header: "X-Admin-API-Key: {admin_api_key}"`
- Capabilities: `email-broadcast`, `email-dryrun`, `email-history`

## Task 3: Testing

Steps 1–3 passed (no deployed API required):
- `bash -n` syntax check: OK
- `help` subcommand: OK (exits 0, prints usage)
- Missing ADMIN_API_KEY: OK (exits non-zero, prints "No admin API key found")

Steps 4–6 (live API tests) deferred — require Phase 3/4 endpoints deployed to api.solvr.dev.

## Acceptance Criteria Status

### Task 1
- [x] File exists at `.claude/skills/solvr/scripts/solvr-admin.sh`
- [x] `bash -n` syntax check passes
- [x] `solvr-admin help` prints usage without requiring API key
- [x] `solvr-admin email send` without `--subject` exits non-zero with error
- [x] `solvr-admin` (no args) prints help and exits 0
- [x] `ADMIN_API_KEY` only in `load_admin_key()` — never echoed or interpolated into URL
- [x] 404 lines total, well under 900-line limit

### Task 2
- [x] File exists at `.claude/skills/solvr-admin/skill.json`
- [x] `jq .` validates JSON
- [x] `name` field is `"solvr-admin"`
- [x] `authentication.header` references `X-Admin-API-Key`

### Task 3
- [x] `bash -n` syntax check passes
- [x] `help` subcommand exits 0
- [x] Missing key produces correct error and non-zero exit
- [ ] `email history` returns data — deferred (API not yet deployed)
- [ ] `email dry-run` returns recipient count — deferred (API not yet deployed)
- [ ] `email dry-run --json | jq '.would_send'` returns integer — deferred
- [x] Zero real emails sent during testing
