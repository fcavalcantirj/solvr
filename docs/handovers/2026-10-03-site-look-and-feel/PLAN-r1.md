---
round: 1
builder_session: fresh Opus 5.5 session started in /Users/fcavalcanti/dev/solvr (plan mode; read-only checks only), answering HANDOVER.md for site-look-and-feel
---

# Plan r1 — the v1.3.6 "one sentence" look and feel for the rest of solvr.dev

> **Context.** This file is the `/handover build` deliverable. Lane S (v1.3.6, live) gave /connect, the guides and
> the home use-case cards a new visual language. Every other page still uses the old one. HANDOVER.md asks for a plan
> to migrate the rest, family by family, mock first, with Felipe approving each look. On ExitPlanMode approval the only
> action is to copy this file to `docs/handovers/2026-10-03-site-look-and-feel/PLAN-r1.md`. Nothing executes until the
> author's REVIEW says APPROVED.

## Blocking constraints, restated
1. **Nothing leaves this machine.** No push, tag, deploy or release. No production writes: never call
   `api.solvr.dev/admin`, never use values from `.env`. Public GETs of solvr.dev and api.solvr.dev for "before"
   screenshots are fine.
2. **All work happens in a new worktree:** `~/dev/solvr-lanes/lane-v`, branch `lane/v-look-and-feel`, cut from main
   `34c0f8e0`.
   - It gets its own DBs on the Docker Postgres at `localhost:5435` and its own ports (API `8710`, web `3710`).
   - `/Users/fcavalcanti/dev/solvr` is read-only for me: no edits, builds, tests or commits there.
   - This session stays rooted there only so the Impeccable plugin is enabled; all work uses absolute paths into the
     lane.
3. **Every family is mocked first, then I stop.**
   - Build the restyle and serve it locally.
   - Send Felipe the URL plus 1440 and 390 screenshots.
   - Then STOP until he writes "design approved" for that family.
   - Tests and commits for a family come only after that approval.
4. **The client stays dumb.** Restyling may not change what a page says.
   - No new copy, numbers or labels.
   - No new client arithmetic, sorting or slicing.
   - Removing words (an eyebrow's text, a section) is a copy change, so it is Felipe's call. Each mock lists the
     words it removes.
5. **`frontend/design-system.test.tsx` is law.**
   - Chroma is allowed only on `destructive*`, `chart-N` and `prompt-accent*`.
   - `--radius` is `0rem`. A corner utility is allowed only in sizes none/sm/md/lg/full; bare `rounded`, `xs`, `xl`
     and up fail.
   - The `SOLVR_` wordmark string is pinned in header and footer.
   - The rhythm `px-4 sm:px-6 lg:px-12 py-12 lg:py-16` is pinned in a 10-file list, and also in
     `overview-sections.test.tsx:285`.
   - Changing any pinned rule means naming the test and its replacement, red first. A test is never loosened
     silently.
6. **Test output.**
   - Every run goes to a log, and I read only `tail -n 10`.
   - On a failure I read only the grep of failing lines from that same log.
   - I never re-run a suite to see more.
   - Full suites and builds go through `/tmp/solvr-heavy.sh`.
7. **No loop runners and few agents.** I never execute `ralph*.sh`. At most 2–3 sub-agents run at once, and an
   Impeccable `critique` counts as 2, so nothing else runs beside it.
8. **Shell traps.**
   - The shell exports `PORT=3000`. Every server starts under `env -i HOME="$HOME" PATH="$PATH"` with explicit
     ports, never :3000.
   - I kill only PIDs I started, recorded in pid files.
   - `NEXT_PUBLIC_API_URL` defaults to production. I set it to `http://localhost:8710` for `next dev` and at build
     time for `next start`.

## Handover discrepancies
I checked the claims below on 2026-10-03 with read-only commands. Where a claim is not mentioned, it matched.

**Matched:**
- `main` is `34c0f8e0`, with `3360cf54` and `74dcc56a` in the log.
- `GET /v1/connect` returns `instruction_version` `2.0`, with `prompt` keys `text`, `segments` and `word_count`, and
  53 words. `/v1/connect/examples` answers 200.
- All 10 reference files exist.
- Every vocabulary class string was found in source, including the side column `lg:sticky lg:top-28 lg:col-span-4
  lg:border-l lg:border-border lg:pl-10` at `components/prompt/prompt.tsx:102`.
- The accent tokens are at `globals.css:40/78`, and the `.prompt-*` classes at `:150-176`.
- Header `h-16`; Footer `full|compact`.
- `/data` is 463 lines, a client page with no Footer, an eyebrow `STATISTICS`, Recharts `2.15.4` and
  `radius={[0, 4, 4, 0]}` at `page.tsx:439`.
- `max-w-7xl`: 108 uses in 59 files.
- Impeccable 4.5.0 is installed with scope project, and `.claude/` is gitignored at `.gitignore:10`. The skill list
  of this session shows `impeccable:impeccable`.
- `/tmp/solvr-heavy.sh` exists, and Postgres answers on `:5435`.

**Discrepancies:**
1. **Eyebrow count.** The handover says ~47 in 34 files. What I measured depends on the pattern:
   - 57 lines in 25 files carry `font-mono` + `text-xs` + `tracking-[0.3em]`;
   - 47 files carry `tracking-[0.3em]` + `font-mono` anywhere;
   - 99 lines use 10px `tracking-[0.2/0.3em]` spans.

   This is not material. The design-system scan below makes it mechanical instead.
2. **Footer.** The handover says "about 15 pages render no Footer". I measured 27 of 48 routes. Only 21 render
   `<Footer`; blog and blog/[slug] do it through `blog-page-client.tsx` and `blog-post-content.tsx`. `/notifications`
   does render one.
3. **Rooms contrast test scope.** It scans only `components/rooms` (`design-system.test.tsx:364`). The `bg-green-500`
   pulse dot on /data (`page.tsx:220`) and `/status`'s 13 status-colour uses are outside it. So "status colours meet
   the rooms contrast test" needs the scan extended (see item 7).
4. **/data already breaks rule 3, before any work.**
   - The client computes the percentages (`page.tsx:173-178`), slices the top 10 (`:369`) and formats "Updated Xm
     ago" (`:79-85`).
   - The live `GET /v1/data/breakdown?window=7d` returns raw counts only: `by_searcher_type`, `total_searches`,
     `zero_result_rate`.
   - `categories` is fetched but never rendered, and `zeroResultPct` is computed but unused.
5. **`gap-px` trap.** `components/homepage/metric.tsx:7-9` explains that MetricGrid deliberately uses cell borders,
   not `gap-px bg-border`. With a partly filled last row, the gap version paints the empty cells in the border
   colour. This collides with item 3's idiom (see Concerns).
6. **`SectionHeading` is shared.** It sits in the 4 /data sections and also in 3 home sections (room-activity,
   room-previews, reusable-posts). Retiring it touches the home, which is open question 3.
7. **Missing routes.** The inventory leaves out `/connect/agent`, a redirect spinner with no visual work,
   `app/global-error.tsx`, and `/docs/guides/*` (already new). `/admin/system` is the only admin page.
8. **The lane S harness is orphaned.** `~/dev/solvr-lanes/` is empty, so the `createRequire` path in
   `/tmp/solvr-lane-s/pages.mjs` points at a deleted worktree. The lane S PNGs still exist.
9. **The file-size CI is already red.** `app/privacy/page.tsx` is 1075 lines and `lib/api-types.ts` 1623 (memory
   says 27 violations). Also, `scripts/check-file-size.sh` uses `find … -name "*.ts" -o -name "*.tsx" -type f
   -print0`: without parentheses only `.tsx` files print, so `.ts` violations go unreported.
10. **A reference page carries an old mark.** The new guides index still uses a mono `tracking-[0.3em]` h2,
    "WHERE TO GO NEXT" (`app/docs/guides/page.tsx`). It is out of scope unless Felipe says otherwise.

## Acceptance checklist — point-by-point
1. **The 8 blocking constraints are restated** in my own words above, before any step.
2. **Verified route inventory:** see "Route inventory" below. It covers every family with file paths and line counts,
   with the current and intended look in one line each. The discrepancies are reported above.
3. **Shared primitives come first.** They go in a new `frontend/components/page/`:
   - `page-container.tsx` holds `PageContainer`;
   - `page-header.tsx` holds `PageHeader` (variants `index | quiet`, h1 plus one line, no eyebrow);
   - `section-header.tsx` holds `SectionHeader`, which replaces `SectionHeading` and retires it once the home is
     decided;
   - `stat-grid.tsx` holds `StatGrid` and `StatCell`;
   - `content-with-aside.tsx` holds `ContentWithAside`;
   - `caption.tsx` holds `Caption` (mono 11px `tracking-[0.18em]`, captions and links only).

   Details are under "Primitives".
4. **Mock first, starting with /data.** The /data mock is served at **http://localhost:3710/data** and produces:
   - `/tmp/solvr-lane-v/data-mock-desktop.png` (1440×900, full page);
   - `/tmp/solvr-lane-v/data-mock-mobile.png` (390×844, full page);
   - "before" shots of prod: `/tmp/solvr-lane-v/data-r0-prod-{desktop,mobile}.png`;
   - state shots, per item 7.

   Then I STOP for "design approved". Each later family repeats mock, STOP, approval, then wiring.
5. **Numbers and labels stay the API's.** I add no new arithmetic, sorting, slicing or copy. The existing /data
   client arithmetic (discrepancy 4) stays byte-identical: I flag it and do not "fix" it, because fixing it means an
   API change, which is Felipe's call (item 11). The source scan in `platform-statistics.test.tsx` stays green. The
   new classes contain no 3-digit numbers, and there is no `toFixed` or `sort`.
6. **Tests to re-point:** see "Tests" below. That section covers the 16 listed suites plus the 20 more suites I
   found that pin classes or uppercase copy. Each one is reported as
   `<file> › <describe> › <it>: <old assertion> → <new assertion> (why)`.
7. **States plan:** see "States". For every family I screenshot loaded, empty, loading and error, plus the live and
   presence states, at 1440 and 390.
   - `design-system.test.tsx`'s status-colour scan is extended from `components/rooms` to every migrated family's
     files. This change is named, and it tightens the test rather than loosening it.
   - Every status dot I touch uses a shade that already passes, such as `bg-green-700 dark:bg-green-400` from
     `room-stats-section.tsx:63`.
8. **Accent: not used on data pages.**
   - `--prompt-accent` appears only where a prompt sentence already renders, as a fill behind ink (the room connect
     panels, which are untouched).
   - No new chromatic token appears, so `CHROMATIC_TOKENS` (`design-system.test.tsx:115`) is unchanged. If Felipe
     ever asks for one, that regex change is named in the commit and the report.
9. **Verification:** see "Verification":
   - focused tests per family;
   - at the end, the full frontend suite, `next build` with `NEXT_PUBLIC_API_URL`, and typecheck;
   - lint before and after (the error count may not rise above 22);
   - `check-file-size.sh` plus `wc -l` on the touched files;
   - a smoke of every touched page at 1440 and 390.
10. **Limits.** At most 2–3 sub-agents at a time, and no other agent runs during `critique`. Impeccable runs once
    per family in a fixed order: `critique` (inspect), one fix batch, `polish`, then `detect --json` to confirm.
    `audit` runs once at the end.
11. **No server or API change is planned.** Two places would need one, and both are flagged as Felipe's call, to be
    done API-first (SPEC.md, then tests, then the handler) only if he says yes:
    - `share_label`s for /data;
    - API-served bar lengths, which in-house div bars would need.

## Route inventory (verified 2026-10-03; lines = `wc -l` of page.tsx)

**Current marks.** Old pages use these:
- an eyebrow: mono `text-xs`/`10px`, `tracking-[0.3em]`, above the heading;
- `max-w-7xl`;
- a mono h1;
- shadcn Cards.

**Intended look.** Migrated pages use these:
- `PageContainer` (76rem);
- a quiet `PageHeader` with no eyebrow;
- `SectionHeader`;
- `gap-px` cell grids;
- `Caption` labels;
- square corners;
- monochrome.

| Family | Routes → files | Current look | Intended look |
|---|---|---|---|
| **Data** (first) | `/data` → `app/data/page.tsx` (463), `components/data/platform-statistics.tsx`, `homepage/{room-stats,api-usage,search-stats,community-totals}-section.tsx`, `homepage/metric.tsx`; `/status` → `app/status/page.tsx` (468); `/leaderboard` → `app/leaderboard/page.tsx` (42) + `leaderboard/leaderboard-page-client.tsx` | eyebrow + 5xl light h1, max-w-7xl/5xl, shadcn StatCards, mono numbers, `green-500` pulse dot, Recharts with rounded bars; leaderboard has a mono h1 | Index `PageHeader`, a `SectionHeader` per section, `StatGrid` with Inter 300 numbers and `Caption` labels, hairline row lists, square monochrome charts, a dot of ≥3:1 next to words |
| **Rooms** | `/rooms` → `app/rooms/page.tsx` (78) + `rooms/{rooms-browser,room-list,room-card,recently-viewed-rooms,create-room-dialog}`; `/rooms/[slug]` (142) → `room-detail-client`, `room-header`, `private-room-view`, `room-archive-nav`, `presence-sidebar`, `participants-panel`, `message-list`, `message-bubble`, `comment-input`, `sse-status-badge`, `connection-status-badge`, `connect-agent-panel`, `room-starter-prompts`; `/rooms/[slug]/history/[page]` (171) + `shared/markdown-content` | 10px eyebrow (`room-header.tsx:21`) above a 2xl–5xl h1, max-w-7xl, no Footer | Quiet h1 (the room name) with its purpose as the one line; `ContentWithAside` (messages \| presence + connect panel); status shades stay contrast-tested |
| **Posts** | `/posts` (58) → `posts/posts-page-client`; `/posts/[id]` (117) → `posts/post-detail`; `/posts/[id]/replies/[page]` (148); `/posts/new` (24) → `post-composer`; `/posts/[id]/edit` (28) → `post-editor` | max-w-3xl/2xl, 3xl–4xl light h1, no Footer | `PageContainer` with a `max-w-[44rem]` reading column, quiet h1, hairline lists |
| **App** (agents/users/settings/referrals/pins/dashboard/admin) | `/agents` (42) → `agents/agents-page-client`; `/agents/[id]` (72) → `agents/agent-profile-client`; `/users` (42), `/users/[id]` (73) → `users/*-client`; `/settings` (328), `/settings/agents` (149), `/settings/api-keys` (486) → `settings/settings-layout`; `/referrals` (246); `/pins` (702); `/dashboard` (228) → `agents/agent-briefing`; `/admin/system` (45) → `admin/ipfs-status` | mono h1 (`settings-layout.tsx:65`), max-w-7xl, mostly no Footer | Quiet sans h1, `StatGrid` for counts, hairline cell lists. How deep the treatment goes is Felipe's call |
| **Marketing** | `/skill` → `skill/{skill-hero,skill-install,skill-preview}`; `/mcp` → `mcp/{mcp-hero,mcp-tools,mcp-setup}`; `/api-docs` → `api/{api-hero,api-quickstart,api-endpoints,api-sdks,api-mcp,api-rate-limits,api-cta}`; `/amcp` → `amcp/{amcp-hero,amcp-features,amcp-recovery}`; `/ipfs` → `ipfs/{ipfs-hero,ipfs-features,ipfs-api}`; `/how-it-works` → `how/{how-hero…how-cta}` (8) | `py-20 lg:py-32` hero, 10px span eyebrow, 6xl light h1, an eyebrow in every section | Index `PageHeader`, then the thing itself set big (the install line or snippet), `SectionHeader`s, `gap-px` feature grids |
| **Docs** | `/docs` (343), `/docs/protocol` (422) | 5 eyebrows; label column first (`docs/page.tsx:183` col-span-4 before col-span-8) | Index header; `ContentWithAside` with content first and the quiet side column on the right |
| **About / legal** | `/about` (741, 9 eyebrows), `/terms` (713), `/privacy` (**1075**, already over the 800 cap) | eyebrows, label-first columns | Quiet header plus a reading column; legal pages are typographic only |
| **Auth** | `/login` (276), `/join` (499), `/claim` (296), `/auth/callback` (155), `/email/unsubscribe` (85); `/connect/agent` (27) is a redirect spinner with no work | mono h1, centred cards | Quiet sans h1, one ~28rem form column, a single ink primary button. Depth is Felipe's call |
| **Blog** | `/blog` (40) → `blog/blog-page-client`; `/blog/[slug]` (107) → `blog-post-content.tsx`; `/blog/create` (355) | mixed old | Index header, reading column |
| **Other** | `/notifications` (36) → `notifications/notifications-inbox`; `/zh/promote` (256); `app/not-found.tsx`; `app/global-error.tsx` | mixed old | Quiet header |
| **Home rest** (open question 3) | `hero-section.tsx`, `live-overview.tsx`, `homepage/{room-activity,room-previews,reusable-posts,closing}-section.tsx` (via `SectionHeading`) | old | Only if Felipe includes it; then `SectionHeading` is deleted |

**Recommended order** (THE ORDER IS FELIPE'S CALL):
1. Data: `/data` as the pilot, then `/status` and `/leaderboard`
2. Marketing
3. Docs
4. About/legal
5. Rooms
6. Posts
7. Blog
8. App
9. Auth
10. Other

## Primitives (`frontend/components/page/`, TDD after the /data approval)
- **`PageContainer`.** `mx-auto w-full max-w-[76rem] px-4 sm:px-6 lg:px-12`. It takes `as` and `className`, and
  `bare` for sections that already carry the pinned rhythm. Those use `mx-auto w-full max-w-[76rem]` only, so the
  10-file rhythm pin stays intact.
- **`PageHeader({ title, lede?, variant })`.**
  - `index` variant: `text-[2rem] font-light leading-[1.15] tracking-[-0.025em] sm:text-[2.5rem]`.
  - `quiet` variant: `text-xl font-normal sm:text-2xl`.
  - The lede: `mt-4 max-w-[44rem] text-base sm:text-lg leading-relaxed text-muted-foreground`.
  - The header renders nothing before the h1.
- **`SectionHeader({ heading, intro?, definition?, id?, as='h2' })`.** The same scale as the guide h2, with no
  eyebrow and `definition` going into `title`. It replaces `SectionHeading` in the 4 /data sections now.
  `SectionHeading` keeps serving the 3 home sections until open question 3 is answered, and is deleted with them.
- **`StatGrid` / `StatCell({ value, label, window?, qualifier?, definition? })`.**
  - The value: Inter 300, `text-[2rem] sm:text-[2.5rem] tabular-nums`.
  - The label and window go in a `Caption`.
  - The cells are `bg-background p-6 sm:p-8`.
  - The look is identical to `grid gap-px border border-border bg-border`. The rules are drawn with the cell-border
    technique (`border-t border-l` on the grid, `border-r border-b` on each cell), because API-driven counts leave
    partial last rows (discrepancy 5).
  - `MetricGrid` becomes a thin wrapper that keeps `data-testid="overview-metric"`.
- **`ContentWithAside({ children, aside })`.** `lg:grid lg:grid-cols-12 lg:gap-x-12`, with the content in
  `lg:col-span-8` and the aside as at `prompt.tsx:102`. `prompt.tsx` itself is not refactored, to keep lane S code
  out of scope.
- **`Caption`.** `font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground`. It renders as `<p>` or
  `<span>`, never directly above a heading.
- **Test:** `components/page/page-primitives.test.tsx`. It checks the heading levels and that no element comes before
  the h1/h2 inside the header. It checks that StatGrid renders N `dt`/`dd` pairs whose text equals the props, the
  container class string, and the order of the aside.

## The /data mock (family 1, round 1)
- **Header.** `PageHeader variant="index"` with "Solvr statistics" and the existing lede, verbatim. The lede is two
  sentences, and trimming it is Felipe's call.
- **Words removed (listed for Felipe):**
  - the `STATISTICS` eyebrow;
  - the `LIVE SEARCH ACTIVITY` eyebrow, which repeats the h2;
  - the section eyebrows `LIVE`, `API`, `SEARCH` and `TOTALS`.
- **The 4 sections.** Each gets a `SectionHeader` (API heading and intro), `StatGrid` replacing MetricGrid's
  internals, a hairline restyle of `CompactTable`, and square spark bars whose heights still come from the API. The
  rhythm string stays on each `<section>`.
- **Live search.**
  - h2 "Live Search Activity", with the pulse dot moved beside it. The word "Live" is already in it. The dot is
    `bg-green-700 dark:bg-green-400`.
  - The 1h/24h/7d control becomes a square segmented control (shadcn Tabs restyled, so the test mock still fits).
    The Switch and its label stay.
  - A 4-cell `StatGrid` holds the same labels and the same `% of total` strings.
  - `ContentWithAside`: the top-queries list (hairline rows) in the content, the charts in the aside.
  - "Updated Xm ago" becomes a `Caption`.
- **Charts (open question 1; default for the mock: keep Recharts, restyled).**
  - `radius={0}`, the existing monochrome `CHART_COLORS`, mono 11px ticks, no rounded caps.
  - The pie is kept and restyled, because removing it is Felipe's call.
  - In-house div bars would need API-served lengths (item 11), so they are not mocked.
- **Footer.** None, unchanged, until Felipe answers open question 2.

## States (every family)

| State | How it is produced |
|---|---|
| Loaded | Local API on `solvr_lane_v_data` |
| Empty | Playwright `page.route` rewrites `:8710` to a second API on `:8711`, backed by `solvr_lane_v` (migrated, empty), so the API's own empty notes render |
| Loading | The route is held and never fulfilled |
| Error | The route is fulfilled with 500, or aborted |
| Live and presence | /data dot online and offline (fixture `live_marker.online` both ways via route); rooms SSE connected and reconnecting, presence dots |

- Every state is shot at 1440 and 390 as `/tmp/solvr-lane-v/<family>-<state>-{desktop,mobile}.png`.
- Server-rendered families (rooms, posts) get their empty state from restarting `next dev` against `:8711`.

## Execution sequence (only after APPROVED)
1. **`next_action`, part 1.**
   - `git -C /Users/fcavalcanti/dev/solvr worktree add -b lane/v-look-and-feel ~/dev/solvr-lanes/lane-v main`
   - `cd ~/dev/solvr-lanes/lane-v/frontend && npm ci > /tmp/solvr-lane-v/npm-ci.log 2>&1; tail -n 10 …`
   - `npm run lint > /tmp/solvr-lane-v/lint-before.log 2>&1; tail -n 3 …`. I expect `22 errors, 172 warnings`.
2. **DBs.**
   - `createdb -h localhost -p 5435 -U solvr solvr_lane_v`, then `migrate … up`.
   - `solvr_lane_v_data` is restored from `db-backups/window/v136-pre-20261003-225310.dump` with
     `pg_restore --no-owner` (question 2).
   - API: `go build -o /tmp/solvr-lane-v/api ./cmd/api`. I start two instances (:8710 data, :8711 empty) with the
     handover's `env -i` recipe, a 32+ char local-only `JWT_SECRET`, and `ALLOWED_ORIGINS=http://localhost:3710`.
     PIDs go in pid files, and I check the ports are free first.
3. **Web.** `env -i HOME="$HOME" PATH="$PATH" NEXT_PUBLIC_API_URL=http://localhost:8710 npx next dev -p 3710 >
   /tmp/solvr-lane-v/dev.log 2>&1`.
4. **Harness.** `/tmp/solvr-lane-v/shots.mjs`, copied from the lane S `pages.mjs` with `createRequire` repointed at
   the lane-v `frontend/package.json`. It adds a page-list argument, full-page 1440×900 and 390×844 shots, the state
   routes, an overflow check and console errors.
5. **Before shots.** `node shots.mjs https://solvr.dev data-r0-prod /data`, which only reads the public page.
6. **Build and review the mock.**
   - Build the primitives and the /data mock, uncommitted.
   - Run Impeccable: `context --target` once, then `critique` (2 agents), one fix batch, `polish`, and
     `detect --json`.
   - Then the mock shots and the state shots.
7. **STOP.** Send Felipe the URL, the before and after pairs, the state shots, the words removed, and open
   questions 1–3.
8. **After "design approved", TDD:**
   - Write the tests below.
   - `git stash` the mock, run the focused tests, and save `/tmp/solvr-lane-v/data-red.log` (RED).
   - `git stash pop`, run them again, and save `data-green.log` (GREEN).
   - Lint on the touched files, `wc -l`, then one commit: `feat(web): /data in the one-sentence page language`
     plus the Co-Authored-By trailer.
9. **The next family,** mocked and stopped in the same way.
10. **End of lane.** `audit` once, then the full verification, then a report to Felipe and the orchestrator. Merging
    is Felipe's call.

## Tests
**New:**
- `components/page/page-primitives.test.tsx`.
- In `design-system.test.tsx`, a new describe, "the one-sentence page language". It holds a `MIGRATED` file list that
  grows with each family, and asserts that those files contain no `tracking-[0.3em]`, no `text-[10px]` eyebrow and no
  `max-w-7xl`.

**Re-pointed (named in each family report):**
- `design-system.test.tsx`
  - In the "restrained status colour" scan, `ROOM_FILES` becomes `STATUS_FILES`, which adds `app/data`,
    `components/data` and later the families' files. This tightens the test.
  - The `generous spacing` SECTIONS list is unchanged.
- `components/homepage/overview-sections.test.tsx:285`: the rhythm stays. Any class assertion on the section
  headings is re-pointed from the eyebrow to the h2.
- `app/data/page.test.tsx` (the rest of the Data family is listed further down)
  - "is headed as the statistics page": it also asserts that no `STATISTICS` text exists.
  - The stat-label and chart testid tests stay as they are (the labels and Recharts are unchanged).
- `components/data/platform-statistics.test.tsx`: unchanged, and must stay green, including the source scan.
- `components/homepage/room-stats-section.test.tsx:234-248`: the dot classes stay pinned.

**Per family, when that family is touched:**

| Family | Suites |
|---|---|
| Data | `app/status/page.test.tsx`, `app/leaderboard/page.test.tsx` |
| Marketing | `components/api/{api-hero,api-mcp,api-rate-limits}.test.tsx`, `app/api-docs/page.test.tsx` |
| About | `app/about/page.test.tsx` |
| App | `app/agents/page.test.tsx`, `app/agents/[id]/page.test.tsx`, `components/agents/{agents-sidebar,agents-list,__tests__/agent-briefing}.test.tsx`, `app/pins/page.test.tsx`, `components/admin/ipfs-status.test.tsx`, `app/admin/__tests__/system.test.tsx`, `app/settings/agents/page.test.tsx`, `app/settings/__tests__/delete-account.test.tsx`, `app/dashboard/page.test.tsx`, `components/users/contributions-list.test.tsx` |
| Rooms and posts | `components/rooms/{message-bubble,comment-input}.test.tsx`, `components/shared/{markdown-content,comments-list,__tests__/edit-post-form}.test.tsx`, `components/search/search-method-badge.test.tsx` |
| Blog | `app/blog/page.test.tsx`, `app/blog/create/page.test.tsx` |
| Auth | `app/join/page.test.tsx` |
| Other | `app/zh/promote/page.test.tsx` |
| Shared | `components/{header,footer,hero-section}.test.tsx`, `components/ui/user-menu.test.tsx` |

The shared suites are re-pointed only if the header, footer or hero is touched (open questions 2 and 3).

## Verification
- **Per family.**
  - `npx vitest run <the family's suites> > /tmp/solvr-lane-v/<family>-tests.log 2>&1; tail -n 10 …`
  - On a failure, `grep -E -- '--- FAIL|_test\.go:[0-9]+:|Error:|expected|actual|FAIL ' <log> | head -40`.
  - `npx eslint <touched files>`.
  - `wc -l` of every touched file must be ≤800.
- **End of lane (heavy runs under `/tmp/solvr-heavy.sh`).**
  - The full `npm test` goes to `frontend-full.log` and must be 0 failed. The count is 1680 plus the new tests.
  - `npm run typecheck`.
  - `NEXT_PUBLIC_API_URL=http://localhost:8710 npm run build` goes to `build.log`; afterwards,
    `git checkout frontend/public/solvr-skill.zip`.
  - `npm run lint` goes to `lint-after.log`, with errors ≤22.
  - `scripts/check-file-size.sh`: the list of violations must be no longer than main's. No touched file may be in it,
    and `privacy` stays pre-existing unless the legal family splits it.
- **Backend.** `git diff --stat main -- backend` must be empty, so the backend baseline is unchanged by construction.
  If an approved API change ever lands, the full backend runs under heavy and must show only the 4 baseline failures.
- **Smoke.** `next start -p 3710` on the build, with the API on :8710. `shots.mjs` covers every touched route at 1440
  and 390, with no overflow and no console errors.

## Concerns
- [HIGH] **Item 3 says the `gap-px bg-border` idiom, but `metric.tsx:7-9` documents a real defect with partial
  rows.** I propose the visually identical cell-border technique (question 1).
- [HIGH] **/data already does client arithmetic: percentages, top-10 slice, time-ago.** Fixing it properly needs
  `/v1/data/breakdown` to serve labels, which is an API change and Felipe's call. The plan leaves it byte-identical
  and flags it rather than fixing it.
- [MEDIUM] **Removing eyebrows removes words, and tests pin some of them as uppercase text.** Every mock lists the
  removed strings, so Felipe approves them knowingly.
- [MEDIUM] **`SectionHeading` is shared with the home.** Fully retiring it depends on open question 3.
- [MEDIUM] **The populated local data comes from a production dump.** It contains user data, though it stays on this
  machine. Dropping `solvr_lane_v*` at the end of the lane is Felipe's call.
- [MEDIUM] **The scope is about 48 routes in 11 families, each gated by an approval,** so this session's context will
  not last the whole lane. I will write a `/handover` at a family boundary when needed.
- [LOW] **`app/privacy/page.tsx` is already 1075 lines.** Touching legal means a structural split, with the copy
  unchanged.
- [LOW] **Dark mode is not mounted,** so every screenshot is light only.
- [LOW] **/data carries dead code** (`categories` and `zeroResultPct` are unused). It is not removed, because nobody
  asked for that.

## Questions for the author
1. Do you accept `StatGrid` built with the cell-border technique, identical to `gap-px bg-border` at full rows and
   correct at partial rows, as meeting item 3?
2. For the populated mock data, may I restore `v136-pre-20261003-225310.dump` into a local `solvr_lane_v_data`? The
   alternative is an empty migrated DB plus route fixtures, which looks less real.
3. Does "the baseline is unchanged" mean the backend 4-failure baseline? If so, is an empty
   `git diff --stat main -- backend` enough evidence when there is no backend change?
4. Is leaving /data's existing client percentages untouched (flagged) acceptable for item 5, or should the plan
   propose the API-first `share_label` change now, as a Felipe decision?
5. Should the rest of the Data family (`/status`, `/leaderboard`) be a second mock round after /data is wired, as
   planned, rather than mocked together with /data?
