# Fix plan — approved by Felipe on 2026-10-05 (read this first after a compaction)

## SHIPPED 2026-10-05 13:05 BRT as v1.3.15 (Felipe: "ship all, verify all after")

- `main` = `82525366` (12 commits `195c975d..17164ccb`, one per batch, plus the journal entry in
  `progress.txt`). Migrations 000143 and 000142 applied on production before the API; API live 13:01:38,
  web live 13:04:38.
- Verified on production (curl, a 1200-page crawl, a guarded browser, one writing funnel check, Lighthouse
  on the recon's 12 URLs): see the v1.3.15 entry at the end of `progress.txt` and
  `evidence/production-after-v1.3.15/`. Median LCP 5076 → 3285 ms, FCP 3164 → 2067 ms, every page scores
  84–94 (was 69–94), 0 Google requests before consent, all 467 posts linked, Cloudflare passes the
  `skill_fetched` report.
- Follow-ups done the same day: Felipe resubmitted the sitemap in Search Console (17:23 UTC; the API shows it
  pending) and fixed the Cloudflare `www` redirect (~15:10 BRT; verified, see row 17 below). **v1.3.16**
  (`2606821c`, web only): headings in user Markdown start at h2, so every page keeps one h1 (26 production
  pages had more than one); the two throwaway agents `ship_check_*` were deleted. Journal: `progress.txt`.
- Still Felipe's: GA admin settings (`SPEC.md` 27.7); HSTS (optional).

## Earlier status (kept): ALL BATCHES DONE AND VERIFIED ON A LOCAL BUILD (before the ship)

- **The change:** worktree `/Users/fcavalcanti/dev/solvr-lanes/lane-seo`, branch `lane/seo-fixes`, HEAD still
  `d6f41457` (0 commits), 375 paths changed (253 modified, 125 new). The whole thing as one file:
  `patches/after-batch-D1b.patch` (cumulative; each earlier `after-batch-<X>.patch` is the state after that
  batch, so the difference between two consecutive files is what that batch added).
- **Final numbers:** frontend 258 files / 3118 tests pass; typecheck 0; lint and file-size findings identical
  to the base (all pre-existing); backend suite run one package at a time fails only the 4 tests that fail at
  the base. Before/after per finding: `FINDINGS.md` section 13.
- **Deploy needs, in this order (Felipe's):** (1) migrations `000142_skill_fetched_funnel_step` and
  `000143_oauth_login_code_origin` on production (production has no `schema_migrations` table: by hand,
  through the admin query route); (2) the API; (3) the web (an archive page answers 5xx against an old API,
  and the users sitemap needs the new API). Then: resubmit `https://solvr.dev/sitemap.xml` in Search
  Console; GA admin: key events `sign_up`, `prompt_copy`, `room_create`, `agent_claim`, the event-scoped
  custom dimensions listed in `SPEC.md` 27.7, retention 14 months.
- **Preview image:** the OpenAI key in `~/.zshrc` answers 401, so nothing was generated or billed; three
  cards were built in code (`preview-options/`), option 1 is wired as `frontend/public/og/solvr-card-v1.png`.
  Another choice is a file swap under a new name (`-v2`), because platforms cache previews by URL.
- **After deploy, measurable only on production:** Cloudflare passing the web server's `skill_fetched`
  report to the API; how Slack/X/LinkedIn/WhatsApp draw the card; the real LCP gain of the deferred tag.

**GOAL (Felipe, 2026-10-05, before going to sleep): "finish the plan … judge and executioner, finish it".**
Run every batch to the end without asking him. Order: A, F, B, C, D, E. Decide small things myself. For the
preview image, generate the options, pick one, wire it, and show him all of them in the morning.
Still his alone: commit, push, merge, deploy; anything written to production; GA settings; resubmitting the
sitemap in Search Console (excluded from the key on 2026-10-04).

**How I run it:** one implementation agent per batch, one at a time, in the lane; I check its tests and diff
before the next. After each batch: save `patches/after-batch-<X>.patch` here and update the log below.

**Lane:** worktree `/Users/fcavalcanti/dev/solvr-lanes/lane-seo`, branch `lane/seo-fixes` (from `d6f41457`),
`npm ci` done; scratch database `solvr_lane_seo` at schema 141; its `DATABASE_URL` is exported by
`<scratchpad>/lane/lane.env`; local API 18400, web 18401 (not 18763-18769).

### Progress log
- 2026-10-05 Step 0 DONE: new Composio key saved (`apikey.scoped.bak` holds the old one); Search Console and
  GA4 read; `FINDINGS.md` section 12 written. Headline: every organic click is a brand click; Google last
  downloaded the sitemap on 2026-04-06 and does not know `/posts/…`, `/connect` or `/docs`.
- 2026-10-05 Batch A DONE and verified (uncommitted in the lane; `patches/after-batch-A.patch`, 51 files).
  Ten items: room login dialog, metadata in `<head>` (`htmlLimitedBots: /.*/`), homepage and guide copies
  reported (`homepage_use_cases`, `guide_page`), room Share shares the API's `?via=share` link and reports,
  post page counts share visits and views, `/posts` search waits 300 ms, `/notifications` noindex, Edit link
  only for the author, 404 has its own title, dead `/feed` and `/problems` links, `CopyButton` says "Copied"
  only after the write, a failed `/seo` read answers 500 instead of a false noindex. `SPEC.md` 25.7, 27.1,
  27.4 updated.
  Measured: frontend 214 files / 1993 tests pass (baseline 207 / 1882), typecheck and build exit 0, lint and
  file-size identical to baseline. Local stack + guarded browser (`<scratchpad>/lane/check-a.out`): anonymous
  room page has no dialog and no failed request; Share copies `…?via=share` and stores `share_link_copied`;
  `join_prompt_copied`, `starter_prompt_copied` ×2 surfaces, `share_visit` for a post and one post view are
  rows in the scratch database; typing "deadlock" makes 1 search request; title, description and canonical
  are in `<head>` for a Chrome and a Googlebot user agent; 0 requests escaped the guard.
  Residuals, not fixed: (1) the room Share link is read when the pointer or focus reaches the button; a tap
  that lands before the API answered shares after the answer, which a strict browser may refuse (second tap
  works). Clean fix later: `share_url` inside the room detail payload. (2) The 404 page carries two robots
  tags (ours `noindex, follow` + Next's automatic `noindex`); harmless. (3) `user-profile-client.tsx:76,92`
  still link `/feed` (batch D touches that file); room starter-prompt copies are still unreported (batch C).
  Harness lesson: `stack.sh up | tail` hung for 13 minutes because the web server's wrapper shell kept the
  pipe open; `stack.sh` now uses `exec` with full redirection, and checks never start the stack in a pipe.
- 2026-10-05 Batch F DONE and verified (uncommitted; `patches/after-batch-F.patch`, cumulative, 96 files;
  migration `000142_skill_fetched_funnel_step` must be applied at deploy; `instruction_version` is 2.1).
  Measured: frontend 215 files / 2016 tests pass; backend run one package at a time
  (`<scratchpad>/lane/backend-p1.sh`) fails only the 4 known tests (IPFS health, 3 referral-code tests),
  25 packages ok. A plain `go test ./...` fails at random because `internal/api` and `internal/db` share
  one database. Guarded browser on `/connect`: the code stays the same through intent, use-case and
  visibility changes, is in the clipboard and in both browser steps. **Real agents, local stack**
  (`<scratchpad>/lane/check-f.out`): Claude Code 2.1.289 (Sonnet 5.5) and Codex CLI 0.160.0 were each given
  the local sentence; both fetched `/skill.md?f=<code>` and sent the code when creating the room, so each
  flow shows `connection_started`, `starter_prompt_copied`, `skill_fetched` (`agent_fetch`),
  `room_created`, `participant_joined`. 0 requests aimed at production; `~/.config/solvr` restored.
  Not verifiable before deploy: that Cloudflare lets the web server's report through to `api.solvr.dev`.
  Follow-up F2 DONE and verified (`patches/after-batch-F2.patch`, 100 files): link-preview bots and
  crawlers are recorded as `bot_fetch` (31 user-agent tokens, one list) and neither they nor a browser
  visit make a code known; `rel="nofollow noreferrer"` on the sentence's link; every answer for `/skill.md`
  carries `Link: <https://solvr.dev/skill.md>; rel="canonical"`; `GET /v1/connect?flow=none` for sentences
  rendered on the server (the resume guide uses it; a test fails if another server reader forgets);
  a flaky race test's helper fixed (names from the UUID). Frontend 216 files / 2028 tests; backend the 4
  known failures. Unverified: the user agents real agent fetch tools send in production.
- 2026-10-05 Real two-agent runs for the guides DONE: `<scratchpad>/agents/EVIDENCE.txt` (241 lines: three
  complete pairs with room transcripts and timings, plus observations across seven first-agent runs).
  Claude+Claude, Claude(builder)+Codex(reviewer), Codex(planner)+Kimi(executor) each reached a two-way
  exchange 29 to 90 s after the room was created; no request left the machine.
  **Harness incident, disclosed:** in the first Claude+Claude attempt my handoff gave the second agent the
  whole answer instead of the prompt; confused, it looked around and listed `~/.solvr/config` (an old agent
  key file from February) with the key masked by its own `sed`; it did not use it. Six characters of that
  key are in a transcript in the scratchpad and in that session's model context. The guard now also moves
  `~/.solvr` aside, the handoff is fixed, and test agents get a "stay in your directory" rule.
  Product finding from the runs: a planner whose room name matched an existing public room worked in that
  existing room (pinning beside the older directive) instead of a room of its own.
- 2026-10-05 Batch B wiring DONE and verified (`patches/after-batch-B.patch`, 128 files). One helper
  (`frontend/lib/seo/link-preview.ts`, origin in `lib/seo/site.ts`) builds Open Graph and Twitter tags for
  every page; tests fail on a hand-built `openGraph`. Measured on the local build: a crawl of 400 pages
  (374 with 200) has og:title equal to the page's own title, og:url equal to the canonical, an absolute
  og:image and a `summary_large_image` card on all of them; the home title is og:title on `/` only; the
  card answers 200, image/png, 1200×630; a blog post with a cover image uses it. Frontend 217 files / 2138
  tests. Fixed on the way: the private-room gate and 404 pages no longer show the home title in previews.
  Unverified until deploy: how Slack, X, LinkedIn and WhatsApp draw the card.
  Found, passed on: `GET /v1/blog/{slug}` answers 500 when `meta_description` or `cover_image_url` is NULL
  (into batch D2); a 404 under `/docs/guides` keeps that layout's canonical (into batch D1).
  Lighthouse on the local build BEFORE the consent batch (median of 3, mobile, tag loaded, hits blocked):
  home 94 / LCP 2944 ms, connect 76 / 3330, post 94 / 3024, room 93 / 3163; 5 Google requests per page.
- 2026-10-05 Batch C part 1 DONE and verified (`patches/after-batch-C1.patch`, 176 files): consent store
  (`solvr_consent` in localStorage, GPC = declined without asking), the bar ("Solvr would like to use Google
  Analytics to learn which pages help. Nothing goes to Google unless you accept." + Privacy, Decline,
  Accept), "Cookie settings" in footer, mobile menu, account menu and inside `/privacy`; Google's tag only
  after Accept + tracked path + page idle, with consent mode (ads denied), signals and ad personalisation
  off; `lib/google-tag.ts` replaces `@next/third-parties` (measured: `content_group` only reaches a hit as
  a config parameter; a consent update alone does not silence a loaded tag, `ga-disable-<id>` does);
  `track()` / `trackOnNextPage()` with redaction; delegated `nav_click` / `cta_click`; `content_group` on
  every page view; `/privacy` cookie text rewritten (six false sentences removed); `SPEC.md` 27.7.
  Measured in the guarded browser: 0 Google requests before Accept, after Decline, under GPC and on
  untracked paths; after Accept exactly 1 `page_view` per load (13 page kinds) and per client navigation
  (7), each with the NEW page's `content_group`. Frontend 228 files / 2444 tests.
  Lighthouse, local build, median of 3 (before → after, no consent given): Google requests 5 → 0 on every
  page; TBT home 94 → 45 ms, connect 121 → 60, post 95 → 26, room 100 → 48; FCP 150 to 300 ms earlier;
  LCP within noise (home 2944 → 2820, connect 3330 → 3481, post 3024 → 3015, room 3163 → 3020). The LCP
  gain seen on production in the recon needs the deploy to be measured.
  My judgement on the screenshot: at 1280×800 the full-width bar hid the home page's primary button, so
  part 2 turns it into a bottom-right card on wide screens.
  For Felipe, not changed: `/privacy` still says IP addresses are "anonymized after 30 days" while
  `backend/internal/db/metric_retention.go:78` says nothing calls the delete; on leaving a page Google's
  tag sends its last hit twice (google-analytics.com and a cookieless copy to www.google.com/g/collect).
- 2026-10-05 Batch C part 2 DONE and verified (`patches/after-batch-C2.patch`, 241 files; **migration
  `000143_oauth_login_code_origin` must be applied before the API deploy**). 29 event names in one closed
  list (`frontend/lib/analytics.ts`), one helper (`lib/funnel.ts`) sends a first-party funnel step and its
  Google Analytics event together; every Copy block must say what it copies or the type check fails; the
  OAuth code exchange answers `is_new_user` and `provider`, so `sign_up` is exact for GitHub and Google too;
  the consent bar is a bottom-right card from the `lg` breakpoint (bottom-left on `/login` and `/join`,
  where the right side covered the submit buttons). Event table and the owner's GA actions: `SPEC.md` 27.7.
  Measured in the guarded browser, consent accepted: each event fired with its parameters (prompt_copy,
  connect_start, use_case_select, visibility_toggle, intent_set, search, sort_change, filter_change,
  load_more, code_copy, join_prompt_copy, room_share, share_visit, room_create, post_create, blog_vote,
  blog_share, agent_claim, api_key_copy, api_key_create, auth_wall_shown, sign_up, login, logout,
  room_comment, page_not_found); no e-mail, password, key, token, intent or comment in any hit. Consent
  declined, same steps: the same 8 funnel rows, 0 Google requests. At 1280×800, 1366×768 and 1440×900 the
  card does not touch the home page's primary button. Frontend 240 files / 2673 tests; backend the 4 known.
  Not driven in a browser: GitHub/Google sign-in (no provider locally), outcome copy, playground copies.
  Not wired: time-window selectors, profile tabs, referral and pin copies, room notify and pin controls.
  Found, passed on: the blog vote answer lacks the score the page reads (into D2); a late `/posts` answer
  can overwrite a newer list (into D1). Seen: the API playground calls `https://api.solvr.dev` from the
  browser by design.
  **For Felipe in GA admin (property `G-HS74SKKSQY`):** key events `sign_up`, `prompt_copy`, `room_create`,
  `agent_claim`; event-scoped custom dimensions `method`, `surface`, `preset`, `role`, `item`, `location`,
  `list`, `sort`, `page`, `results`, `direction`, `visibility` (`status` can wait); event retention 14
  months (it is 2 months); Google signals can stay on in the property (the code now turns it off per hit).
- 2026-10-05 Batch D part 1 DONE and verified (`patches/after-batch-D1.patch`, 283 files; no migration).
  `/connect` now has, in its server HTML, the h1 "Connect two agents", three steps, one paragraph naming
  Claude Code, Codex, Kimi Code, Hermes and OpenClaw, and links to the guides and rooms (137 words; title
  "Connect two agents in a shared room"); the panel stays the hero. Descriptions and one first-screen
  sentence of `/how-it-works`, `/about`, `/skill`, `/api-docs`, `/blog`, `/docs/guides` now say Solvr
  connects agents. `GET /v1/posts?indexable=true` applies the sitemap's own rule (one predicate) and every
  list answer has `meta.total_pages`; archive pages `/posts/page/{n}` (50 per page, self-canonical,
  pager) are linked from `/posts`; agent and user profiles list their author's indexable posts in the
  server HTML; the docs menu (with `/docs/protocol`) is in the HTML of every page; a 404 carries no
  canonical; a superseded `/posts` answer no longer overwrites a newer list.
  Measured: 139 posts in the scratch post sitemap, each linked from exactly one of 3 archive pages, none
  extra; profiles list 50 links equal to the API's first page; frontend 249 files / 2812 tests; backend
  the 4 known failures.
  **Deploy order: the API before the web** (an archive page answers 5xx when the list has no
  `total_pages`, rather than listing posts the sitemap would not).
  Residuals: archive pages are not in the sitemap (reached by links); `/connect` is served with
  `s-maxage=31536000` (Next's default for a static page; Cloudflare does not cache HTML today).
- 2026-10-05 Batch D part 2 DONE and verified (`patches/after-batch-D2.patch`, 336 files; no migration).
  `GET /v1/agents/{id}/seo` and `GET /v1/users/{id}/seo` decide indexability from ONE rule built on the
  post and room sitemap rules (`db/profile_seo.go`); profile pages render `noindex, follow` when false and
  take title, description and previews from the API ("<name> (AI agent)", "<name> (@username)", real
  counts); the agent sitemap uses the same rule (the stored-reputation rule is gone). A display name that
  contains an e-mail address, or is empty, is served as the username in every public answer (one rule in
  Go and SQL); sign-up and `PATCH /v1/me` refuse an e-mail as name; no public answer carries an agent's
  e-mail. Blog descriptions and excerpts are composed clean at read time (Markdown removed, cut at a
  word); NULL blog columns no longer answer 500; the vote answer carries the new score. Agent JSON-LD is a
  ProfilePage about a Thing; `app/sitemap-users.xml` (never in the index) removed. Measured on the local
  stack: bare agent and bare user `noindex, follow` and in no sitemap; profiles with content indexable;
  a user stored with an e-mail as name shows the username on profile, post and list (row unchanged).
  Frontend 250 files / 2835 tests; backend the 4 known. Passed to batch E: a users sub-sitemap for the
  people who are now indexable. **Production data, Felipe's call:** the two stored rows whose display name
  is an e-mail stay as they are; after this deploy they are no longer shown.
- 2026-10-05 Batch D part 3 DONE and verified (`patches/after-batch-D3.patch`, 347 files). Nine guides,
  all in the server HTML, each with exactly one h1 and a run record (agents and versions, date, build,
  what was observed): the three use cases rewritten as how-tos (512 to 682 words of their own text, real
  room excerpts from the runs), the resume guide corrected and listed, and five per-agent guides: "Make two
  Claude Code agents talk to each other", "Connect Claude Code and Codex", "Connect Kimi Code to another
  agent", "Connect Hermes agents", "Connect OpenClaw agents" (the last two say plainly "not run for this
  guide" and why). The guides index lists them by use case and by agent; `/connect` links each agent name
  to its guide; the five new URLs are in the core sitemap. Every quoted room line was checked verbatim
  against `evidence/real-agent-runs-2026-10-05.txt`; I spot-checked the Claude Code and Hermes pages and
  the one inferred-looking sentence (skill needs a restart) is the agent's own words in its transcript.
  Frontend 251 files / 2925 tests; `TestGuide_*` 4 of 4.
- 2026-10-05 Batch E DONE and verified (`patches/after-batch-E.patch` = the whole plan, cumulative, 374
  files). With the API unreadable every content sitemap answers 503 with `Retry-After: 120` (web and API),
  collection and detail pages answer a retryable 500 (Next 15 has no 503 helper), a real empty list stays
  200; the users sub-sitemap is back and lists exactly the people whose `/seo` says indexable; all 31
  indexable routes have exactly one h1 (a test renders them all; `/status` got one); `/docs` links all nine
  guides; `CLAUDE.md` "Deployment Constraints" and `SPEC.md` 27.1 describe the sitemaps as they are; the
  SPEC no longer says tokens live in httpOnly cookies; `scripts/check-file-size.sh` now checks `.ts` too
  (it finds `frontend/lib/api-types.ts`, already 1623 lines at the base; no file crossed 800 lines because
  of this plan). Frontend 255 files / 3086 tests; backend the 4 known failures (a second flaky test with
  clock-built ids fixed: 81 of 300 runs failed before, 0 after).
- 2026-10-05 Final verification on the finished build (`<scratchpad>/lane/final-verify.sh`, outputs in
  `<scratchpad>/lane/final/`; my own instance 18420/18421, database `solvr_lane_seo_run`): local crawl of
  128 pages (60 sitemap URLs, 97 indexable): 0 listed posts without an inbound link, 0 indexable pages
  without og:image, 0 pages besides `/` with the home og:title, 0 with metadata in `<body>`, 0 blog
  descriptions with Markdown, 0 sitemap URLs that are not 200 and indexable, exactly one h1 on every
  indexable page; the only 404 links are production-only entities (footer credits, the three example
  rooms, which answer 200 on production). The repo's `seo-verify.mjs` `sample`, `structured` and room
  `crawl` pass. Batch A browser regression clean (no dialog, Share/join/copies/share visit/view report,
  1 search per typed word, 0 escaped). One visit to `/connect` is still one flow.
  **Regression found by the final Lighthouse and fixed (batch D1b):** D1's server text under the panel
  raised `/connect` CLS from 0.302 to 0.461. D1b renders the default sentence in the server HTML
  (`GET /v1/connect?flow=none`, revalidated every 5 minutes; the browser read still mints the visit's
  flow and replaces it): CLS 0.187, LCP 3442 → 2788 ms, score 71 → 87, and the full 53-word sentence is
  now in what crawlers read. D1b follow-up: while the code is pending, an empty 11ch placeholder holds
  the width `?f=<8 chars>` will take, so nothing re-wraps: `/connect` CLS 0 in all 3 runs, LCP 2786 ms,
  score 96 (76 before the plan). Frontend 258 files / 3118 tests. Final patch `patches/after-batch-D1b.patch`.
  **Disclosure:** batch A's agent ran a plain `npm run build` at 01:42, which prerenders against the
  production API: a few public GETs (examples, overview) reached `api.solvr.dev`. No funnel, search or
  view row. Rules now force the build onto the local API.
  Stack instances: `STACK_INSTANCE=lead` = ports 18420/18421, database `solvr_lane_seo_run`, served from a
  copy of the build (keeps running while agents edit and rebuild); default instance = 18400/18401 for agents.
  Real two-agent runs for the guides: `<scratchpad>/lane/run-pairs.sh` → `pairs.out`, transcripts in
  `<scratchpad>/agents/p*-*/` (Claude+Claude, Claude+Codex, Codex+Kimi). Hermes here is not logged in
  to any model provider and OpenClaw is not installed: both guides must say "not run".
  Design decisions of batch F beyond the plan, taken from the code: the flow id itself is the code (8 characters, `^[a-hjkmnp-z2-9]{8}$`); `GET /v1/connect?flow=<code>`
  keeps one flow across the panel's re-reads (today each re-read mints a new id, so `connection_started`
  and `starter_prompt_copied` of one visit often differ); room create keeps `flow_id` only when well formed
  AND already seen in a funnel step; `skill_fetched` is reported by `frontend/middleware.ts` and the API
  marks it `agent_fetch` or `browser_visit` (the skill link in the sentence is clickable); report gains
  `website_flow_steps`. Known gap, by design: the home cards and the guides show the static example
  sentences (no flow, no code). Measured: production serves `/skill.md` with `cf-cache-status: DYNAMIC`,
  so the request reaches the origin.
- **Working files for the remaining batches** (they survive a compaction): briefs in
  `<scratchpad>/briefs/` (`RULES.txt`, `batch-B.txt`, `batch-C1.txt`, `batch-C2.txt`, `batch-D1.txt`,
  `batch-D2.txt`, `batch-E.txt`; D3 = the guides, written after the real-agent runs). Launch pattern: one
  `general-purpose` agent, prompt = "read RULES.txt and batch-X.txt and execute". Helpers in
  `<scratchpad>/lane/`: `stack.sh`, `seed.sh` (fixtures + blog row + 15 paths), `save-patch.sh <X>`,
  `proxy.sh` + `proxy.mjs` (18402 web, 18403 API: production origins rewritten to local, `/install.sh`
  made local) and `agent-run.sh <claude|codex> <run> <prompt-file>` for real agents against the local
  stack. Headless modes found: `claude -p` (2.1.289), `codex exec` (0.160.0), `kimi -p` (0.42.0),
  `hermes -z` (0.16.0); OpenClaw is not installed. Before any real-agent run: move `~/.config/solvr` aside
  and restore it after (it holds the owner's real agent key), snapshot `~/.claude/skills/solvr`.
- Batch B, image: **the OpenAI key is invalid** (measured: `OPENAI_API_KEY` from `~/.zshrc` gets HTTP 401 on
  `GET /v1/models`; Impeccable's docs name that variable as the only way in). Nothing was rendered, nothing
  billed. I did not look for other keys. Fallback done: three cards built in code with the site's own fonts
  and tokens, in `preview-options/` (sources and the four model prompts in `preview-options/source/`; with a
  valid key: `bash preview-options/source/render.sh` after fixing its paths, or re-run from the scratchpad
  `og/render.sh`). My pick: option 1 (the home page's headline: the only one whose text survives at feed
  thumbnail size, and it matches the page the link opens). Wiring: not started (waits for batch F).
- Batch C: not started.
- Batch D: not started.
- Batch E: not started.

**Google access (verified read-only):** Search Console `sc-domain:solvr.dev` and GA4 `properties/523300499`
("solvr-web", stream `G-HS74SKKSQY`), each through its own Composio connection (connection and user ids are
kept with the key in `~/.config/composio` and the read-only helper `harness/google_read.py`, not in this
file). GA tools need `"version": "20260924_00"` in the execute body. Read-only tools only.

**Felipe's working style seen here:** he reacts to concrete proposals inside a plan, not to abstract
questions ("why are we talking about this now?", "what?????"). Decide small things; put choices in the plan
with a recommendation.

---

# Solvr: fix what the SEO recon found, measure everything, read Google's data

## Context

The recon report (`docs/handovers/2026-10-04-seo-360-recon/FINDINGS.md`) found that solvr.dev is not findable
for "connect two agents", that public room pages open behind a login dialog, that 95% of posts have no
internal link, that no page has a preview image, and that the main conversion is not measured. On 2026-10-05
Felipe answered the open decisions and gave a Composio key that can execute tools (verified read-only: Search
Console has the domain property `sc-domain:solvr.dev`; GA4 property `properties/523300499` holds
`G-HS74SKKSQY`). This plan turns section 8 of the report into work, in batches he can ship one at a time.

## Decisions (Felipe, 2026-10-05)

| Topic | Decision |
|---|---|
| Preview image | Generate with Impeccable's image generation (OpenAI key); show him before wiring |
| GA events | Everything: searches, clicks, copies, sign-ups, votes, shares, navigation |
| Google tag loading | Delegated: load only after consent, when the page is idle |
| Consent | One-time banner for **everyone**; GA loads only after Accept |
| Empty profiles | Hide the empty ones from search; keep and enrich the ones with content |
| Guides | May name agents: Claude Code, Codex, Kimi Code, OpenClaw, Hermes. No more questions on this: I write them, run the agents installed here against a local stack, and each guide says which agents were run |
| Cloudflare `www` redirect and HSTS | Deferred; Felipe fixed the `www` redirect on 2026-10-05 (verified); HSTS still open (optional) |
| Website visit to room (report row 10) | "Love it. Spec and do it properly": batch F |
| Composio | The new key replaces the one in `~/.config/composio/apikey` |

## How the work is run

- **Isolation:** worktree `/Users/fcavalcanti/dev/solvr-lanes/lane-seo`, branch `lane/seo-fixes` from
  `origin/main` (`d6f41457`); scratch database `solvr_lane_seo` in the `solvr-postgres` container (5435);
  local API on 18400, web on 18401. The main tree is not touched.
- **TDD** (repo rule 1): a failing test first for every change. Test output goes to a log; I read
  `tail -n 10`, and grep failures from the saved log (rule 8).
- **No commit, merge or deploy without Felipe's word.** After each batch I show what changed and how it was
  verified; he says commit, and separately deploy.
- **API-first** (rules 3 and 4): anything that decides eligibility or composes text lives in the API, is
  written into `SPEC.md` first, and gets an OpenAPI entry.
- No `ralph*.sh`. No subagents beyond the two read-only code maps already used for this plan.
- Loops and multi-step commands run as `bash` scripts (zsh traps).
- **Context:** this session is at 84%. Order of execution here: Step 0 (key and Google data), then batch A,
  then batch F. Before starting batch B, I stop and say so: the remaining batches are better run from a fresh session
  with `/handover`, which Felipe starts if he wants it. This plan and `FINDINGS.md` carry everything a new
  session needs.
- Two read-only code maps (frontend interaction points and metadata; backend funnel validation and profile
  data) were requested for this plan and had not returned when it was written. File lists below are from my
  own reading; the maps refine them at the start of each batch.

## Step 0 — Key and Google data (no code)

1. Replace `~/.config/composio/apikey` with the new key (mode 0600; the old scoped key kept beside it as
   `apikey.scoped.bak`). Never print either. Update the memory note (no value).
2. Read Google, **read-only tools only** (never `SEND_EVENTS`, `CREATE_*`, `UPDATE_PROPERTY`,
   `SUBMIT_SITEMAP`, `ADD_SITE`, `DELETE_SITE`). GA calls need `version: "20260924_00"`.
   - Search Console (`sc-domain:solvr.dev`): search analytics by query, page, date and country for the last
     16 months and since 2026-10-02, split brand / non-brand; submitted sitemaps and their status; URL
     inspection of about 12 URLs (one per page type, an orphan post, the room of F01).
   - GA4 (`properties/523300499`): data stream, retention, Google signals, key events, custom dimensions;
     reports for 90 days and since 2026-10-02: channels and sources, landing pages, pages, events by name,
     hostnames, device, country.
3. Add a "Google data" section to `FINDINGS.md`, replacing the "unavailable" rows; raw answers stay in the
   scratchpad, aggregates go to `evidence/`. The 14 Lighthouse page views of 2026-10-05 are called out.

## Batch A — Stop the losses (small, ships first)

| Fix | Change | Test first |
|---|---|---|
| Login dialog on public rooms (F01) | `hooks/use-room-members.ts` takes `enabled`; `components/rooms/presence-sidebar.tsx` passes the signed-in state from `useAuth`; `lib/api.ts` `getMembers` passes `skipAuthEvent: true` | hook makes no request when disabled; a 401 from `getMembers` emits no auth event; sidebar renders for an anonymous viewer without a request |
| Metadata in `<head>` for every user agent (F06) | `frontend/next.config.mjs`: `htmlLimitedBots: /.*/` | `next.config.test.ts` asserts it |
| Homepage copy buttons report (F05) | the homepage use-case section passes `onCopied` to `CopyPromptButton` and reports `starter_prompt_copied` through `api.postFunnelEvent` | component test: one report after a successful copy, none after a blocked clipboard |
| Share button (F05) | `components/rooms/room-header-actions.tsx` shares the API's `share_url` (`GET /v1/rooms/{slug}/share`, carries `?via=share`) and reports `share_link_copied` | component test |
| Share visits on posts (F05) | `useShareVisit` on the post page | hook/page test |
| Post views (F05) | call `useViewTracking` on the post page (`POST /v1/posts/{id}/view` exists) | hook test |
| Index hygiene (F12) | `/notifications` joins `NOINDEX_ROUTES`; the Edit link renders only for the author; `/api-docs` and `/how-it-works` link `/posts`; `app/not-found.tsx` gets its own title | `route-policy.test.ts`, component tests |

Backend only if the code map shows the funnel refuses a homepage surface or a post source: then the accepted
value is added in `backend/internal/models/funnel_event.go`, in the contract, in `SPEC.md` and in the tests.

**Check:** local stack, guarded browser (`harness/local-browser*.mjs` from the recon, pointed at the lane):
an anonymous room page shows no dialog and no 401; the three room actions report; `curl` with Chrome and
Googlebot user agents shows title, description and canonical inside `<head>` on a guide and a transcript.

## Batch B — Link previews (F04)

1. **Image.** Invoke the `impeccable` skill, run its `context` launcher, confirm image generation is
   available with `OPENAI_API_KEY`, and generate three options for one surface: the 1200×630 preview card
   ("Connect your agents. Let them work together.", the `SOLVR_` wordmark, the site's off-white, near-black,
   Inter and JetBrains Mono). Open the three for Felipe. Nothing is wired until he picks. Impeccable may add
   `PRODUCT.md`, `DESIGN.md` and `.impeccable/` to the lane; they are listed for him, not committed silently.
2. **Wiring.** One helper in `frontend/lib/seo/` builds `openGraph` and `twitter` for a page: its own title
   and description, `url` from the canonical, `siteName`, the default image with `alt`, card
   `summary_large_image`. `indexableMetadata()` and the root layout use it, and so does every
   `generateMetadata` that defines its own `openGraph` (post, replies, room, transcript, agent, user, blog
   post, guide).
3. **Test first:** `app/seo-page-metadata.test.ts` and `app/route-metadata.test.ts` assert `og:image`,
   `og:url` and a page-specific `og:title` for every indexable route and every dynamic type.

**Check:** the recon crawler against the local build: `og:image`, `og:url` and `twitter:image` on 100% of
pages; the homepage title appears as `og:title` on the homepage only.

## Batch C — Consent, tag loading, GA events (F05, F09, F10)

1. **Consent, one time, everyone.** A bottom bar (not a dialog: it must not cover content), two equal
   buttons, one sentence, a link to `/privacy`. The choice is stored in `localStorage` and remembered; a
   "Cookie settings" link in the footer reopens it. A browser that sends Global Privacy Control counts as
   Decline. No server involvement.
2. **Tag loading.** `components/site-analytics.tsx` loads the Google tag only when consent is granted, the
   path is not one of the three untracked ones, and the page is idle (`next/script` `lazyOnload`). Config
   turns Google signals and ad personalisation off (`allow_google_signals: false`,
   `allow_ad_personalization_signals: false`). No consent, no tag, no Google request.
3. **Events.** `lib/analytics.ts` becomes one typed `track(event, params)` that does nothing without consent,
   never runs on the untracked paths, and strips emails and Solvr keys from every parameter. Two ways in:
   - explicit calls where something succeeds: `sign_up`, `login`, `logout`, `search` (posts box),
     `prompt_copy`, `connect_start`, `use_case_select`, `room_share`, `join_prompt_copy`, `post_vote`,
     `post_bookmark`, `reply_submit`, `post_create`, `agent_claim`, `api_key_create`, `code_copy`,
     `sort_change`, `load_more`, `page_not_found`, `auth_wall_shown`;
   - one delegated click listener for elements marked `data-track` (header, footer, mobile and docs menus,
     calls to action): `nav_click` and `cta_click` with the item and its location.
   Every page view also carries `content_group` (home, post, room, transcript, agent, user, blog, docs,
   guide, collection, account). Google's automatic events stay (scroll, outbound click, download, form).
   Where a funnel step exists for the same action, one helper sends both.
4. **Docs.** `SPEC.md` gains "27.7 Analytics events and consent" with the event table; `/privacy` names
   Google Analytics (after consent), Cloudflare Web Analytics and the first-party funnel.
5. **GA admin.** I hand Felipe the list of key events and custom dimensions to register (or register the
   custom dimensions through Composio if he says so); nothing is written to GA without his word.

**Test first:** consent store and banner; the tag is absent before Accept and present after; `track()`
no-ops without consent and redacts; one test per wired event.
**Check:** guarded local browser: zero Google requests before Accept; after Accept one `page_view` per load
and per navigation, each event above seen once with its parameters, no hit on the untracked paths.
Lighthouse on the local build before and after, for the first-paint numbers.

## Batch D — Findability (F02, F03, F07, F08)

1. **`/connect`:** server-rendered heading and text that say what the page does; the interactive panel stays.
2. **Headings** of `/how-it-works`, `/about`, `/skill`, `/api-docs`, `/blog` aligned with "connect your
   agents".
3. **Guides.** `SPEC.md` 27.5 updated first: guides may name agents; each states which agents were run, when
   and at which commit; an agent that was not run is named as untested. Then the four guides become full
   how-tos, plus one per agent with distinct setup (Claude Code, Codex, Kimi Code, Hermes, OpenClaw). I run
   the installed agents against the local stack with the lane L recipe; transcripts go into the guides.
   Claude Code and Codex also link the existing public rooms of 2026-10-03. The resume guide is listed.
4. **Posts reachable by links.** Archive pages `/posts/page/{n}` (server-rendered, self-canonical, linked
   from `/posts` and from each other; `GET /v1/posts?page=&per_page=50`); an author's posts listed on agent
   and user profiles; `/docs/protocol` linked in plain HTML. `SPEC.md` 27.2 updated first.
5. **Profiles.** `SPEC.md` 27.1 first, then the API decides: `GET /v1/agents/{id}/seo` and
   `GET /v1/users/{id}/seo` answer `indexable` (true only with public posts, replies or rooms), title and
   description, following the existing post and room endpoints. Pages render `noindex, follow` when false.
   The orphan `app/sitemap-users.xml` route is removed. Agent JSON-LD stops claiming `SoftwareApplication`.
   The API stops serving an email address as a display name.
6. **Blog descriptions:** composed by the API with the function post descriptions already use (Markdown
   removed, cut at a word); `BlogPosting` gains `author`.

**Check:** the recon crawler and `analyze.mjs` on the local build: no listed post without an inbound link;
empty profiles `noindex`; `seo-verify.mjs sample` and `structured` pass.

## Batch E — Robustness and docs (F13, F14)

- Content sitemaps and collection pages answer 503 when the API cannot be read (test with the local API
  stopped, as in the recon).
- `CLAUDE.md` "Deployment Constraints" and `SPEC.md` 27.1 describe the sitemap index and the route list as
  they are.

## Batch F — A website visit can be tied to the room it produced (report row 10)

Today the site records "prompt copied" and the server records "room created", and nothing joins them:
53 website flows, 0 attributed rooms. The fix keeps the sentence slim: it gains no word.

**Design, written into `SPEC.md` first (connect contract and funnel):**
- The flow id that `GET /v1/connect` already mints (`handlers/connect.go:185`) gets a short public code. The
  sentence's skill link becomes `https://solvr.dev/skill.md?f=<code>` (`handlers/connect_slim.go:27,163,226`).
  Nothing else in the sentence changes.
- `skill/SKILL.md` gains one rule beside the existing `source_room` rule (line 90): when the skill link you
  were given carries `?f=<code>`, send `"flow_id": "<code>"` when you create the room.
  `scripts/sync-skill.sh` publishes it.
- `POST /v1/rooms` already accepts `flow_id` and `room_created` already stores it. The API validates the code
  and ignores an unknown or malformed one; a room is never refused because of it.
- New server step `skill_fetched`: `frontend/middleware.ts` reports a request for `/skill.md?f=<code>` to the
  API without delaying the response; the API validates and records it. Skill reads get counted for the
  first time.
- Reports: `flow_to_room.website` in `GET /admin/activation-analytics` becomes a real rate, shown beside the
  rooms that carried no code; the baseline funnel lists `skill_fetched`.

**Test first:** connect contract (the link carries the code, the rest of the sentence is byte-equal); funnel
contract and ingest; room create with a valid, an unknown and a malformed code; the middleware; the guide
tests that follow the served sentence literally (`TestGuide_*`).

**Check:** local stack; Claude Code and Codex each given the sentence from the local `/connect`; the scratch
database shows one flow with `connection_started`, `starter_prompt_copied`, `skill_fetched`, `room_created`
and the activation steps. An agent may ignore the rule, so the report always shows attributed and
unattributed rooms.

## Not in this plan

- Report row 17 (Cloudflare `www` redirect and HSTS): the `www` redirect is DONE. Felipe edited the "www to apex"
  rule on 2026-10-05 ~15:10 BRT; measured the same day: `http://www` and `https://www`, any path, answer one 301 to
  `https://solvr.dev/<path>?<query>` (query kept), and the target answers 200. HSTS is still absent (optional).
- A fix written into the production database (the two profiles that show an email): listed for Felipe when
  batch D reaches it.

## Verification, every batch

1. Frontend `npm test` and `npm run typecheck`; backend `go test` for touched packages; file-size check.
2. Local stack from the lane; the recon harness (`crawl.mjs`, `analyze.mjs`, `local-guard.mjs`,
   `local-browser*.mjs`, `frontend/scripts/seo-verify.mjs`) re-run against it.
3. A short written result per batch: what changed, what was measured, what is left.
4. After Felipe deploys a batch: the same crawler against production with curl only.

## Critical files

`frontend/next.config.mjs`, `frontend/app/layout.tsx`, `frontend/lib/seo/route-policy.ts`,
`frontend/components/site-analytics.tsx`, `frontend/lib/analytics.ts`, `frontend/lib/api.ts`,
`frontend/hooks/use-room-members.ts`, `frontend/components/rooms/{presence-sidebar,room-header-actions,room-detail-client}.tsx`,
`frontend/components/prompt/copy-prompt-button.tsx`, `frontend/app/{posts,rooms,agents,users,blog,docs,connect}/…`,
`backend/internal/models/funnel_event.go`, `backend/internal/api/handlers/{funnel,posts,agents,users,blog}.go`,
`backend/internal/db/sitemap.go`, `SPEC.md` Part 27.

---

## Appendix — backend code map (read-only agent, 2026-10-05; paths under `backend/`)

**Funnel ingest (`POST /v1/analytics/funnel`)**
- `entry_surface` is free text up to 60 chars, no allowlist: a new surface needs NO backend change (a typo
  silently makes a new bucket). Values in use: `connect_page`, `homepage_panel`, `room_page`.
- A new EVENT NAME (batch F `skill_fetched`) needs: a migration (event CHECK, `migrations/000136…up.sql:16-20`),
  the event maps in `internal/models/funnel_event.go:96-109`, the contract entry (:130-186), the hard-coded
  400 message in `internal/api/handlers/funnel.go:69-70`, and the test that counts 9 events
  (`handlers/funnel_test.go:95`).
- `source.kind` may be `room` or `post` (`handlers/funnel_source.go:18,37-60`); an unresolvable source is
  dropped and the step still recorded. `flow_id` optional, ≤64, never checked against issued ids.
- Traps: handler caps `preset`/`role`/`instruction_version` at 60 but the DB at 50/50/20 (silent
  `recorded:false`); an invalid bearer gets 401, so stale sessions lose steps; no rate limit, no dedupe.
- Server steps: `room_created` `handlers/rooms.go:210,222-245`; `participant_joined`
  `handlers/rooms_presence.go:144`; `first_two_way_exchange` `handlers/rooms_entry_submit.go:155`.
- SPEC: funnel text is 25.7 (`SPEC.md:5574-5588`), `flow_id` in 25.6 (:5507); 27.1 :6031; 27.6 :6221.

**Profiles (batch D)**
- `GET /v1/agents/{id}` → `{agent, stats{posts_created, contributions, upvotes_received, reputation}}`
  (`handlers/agents.go:467-484`). **It exposes the agent's `email` when set** (`models/agent.go:47`): fix.
  No rooms count. `GET /v1/users/{id}` → same stats, no email (`handlers/users.go:97-123`).
- Lists to link an author's posts: `GET /v1/posts?author_type=&author_id=` (per_page ≤50),
  `GET /v1/replies?author_type=&author_id=`, `GET /v1/agents/{id}/activity`.
- `/seo` pattern to copy: `handlers/posts_seo.go:29-68`, `handlers/rooms_seo.go:29-68`; shape
  `{indexable, title, description ≤160}`; OpenAPI `internal/api/openapi_paths_seo.go`; family
  `internal/api/route_families.go:477-485`. A child route must answer like its parent
  (`router_child_list_contract_test.go:57,79-84`).
- The agent sitemap rule reads the stored bonus column (`internal/db/sitemap.go:24`), not the reputation
  users see (`internal/db/reputation_canonical.go:36-37`): an indexability rule must pick one.
- **Frontend bug to fix with it:** `fetchSEO` turns any failed `/seo` answer, 500 included, into null and
  the post page renders that as `noindex` (`frontend/app/posts/[id]/page.tsx:70`).
- Display names: email signup stores `display_name` verbatim (`handlers/auth.go:55-61,102`) and
  `PATCH /v1/me` does not validate (`handlers/users.go:346-348`): that is how emails became names.

**Posts and blog**
- `GET /v1/posts`: `per_page` max 50; `meta` has total/page/per_page/has_more, no `total_pages`. Its rule
  is not the sitemap's rule (it does not check publication or moderation state). No related-posts endpoint.
- Blog excerpt = supplied, else `GenerateExcerpt(body, 500)` on raw Markdown, byte-cut
  (`models/blog_post.go:107-118`), not regenerated on update; `meta_description` only if supplied. Reuse
  `seo.PostDescription` (`internal/seo/excerpt.go:59-64`). `GET /v1/blog/{slug}` has no status filter.

**New routes and tests**
- Every new route needs: an OpenAPI op (`openapi_coverage_test.go:15`), a route-family entry
  (`route_families.go:31`; tests `route_families_test.go:52,105`) and a `SPEC.md` Part 26 row (:5748-5761).
- Integration tests need `DATABASE_URL`. Known failing baseline: `progress.txt:13312,13333`.

**Sitemaps 503 (batch E)**
- Frontend routes never check `res.ok` (`frontend/app/sitemap-posts.xml/route.ts:8-27`); tests pin the
  empty-200 behaviour (`…/route.test.ts:38`, `frontend/app/sitemap.xml/route.test.ts:50`).
- Backend runtime DB failure answers 500 (`handlers/sitemap.go:53,98,140`; tests `sitemap_test.go:154-158,
  302-306,526`). 503 pattern to copy: `healthReadyHandler` (`internal/api/router.go:1051-1062`).

**Request log:** `api_request_events` stores no user agent; Next.js server fetches look like anonymous
agents; `/v1/sitemap/*` is counted.

## Appendix — frontend code map (read-only agent, 2026-10-05; paths under `frontend/`)

**Corrections this map makes to the plan above**
1. **Posts have no vote, bookmark or reply UI.** `VoteButton`, `useBookmarks`, `CommentsList`, `EditPostForm`,
   `useCreatePost` and `useViewTracking` have no caller. Batch C events become `blog_vote`
   (`app/blog/[slug]/blog-post-client.tsx:36-53`) and `room_comment` (`components/rooms/comment-input.tsx:34-60`);
   drop `post_vote`, `post_bookmark`, `reply_submit`.
2. **The `/posts` search box searches on every keystroke, with no debounce**
   (`components/posts/posts-page-client.tsx:39-46`, `posts-list.tsx:30-54`). Each keystroke is a row in
   `search_queries`, so the server's search counts are inflated. Add a debounce in batch A and send the GA
   `search` event when typing settles. `/rooms` search has a submit (`components/rooms/room-list.tsx:73-78`).
3. **OAuth cannot tell sign-up from login**: the exchange returns no new-user flag
   (`backend/internal/api/handlers/oauth_login_code.go:112-116`). `sign_up` is exact only for email
   (`app/join/page.tsx:69-90`) unless the API adds the flag (API-first).
4. **Events that happen on untracked paths or right before a full page load are lost**: claim
   (`app/claim/page.tsx:96-112`), OAuth callback (`app/auth/callback/page.tsx:24-90`), email login
   (`app/login/page.tsx:26-47`, full load at :42), logout (`hooks/use-auth.tsx:105-111`). Queue them in
   `sessionStorage` and send on the next tracked page. `sendGAEvent` drops silently when GA is not set up.
5. **The footer is a server component and is missing on many pages** (`/posts*`, `/rooms*`, `/agents*`,
   `/users*`, `/leaderboard`, `/data`, login, join, settings, 404). "Cookie settings" needs a second home
   (header account and mobile menu, `components/header.tsx:151-305`, `components/ui/user-menu.tsx`).
6. **Consent must be client-only.** `/` is shared-cached 60 s and `/about`, `/how-it-works`, `/terms`,
   `/privacy` for a day (`next.config.mjs:34,56-59`); nothing reads cookies on the server today.
7. **Never send copied text as event data**: the API playground's curl copy carries the typed token
   (`components/api/api-playground.tsx:97-109`) and the API-key copy is a secret
   (`app/settings/api-keys/page.tsx:103-107`).
8. Small bugs found: the OAuth callback's default return URL is the retired `/feed`
   (`app/auth/callback/page.tsx:81-85`); `CopyButton` always says "Copied" (`components/page/copy-button.tsx:20-24`);
   no Toaster is mounted, so `toast.error` in `comment-input.tsx:53,55` shows nothing; `/blog?tag=` links are
   never read (`components/blog/blog-page-client.tsx:278-282`).

**Tests that pin today's behaviour (update them first, TDD)**
- `components/site-analytics.test.tsx:63-68` (layout must render `<SiteAnalytics` and not mention
  GoogleAnalytics; :73-110 any page reading a token from its URL must be untracked).
- `lib/operator-analytics-privacy.test.ts:133-140` (`lib/analytics.ts` must contain `sendGAEvent` and no
  `fetch(`), :29-44 and :142-161 (banned reporting terms; no analytics-named files in `public/`).
- `components/footer.test.tsx:105-122` (exact compact-footer links); `interaction-system.test.tsx:167-176,179+`
  (every link resolves, every button has a handler); `design-system.test.tsx:606-620` (800 lines per file).
- `components/rooms/room-header-actions.test.tsx` (Share copies the canonical URL with no credentials: the
  `?via=share` link is not a credential, the test changes with the fix); `hooks/use-room-members.test.ts`;
  `components/rooms/presence-sidebar.test.tsx:46-80`.
- `app/route-metadata.test.ts` (12 noindex layouts, 13 indexable), `app/seo-page-metadata.test.ts`,
  `app/titles.test.ts`, `lib/seo/route-policy.test.ts`, `next.config.test.ts`,
  `app/sitemap-posts.xml/route.test.ts:38`, `app/sitemap.xml/route.test.ts:50`.
- Mocking pattern: inline `vi.mock('@/lib/api', () => ({ api: { …: vi.fn() } }))`; `use-auth` and
  `next/navigation` mocked per file; no shared render helper. Nothing mocks `sendGAEvent` yet.

**Batch A pointers**
- `CopyPromptButton` (`components/prompt/copy-prompt-button.tsx:29-37`, `onCopied` fires only after a
  successful write) is used by `components/connect/connect-panel.tsx:145-151` (reports),
  `components/homepage/use-cases-section.tsx:32-37` (does not), `app/docs/guides/[slug]/page.tsx:75` (does
  not). Room starter prompts are not reported either (`components/rooms/room-starter-prompts.tsx:69-78`).
- "Connect agents now" is a button that opens an inline `ConnectPanel` (`components/homepage/hero-section.tsx:61-73`,
  panel :111-115); that is what sends `connection_started` with `homepage_panel`.
- Room Share: `components/rooms/room-header-actions.tsx:34-38` → `hooks/use-share.ts:31-57` (Web Share
  :38-40, else clipboard :41-44).
- Edit link: `components/posts/post-detail.tsx:125`. `app/notifications/page.tsx` is a client page with no
  layout (root title, indexable). `app/not-found.tsx:3-18` exports no metadata.

**Batch B pointers**
- A child segment's `openGraph` or `twitter` replaces the parent's whole object, and Next copies a page's
  title into Open Graph only when Open Graph has none; the root has both, which is why 55 pages show the
  homepage's. Segments that define their own: `app/layout.tsx:15`, `app/agents/[id]/page.tsx:35-44`,
  `app/blog/[slug]/page.tsx:37-49`, `app/data/layout.tsx:11-14`, `app/posts/[id]/page.tsx:71-78`,
  `app/rooms/[slug]/page.tsx:74`, `app/users/[id]/page.tsx:36-45`. A root `opengraph-image` file does not
  reach them, so the helper must set `images` explicitly.
- Helpers: `indexableMetadata` `lib/seo/route-policy.ts:82` (12 layouts), `noindexMetadata` :76 (12 layouts).
- JSON-LD builders, all in `components/seo/json-ld.tsx`: agent :123-149, blog :152-196, user :199-228,
  post :70-120, room :233-285, breadcrumb :49-61.
- Blog description order (`app/blog/[slug]/page.tsx:30-32`): `meta_description`, then `excerpt` (500
  characters of raw Markdown from the backend), then body. `cover_image_url` is read (:76) and unused.

**Batch C pointers**
- GA library facts: `@next/third-parties` 15.2.4 calls `gtag('config')` on mount, has no consent mode, and
  unmounting does not unload the tag. `SiteAnalytics` sits after `</body>` (`app/layout.tsx:64`); the ID
  falls back to the hardcoded production ID (:10). There is no Content-Security-Policy.
- Mount points for the banner and the click listener: `components/providers.tsx:5-7` or `app/layout.tsx:61-64`.
  Primitives: `components/ui/{dialog,alert,button,switch}.tsx`.
- Storage keys in use: `auth_token`, `auth_return_url`, `solvr_referral_code`, `solvr:recently-viewed-rooms`
  (local); `solvr_pending_claim_token`, `solvr_share_visit:{kind}:{ref}`, `solvr_session_id`,
  `solvr_viewed_posts` (session).
- Navigation: `components/header.tsx` (logo :42-44, links :48-75, docs menu :78-112 from `DOCS_LINKS` :17-24,
  log in :119-124, connect :126-131, mobile menu :151-305); `components/footer.tsx` (arrays :8-31, compact
  :108-118, legal row :154-189); account menu `components/ui/user-menu.tsx:90-147`.
- `/connect` controls: `components/prompt/role-pair-switch.tsx:40-47`, `components/prompt/prompt-sentence.tsx:233-247`
  (private toggle), :328-335 (intent, debounced 300 ms).
- Sort and load more: `posts-page-client.tsx:48-69`; `rooms/room-list.tsx:67-71,80-101,186-195`;
  `agents-page-client.tsx:84-90`; `users-page-client.tsx:128-141,196-201`; `leaderboard-page-client.tsx:73-85`.
- Copy buttons to wire as `code_copy`: `/skill` (`skill-hero.tsx:21-24`, `skill-install.tsx:19,40-43`,
  `skill-preview.tsx:50`), `/mcp` (`mcp-hero.tsx:43,66`, `mcp-setup.tsx:25`), `/api-docs` (`api-hero.tsx:47`,
  `api-quickstart.tsx:74`, `api-endpoints.tsx:256`, `api-sdks.tsx:116,120`, `api-mcp.tsx:115-116`).
- Account events: sign-up `app/join/page.tsx:79-85`; claim in settings `components/claim-agent-form.tsx:18-47`;
  API key `app/settings/api-keys/page.tsx:56-72`; post create `components/posts/post-composer.tsx:44-56`;
  room create `components/rooms/create-room-dialog.tsx:61-86`.
- Privacy text to rewrite: `app/privacy/page.tsx:347-352` (Analytics "Legitimate interest") and
  `components/legal/privacy-later-sections.tsx:238-312` (Cookies and Tracking).

**Batch D pointers**
- `/connect` server HTML holds only "READING THE SENTENCE..." (`components/connect/connect-panel.tsx:44-50`);
  the `<h1>` exists only after the browser fetch (:124,128).
- Guides: `app/docs/guides/page.tsx:26-84`; `app/docs/guides/[slug]/page.tsx:37-59` (`UseCaseGuide` :61-88,
  `ResumeGuide` :103-162); fields in `lib/docs/workflow-guides.ts:12-27`.
- Profiles fetch only the profile on the server (`app/agents/[id]/page.tsx:16`, `app/users/[id]/page.tsx:16`);
  activity and posts load in the browser (`hooks/use-agent-activity.ts:72`, `hooks/use-user.ts:115-119`).
- `/posts` server fetch: `app/posts/page.tsx:30-41` (20 posts; total and `has_more` discarded);
  `posts-list.tsx` refetches page 1 on mount (:52-54) and starts with `hasMore` false (:26).
