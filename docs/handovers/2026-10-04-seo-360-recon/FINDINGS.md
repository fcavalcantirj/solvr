# Findings — 360° SEO and tracking recon of solvr.dev

- **Measured:** 2026-10-04 evening BRT (2026-10-05, about 01:00 to 02:40 UTC), production at v1.3.14 (`a187ea0b`).
- **Scope:** read-only. Nothing was fixed, committed, deployed or submitted anywhere.
- **Update 2026-10-05:** the approved fix plan was carried out on a local branch (uncommitted, not deployed).
  Section 13 shows each fix and what was measured on the local build.
- **Labels:** MEASURED = I ran it in this session and the output is in `evidence/`. INFERRED = reasoning from
  measured facts, stated as such. "Unavailable" = I could not read it; it is never a zero.
- **Google Analytics and Search Console were read on 2026-10-05**, after Felipe supplied a key that can
  execute tools. Section 12 holds those numbers; sections 1 to 11 were written before them and still say
  "unavailable" where they depended on Google.
- **Correction to what I first reported:** the Lighthouse runs on production were not clean for GA. 14 page
  views reached the property through an endpoint my block list missed. Section 10 has the detail.
- **Before committing this folder:** it is untracked and the repository is public. `evidence/crawl.jsonl` holds
  public page data, including the names shown on 354 public profile pages (three email addresses that
  production displays were redacted from it).

## 1. The answer

**"Are we doing everything, tracking everything, on all pages, sending it all to GA?" No.** The crawl
mechanics are in good shape. What is missing is what makes the site findable and what tells you whether a
visit turned into a connection.

What works (all MEASURED, 1,152 production URLs fetched):

- 655 pages that should be indexable answer 200, name themselves as canonical and carry no `noindex`. Every
  account page carries `noindex`. Every not-found URL answers a real 404. Zero mismatches.
- The sitemaps, `GET /v1/sitemap/urls`, `GET /v1/sitemap/counts` and the server baseline agree by count and by
  id for posts (467), agents (71), blog posts (32) and rooms (28). `lastmod` matches the API on all 598.
- 60 of 60 legacy URLs redirect once (308) to the right post, with the query string kept.
- The GA tag is on 1,093 of 1,096 pages; the three without it are the three deliberate ones. In a browser, GA
  sends exactly one `page_view` per page load and one per client-side navigation, with the right address.
- Six user agents (Chrome, Googlebot, Bingbot, GPTBot, Facebook, Twitter) get the same status, title,
  canonical and structured data on 19 page types. All 1,541 JSON-LD blocks parse.

What is wrong, in order of impact:

1. **Every public room page opens a "Login required" dialog over the content for anonymous visitors**
   (F01, confirmed on production).
2. **The site does not say "connect two agents" anywhere a search engine would weigh it**, and it surfaced
   in 0 of 6 non-brand searches (F02).
3. **446 of 467 posts have no internal link pointing at them** (F03).
4. **No page has a preview image**, and 55 pages share the homepage's preview title (F04).
5. **The main conversion is not measured**: copying a prompt on the homepage records nothing, and a website
   visit cannot be tied to the room it produced (F05).
6. **Whether Google indexed any of this is unknown** until the Composio key can execute (section 9).

## 2. Non-brand search: "connect two agents" / "connect your agents"

Felipe's target, stated during the run: the site should appear for these searches without the brand name.

### 2.1 Does it appear? (INFERRED signal, not a Google ranking)

Six non-brand searches through this session's web search tool, a third-party index with US results
(`evidence/nonbrand-search.json`). It is not Google and not Search Console.

| Query | solvr.dev in results | Who was there |
|---|---|---|
| connect two agents | no | Relevance AI docs, Microsoft Copilot Studio (A2A), Pega forum, dev.to |
| connect your agents | no | Zowie "Agent Connect", eesel.ai, StackOne, Infisical, Agno |
| connect two AI coding agents so they work together in a shared room | no | AgentRoom, agmsg, kooo, glama.ai, Product Hunt |
| how to make two Claude Code agents talk to each other | no | GitHub repos, glama.ai and mcpservers.org listings (cc2cc, switchboard-mcp), Hacker News |
| connect a planner agent and an executor agent | no | tutorial blogs, readthedocs, PyPI |
| let Claude Code and Codex agents collaborate with each other | no | comparison articles |

Two readings:

- The two head terms are held by enterprise documentation with a different meaning (connecting agents to
  business tools, the A2A protocol). They are ambiguous and hard.
- The specific searches ("two Claude Code agents talk to each other", "shared room for coding agents") are
  held by small projects, GitHub repositories and directories. kooo, AgentRoom and agmsg make the same
  promise as Solvr. Those are the winnable searches, and the results there are directory listings and
  launch posts as much as product pages.

A brand-restricted search returned `https://solvr.dev/` under its old title, "Solvr — Collective Intelligence
for Humans & AI", plus a legacy `/problems/…` URL. That index still holds the site from before v1.3.

### 2.2 What the pages say (MEASURED, server HTML, `evidence/nonbrand-onpage.json`)

| Page | Words | `<h1>` | Target phrases found |
|---|---|---|---|
| `/` | 1,401 | Connect your agents. Let them work together. | "connect your agents" ×2, "two agents" ×3, Codex ×5, Claude Code ×3 |
| `/connect` | **40** | **none** | Claude Code ×1 |
| `/docs` | 876 | Connect your agents | "connect two agents" ×1, "connect your agents" ×1 |
| `/docs/guides` | 417 | Guides | "two agents" ×2 |
| guide: planner and executor | 167 | Connect a planner and an executor | planner ×4, executor ×4 |
| guide: share context | 187 | Share context between two agents | "two agents" ×2 |
| guide: builder and reviewer | 166 | Connect a builder and a reviewer | none |
| guide: resume in a second CLI | 352 | Resume a collaboration in a second CLI | "two agents" ×1 |
| `/how-it-works` | 909 | Curated continuity for the agent era | none |
| `/about` | 888 | The infrastructure for collective intelligence | "shared room" ×1 |
| `/skill` | 442 | Become a knowledge builder | none |
| `/mcp` | 1,136 | Native AI integration | Claude Code ×5, Cursor ×6 |

- **"connect two agents" appears once on the whole site** (`/docs`). "Talk to each other" appears nowhere.
- **`/connect`, the page titled "Connect your agents", serves 40 words and no heading.** Its content is
  fetched by the browser after load.
- **The four guides are the only pages written for a specific intent, and they hold 166 to 352 words each**,
  page header and footer included.
- Five pages still open with the previous positioning: `/how-it-works`, `/about`, `/skill`, `/api-docs`
  ("API for the collective mind") and `/blog` ("Thoughts on collective intelligence").
- 32 blog posts: 10 mention agents, Claude or Codex in the title, none is about connecting two agents; most
  are ESP32 and M5Stick write-ups. 467 posts: no title contains "room".

### 2.3 Proposals (nothing done; each is Felipe's call)

1. Choose the query set. My recommendation: keep the head term on the homepage, and aim the pages below at
   the specific searches that name the tools.
2. Give `/connect` server-rendered text and an `<h1>` that says what the page does.
3. Turn each guide into a full how-to with a real transcript. The live acceptance run of 2026-10-03 used
   Claude Code and Codex, so SPEC 27.5 now allows naming them: "Connect Claude Code and Codex in a shared
   room" is a page nobody in section 2.1 has.
4. Fix F01 first. Rooms are the proof pages a guide links to, and today they open behind a login dialog.
5. Off-site listings (MCP directories, Product Hunt, a Show HN) are how the competing projects appear. This
   recon submitted nothing.
6. Read the real queries in Search Console once the key works: `backend/cmd/seo-report` already splits
   brand from non-brand.

## 3. Page types at a glance

| Page type | URLs | Reachable by a crawler | Indexable as intended | Describes itself | In a sitemap | Measured |
|---|---|---|---|---|---|---|
| Home | 1 | yes | yes | yes, no image | yes | page view; three copy buttons record nothing |
| Docs and marketing | 21 | `/docs/protocol` has no crawlable link | yes | own title and description; preview title is the homepage's on all 20 | 16 listed, 5 unlisted by policy | page view; `/connect` also reports to the funnel |
| Guides | 4 | resume guide has no link | yes | tags arrive in `<body>` for Googlebot on 3; preview title is the homepage's | yes | page view |
| Post | 467 | **446 have no internal link** | yes | complete, no image | yes | page view; view counter dead |
| Room | 28 | yes | yes; one-sided and private rooms are `noindex` | complete, no image | yes | page view, `room_viewed`; **login dialog blocks the rest** |
| Room transcript | 31 | yes | yes | tags in `<body>` for Googlebot on 30; preview title is the homepage's | **no** | page view |
| Agent profile | 71 + 50 found unlisted | 37 of 71 have no link | all indexable, unlisted ones too | thin; structured data incomplete | 71 | page view |
| User profile | 354 | 292 have no link | indexable | thin ("<name> on Solvr") | only in a sitemap nothing references | page view |
| Blog post | 32 | 11 have no link | yes | description is raw Markdown, about 500 characters | yes | page view, blog view counter |
| Collections | 6 | yes | yes; `/posts` and `/rooms` filters are `noindex` | yes | yes | page view; searches reach the server, not GA |
| Account and sign-in | 17 | n/a | 16 `noindex`; **`/notifications` is indexable** | n/a | none | page view, except the three untracked paths |
| Legacy URLs | 60 sampled | one 308 each | n/a | n/a | none | n/a |

## 4. Findings

Ranked: blocks visitors or indexing, then misdescribes pages, then leaves things unmeasured, then hygiene.

### F01 — A login dialog covers every public room page for anonymous visitors

- **MEASURED on production.** Lighthouse's own screenshot of `https://solvr.dev/rooms/plant-faces-v0` shows
  "Authentication Required — Login required to continue" over the page
  (`evidence/room-login-modal-production-lighthouse.jpg`), with a 401 in the console.
- **MEASURED on a local build of the deployed commit** (`evidence/room-login-modal-local-build.png`,
  `evidence/local-browser-extra.json`): the dialog is visible, `body` is `overflow: hidden`, and Share, Copy
  outcome, Get join prompt and every header link are covered. Five-second clicks on each timed out.
- **Cause (code):** `components/rooms/presence-sidebar.tsx:21` calls `useRoomMembers` for every viewer.
  `GET /v1/rooms/{slug}/members` is owner-only and answers 401 to anonymous callers (production, MEASURED).
  `lib/api.ts` `getMembers` does not pass `skipAuthEvent`, so `lib/api-base.ts:169` opens the dialog.
  The hook dates from `85155fbf` (2026-09-23) and reached production with v1.3 on 2026-10-02.
- **Impact:** 29 public room pages, the product's showcase. A visitor from a search result or a shared link
  meets a login wall on a page whose own text says "reading stays open to everyone". The three funnel steps
  that start on this page cannot be clicked until the dialog is dismissed. Google's guidance: intrusive
  dialogs "make it hard for Google and other search engines to understand your content, which may lead to
  poor search performance".
- **Proposed fix:** do not call the members route for a viewer who is not the owner, or pass `skipAuthEvent`.
  One line, plus a test that an anonymous room view opens no dialog.

### F02 — The site does not target the searches it wants

Section 2. MEASURED on-page; the search results are an INFERRED signal.

### F03 — 446 of 467 posts have no internal link

- **MEASURED** (`evidence/analysis.json`, `linkGraph`): links in the server HTML of all 1,096 pages.

| Page type | Pages | No inbound link at all |
|---|---|---|
| Post | 467 | **446** |
| Agent profile | 71 | 37 |
| User profile | 354 | 292 |
| Blog post | 32 | 11 |
| Room | 28 | 0 |
| Room transcript | 31 | 0 |
| `/docs/protocol` | 1 | 1 (its only link is inside a JavaScript menu, `components/header.tsx:23`) |
| Guide "resume across two CLIs" | 1 | 1 (`listed: false` in `lib/docs/workflow-guides.ts`, yet `sitemap: true`) |

- **Cause:** `/posts` renders 20 posts and loads the rest in the browser (`components/posts/posts-list.tsx`);
  it has no `rel="next"` and no page links. `/posts?page=2` is `noindex` with the canonical on `/posts`.
  Profile pages do not link their author's posts.
- **Impact:** 95% of the knowledge base is reachable only through the sitemap. A page nothing links to gets
  no internal weight, which is the usual reason for "Discovered, currently not indexed". Whether that
  happened here is unavailable (Search Console).
- **Proposed fix:** crawlable archive pages for posts (linked, self-canonical), each author's posts on the
  profile page, and related posts on post pages.

### F04 — No preview image anywhere; 55 pages carry the homepage's preview title

- **MEASURED:** 0 of 1,096 pages have `og:image`, `twitter:image` or `og:url`. 589 declare
  `twitter:card: summary_large_image`, a card built around an image; 507 declare `summary`.
- **MEASURED:** 15 static pages, 5 unlisted static pages, 4 guides and 31 room transcripts have
  `og:title` and `og:description` equal to the homepage's. Cause: `indexableMetadata()` in
  `lib/seo/route-policy.ts` sets title, description and canonical only, so Open Graph is inherited whole.
- **Impact:** a room or guide link pasted in Slack, X, LinkedIn, WhatsApp or Discord shows no image, and for
  those 55 pages the wrong title. For a product that spreads by people sharing a room link, that is lost
  clicks on every share.
- **Proposed fix:** one default image (1200×630) in the root layout, `og:url` from the canonical, and page
  titles passed into Open Graph in `indexableMetadata()`. Whether an image gets designed is Felipe's call.

### F05 — The main conversion is not measured, and the funnel cannot be joined end to end

All MEASURED on the local build unless noted (`evidence/local-browser.json`, `local-browser-extra.json`).

- **The three "Copy prompt" buttons on the homepage record nothing**: no funnel call, no GA event. The copy
  itself works. `components/prompt/copy-prompt-button.tsx` has no reporting call. The same action on
  `/connect` does report `starter_prompt_copied`.
- **A website visit cannot be tied to the room it produced.** The browser reports its two steps with a flow
  id, and the prompt never carries that id (`backend/internal/api/handlers/connect.go:72`: "the sentence
  never carries it"). Production, 30 days: 53 website flows, 0 rooms attributed to them
  (`evidence/admin-reports-reduced.json`, `flow_to_room.website`). The 0 is structural.
- **GA receives no product event.** The ten helpers in `lib/analytics.ts` have no caller. GA gets page views
  and Google's automatic events only (section 5.1), so nothing in GA marks a sign-up, a copied prompt or a
  connection.
- **Four of six browser funnel steps have no row in production** (`starter_prompt_copied`,
  `join_prompt_copied`, `share_visit`, `share_link_copied`), 30-day baseline at 2026-10-05 02:07 UTC. Locally
  all four fire; `join_prompt_copied` and `share_link_copied` only after the F01 dialog is dismissed
  (`evidence/local-browser-room-actions.json`).
- **The Share button of a room page reports nothing and copies a bare link.**
  `components/rooms/room-header-actions.tsx:35-37` shares `/rooms/{slug}` without `?via=share` and sends no
  funnel step. Only the less visible path, Copy outcome and then Copy, reports `share_link_copied` and copies
  a `?via=share` link. A share through the main button is invisible, and so is the visit it brings.
- **`share_visit` is not wired on post pages.** The contract says "a public room or post page"; the only
  caller is `components/rooms/room-detail-client.tsx:101`.
- **Post views are never recorded by the site.** `hooks/use-view-tracking.ts` has no caller. Production: 218
  of 467 posts show 0 views, 800 views in total. Blog views are recorded.
- **Funnel numbers are three days old and include test traffic.** The funnel reached production on
  2026-10-02; the 7-day and 30-day rows are identical. All 19 `room_viewed` rows and all 4 homepage rows fall
  in the last 24 hours, when the author session loaded production in a browser several times. Rows store no
  user agent (`models/funnel_event.go`), so that traffic cannot be removed.
- **Proposed fix:** report the homepage copy; make the Share button report its step and carry `?via=share`;
  decide whether GA should receive a few key events (Open question 2 of the handover) and mark them as key
  events; wire `share_visit` on posts; either call the view hook or delete it; add an origin marker to
  funnel rows.

### F06 — For Googlebot's user agent, 34 indexable pages carry title, description and canonical inside `<body>`

- **MEASURED** (`evidence/technical.json`, `uaMatrix`; crawl): `/docs/guides`, 3 guides and 30 of 31 room
  transcripts. Chrome, Googlebot and GPTBot receive the tags in `<body>`; Bingbot, Facebook and Twitter
  receive them in `<head>`. They stay in `<body>` after JavaScript runs (local browser).
- Lighthouse on production reports "Document does not have a meta description" on both pages of this kind
  it tested, the guide and the transcript (SEO score 91 instead of 100).
- **Why it matters:** Google's documentation: "The `rel="canonical"` link element is only accepted if it
  appears in the `<head>` section". Robots rules are different: Google "will respect robots meta tags in the
  body", so the `noindex` of the post edit pages, also streamed, still counts for Google.
- **Cause:** Next.js streams metadata to user agents outside its `htmlLimitedBots` list, and Googlebot is
  not on it.
- **Proposed fix:** `htmlLimitedBots: /.*/` in `next.config.mjs`, the documented switch that turns
  streaming metadata off.

### F07 — 475 thin profile pages are indexable

- **MEASURED:** 354 user profiles hold a median of 146 characters of visible text, navigation included, and
  the description "<name> on Solvr". 71 listed agent profiles hold a median of 217; 50 more agent profiles
  found through links are not in the sitemap and are just as indexable (median 199).
- **User sitemap:** `/sitemap-users.xml` answers 200 with 354 URLs and nothing references it. The backend
  excludes users on purpose (`backend/internal/db/sitemap.go:96,174`, "profile pages have no SEO value",
  2026-04-05) and reports their count as 0; the frontend route was never removed.
- **Structured data:** the 71 agent profiles are typed `SoftwareApplication` without `offers.price` and
  without a rating, both required by Google for that type. 7 user `ProfilePage` blocks have no `name`.
  14 agent profiles share the description "AI agent on Solvr".
- **Impact:** hundreds of near-empty pages compete for crawl attention with 467 posts that have no links.
- **Proposed decision (Felipe's, per the handover):** `noindex` the profiles that hold nothing, or give them
  content (their posts, rooms, replies); then remove the orphan sitemap or reference it.

### F08 — Blog descriptions are raw Markdown, about 500 characters long

- **MEASURED:** 29 of 32 blog descriptions contain Markdown syntax (`**`, list dashes); median length 497.
  The same text is the `og:description` and the JSON-LD `description`. `BlogPosting` has no `author` and no
  `image` on 32 of 32. Blog cards are `summary` with no `twitter:site`.
- **Proposed fix:** strip Markdown and cut at a word boundary near 160 characters, as post pages already do.

### F09 — Mobile lab speed is below Google's "good" line; the cause of the late first paint is not known

- **MEASURED, lab** (`evidence/lighthouse.json`): Lighthouse 12.8 on 12 production URLs, mobile, simulated
  slow 4G. Performance 69 to 77 on 11 pages (94 on the room transcript). Simulated LCP 4.8 to 6.1 s on those
  11; Google's "good" line is 2.5 s. Blocking time 44 to 164 ms and layout shift 0 are good. Server response
  68 to 285 ms is good. Pages weigh about 500 kB; `gtag.js` is 175 kB of that, 74 kB unused.
- **MEASURED, same runs, no throttling:** first paint at 1.26 to 1.79 s on 11 pages (2.4 s on a repeat of
  the post page) and 0.46 to 0.51 s on the transcript page, although both kinds have their HTML, CSS, fonts
  and scripts by about 0.4 s.
- **It is not the application code and not network delay** (`evidence/first-paint.json`). The same
  Lighthouse on a local build of the deployed commit paints the post, room, transcript and home pages at
  0.15 to 0.19 s. With 150 ms of latency added, the local build paints at 0.44 to 0.50 s.
- **It goes away when the third-party scripts are blocked.** One production post page, two runs each: Google
  tag and Cloudflare beacon active, 1.35 and 2.41 s; Google tag blocked, 0.60 and 1.32 s; both blocked, 0.51
  and 0.42 s. In the trace, layout is finished at 0.54 s and the browser asks for its first rendering frame
  only at 1.38 s, with no long task in between.
- **The Google tag costs about 1.7 s of simulated mobile LCP on that page:** 5.0 to 5.1 s with it, 3.2 to
  3.5 s without.
- **Not established:** how those scripts hold back the first frame, and how the delay splits between the
  tag and the beacon. Two runs each are too few to say.
- **Real-user numbers are unavailable.** The PageSpeed API answered 429 (daily quota), so there is no Chrome
  UX Report data here. Lighthouse shows Cloudflare's Web Analytics beacon on every production page
  (`static.cloudflareinsights.com/beacon.min.js`, `/cdn-cgi/rum`): the Cloudflare dashboard should already
  hold real-user Core Web Vitals.
- Most pages send `cache-control: no-store`, which also disables the browser's back/forward cache.

### F10 — GA hits also go to Google's advertising endpoints, with no consent mechanism

- **MEASURED** (local build, real tag): each page view goes to `analytics.google.com/g/collect` and to
  `stats.g.doubleclick.net/g/collect`; `google.com.br/ads/ga-audiences` and `googletagmanager.com/td` are
  called too. The hits carry `npa=0` and a consent string with nothing set (`gcd=13l3l3l3l1l1`).
- `/privacy` describes analytics as "privacy-focused" and names neither Google Analytics nor Cloudflare.
- **Felipe's call** (legal, per the handover): keep as is, turn Google signals off in GA, or add consent.

### F11 — `http://www.solvr.dev` redirects to itself; no HSTS

- **MEASURED:** `http://www.solvr.dev/` and `/posts` answer 301 with their own address as `Location`.
  `https://www` and `http://` on the apex redirect correctly. No response carries
  `Strict-Transport-Security`. `x-powered-by: Next.js` is sent.
- **Impact:** low; an old-style link to `http://www` loops.
- **Update 2026-10-05 (MEASURED after Felipe edited the Cloudflare rule, ~15:10 BRT):** fixed. The rule's new target is
  `concat("https://solvr.dev", http.request.uri.path)` with Preserve query string. `http://www.solvr.dev/connect?q=1&x=y`
  and `https://www.solvr.dev/connect?q=1&x=y` answer one 301 to `https://solvr.dev/connect?q=1&x=y`; `http://www.solvr.dev/`
  to `https://solvr.dev/`; the chain ends in one hop with 200. HSTS (`Strict-Transport-Security`) is still absent: open, optional.
- **Proposed fix:** a Cloudflare redirect rule for the `www` host and HSTS. Both are Cloudflare changes,
  Felipe's.

### F12 — Index hygiene

All MEASURED.

- `/notifications` answers 200 with the homepage's title, no canonical and no `noindex`. It is in neither
  table of `route-policy.ts`.
- The 31 room transcripts are indexable and in no sitemap.
- Every post page links `/posts/{id}/edit` for everyone (`components/posts/post-detail.tsx:125`): 467
  crawlable links to `noindex` pages.
- `/api-docs` links `/feed` and `/how-it-works` links `/problems`, both retired. Four posts and one blog post
  link `/ideas/{id}`. All redirect correctly.
- `/connect` and `/status` have no `<h1>`. 26 pages have more than one (Markdown headings in posts and blog).
- 248 of 467 post titles exceed 70 characters with the " | Solvr" suffix; 52 exceed 100.
- A not-found page is tracked in GA under the homepage's title, so 404 views cannot be told apart there.
- `/favicon.ico` answers 404, there is no web manifest, and every page carries
  `<meta name="generator" content="v0.app">`.
- 57 posts that are pending (5) or rejected (52) answer 200 with their content to anyone who has the link.
  They are `noindex, follow` and out of the sitemap, so search is unaffected; SPEC 27.1 records the access
  itself as an open question (`evidence/nonpublic-posts-production.json`).
- **Two public profile pages show an email address as the person's name**, in the title, the heading and
  the description: `/users/6bc3eb04-e6a9-4579-bcdf-12e37f0c5c51` and
  `/users/75285b94-d5c8-4631-9f11-6066fad2a056`. One post's description contains an email address
  (`/posts/cc3bb952-1656-4906-a6a5-ff71e89bc08b`). All three pages are indexable. The addresses are redacted
  in `evidence/crawl.jsonl`.

### F13 — During an API outage, list pages and sitemaps answer 200 and empty

- **MEASURED on the local build with the local API stopped** (`evidence/api-down-status.json`); INFERRED for
  production from the same code.
- Detail pages do the right thing: post, replies, room, transcript, agent, user and blog pages answer 500,
  never 404. This is the SPEC 27.4 rule and it holds.
- `/posts`, `/rooms`, `/agents`, `/users`, `/blog` and `/leaderboard` answer 200 with nothing in them.
- The four content sitemaps answer 200 with zero URLs: `app/sitemap-posts.xml/route.ts:25` returns an empty
  sitemap on any error.
- **Proposed fix:** answer 503 from the sitemaps and the collections when the API cannot be read.

### F14 — Documentation drift

- `CLAUDE.md` "Deployment Constraints" describes one flat `frontend/app/sitemap.ts` with about 100 URLs.
- `SPEC.md` 27.1 lists `/ipfs` as indexable and listed; 27.5 says three guides, there are four.
- The funnel contract says a post page reports `share_visit`; no post page does.
- `package.json` carries `@vercel/analytics`, never imported.

## 5. Tracking, layer by layer

### 5.1 Google Analytics (property `G-HS74SKKSQY`)

MEASURED in a browser on a local build of the deployed commit. The real tag was loaded so that Google's own
configuration for the property applied; every hit was intercepted and none reached Google (0 escaped
requests in three runs, `evidence/local-guard-selftest.json`).

| Behaviour | Result |
|---|---|
| Page view on a page load | exactly 1 on each of 28 page types, the 404 page included. A URL carrying `?q=` sends `view_search_results` first and its page view 5.5 s later |
| Page view on client-side navigation | exactly 1 in 22 of 23 transitions, sent about 6 s later, right address and referrer; back and forward included. The one miss (logo to home) did not repeat in 6 further tries |
| The three untracked paths | no tag, no hit; after leaving them, no hit carries the fake code, token or email |
| Automatic events seen | `scroll` (90%), `click` (outbound), `file_download` (the zip), `form_start` (login form), `view_search_results` (a URL carrying `?q=`), `user_engagement` |
| Events written by Solvr | none |
| Search box on `/posts` | the address does not change, so GA sees nothing; the server logs the search |

So the property's Enhanced Measurement is on for history changes, scrolls, outbound clicks, site search,
forms and downloads. That is read from behaviour; the settings themselves are unavailable.

### 5.2 The server funnel

| Contract step | Emitted by | Verified |
|---|---|---|
| `connection_started` | `components/connect/connect-panel.tsx:97` | fires on `/connect` load and on "Connect agents now" (home) |
| `starter_prompt_copied` | `connect-panel.tsx:111` | fires on `/connect`; **not** from the homepage's copy buttons |
| `room_created` | backend | fires (local) |
| `participant_joined` | backend | fires (local) |
| `first_two_way_exchange` | backend | fires once per room (local) |
| `room_viewed` | `components/rooms/room-detail-client.tsx:95` | fires on public rooms, not on a private room's gate |
| `join_prompt_copied` | `components/rooms/connect-agent-panel.tsx:62` | fires from Get join prompt, then Copy join prompt, once the F01 dialog is dismissed (local) |
| `share_visit` | `hooks/use-share-visit.ts:41` | fires on a room opened with `?via=share`; not on posts |
| `share_link_copied` | `components/rooms/share-outcome.tsx:40` | fires from Copy outcome, then Copy (local); **the Share button does not report** |

Production, 30-day baseline at 2026-10-05 02:07 UTC. **These numbers include the author session's browser
tests and cover three days of real data.**

| Step | Count |
|---|---|
| `connection_started` from `/connect` | 49 |
| `connection_started` from the homepage panel | 4 |
| `room_created` | 15 |
| `participant_joined` | 14 |
| `first_two_way_exchange` | 6 |
| `room_viewed` | 19 |
| Rooms activated | 5 |
| Searches / with no result | 255 / 9 |

### 5.3 Who records what

| Action | GA | Funnel | Elsewhere on the server |
|---|---|---|---|
| Page view | yes | no | request log |
| Search | only for a URL with `?q=` | no | `search_queries` |
| Copy a prompt on `/connect` | no | yes | no |
| **Copy a prompt on the homepage** | **no** | **no** | **no** |
| Agent creates a room, joins, first exchange | no | yes | rooms tables |
| Open a room | yes | yes | no |
| **Share a room with the Share button** | **no** | **no** | **no** |
| Share a room through Copy outcome | no | yes | no |
| Open a post | yes | no | view counter never called |
| Open a blog post | yes | no | blog view counter |
| Sign up, log in | `form_start` only | no | users table |
| Vote, reply, post | no | no | their own tables |
| Outbound click, zip download, scroll | yes (automatic) | no | no |
| An agent fetches `skill.md`, `install.sh` or `llms.txt` | no | no | none found: they are static files (INFERRED) |

The last row matters for an agent-first product: the first thing an agent does is read `skill.md`, and
nothing in the application counts it. Cloudflare's logs would.

### 5.4 Search Console

- MEASURED: the DNS zone has a `google-site-verification` record. There is no verification meta tag and no
  `BingSiteAuth.xml`. `robots.txt` names the sitemap index.
- Unavailable: whether a property exists, sitemap status, indexed pages, queries, crawl stats.

## 6. Technical layer

| Check | Result (MEASURED) |
|---|---|
| `robots.txt` | blocks three crawlers that only spend crawl budget, allows the rest, names the sitemap; the `Host:` line is not a standard directive |
| API host | `api.solvr.dev/robots.txt` disallows everything, so API JSON stays out of search |
| Legacy URLs | 60 of 60: one 308, same id, query kept, target 200; an unknown id ends in a real 404 |
| Canonical host | `http://` and `https://www` redirect to the apex; `http://www` loops (F11; fixed 2026-10-05, see F11) |
| Trailing slash, case | trailing slash 308 to the bare path; upper-case paths 404; `//posts` answers 200 |
| Status codes | 404 with `noindex` on 25 of 25 not-found URLs; private room 200 `noindex` with no transcript, its transcript 404; 11 of 11 deleted accounts 404 and out of the sitemaps |
| Response time | median time to first byte 136 to 259 ms over 19 page types, five runs each, from Brazil |
| Compression, protocol | Brotli, zstd and gzip offered; HTTP/2; HTTP/3 advertised |
| Edge cache | `cf-cache-status: DYNAMIC` on every HTML response, including pages that send `s-maxage` |
| Core Web Vitals | lab only (F09); field data unavailable |

## 7. Sitemap reconciliation (MEASURED)

| Sitemap | In the index | URLs | API list | Counts endpoint | Baseline | Only in one | `lastmod` differs |
|---|---|---|---|---|---|---|---|
| core | yes | 21 | 21 in `route-policy.ts` | n/a | n/a | 0 | none carried, by design |
| posts | yes | 467 | 467 | 467 | 467 | 0 | 0 |
| agents | yes | 71 | 71 | 71 | 71 | 0 | 0 |
| blog | yes | 32 | 32 | 32 | 32 | 0 | 0 |
| rooms | yes | 28 | 28 | 28 | 28 | 0 | 0 |
| users | **no** | 354 | 354 | **0** | n/a | 0 | 0 |
| room transcripts | no sitemap | n/a | 31 pages over 28 rooms | n/a | 31 | n/a | n/a |

Indexed total: 619. The public API lists 29 public rooms; the one outside the sitemap is one-sided and
`noindex`, as the rule says. No post has more than 22 replies, so no second reply page exists on production.

## 8. Fix plan — approve or reject each line

Nothing below has been done. Size: S is under an hour, M is a day, L is more.

| # | Fix | Finding | Size | Decision |
|---|---|---|---|---|
| 1 | Stop the login dialog on public room pages; test it | F01 | S | ☐ |
| 2 | `/connect`: server-rendered text and an `<h1>` | F02 | S | ☐ |
| 3 | Rewrite the four guides as full how-tos; add one that names Claude Code and Codex | F02 | M | ☐ |
| 4 | Align the headings of `/how-it-works`, `/about`, `/skill`, `/mcp`, `/api-docs` with the current positioning | F02 | S | ☐ |
| 5 | Crawlable post archive pages; posts listed on profile pages; related posts | F03 | M | ☐ |
| 6 | Link `/docs/protocol` in HTML; list the resume guide or take it out of the sitemap | F03 | S | ☐ |
| 7 | Default preview image, `og:url`, page titles in Open Graph | F04 | S, plus the image | ☐ |
| 8 | Report the homepage copy buttons to the funnel | F05 | S | ☐ |
| 9 | Decide which key events GA receives; send them | F05 | M | ☐ |
| 10 | Carry the flow from the website into the room, or drop the website-to-room rate | F05 | M | ☐ |
| 11 | Share button: report the step and add `?via=share`; `share_visit` on posts; call or delete the post view hook; origin marker on funnel rows | F05 | S | ☐ |
| 12 | `htmlLimitedBots: /.*/` | F06 | S | ☐ |
| 13 | Profiles: `noindex` the empty ones or give them content; settle the users sitemap; fix or drop `SoftwareApplication` | F07 | M | ☐ |
| 14 | Clean blog descriptions; add `author` to `BlogPosting` | F08 | S | ☐ |
| 15 | Load the Google tag after the page is idle and re-measure; decide whether Cloudflare's beacon stays | F09 | S | ☐ |
| 16 | Consent, Google signals, and what `/privacy` says | F10 | Felipe | ☐ |
| 17 | `www` redirect rule and HSTS in Cloudflare | F11 | S | ☑ `www` done by Felipe 2026-10-05; HSTS open (optional) |
| 18 | `/notifications` `noindex`; hide the Edit link from non-owners; fix the two retired links; 404 title; stop showing an email address as a profile name | F12 | S | ☐ |
| 19 | Sitemaps and collections answer 503 when the API is down | F13 | S | ☐ |
| 20 | Update `CLAUDE.md` and `SPEC.md` 27.1 and 27.5 | F14 | S | ☐ |

My order: 1, 7, 12 and 8 are small and each removes a loss that happens on every visit. Then 2, 3 and 5,
which are what make the site findable for the searches in section 2.

## 9. Unavailable, and what unblocks it

| Missing | Why | Unblocked by |
|---|---|---|
| Search Console and GA4 | read on 2026-10-05 | section 12. Still unread: Search Console's crawl stats and page-indexing totals (no tool for them) |
| Real-user Core Web Vitals | PageSpeed API out of daily quota, then no answer on a second try; Chrome UX Report needs a key | Search Console once readable, or the Cloudflare Web Analytics dashboard |
| How third-party scripts delay the first frame | narrowed in F09, mechanism open | load the tag later and re-measure, or a DevTools trace with rendering categories |

With a working key I would use only these read tools: Search Console list sites, search analytics query,
list and get sitemap, inspect URL; Analytics list accounts, properties and data streams, key events, custom
dimensions, retention and signals settings, run report. The toolkit also offers tools that write
(`SEND_EVENTS`, `CREATE_*`, `UPDATE_PROPERTY`, `SUBMIT_SITEMAP`, `ADD_SITE`, `DELETE_SITE`); I will not call
them.

## 10. What this recon added to production

- About 1,900 GET requests over two hours, at most two per second. Each page render calls the API, and
  the API logs requests in `api_request_events`, which feeds "API usage" on `/data`.
- **18 Lighthouse page loads from this machine, and 14 of them added a page view to GA.** My block list
  covered `analytics.google.com` but not the tag's fallback, `www.google.com/g/collect`. The tag retried
  there and Google answered 204. Those 14 are the runs of 2026-10-05 02:04 to 02:07 and 02:35 to 02:36 UTC:
  each a new visitor and a new session, mobile emulation, from Brazil, on the 12 URLs of
  `harness/lighthouse.sh` plus the post and transcript pages again. The last 4 runs blocked the tag itself
  and sent nothing. I found this by reading the request list of each run, after I had already told Felipe
  the runs were clean.
- Funnel rows added: 0. The funnel and view-counter calls show as blocked in every run, and the counts were
  identical before and after (49, 4, 15, 14, 6, 19).
- Cloudflare Web Analytics: 16 of the 18 loads sent its beacon.
- No search was run on production and no content was created. The only POST was one read-only SELECT
  through the admin query route, which the owner allowed on 2026-10-04 for the audit.
- The local test stack (worktree, database `solvr_lane_seo_recon`, two processes) was created and removed
  twice; nothing of it remains. `git status` shows only this untracked folder.

## 11. Method, deviations and evidence

**Deviations from PLAN-r1:**

- Google data was not read (Composio key scope). The owner's decision to read it through Composio stands.
- PageSpeed Insights was replaced by Lighthouse run locally against production (the API quota was
  exhausted), which also let me block the calls that would have added events. No field data came with it.
- The `site:` checks became the non-brand searches of section 2, at Felipe's direction during the run.
- A blog fixture was inserted by SQL in the scratch database (the local rate limit refused the API call).
- Posts awaiting moderation were found on production with one read-only SELECT through
  `POST /admin/query`, as the owner's decision of 2026-10-04 allows.
- The first Lighthouse block list missed one Google endpoint (section 10). `harness/lighthouse.sh` is
  corrected.

**Harness (`harness/`):** `inventory.mjs` (page types and crawl list), `crawl.mjs` and `lib.mjs` (GET-only
crawler), `analyze.mjs`, `technical.mjs`, `lighthouse.sh`, `local-guard.mjs` (browser that can reach only
the local stack and `gtag.js`), `local-seed.mjs`, `local-browser.mjs`, `local-browser-extra.mjs`, `local-browser-room-actions.mjs`. The crawler
reuses the exported checks of `frontend/scripts/seo-verify.mjs`. `psi.mjs` is kept but produced nothing.

**Evidence (`evidence/`):**

| File | Holds |
|---|---|
| `commands.log` | every command, with its UTC time |
| `inventory.json`, `crawl-list.json` | page types, sitemaps, API lists, the 1,094 URLs crawled |
| `crawl.jsonl`, `crawl-second-hop.jsonl` | one row per URL: status, headers, every tag, links |
| `analysis.json` | the tables of sections 3, 4 and 7 |
| `technical.json` | host matrix, legacy redirects, API host, compression, timings, six user agents |
| `lighthouse.json` | scores, lab metrics and failed audits for 12 URLs |
| `local-browser.json`, `local-browser-extra.json`, `local-browser-room-actions.json`, `local-guard-selftest.json` | GA hits and funnel calls seen in the guarded browser |
| `first-paint.json` | every first-paint measurement of F09, production and local |
| `nonpublic-posts-production.json` | pages of pending, rejected, family-only and deleted posts on production |
| `api-down-status.json` | status codes with the local API up and stopped |
| `nonbrand-search.json`, `nonbrand-onpage.json` | section 2 |
| `seo-baseline-2026-10-05-{24h,7d,30d}.json`, `admin-reports-reduced.json` | server baseline and admin reports, aggregates only |
| `next-build-routes.txt` | JavaScript size per route from the build |
| `room-login-modal-*.{jpg,png}` | F01 |

Google's rules quoted in F01, F06 and F07 were read from developers.google.com on 2026-10-04; the
`htmlLimitedBots` behaviour from nextjs.org the same day.

## 12. Google data (read 2026-10-05, read-only, `evidence/google-search-console.json`, `evidence/google-analytics.json`)

All MEASURED through the Search Console and GA4 APIs. Search Console lags about two days.

### 12.1 Search Console (`sc-domain:solvr.dev`, data since 2026-02-04)

| Month | Clicks | Impressions | Average position |
|---|---|---|---|
| 2026-02 | 13 | 92 | 4.6 |
| 2026-03 | 11 | 283 | 5.9 |
| 2026-04 | 8 | 720 | 6.6 |
| 2026-05 | 21 | 1,020 | 6.0 |
| 2026-06 | 24 | 1,390 | 8.2 |
| 2026-07 | 7 | 801 | 11.7 |
| 2026-08 | 21 | 1,202 | 13.5 |
| 2026-09 | 76 | 2,276 | 7.1 |
| 2026-10 (4 days) | 11 | 166 | 5.1 |

- **Every organic click is a brand click.** Last 90 days: 74 queries. The 13 that contain "solvr" brought 80
  clicks from 1,698 impressions. The 61 others brought **0 clicks** from 163 impressions.
- **The site has never been shown for "connect … agents".** One query in 90 days mentions "agent" or
  "connect" ("claude subscription with hermes agent", 1 impression, position 42). This confirms section 2
  with Google's own data.
- The non-brand impressions are exact error strings and niche terms answered by a post, a room or a blog
  post (an OpenClaw gateway error, an ESP32 jammer name), and misspellings of the brand.
- 94 pages had an impression in 90 days. The homepage holds 2,078 of them and 92 clicks; 12 rooms hold 606
  and no click; 21 blog posts hold 553 and 10 clicks; 37 legacy `/ideas` and `/problems` URLs still hold 552.
  **No `/posts/…` URL has an impression yet.**
- Countries by impressions: United States 1,908, India 318, United Kingdom 135, Brazil 98.

**Google does not know the new site.** URL inspection, 16 URLs:

| URL | Google says |
|---|---|
| `/`, `/skill`, `/rooms/plant-faces-v0`, `/agents/agent_claude_opus_eval` | Submitted and indexed |
| `/connect`, `/docs`, `/posts`, three `/posts/{id}`, a guide, a room transcript, a user profile | **URL is unknown to Google** |
| `/rooms/fix-openclaw-oauth-cli`, the welcome blog post, a legacy `/problems/{id}` | Crawled, currently not indexed |

- **The sitemap index was last downloaded by Google on 2026-04-06, the day it was submitted.** Google has
  not read it since, so it has never seen the `/posts/…` URLs that replaced `/problems`, `/ideas` and
  `/questions` on 2026-10-02, nor `/connect` or `/docs`.
- **Proposed action, Felipe's:** resubmit `https://solvr.dev/sitemap.xml` in Search Console (Sitemaps,
  enter the URL, Submit). It takes one minute and is the fastest way to get the new URLs crawled. I did not
  submit it: sitemap submission through the key was excluded on 2026-10-04.

### 12.2 GA4 (`properties/523300499`, 2026-07-07 to 2026-10-04)

- 645 sessions, 619 users, 817 page views. 169 sessions were engaged (26%); average session 48 s.
- Channels: Direct 477, Organic Search 143 (Google 98, Bing 42), Referral 14, Organic Social 4, one session
  from chatgpt.com.
- **Events received: only Google's automatic ones.** `page_view` 817, `session_start` 642, `first_visit` 613,
  `user_engagement` 324, `scroll` 200, `form_start` 6, `view_search_results` 1. No outbound click, no
  download, no product event.
- Sessions by month: February 838, March 677, April 239, May 135, June 83, July 166, August 235,
  September 171. Since v1.3: 10, 16 and 36 sessions on October 2, 3 and 4; the last figure includes the
  author session's browser tests and my 14 Lighthouse page views.
- Landing pages: `/` 211, `/blog` 15, `/leaderboard` 12, `/settings/agents` 12. Eight sessions landed on
  the literal path `/rooms/$`; I have not found what links to it.
- `/auth/callback` shows 11 page views in the window, from before it became an untracked path (2026-09-28).
- Hostnames: `solvr.dev` 644 sessions, `localhost` 1.
- **Settings:** event data is kept for **two months** (the minimum; 14 months is available). Google signals
  is **enabled**. Key events are GA defaults (`purchase`, `close_convert_lead`, `qualify_lead`, `form_start`,
  `file_download`); none is a Solvr action. No custom dimension exists.

### 12.3 What this changes

1. Findability is worse than section 2 inferred: non-brand search brings nothing, and Google has not read
   the sitemap since April. Resubmitting the sitemap comes before any content work.
2. The measurement batch should also set retention to 14 months and replace the default key events. Both
   are GA settings: Felipe's, or mine if he allows writes through the key.


## 13. After the fix plan (local build, 2026-10-05)

Felipe approved the fix plan on 2026-10-05 and it was carried out overnight in the worktree
`/Users/fcavalcanti/dev/solvr-lanes/lane-seo` (branch `lane/seo-fixes`, base `d6f41457`). **Nothing is
committed, merged or deployed.** The whole change is `patches/after-batch-D1b.patch`; each batch's state is in `patches/after-batch-<X>.patch` and the log is in `FIX-PLAN.md`.
Everything below was MEASURED on a local build against a scratch database, with curl, a network-guarded
browser, real agent CLIs and Lighthouse. Production numbers can only be measured after a deploy.

| # | Fix (section 8) | Status | Measured on the local build |
|---|---|---|---|
| 1 | Login dialog on public rooms | Done (A) | Anonymous room page: no dialog, no failed request |
| 2 | `/connect` text and `<h1>` | Done (D1, D1b) | h1 "Connect two agents"; the 53-word sentence, three steps and links in the server HTML (was 40 words, no h1) |
| 3 | Guides | Done (D3) | 9 guides, 387 to 689 words of their own text; 5 per agent (Claude Code, Codex, Kimi Code, Hermes, OpenClaw); each says which agents were run (Claude Code, Codex and Kimi Code were; Hermes and OpenClaw were not, and say so) |
| 4 | Headings | Done (D1) | `/how-it-works`, `/about`, `/skill`, `/api-docs`, `/blog`, `/docs/guides` now say Solvr connects agents; `/mcp` unchanged |
| 5 | Post archive, profile post lists | Done (D1); related posts not done | 139 of 139 sitemap posts linked from archive pages in the D1 run; 0 listed posts without an inbound link in the final crawl |
| 6 | `/docs/protocol` link; resume guide | Done (D1, D3) | In the docs menu of every page's HTML; resume guide listed and corrected |
| 7 | Preview image, `og:url`, page titles | Done (B) | 0 indexable pages without og:image; 0 pages besides `/` carrying the home og:title. The OpenAI key answered 401, so the card was built in code (three options in `preview-options/`, option 1 wired) |
| 8 | Homepage copy buttons | Done (A) | `starter_prompt_copied` stored for the home cards and the guides |
| 9 | GA key events and events | Done in code (C); GA settings are Felipe's | 29 events, each seen firing once with its parameters; list of key events and custom dimensions in `SPEC.md` 27.7 |
| 10 | Website visit to room | Done (F, F2) | Real Claude Code and Codex agents given the local sentence: each flow stored `connection_started`, `starter_prompt_copied`, `skill_fetched`, `room_created`, `participant_joined` |
| 11 | Share, share visits on posts, post views | Done (A); origin marker on funnel rows not done | Share copies `…?via=share` and reports; a post's share visit and view are stored |
| 12 | `htmlLimitedBots` | Done (A) | 0 indexable pages with title, description or canonical in `<body>` (Googlebot user agent) |
| 13 | Profiles | Done (D2, E) | Profiles with no public content `noindex, follow` and in no sitemap; the users sitemap lists exactly the indexable people; no e-mail shown as a name; `SoftwareApplication` gone |
| 14 | Blog descriptions, `author` | Done (D2) | 0 blog descriptions with Markdown; NULL blog columns no longer answer 500 |
| 15 | Google tag after idle | Done (C) | 0 Google requests before consent (was 5 per page); total blocking time about halved on every page; LCP unchanged locally (the production gain needs a deploy to measure) |
| 16 | Consent, signals, `/privacy` | Done in code (C) | 0 Google requests before Accept, after Decline, under Global Privacy Control; signals and ad personalisation off per hit; six false sentences removed from `/privacy` |
| 17 | `www` redirect, HSTS | `www` done by Felipe (2026-10-05); HSTS open (optional) | one 301 from `http://www` and `https://www` to `https://solvr.dev/<path>?<query>` |
| 18 | Index hygiene | Done (A, B, D2) | `/notifications` noindex; Edit link only for the author; dead links fixed; 404 titled and without canonical |
| 19 | 503 when the API is down | Done (E) | Content sitemaps 503 with `Retry-After: 120`; collection and detail pages a retryable 500 (Next 15 has no 503 helper) |
| 20 | Docs drift | Done (E) | `CLAUDE.md` and `SPEC.md` 27.1 describe the sitemap index and routes as they are |

**Lighthouse, mobile, local build, median of 3 (before the plan → final):** home score 94 → 94, total
blocking time 94 → 34 ms; `/connect` 76 → 96, LCP 3330 → 2786 ms, CLS 0.302 → 0 (its sentence is now
in the server HTML, with the width of the flow code reserved); post and room pages unchanged in score,
blocking time 95–100 → 39–42 ms. Google requests per page before consent: 5 → 0.

**Not done, by design or left for Felipe:** related posts; an origin marker on funnel rows; the time-window
selectors, profile tabs, referral and pin copies are not tracked; archive pages are not in the sitemap
(reached by links); `/mcp` headings; Cloudflare's beacon (his call); everything that writes to production
or to Google (resubmitting the sitemap, GA settings, the two stored profile names).

### 13.1 On production, after the v1.3.15 deploy (2026-10-05 13:05 BRT)

Deployed by Felipe's word ("ship all, verify all after"). MEASURED on production right after the deploy:
- curl on every page type and the nine guides: one h1, metadata in `<head>` for Chrome and Googlebot user
  agents, `og:image` / `og:url` / `summary_large_image` everywhere, `noindex` where intended, 404 titled.
- Crawl of 1200 pages: 0 indexable pages without `og:image`, 0 besides `/` with the home `og:title`, 0
  with metadata in `<body>`; every sitemap URL answers 200 and is indexable. All 467 sitemap posts are
  linked from the 10 archive pages (F03: was 446 without any link).
- Sitemaps: posts 467, agents 78, users 62 (profiles with content only; F07 was 475 thin indexable
  profiles), blog 32, rooms 28, core 26.
- Guarded browser: consent bar, 0 Google requests before Accept, tag only after Accept; public room
  without the login dialog (F01); `/connect` keeps one flow code from first paint to its funnel step.
- Funnel chain: `/skill.md?f=<code>` stored `skill_fetched` through Cloudflare, and a test room created
  with the code stored `room_created` with the same flow (F05, row 10).
- Lighthouse, the same 12 URLs as section 9's F09 (mobile, simulated; before = one run at 02:04 UTC, after =
  median of 3): performance 69–94 → 84–94; median FCP 3164 → 2067 ms; median LCP 5076 → 3285 ms; total
  blocking time 44–164 → 23–48 ms; about 145 KB less per page. Table:
  `evidence/production-after-v1.3.15/lh-prod-compare.txt`.
- New, pre-existing: Markdown "#" headings inside post and blog bodies render as extra h1s (12 of 467
  posts, 9 of 32 blog posts).
