# Agent-client docs and extension directories (idx 91)

**Planning artifact. Nothing here authorizes a submission, a listing, an account, an email or a post.** Each of
those needs a separate decision by the owner. The requirements below come from read-only fetches of each
directory's public page. They change, so **verify again on the day of any submission.**

Legend: **VERIFIED 2026-10-02** means the requirement was read on the public page that day. **UNVERIFIED** means
the page did not state it, or it was not checked; treat it as unknown.

| Directory / docs | URL checked | What a listing or install needs | Status |
|---|---|---|---|
| Official MCP Registry | https://github.com/modelcontextprotocol/registry | Publish with the `mcp-publisher` CLI. Prove namespace ownership: GitHub OAuth or OIDC for `io.github.<user>/<name>`, or a DNS/HTTP challenge for a domain namespace such as `dev.solvr/<name>`. | VERIFIED 2026-10-02 (README). Detailed `server.json` fields: UNVERIFIED (the quickstart was not read). |
| Smithery | https://smithery.ai/docs/build/publish | Either a hosted URL server ("Streamable HTTP transport", OAuth if auth is needed, 401 not 403 for protected endpoints) or a local `.mcpb` bundle. Metadata is scanned automatically or served at `/.well-known/mcp/server-card.json`. Publish via smithery.ai/new or the CLI; no GitHub link is required. | VERIFIED 2026-10-02 |
| Claude Code plugin marketplace (self-hosted) | https://code.claude.com/docs/en/plugin-marketplaces, https://code.claude.com/docs/en/plugins/publish | A git repo with `.claude-plugin/marketplace.json` (`name`, `owner`, `plugins[]` with `name` and `source`). The entry name equals the `plugin.json` name. Validate with `claude plugin validate --strict`. Users run `claude plugin marketplace add <owner>/<repo>`, then `claude plugin install <name>@<marketplace>`. No submission step. | VERIFIED 2026-10-02 |
| Anthropic's directory (claude.ai / Cowork / Claude Code) | https://code.claude.com/docs/en/plugins/publish | Submit from the developer portal (claude.ai/directory/manage). Needs a paid claude.ai plan and a GitHub repository holding the plugin. The portal applies extra rules beyond the CLI validation; see claude.com's pre-submission checklist. Each version is reviewed before publication. `claude-plugins-official` takes no portal submissions. | VERIFIED 2026-10-02. The claude.com checklist itself: UNVERIFIED (not read). |
| Cursor | https://cursor.com/docs/context/mcp | Users add servers in `.cursor/mcp.json` or `~/.cursor/mcp.json` (`mcpServers`: `command`/`args`/`env`, or `url` for a remote HTTP server, with optional `auth`). "Add to Cursor" exists on Cursor Marketplace listings; community servers are catalogued at cursor.directory. | VERIFIED 2026-10-02 (user config). Marketplace / cursor.directory submission requirements: UNVERIFIED. |
| VS Code (Copilot MCP) | https://code.visualstudio.com/docs/copilot/customization/mcp-servers | Users add servers in `.vscode/mcp.json`, `.mcp.json`, the user profile, or `~/.copilot/mcp-config.json`. Remote servers use `"type": "http"` plus a URL. A gallery is browsable with `@mcp` in Extensions, and `--add-mcp` takes JSON. | VERIFIED 2026-10-02 (user config). Gallery listing process: UNVERIFIED (not stated on the page). |
| Glama MCP directory | https://glama.ai/mcp/servers | Has an "Add Server" link and "Claimed"/"Official" badges. | UNVERIFIED: the page does not state submission requirements (license, Dockerfile, claim process). |
| PulseMCP | https://www.pulsemcp.com/submit | "submissions and changes are temporarily paused"; the page points creators to the Official MCP Registry first. | VERIFIED 2026-10-02 (paused) |
| ClawHub (OpenClaw skills) | https://clawhub.ai | `clawhub login`, then `clawhub skill publish ./my-skill --slug my-skill --version 1.0.0`. | VERIFIED 2026-10-02 (commands). SKILL.md format rules and review process: UNVERIFIED. |
| mcp.so | not checked | — | UNVERIFIED |
| npm (`@solvr/*` packages) | not checked | Standard npm publish; already in use by `packages/`. | UNVERIFIED this session |
| PyPI (`solvr` SDK) | not checked | Standard PyPI publish. | UNVERIFIED this session |
| pkg.go.dev (Go SDK) | not checked | Indexed from a public module path; no submission. | UNVERIFIED this session |

## Baseline rule

Every listing links to the **no-install HTTPS flow** (https://solvr.dev/connect, contract at
https://api.solvr.dev/v1/connect) and a public demonstration room. No directory, package or plugin is a
prerequisite for using Solvr, so losing any single ecosystem partnership changes nothing about how an agent
connects.
