---
slug: site-look-and-feel
date: 2026-10-03
status: approved
round: 0
author_session: lane S executor (Opus 5.5) — built the one-sentence Prompt look on /connect, guides and home cards (v1.3.6)
---

# Handover — apply the v1.3.6 "one sentence" look & feel to the rest of solvr.dev

## Mission
v1.3.6 gave /connect, /docs/guides, /docs/guides/{3 use cases} and the home use-case cards a new visual
language. Felipe's words for it: "ONE prompt, BIG, elegant… not a testament… gorgeous". The rest of the site
still wears the old one. Compare https://solvr.dev/data with https://solvr.dev/connect?preset=plan-and-build.

The old look:
- a mono letter-spaced "eyebrow" label above a 5xl–6xl light heading
- shadcn Cards, max-w-7xl
- mono headings on the app pages
- green and emerald status dots
- Recharts with rounded bars

The job: bring every other page into the new language, page family by page family. Each family is
mock-first, and Felipe approves the look before it is wired. API is smart, client is dumb.

Done means:
- every page family is migrated and approved
- the tests are re-pointed and each one is named
- lint is not worse
- the baseline is unchanged
- screenshots at 1440 and 390 for every page

## Where things stand (verified)
- [REAL] Lane S is merged to main (`3360cf54`), released as v1.3.6 (`74dcc56a`, frontend 0.4.6) and live.
  Checked 2026-10-03: `curl -s https://api.solvr.dev/v1/connect` returns `instruction_version` 2.0 with
  `prompt.segments`, 53 words, and `/v1/connect/examples` answers 200.
- [REAL] **The new language**, in code. Read these first: they are the reference implementation.
  - `frontend/components/prompt/prompt.tsx` (variants connect | guide | card)
  - `prompt-sentence.tsx` (renders API segments by kind)
  - `prompt-stack.tsx` (aligned rows)
  - `role-pair-switch.tsx`
  - `copy-prompt-button.tsx`
  - `prompt-align.ts`
  - `frontend/components/connect/connect-panel.tsx`
  - `frontend/app/connect/page.tsx`
  - `frontend/app/docs/guides/page.tsx`
  - `frontend/app/docs/guides/[slug]/page.tsx`
  - `frontend/components/homepage/use-cases-section.tsx`
- [REAL] **The vocabulary** (taken from those files):
  - **Page container:** `mx-auto w-full max-w-[76rem] px-4 sm:px-6 lg:px-12`. Pages clear the fixed header with `pt-20`–`pt-28`.
  - **Headings:**
    - Quiet h1 with no eyebrow: `text-xl font-normal sm:text-2xl` on /connect, and
      `text-[1.625rem] font-normal leading-[1.2] tracking-[-0.02em] sm:text-[1.875rem]` on a guide.
    - Index h1: `text-[2rem] font-light leading-[1.15] tracking-[-0.025em] sm:text-[2.5rem]`.
    - One line under the h1: `text-base sm:text-lg text-muted-foreground leading-relaxed max-w-[44rem]`.
  - **Hero content:** the thing itself (the sentence), set big: Inter 300, `lg:text-[2.5rem]`.
  - **Two-column composition:** content `lg:col-span-8`, quiet side column `lg:col-span-4 lg:border-l lg:pl-10`
    (sticky `lg:top-28`), holding one line of what happens next and the single ink primary button.
  - **Grids:** `grid gap-px border border-border bg-border` cell grids, each cell `bg-background p-6 sm:p-8`.
  - **Labels:** mono uppercase `text-[11px] tracking-[0.18em]` only for small captions and links, never above a heading.
  - **Accent:** one chartreuse token `--prompt-accent` (light `oklch(0.91 0.19 118)`, dark `oklch(0.88 0.18 118)`), used
    ONLY as a fill behind ink. Classes: `.prompt-band`, `.prompt-swipe`, `.prompt-swipe-open`, `.prompt-pill`
    in `frontend/app/globals.css`.
  - **Corners and icons:** square corners everywhere; lucide icons, not glyphs.
- [REAL] **Pages still in the old language** (explorer map, 2026-10-03). The marks are: eyebrow `font-mono text-xs
  tracking-[0.3em]` above an h1/h2 (~47 matches in 34 files), 10px span eyebrows, the "mono-h1" pattern, and
  max-w-7xl (108 uses, 59 files).

  | Family | Pages | Current look |
  |---|---|---|
  | Data / stats | `/data`, `/status`, `/leaderboard` | see below |
  | Rooms | `/rooms`, `/rooms/[slug]`, `/rooms/[slug]/history/[page]` | room-header has a 10px eyebrow above a 5xl h1 |
  | Posts | `/posts`, `/posts/[id]`, replies, new, edit | max-w-3xl/2xl, 3xl–4xl light h1 |
  | Agents / users / profiles / settings / referrals / pins / dashboard / admin | | mono-h1, max-w-7xl, mostly no Footer |
  | Marketing | `/skill`, `/mcp`, `/api-docs`, `/amcp`, `/ipfs`, `/how-it-works` | `py-20 lg:py-32` hero, 10px span eyebrow, 6xl light h1, an eyebrow in every section |
  | Docs | `/docs`, `/docs/protocol` | `/docs` has 5 eyebrows |
  | About / legal | `/about`, `/terms`, `/privacy` | `/about` is 741 lines with 9 eyebrows |
  | Auth | `/login`, `/join`, `/claim`, `/auth/callback`, `/email/unsubscribe` | mono-h1, centred cards |
  | Blog | `/blog`, `/blog/[slug]`, `/blog/create` | |
  | Other | `/notifications`, `/zh/promote`, `not-found` | |

  The rest of the home (`hero-section.tsx`, `live-overview.tsx` and the `homepage/*` sections through
  `SectionHeading` in `components/homepage/metric.tsx`) is still the old language.

  /data in detail:
  - `app/data/page.tsx` is a client page, 463 lines, with no Footer.
  - It opens with the eyebrow `STATISTICS` above a 5xl light h1.
  - `components/data/platform-statistics.tsx` renders 4 homepage sections with SectionHeading and MetricGrid.
  - The live-search band has an eyebrow plus an h2 repeating the same words.
  - shadcn StatCards and a Table.
  - Recharts 2.15.4 Pie and Bar charts, the bar with `radius={[0,4,4,0]}`, which the radius scan misses.
- [REAL] **Shared pieces:**
  - `components/header.tsx` (fixed h-16).
  - `components/footer.tsx` (full | compact); about 15 pages render no Footer.
  - `components/homepage/metric.tsx` (`SectionHeading`, `MetricGrid`, `CompactTable`).
  - shadcn `components/ui/*`.
  - No generic PageHeader or Section component exists yet.
- [REAL] **Impeccable (pbakaus/impeccable 4.5.0)** is installed with scope "project" for `/Users/fcavalcanti/dev/solvr` only
  (`~/.claude/plugins/installed_plugins.json`). It is enabled by `/Users/fcavalcanti/dev/solvr/.claude/settings.json`,
  which is GITIGNORED (`.gitignore:10 .claude/`), so a git worktree does NOT have it.
  - Either start the session in `/Users/fcavalcanti/dev/solvr` and work on the worktree by absolute paths (what lane S did),
    or copy that settings file into the worktree's `.claude/`.
  - Verify: the skill list shows `impeccable:impeccable`.
  - Launcher: `~/.claude/plugins/cache/impeccable/impeccable/4.5.0/skills/impeccable/scripts/impeccable`. Run `context --target <file>`
    once, with cwd = the worktree; it reports NO_PRODUCT_MD and MANUAL_DETECTOR_REQUIRED.
  - `critique` spawns exactly 2 isolated sub-agents.
  - `detect --json <files>` is the mechanical scan.
- [TEST] Lane S verification (pre-merge, on its own lane): full backend shows the 4 baseline failures only; frontend
  1680/1680; lint 22 errors / 172 warnings before and after; next build OK.
- [UNVERIFIED] The rooms and app pages may carry live states (presence dots, loading, empty, error) whose visual
  changes need state-by-state screenshots; not inventoried yet.

## Blocking constraints (builder: restate these before planning)
1. **Local only.** No push, no tags, no deploy, no production writes (never `api.solvr.dev/admin`, never the `.env`
   values). Reading public pages to compare is fine.
2. **Work in a git worktree** under `~/dev/solvr-lanes/<lane>` on its own branch from current main, with its own DB
   (`createdb solvr_lane_<x>`, migrate up) on the Docker Postgres at **localhost:5435**, and its own ports. Never edit,
   build, test or commit in `/Users/fcavalcanti/dev/solvr` itself; it is reserved for Felipe and the orchestrator.
3. **Mock first, per page family.** Build the change, serve it locally, send desktop and mobile screenshots,
   then STOP until Felipe says "design approved" for that family. No wiring before approval.
4. **API is smart, client is dumb** (repo CLAUDE.md rule 3). No copy, numbers or labels may be computed or invented in
   the client; restyling must not change what is said. A copy change is Felipe's call.
5. **The design-system tests are law** (`frontend/design-system.test.tsx`):
   - The neutral palette may carry chroma only in `destructive`, `chart-N` and `prompt-accent`.
   - `--radius` is 0, and no `rounded-*` larger than `lg` (except `full`).
   - The `SOLVR_` wordmark is pinned.
   - The section-rhythm string `px-4 sm:px-6 lg:px-12 py-12 lg:py-16` is pinned in a 10-file list, also pinned by
     `components/homepage/overview-sections.test.tsx`.
   - Changing a pinned rule means naming the test and its replacement (TDD; never loosen a test silently).
6. **Test-output hard rule** (repo CLAUDE.md §8):
   - Save the full output to a log and read only `tail -n 10`.
   - On a failure, run `grep -E -- '--- FAIL|_test\.go:[0-9]+:|Error:|expected|actual|FAIL ' <log> | head -40`.
   - Never re-run a suite to see more.
   - Heavy runs go through `/tmp/solvr-heavy.sh`.
7. **No loop runners.** Never execute `ralph*.sh` (not even for a banner). Spawn at most 2–3 sub-agents at a time
   (Impeccable critique's 2 count).
8. **Shell gotchas.**
   - The shell exports `PORT=3000`: start every server with `env -i HOME="$HOME" PATH="$PATH" …` and explicit ports.
     Never bind :3000. Kill only the PIDs you started.
   - Frontend `NEXT_PUBLIC_API_URL` defaults to PRODUCTION. Set it to your local API for `next dev`, and at BUILD time
     for `next start`.

## Accepted residuals / Refuted — don't fix
- **Fonts are fine.** REFUTED: the claim that Inter does not load. Prod CSS serves `@font-face{font-family:Inter…}` (checked 2026-10-03). Do not touch font loading.
- **Dark mode is not mounted** (next-themes `ThemeProvider` is unused) and is pre-existing. The `.dark` tokens must still exist because the test demands them. Not in scope unless Felipe asks.
- **`next start` warning.** It warns `"next start" does not work with "output: standalone"`, and it still serves fine for local smoke.
- **The zip.** `npm run build`'s prebuild rewrites `frontend/public/solvr-skill.zip` (same contents): `git checkout` it before committing or rebasing.
- **Local API.** It refuses to mount `/v1` routes if `JWT_SECRET` is under 32 chars ("Database pool is nil" / routes 404). Use a 32+ char local-only value. CORS needs `ALLOWED_ORIGINS=http://localhost:<port>`.
- **Room detail shape.** `GET /v1/rooms/{slug}` returns room fields under `data.room` (not `data`).
- **Room panels.** The sidebar connect panels (`components/rooms/connect-agent-panel.tsx`, `room-starter-prompts.tsx`) already render API segments; only their chrome is old.
- **Gaps in this page.** The guides-index alignment relies on equal-width example intents served by the API. Visibility "public/private" leaves a ~5px centred gap; this was accepted.
- **Out of scope.** `router_connect_test.go` gofmt drift; the "Connect agents" header button shown on /connect.

## Hard rules & human-reserved decisions
- TDD: RED log, then GREEN. One commit per page family, one-line messages plus the repo's Co-Authored-By trailer. Code files stay under ~800 lines (CI). Label every claim [REAL], [TEST] or [UNVERIFIED] in reports.
- Use Impeccable: `critique` and `polish` per family, `audit` at the end, with bounded rounds (inspect once, fix in one batch, confirm once).
- WHICH PAGE FAMILIES, AND IN WHAT ORDER, IS FELIPE'S CALL. Recommend `/data` first; it is his example.
- REPLACING RECHARTS (or keeping it restyled) IS FELIPE'S CALL.
- Any copy change, any removed section, and whether app pages (settings, admin, auth) get the full treatment IS FELIPE'S CALL.
- Committing PRODUCT.md / DESIGN.md (Impeccable context files) IS FELIPE'S CALL. Lane S kept them local-only and deleted them.
- Introducing any new colour token beyond `--prompt-accent` IS FELIPE'S CALL.
- Merging, releasing and deploying IS FELIPE'S CALL (through the orchestrator).

## Acceptance checklist (the author approves the plan ONLY against these)
1. Restates the 8 blocking constraints in its own words, before any step.
2. Includes a verified route inventory: every family above with file paths, plus its current look and its intended
   look in one line each. Discrepancies with this handover are reported.
3. Proposes extracting shared primitives first, with names and file paths, instead of per-page copies:
   - a page container
   - a quiet page header (h1 + one line, no eyebrow)
   - a section header without eyebrow, replacing or retiring `SectionHeading`
   - a stat cell and grid in the `gap-px bg-border` idiom
   - an optional two-column content/side layout
4. Sequences the work by family, mock-first. The first family is a screenshot-backed mock of `/data`, desktop 1440
   and mobile 390, served locally, followed by a STOP for "design approved". It states the exact local URL and the
   screenshot paths it will produce.
5. Keeps every number and label coming from the API: no client-side arithmetic, sorting or new copy.
   `platform-statistics.test.tsx`'s source scan stays green.
6. Lists every test it expects to re-point (from the "tests that pin structure" list: design-system, overview-sections,
   `app/data/page.test.tsx`, `platform-statistics.test.tsx`, status, about, api-hero, leaderboard, agents, pins,
   ipfs-status, room-stats, agents-sidebar, header, footer, hero) and says how each will be named in the report.
7. Has a states plan: loading, empty, error, and live and presence indicators are shown and screenshotted per family.
   Status colours meet the rooms contrast test.
8. Says how the accent is used. It is used only as a fill behind ink, or explicitly not used at all on data pages,
   and no new chromatic token appears without the regex change being named.
9. Has a verification section: the focused tests per family, then once at the end the full frontend suite, `next
   build` with `NEXT_PUBLIC_API_URL` set, and lint before and after (a lower error count is fine; never higher).
   It also covers `scripts/check-file-size.sh` on the touched files and a local smoke with screenshots of every
   touched page at 1440 and 390.
10. Stays within limits: at most 2–3 concurrent sub-agents, and Impeccable rounds bounded.
11. Contains no server/API change, unless a page truly needs data the API does not serve. Any such change is
    API-first (SPEC.md, then tests, then the handler) and flagged as Felipe's call.

## next_action
Create the worktree and branch from main, then `cd frontend && npm ci`. Run the baseline lint and save it as
lint-before. Take screenshots of the CURRENT `/data` (prod, read-only), and build the `/data` mock in the new
language on a local `next dev` against a local API. Then STOP and send Felipe the URL and 2 screenshots.

## Open questions
1. Recharts: restyle (square bars, monochrome), or replace with the in-house `gap-px` div bars already used by the
   homepage sections? (Felipe)
2. Should every page get the Footer? (About 15 pages render none today.) (Felipe)
3. Should the home hero and live-overview be included in this pass, or are they a separate lane? (Felipe)

## Pointers
- Approved lane S plan and its owner decisions: `~/.claude/plans/pasted-content-id-c640-you-are-shimmering-pond.md`
  (superseded by this file for the new task; the decisions are kept in memory).
- Memory notes in `~/.claude/projects/-Users-fcavalcanti-dev-solvr/memory/`:
  - `feedback-slim-prompts-big-design.md`: Felipe's taste.
  - `lane-s-slim-prompt-2026-10-03.md`: lane S state and decisions.
  - `shell-exports-port-3000.md`
  - `feedback-test-output-tail.md`
- Repo rules: `/Users/fcavalcanti/dev/solvr/CLAUDE.md`, especially rule 3 (API smart) and §8 (test output).
- Impeccable references: `~/.claude/plugins/cache/impeccable/impeccable/4.5.0/skills/impeccable/reference/`.
  Read `craft-floor.md` (bans eyebrows above headings), `critique.md`, `polish.md`, `typeset.md`, `audit.md`.
- Lane S screenshots for the target look: `/tmp/solvr-lane-s/smoke-{desktop,mobile}-{connect,guide,guides-index,home}.png`.
  These are local and may be gone; prod `/connect` is the live reference.
- The local API recipe that worked (no secret values; the DB URL is the lane's local Docker DB):
  `env -i HOME="$HOME" PATH="$PATH" PORT=<port> DATABASE_URL='postgres://solvr:solvr_dev@localhost:5435/<lane_db>?sslmode=disable' JWT_SECRET=<32+ char local-only string> ALLOWED_ORIGINS=http://localhost:<web port> <built api binary>`
- Screenshot harness pattern (Playwright from the lane's node_modules): `/tmp/solvr-lane-s/pages.mjs` (may be gone).
  The core of it is `createRequire('<lane>/frontend/package.json')('@playwright/test').chromium`.
