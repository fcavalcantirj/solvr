---
round: 1
builder_session: fresh Builder session, 2026-10-04 (Fable 5.1); read the handover directory and the files it points to; re-measured every [REAL] claim on production and on main @ d6f41457
---

# Plan r1 — 360° SEO and tracking recon of solvr.dev

Every measurement cited here is mine, taken 2026-10-04 in the evening BRT (2026-10-05 ≈01:00–02:00 UTC).

## Blocking constraints, restated

1. **Recon only.** I produce a report. No product code change, no commit, push or deploy, no DNS or
   Cloudflare change, nothing submitted to Search Console, Bing Webmaster or IndexNow. I extend this to any
   third party that would fetch the site for me (PageSpeed Insights, rich-result testers).
2. **Production is read-only.** GET requests and read-only admin GET routes only. No POST to
   `/v1/analytics/funnel`, no content created on production. Event firing is tested on a local build.
   Three things would break this silently, so they are rules of the run:
   - **No JavaScript-executing browser on production.** A browser there sends GA page views,
     `connection_started` (`/`, `/connect`), `room_viewed` (room pages) and post/blog view POSTs
     (`hooks/use-view-tracking.ts`, `app/blog/[slug]/blog-post-client.tsx`). Production is fetched with curl.
   - **No `GET /v1/search` on production.** Every call inserts a `search_queries` row
     (`backend/internal/api/handlers/search.go:259-314`), and the 30-day search count is an audited number.
     `/posts?q=…` is safe with curl: the search runs in the browser (`components/posts/posts-list.tsx`).
   - The repo habit "search Solvr first, post what you learn" is suspended for this recon for that reason.
3. **Only numbers I measured.** Each claim is MEASURED (command and date in the evidence) or INFERRED
   (reasoning stated). GA4 and Search Console figures I cannot read are "unavailable", never zero.
4. **No secrets in any artifact.** `ADMIN_API_KEY` is read from `/Users/fcavalcanti/dev/solvr/.env` into a
   shell variable, never echoed or saved. That file also holds production database credentials and deploy
   webhooks: I do not use them.
5. **Browser user agent on every request** to solvr.dev and api.solvr.dev. The 403/1010 did not reproduce
   today (D3); the rule stays.

## Handover discrepancies

| # | Handover says | Measured | Effect on the plan |
|---|---|---|---|
| D1 | `sitemap-core.xml` (22 URLs) | 21 `<loc>`, equal to the 21 `sitemap: true` rows of `route-policy.ts` (`/ipfs` is `sitemap: false`). Indexed total 619 = 21+467+71+32+28 | The report uses 21 / 619 |
| D2 | No page has `og:image`, `og:url`, `twitter:image`, "although `twitter:card` is `summary_large_image`" | True for image and url on all six. `twitter:card` is `summary` on `/agents/agent_claude_opus_eval` (no `twitter:site` either); `summary_large_image` on the other five | Card type audited per page type |
| D3 | Constraint 5: Cloudflare answers 403/1010 to `Python-urllib` | Not reproduced. Real `urllib.request` default UA: 200 on `solvr.dev/robots.txt` and `api.solvr.dev/v1/sitemap/counts`. curl with the UA strings `node`, `undici`, `curl/8.7.1`, Googlebot (desktop, smartphone), bingbot, GPTBot, ClaudeBot, PerplexityBot, facebookexternalhit, Twitterbot, Slackbot, LinkedInBot: 200 on `/robots.txt` and `/` | Rule kept. A user-agent matrix becomes measurable (P2) |
| D4 | GA evidence: `grep -c G-HS74SKKSQY` | The ID is also in the RSC payload where the tag is not loaded: `/claim`, `/auth/callback`, `/email/unsubscribe` contain the ID once and `googletagmanager.com/gtag/js` zero times. The six sampled pages: ID twice, script once | "Tag loaded" is measured by the script URL, not the ID. The claim itself holds |
| D5 | Consent grep "finds only prose on /privacy" | Also prose in `app/terms/page.tsx:515`. No consent code anywhere | None |
| D6 | "Next.js 15 (App Router, ISR)" | Only `/` answers with shared-cache headers (`s-maxage=60`, `x-nextjs-cache: STALE`). The other five sampled pages answer `private, no-cache, no-store`. `cf-cache-status: DYNAMIC` on all six | Cache behaviour per page type goes into P4 |
| D7 | `[UNVERIFIED]` "the domain may be verified by DNS: unknown" | `dig TXT solvr.dev` returns a `google-site-verification=` record | INFERRED: the domain was verified with a Google product at some point. Which account holds it: Felipe |
| D8 | "read `CLAUDE.md` first" | `CLAUDE.md` "Deployment Constraints" still describes one flat `frontend/app/sitemap.ts` (~100 URLs); the code is an index plus six route handlers. `SPEC.md` 27.1 lists `/ipfs` as indexable and listed; 27.5 says three guides, `route-policy.ts` has four | Reported as doc-drift findings |
| D9 | Baseline funnel counts | The author's `pagetext.mjs` loaded 24 production pages in headless Chromium on 2026-10-04 with nothing blocked | The 30-day funnel and GA numbers contain test traffic. Funnel rows store no user agent (`models/funnel_event.go`), so it cannot be separated afterwards. Stated beside every funnel number |

**Confirmed as written:** production serves v1.3.14 behaviour (`crystallized_posts` is gone from
`/v1/overview`; `/ipfs` is out of the core sitemap; `main` = `origin/main` = `d6f41457` with `a187ea0b` below
it). `layout.tsx:10` and `UNTRACKED_PATHS` verbatim. Ten helpers in `lib/analytics.ts`, zero importers, and no
`gtag(`/`dataLayer`/`sendGAEvent` call outside that file. Funnel contract: the nine steps, same order. Baseline
30d: 467/28/31/71/32 = 629; 255 searches, 9 zero-result; 15 rooms created, 5 activated, 6 first two-way; 48 + 4
`connection_started`; five milestones `unavailable`. `robots.txt` byte-equal. Index = five sitemaps; posts 467,
agents 71, blog 32, rooms 28; `sitemap-users.xml` 200 and not in the index. Title, description, canonical on
all six; JSON-LD on all but `/blog`; no `hreflang`; no `google-site-verification` meta. Every file and endpoint
of the "machinery" list exists (`SPEC.md` Part 27 at line 6024). `/tmp/solvr-orch/validate` still exists;
`progress.txt` lines 13329-13333 say "33 pages, 331 links, 0 broken".

**Open questions the verification already narrowed:**
- Q3 (users sitemap): the backend leaves users out on purpose: `backend/internal/db/sitemap.go:96,174-175`
  ("profile pages have no SEO value", commit `4a797606`, 2026-04-05) hardcodes their count to 0. The frontend
  route `app/sitemap-users.xml/route.ts` was never removed and still lists 354 profiles; a profile page is
  200, indexable and self-canonical. Whether that is wanted stays Felipe's call.
- Q4 (Open Graph image): no `opengraph-image` file under `frontend/app`; the largest logo in
  `frontend/public` is 256 px. An image would have to be designed. Felipe's call.

## Acceptance checklist — point-by-point

1. **Constraints restated, production unchanged** → section above; P0 makes the prod harness GET-only and
   keeps browsers off production.
2. **Every page type, with its sample** → P1: 39 static routes (all sampled) and 9 dynamic patterns with
   named URLs. Two patterns have fewer than five instances; I take all of them and say so (Concerns).
3. **Per page type from production HTML** → P2: status, title, description, canonical, robots meta,
   Open Graph and Twitter tags including image, JSON-LD validity, `hreflang`, GA tag, on **every** sitemap URL,
   not only the sample.
4. **Indexability both ways** → P3: indexable ⇒ listed and not `noindex`; non-indexable ⇒ unlisted and
   `noindex` or 404, with production specimens for private rooms, one-sided rooms, account pages, deleted
   accounts and not-found pages.
5. **Tracking in three layers** → P5: GA4 (tag from prod HTML; page views and events in a browser on a local
   build), server funnel (each contract event mapped to its caller, then fired locally), Search Console
   (what is verifiable without access, and the exact data asked from Felipe).
6. **Sitemap reconciliation** → P3: sitemap files vs `GET /v1/sitemap/counts` vs baseline `indexable`, by
   count and by id set.
7. **Technical layer** → P4: robots.txt, legacy redirects, canonical host, crawl-relevant headers, Core Web
   Vitals with the reason field data cannot be measured here.
8. **Needs from Felipe** → "What I need from Felipe", with what can be concluded without it.
9. **One findings report, nothing fixed** → P6.

## The plan

### P0 — Guard rails
- **Artifacts:** `docs/handovers/2026-10-04-seo-360-recon/FINDINGS.md` (the report), `evidence/` (raw
  outputs: one JSONL row per fetched URL, request logs, a timestamped command log), `harness/` (the scripts).
  Untracked, not committed.
- **Production harness:** a Node script that shells out to curl with the browser UA. Sequential, at most
  2 requests/s, GET only (no code path for another method), stops after three consecutive 429/5xx. It imports
  the exported checks of `frontend/scripts/seo-verify.mjs` (`parseHead`, `extractLinks`, `checkPage`,
  `structuredProblems`, `legacyProblems`, `isNoindex`); that file is not edited.
- **Admin reads:** `GET /admin/seo/baseline` for 24h, 7d and 30d (saved with
  `scripts/seo/weekly-baseline.sh` into `evidence/`), `GET /admin/activation-analytics`,
  `/admin/share-attribution`, `/admin/growth/acquisition-loop`, `/admin/search-analytics/summary`,
  `/admin/users/deleted`, `/admin/agents/deleted`. Apart from the baseline, bodies are reduced to counts and
  ids before anything is saved (no emails, no names).
- Long outputs go to files; I read tails and greps. No `ralph*.sh`. No subagents.

### P1 — Inventory (sources: the six sitemap files, `frontend/lib/seo/route-policy.ts`, the 48 `page.tsx` of `frontend/app`)

**Static routes: all 39.**

| Group | Routes | Expected |
|---|---|---|
| Indexable, in core sitemap (17) | `/` `/posts` `/rooms` `/connect` `/docs` `/docs/protocol` `/docs/guides` `/agents` `/data` `/users` `/blog` `/leaderboard` `/about` `/how-it-works` `/api-docs` `/mcp` `/skill` | 200, self-canonical, no `noindex`, listed |
| Indexable, not listed (5) | `/ipfs` `/amcp` `/privacy` `/terms` `/status` | 200, self-canonical, unlisted |
| `noindex` by policy (15) | `/login` `/join` `/claim` `/auth/callback` `/settings` `/settings/agents` `/settings/api-keys` `/dashboard` `/referrals` `/pins` `/email/unsubscribe` `/admin/system` `/connect/agent` `/blog/create` `/posts/new` | `noindex`, unlisted |
| In the route tree, in neither policy table (2) | `/notifications` `/zh/promote` | measured, then reported |

**Dynamic patterns: 9.** Spread rule for sitemap-backed types: positions 0, n/4, n/2, 3n/4, n-1 of the
sitemap, plus the handover's own sample.

| Pattern | Population | Named sample |
|---|---|---|
| `/posts/{id}` | 467 | `d8745348-0642-4900-a06b-37f0625f7532`, `83656c92-b68a-4f57-a9fd-fab4447e5385`, `691c1d5b-758c-453d-8f0f-b872a2a63ed6`, `d2a1fae2-e734-49cb-8307-0447fce8e63f`, `6ace803a-ef27-4cb4-8cd7-e4730962938c`, `9ff57184-edb6-43cd-b1c5-2e2bfba230b7`; plus the first human-authored and first agent-authored post in sitemap order if the six lack either (their JSON-LD differs) |
| `/rooms/{slug}` | 28 indexable | `plant-faces-v0`, `live-proof-slim-1003`, `esp-atlas-build`, `claude-faces-coord`, `help-with-hermes-agent`, `mackjack-ops`. Non-indexable specimens: `planner-and-executor-workroom` (public, not in sitemap), `acceptance-1003-pv24` (private) |
| `/rooms/{slug}/history/{n}` | 31 pages (25 rooms × 1, 3 rooms × 2) | `secure-my-supa-360/history/1` and `/2`, `felipe-health-improvement/history/1` and `/2`, `onvida-dev-20260707/history/1` and `/2`, `plant-faces-v0/history/1`. Negatives: `/history/3`, `/history/01`, `/history/0` |
| `/agents/{id}` | 71 | `agent_claude_opus_eval`, `agent_jogo_da_velha_planejador`, `agent_fam_foreign_1783190476`, `agent_openclaw_01`, `agent_KevinAI`, `agent_test_claim_agent_1770517937` |
| `/users/{id}` | 354 (sitemap not in the index) | `f0926f08-49b6-4eb3-87db-efb6bc2aa61b`, `a6fde9d7-163e-43bf-8f9c-91d0b771ce11`, `fa1b4d2b-b8bd-496a-a3c4-e6720cb3da77`, `4a9a39da-3050-4686-8221-4d5a000d2c26`, `f8e01e12-9544-433a-be60-53d381edd153` |
| `/blog/{slug}` | 32 | `can-a-zero-shot-llm-jev-play-a-tetris-style-game-on-an-m5sticks3-measured-not-at-06b-barely-at-4b`, `building-an-esp32-nrf24-jammer-from-7-dissected-projects-the-assembly-manifest`, `wifisetsleepfalse-can-silently-do-nothing-on-esp32-and-getsleep-will-confirm-the-lie`, `a-raspberry-pi-5-that-codes-our-hermes-fork-on-the-official-claude-agent-sdk-architecture-and-real-numbers`, `welcome-to-solvr-how-ai-agents-are-building-the-future-together` |
| `/docs/guides/{slug}` | 4 exist | all four: `connect-planner-executor`, `share-context-between-agents`, `connect-builder-reviewer`, `resume-across-two-clis`; negative: an unknown slug |
| `/posts/{id}/replies/{n}`, n ≥ 2 | unknown: exists only above 100 replies. P1 reads reply counts from `GET /v1/posts` | every instance found; else the type has zero instances on production and I check `/replies/1` (308) and `/replies/2` (404) on the six posts |
| `/posts/{id}/edit` | account page per post | the six post ids, expected `noindex` |

Also inventoried, not pages: `robots.txt`, the sitemap index and six sub-sitemaps, `llms.txt`, `skill.md`,
`skill.json`, `heartbeat.md`, `install.sh`, `solvr-skill.zip`, icons, and `api.solvr.dev` itself.

### P2 — What production serves (curl only)
- **Full crawl, not a sample:** the 973 sitemap URLs (619 indexed + 354 users), the 31 history pages, the
  39 static routes and the negatives. About 1,100 GETs, roughly ten minutes.
- **Per URL:** status and redirect target; title; description; canonical; robots meta and `X-Robots-Tag`;
  `og:title/description/type/url/image/site_name`; `twitter:card/title/description/image/site`; each JSON-LD
  block (parses, `@type`, absolute URLs, real dates via `structuredProblems`, and required properties read
  from Google's current documentation at run time, cited, not from memory); `hreflang`; `<html lang>`;
  viewport; `<h1>` count; visible-text length; the gtag script; internal links; `cache-control`,
  `cf-cache-status`, compression; time to first byte and size.
- **Across the whole set:** duplicate titles and descriptions (SPEC 27.3 promises unique post titles),
  missing or over-long descriptions, whether each tag sits in `<head>` or `<body>`, and an inbound-link count
  per sitemap URL (pages reachable only through the sitemap).
- **User-agent matrix** on one URL per page type (19 URLs × Chrome, Googlebot smartphone, bingbot, GPTBot,
  facebookexternalhit, Twitterbot): status, tag placement, tag differences. Labelled "spoofed user agent from
  a home IP": what the real Googlebot receives is a Search Console URL Inspection (Felipe).

### P3 — Indexability both ways, and reconciliation
- **Indexable ⇒ listed and clean:** each of the 619 indexed URLs must be 200, self-canonical, without
  `noindex`.
- **Indexable but unlisted** (reported with the decision owner): the 31 history pages, the 354 profiles, the
  five unlisted static routes, any route missing from both policy tables, query variants of collections
  (`/posts?sort=`, `/posts?utm_source=`, `/agents?sort=`, `/blog?tag=`, `/rooms?…`, `/users?…`,
  `/leaderboard?…`).
- **Non-indexable ⇒ unlisted and `noindex`/404:** the 15 policy routes; `/notifications`;
  `/posts/{id}/edit`; public rooms the API lists but the sitemap does not (page through `GET /v1/rooms`);
  the private room (page `noindex`, no transcript, history 404); not-found URLs of every type; deleted users
  and agents (ids from the read-only admin routes, expected 404). Posts awaiting moderation: Question 1.
- **Reconciliation:** `<loc>` count and id set of each sitemap file vs `GET /v1/sitemap/urls?type=…` vs
  `GET /v1/sitemap/counts` vs baseline `indexable`; index `lastmod` vs `counts.lastmod`; the users gap
  explained from the code; the 31 history pages vs `data.history.total_pages` summed over the 28 rooms
  (already equal: 31).

### P4 — Technical layer
- **robots.txt:** rules per user agent, the non-standard `Host:` line, the sitemap reference; `robots.txt`
  and `X-Robots-Tag` of `api.solvr.dev` (can API JSON be indexed?).
- **Legacy URLs (SPEC 27.4):** `/problems`, `/questions`, `/ideas`, `/feed`, `/new`, and 20 post ids under
  each legacy prefix through `legacyProblems`: one 308, query string kept, target 200 or 404. Checked, not
  redesigned. The output doubles as the 2026-10-09 review if Felipe wants it.
- **Canonical host:** http/https × apex/www × trailing slash × upper-case path × doubled slash; every chain
  followed to its end.
- **Headers:** HSTS, `x-powered-by`, `vary`, cache policy per page type, Cloudflare cache status,
  compression, HTTP/2 and HTTP/3, `X-Robots-Tag` on the non-HTML assets.
- **Status semantics:** 404 with `noindex` per type on production; the "API down ⇒ 500, never 404" rule on
  the local build, by stopping the local API.
- **Core Web Vitals:** field data comes only from the Chrome UX Report through Search Console (Felipe); the
  report says "unavailable" until then. Lab: `next build` First Load JS per route, and LCP, CLS and long-task
  time through a `PerformanceObserver` in the local browser run, labelled "local lab, not production".
  Production: curl time to first byte and transfer size, five runs per page type. No Lighthouse or PageSpeed
  run against production: both execute the page and would fire GA and funnel events.

### P5 — Tracking, three layers

**GA4**
- Tag presence: the gtag script column of the P2 crawl, expected absent only on the three untracked paths.
- Code fact: `@next/third-parties` 15.2.4 only calls `gtag('config', id)`. It sends nothing on route
  changes, so page views on client-side navigation depend on the property's Enhanced Measurement setting,
  which arrives inside the `gtag.js` Google serves for the real ID. That is why the browser run uses the real
  ID and intercepts the hits.
- **Local build, isolated:** worktree `git worktree add --detach /Users/fcavalcanti/dev/solvr-lanes/seo-recon a187ea0b`
  (the deployed commit); database `solvr_lane_seo_recon` in the `solvr-postgres` container (port 5435),
  migrated from empty to head with `migrate … up` (the backend's scratch-database tests apply every up
  migration to an empty database the same way); API on 18400 and web
  on 18401, with `DATABASE_URL`, `PORT`, `JWT_SECRET`, `ALLOWED_ORIGINS` and `NEXT_PUBLIC_API_URL` passed
  inline; web built and started as `frontend/Dockerfile` does (`npm run build`, then
  `node .next/standalone/server.js` with `public` and `.next/static` copied in). Build log to a file, tail
  only. Seeded through the local API: two agents, one human, a post by each, a pending post, a public
  two-way room, a public one-sided room, a private room, a blog post; a state that needs an external service
  is set by SQL on this database. Each seeded object is confirmed by a GET before use; whatever cannot be
  seeded is reported "not measured locally".
- **Nothing leaves the machine but the `gtag.js` download.** Playwright 1.58.1 with a default-deny route:
  only the two local origins and `https://www.googletagmanager.com/gtag/js*` pass; every other request is
  answered 204 by the harness and logged; service workers blocked. Spiked today, offline, with a fake ID: the
  `sendBeacon`, `fetch` keepalive and image transports to `/g/collect` were all seen and all intercepted. The
  run starts with that self-test on a fake ID and stops if any request reaches the network.
- **Matrix:** first load of every page type (`page_view` count, expected 1); eight client-side transitions
  including list to detail, back/forward and `/posts?q=` (count per transition: 0 means lost page views, 2
  means duplicates); every event name seen while scrolling, clicking an outbound link, downloading
  `solvr-skill.zip`, submitting the login form, searching, voting; the `dl` and `dr` of hits after leaving
  each untracked path; zero custom events as the code predicts.

**Server funnel**
- Static: each of the nine contract events to its caller with `file:line`. Six browser events are in the
  site code (`components/connect/connect-panel.tsx:97,111`, `components/rooms/room-detail-client.tsx:95`,
  `components/rooms/connect-agent-panel.tsx:62`, `components/rooms/share-outcome.tsx:40`,
  `hooks/use-share-visit.ts:41`). Three are `source_channel: server` and are mapped to their backend
  emitters (Question 2).
- Dynamic, local: perform each browser action, capture the POST to the local `/v1/analytics/funnel`, its
  answer and the row; drive the three server events through the local API and read the local rows.
- Production, read-only: baseline funnel by step for 24h, 7d and 30d, plus the admin GET reports of P0,
  to list the steps with no rows.
- **Inventory table:** every user action against where it is recorded (GA event, funnel step, another server
  table, nowhere): page view, search, post view, blog view, vote, bookmark, reply, post, sign-up, login,
  claim, API key, prompt copy, room view, share, outbound click, and fetches of `skill.md`, `install.sh`,
  `llms.txt` and the zip.

**Search Console**
- Without access: the DNS verification record, the absent meta tag, no `BingSiteAuth.xml`, what the
  sitemaps and robots.txt offer a crawler. At most five `site:` web searches as a weak outside signal,
  labelled INFERRED, never turned into a count.
- With Felipe's data: `backend/cmd/seo-report` joins the saved baseline with the Performance exports.

### P6 — The report (`FINDINGS.md`)
1. The answer to "are we doing everything, tracking everything, on all pages": one matrix, page type ×
   crawlable / indexable as intended / described / listed / measured.
2. Findings: id, MEASURED or INFERRED, evidence file and command, pages affected, impact, proposed fix,
   decision owner.
3. Gap list ranked by impact: blocks indexing > misdescribes the page > unmeasured > hygiene, weighted by
   pages affected.
4. Fix plan, one approve/reject line per item. Nothing is fixed.
5. "Unavailable without GA4 or Search Console", and method, commands and evidence index.

Then I stop my own processes by PID, drop `solvr_lane_seo_recon` by exact name and remove my worktree by
exact path.

## What I need from Felipe (how he shares it is his call)

Three ways, any mix: **exports and screenshots** (no credentials; the list below is exact), **read-only API
access** (GA4 Data API and Search Console API; adds URL Inspection for every sampled URL), or **nothing for
now** (those parts stay "unavailable").

- **Search Console:** does a property for solvr.dev exist, of which kind, under which account. Then:
  Performance export (Queries.csv and Pages.csv) for the last 3 months and since 2026-10-02; Page indexing
  (totals and each reason's list); Sitemaps (submitted files, status, discovered URLs); Crawl stats; Core Web
  Vitals; Enhancements (Breadcrumbs, Discussion forum, Profile page); URL Inspection of ten URLs I name, one
  per page type.
- **GA4:** Admin → data stream → Enhanced measurement switches and site-search parameters; data retention;
  internal-traffic filters and unwanted referrals; key events; custom definitions; product links (Search
  Console, BigQuery). Reports for the last 90 days and since 2026-10-02: Traffic acquisition, Pages and
  screens, Landing page, Events, and sessions by Hostname (local builds send to the production property
  through the hardcoded fallback ID).

**Concluded without it:** everything the site emits and the server records: tags, sitemaps, indexability,
structured data, redirects, headers, which events the code and the browser send, the funnel.
**Not concluded without it:** whether Google accepted, crawled or indexed anything; impressions, clicks,
position; real-user Core Web Vitals; whether GA receives and keeps the hits; sessions and sources; the GA
property's settings.

## Early signals (measured during verification; candidates, re-measured with evidence in the run)

1. `http://www.solvr.dev/` answers 301 with `Location: http://www.solvr.dev/`, itself; curl stopped after
   five hops. `https://www` → apex and `http://` apex → https are correct.
2. `/posts` links 20 posts in server HTML, with no `rel="next"` and no page links. The other 447 listed
   posts have no crawl path from the collection.
3. No `og:image` or `twitter:image` on six of six pages; five declare `summary_large_image`.
4. `/blog` and the guide page carry the homepage's Open Graph title and description.
5. `/notifications` answers 200 with no `noindex` and no canonical.
6. 354 profiles are indexable but listed only in a sitemap nothing references; 31 history pages are indexable
   and in no sitemap.
7. The guide page sends title, description and canonical inside `<body>` to a browser user agent.
8. The 30-day baseline has no row for `starter_prompt_copied`, `join_prompt_copied`, `share_visit` or
   `share_link_copied`, although each has a caller. I do not know why yet; P5 settles it.
9. No `Strict-Transport-Security`; `x-powered-by: Next.js`; `/favicon.ico` 404; no web manifest;
   `<meta name="generator" content="v0.app">` on every page; `@vercel/analytics` installed and never imported.

## Concerns

- [HIGH] A browser on production pollutes the audited numbers. Mitigation: curl only on production; the
  browser runs locally behind a default-deny route.
- [HIGH] `backend/.env` points `DATABASE_URL` at `localhost:5433`, which now belongs to another project's
  Postgres, and the local `solvr` database is at schema 102 of 141. The recon uses neither: its own database,
  environment passed inline, explicit ports (the shell exports `PORT=3000`), only its own PIDs killed.
- [MEDIUM] The numbers under audit already contain test traffic (D9) that cannot be separated.
- [MEDIUM] "At least 5 URLs of each dynamic type" cannot be met for guides (4 exist) and perhaps reply pages
  (none found yet). I take every instance plus negatives.
- [MEDIUM] About 1,500 GETs to production in total (crawl, user-agent matrix, redirects, timings) at
  2 requests/s. Each page render calls the API; no view or search write happens server-side (both are
  browser calls).
- [MEDIUM] A spoofed bot user agent is not the real bot; Cloudflare may treat verified crawlers differently.
- [MEDIUM] `migrate up` from empty to 141 is [TEST]-backed, not run by me. If it fails I stop and report.
- [LOW] The repo is public and the evidence is untracked beside the handover: it holds only public page
  data and aggregates.
- [LOW] Local lab vitals are not production vitals.

## Questions for the author

1. Posts awaiting moderation: do you know a pending or rejected post id on production? Otherwise I prove
   that row on the local build only and label it so. I will not use `POST /admin/query`, even for a SELECT,
   without your yes.
2. Item 5 asks that "each contract event has a caller in the site code". Three of the nine are emitted by
   the server (`room_created`, `participant_joined`, `first_two_way_exchange`). I map those to their backend
   emitters. Agreed?
3. Report, evidence and harness under the handover directory, untracked and uncommitted: agreed?
4. A worktree at `a187ea0b`, the database `solvr_lane_seo_recon` and ports 18400/18401: any clash with
   lanes still in use?
5. No Lighthouse or PageSpeed run against production. Agreed, or does Felipe want production lab numbers
   knowing each run adds events?
6. Did the Cloudflare rule behind constraint 5 change on 2026-10-04? It changes nothing in the plan.
