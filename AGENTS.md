# Solvr agent instructions

Read [CLAUDE.md](CLAUDE.md) for the repository's project guidelines, test-output rule,
architecture, deployment constraints and operator-secret handling. Consult `SPEC.md` for the
behavior being changed and `progress.txt` for recorded releases. Preserve unrelated work.

## Resume checkpoint: connectivity incident

The durable session handoff is [2026-10-23 connectivity](docs/handovers/2026-10-23-connectivity.md).
Read it before resuming incident work; revalidate time-sensitive facts against production.

- At **2026-10-23 02:36 UTC**, API status was operational and 5/5 fresh normal-DNS requests
  succeeded in **0.16–0.70 seconds**. Both IPv6 addresses worked again. Cloudflare IPv4
  `172.67.190.162` still failed three connection checks. Full route recovery is unproven.
- Deployed API: `b251f95a` (PR #12). Deployed web: `47b44aa1` (PR #11), version 0.4.21.
  Skill 4.0.1 is published and installed locally; credentials were preserved. Both PRs' CI passed.
- The API trusts its configured socket proxy, then only the actual peer Traefik appends to
  `X-Forwarded-For`. Production Traefik preserves incoming forwarding headers, so never replace
  this with unconditional `X-Real-IP` or `CF-Connecting-IP` trust. Live spoof checks passed.
- **No DNS cutover happened.** Both hostnames remain proxied. A temporary DNS-only change was
  prepared but awaited the browser tool's action-time confirmation for bypassing edge protection.
  The user's later requests to check speed and save context did not approve that final action.
  Recheck routing and current DNS before proposing or applying a change; a stale editor is not
  authoritative state. Read-only checks remain within the existing investigation authorization.
- “Active rooms: 1” was accurate for the hero's fixed 24-hour window; the 7-day and 30-day
  section counts matched SQL at 6 and 33. Do not inflate or hardcode this number.
- Private evidence and rollback snapshots are preserved outside this repository at
  `../solvr-ops/incidents/2026-10-23-connectivity/` with owner-only permissions. Panel snapshots
  contain secrets: never print, commit or upload their raw contents. The handoff names safe files
  and filtered operator commands. Do not rely on `/tmp` or the browser session surviving.

The remaining routing investigation is open. A fast page or successful IPv6 request alone does
not establish that every advertised route recovered. Keep the separate `/blog/tags` SSR bug and
unrelated handover folders distinct from this incident's completed fixes.
