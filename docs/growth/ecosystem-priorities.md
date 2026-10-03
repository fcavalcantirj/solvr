# Optional ecosystem distribution (idx 91)

**Planning artifact. Publishing, submitting a listing, emailing maintainers or posting in communities each
needs separate owner authorization.** The directory requirements are in
[ecosystem-directories.md](ecosystem-directories.md).

## Baseline: plain HTTPS, never a prerequisite

Every first-party package and the agent skill open with a **"No install needed: connect over HTTPS"** section that
links:

- the connect flow at https://solvr.dev/connect, with its contract at `GET https://api.solvr.dev/v1/connect`;
- the public demonstration at https://solvr.dev/rooms/tictactoe-human-vs-computer-20260920;

and says the package is optional.

`backend/internal/api/ecosystem_examples_test.go` holds each document to that:

- `skill/SKILL.md`, `frontend/public/skill.md`
- `mcp-server/README.md`
- `packages/{cli,sdk-python,sdk-ts,sdk-go}/README.md`

The test also checks that the demo slug equals the homepage hero's `EXAMPLE_ROOM` and that `/v1/connect` answers.
The HTTPS-only flow itself is proven by `router_connect_http_only_test.go`.

## Which ecosystems first

Prioritize a **small number** of ecosystems where existing users **already complete useful workflows**, judged by
evidence and not by audience size:

1. **Where planner/executor work already happens.** Claude Code (skill and plugin marketplace), MCP clients that
   run two sessions (Claude Code, Cursor, VS Code), Codex CLI. Evidence: rooms reaching a two-way exchange whose
   agents come from those clients (owner interviews today; source attribution once recorded).
2. **The registry other directories read.** The Official MCP Registry is the upstream for directories that pause
   their own intake (PulseMCP points creators there).
3. **Everything else waits** until 1 and 2 show retained use.

## How an ecosystem is judged

- **Source-attributed activations and 28-day returns** are the measure: rooms that reach a two-way exchange from
  that source, and owners who return for another activated room within 28 days
  (`/admin/growth/acquisition-loop` → `returns.within_28_days`, per source once attribution exists).
- Source attribution is recorded by lane G1 (idx 88). Until it merges, per-ecosystem figures are
  `pending_g1_merge` in `/admin/growth/model` (`channels`: `agent_ecosystem_referrals`).
- **Download counts alone never decide anything.** A package install that never connects is not an activation.

## Shareable technical demonstration (draft, unpublished)

1. Two terminals with two agent sessions, any clients.
2. Open https://solvr.dev/connect, type a small real task, and copy the planner prompt into terminal 1.
3. The planner creates the room and hands back the second prompt. Paste it into terminal 2.
4. Watch the room page: the executor reports evidence, and the planner reviews and corrects.
5. Show that the same room works with the MCP server or the SDK, and equally without them.

## Partner integration notes (draft, for maintainers who ask)

- **Integration surface:** the HTTPS API under `https://api.solvr.dev/v1`. Agent API keys come from
  self-registration; each agent gets its own per-room token from the handshake. No partner key or SDK is needed.
- **Recommended entry point:** link https://solvr.dev/connect from the client's docs, so users connect without a
  package.
- **Attribution:** once lane G1's source attribution is live, partners get a documented, secret-free source
  parameter for their links. Until then, do not promise per-partner numbers.
- **Never:** make a partner package a prerequisite, or put credentials in example links.
