---
slug: seo-360-recon
date: 2026-10-04
status: approved
round: 0
author_session: the session that shipped v1.3.7–v1.3.14 on 2026-10-04 and ran the full solvr.dev alignment pass (site, skill docs, API, OpenAPI)
---

# Handover — 360° SEO and tracking recon of solvr.dev

## Mission

A **read-only** 360° recon of solvr.dev's search visibility and measurement. The owner (Felipe) asks: *are we doing
everything, tracking everything, on all pages, sending it all to GA?* Answer with evidence, per page type: is every
public page crawlable, indexable where it should be, correctly described (title, description, canonical, Open Graph,
structured data), listed in the sitemap, and measured (GA4 page views and events, the server's own funnel, Google
Search Console)?

"Done" is one findings report: every claim measured, a gap list ranked by impact, and a fix plan Felipe can approve
item by item. **You fix nothing in this handover.**

## Where things stand (verified)

All `[REAL]` items were verified on 2026-10-04 against production or the code at `main`.

- [REAL] **Production is v1.3.14** — commit `a187ea0b`, journal commit `d6f41457` on `main`. Frontend: Next.js 15
  (App Router, ISR). API: Go. Site `https://solvr.dev`, API `https://api.solvr.dev`. Evidence: `progress.txt`
  (entries dated 2026-10-04), `git log --oneline -3`.

- [REAL] **GA4 is loaded on every page except three.** Evidence, verbatim from `frontend/app/layout.tsx:10`:

  ```ts
  const GA_MEASUREMENT_ID = process.env.NEXT_PUBLIC_GA_ID || 'G-HS74SKKSQY'
  ```

  and from `frontend/components/site-analytics.tsx`:

  ```ts
  const UNTRACKED_PATHS = ["/claim", "/auth/callback", "/email/unsubscribe"];
  ```

  Those three addresses carry secrets, so the tag is never loaded there. The prod HTML of `/`, a post, a room, an
  agent profile, a guide and `/blog` each contain `G-HS74SKKSQY` (checked with `curl -s -A 'Mozilla/5.0' <url> | grep -c G-HS74SKKSQY`).

- [REAL] **GA receives page views only: no custom event is ever sent.** `frontend/lib/analytics.ts` exports ten
  helpers (`trackPageView`, `trackPostView`, `trackPostCreate`, `trackVote`, `trackBookmark`, `trackReport`,
  `trackSearch`, `trackLogin`, `trackSignUp`, `trackEvent`) and none has a caller. Evidence:

  ```bash
  cd frontend && grep -rln "lib/analytics['\"]" app components hooks lib | grep -v "\.test\."   # prints nothing
  ```

- [REAL] **There is no cookie-consent or consent-mode code.** `grep -rniE "consent|cookie banner" frontend/app frontend/components`
  finds only prose on `/privacy`.

- [REAL] **The server has its own funnel, separate from GA.** `POST /v1/analytics/funnel` records steps;
  `GET https://api.solvr.dev/v1/analytics/funnel/contract` lists them:
  `connection_started, starter_prompt_copied, room_created, participant_joined, first_two_way_exchange, room_viewed, join_prompt_copied, share_visit, share_link_copied`.

- [REAL] **The server's SEO baseline** — `GET https://api.solvr.dev/admin/seo/baseline?window=30d` with the header
  `X-Admin-API-Key` (read-only, operator only):
  - indexable: 467 posts, 28 rooms, 31 room-history pages, 71 agents, 32 blog posts (629 content pages);
  - last 30 days: 255 searches (9 with zero results), 15 rooms created, 5 rooms activated, 6 first two-way
    exchanges, 48 `connection_started` from the connect page and 4 from the homepage panel;
  - milestones `sitemap_acceptance`, `crawling`, `indexing`, `traffic` and `core_web_vitals` are all
    **`unavailable`**: each depends on Google Search Console, which the server has never been given.

- [REAL] **robots.txt** (`https://solvr.dev/robots.txt`), verbatim:

  ```
  User-Agent: semrushbot
  Disallow: /

  User-Agent: ahrefsbot
  Disallow: /

  User-Agent: MJ12bot
  Disallow: /

  User-Agent: *
  Allow: /

  Host: https://solvr.dev
  Sitemap: https://solvr.dev/sitemap.xml
  ```

  No path is disallowed for other crawlers (not `/admin`, `/settings`, `/dashboard` or `/login`).

- [REAL] **The sitemap index lists five sitemaps**: `sitemap-core.xml` (22 URLs), `sitemap-posts.xml` (467),
  `sitemap-agents.xml` (71), `sitemap-blog.xml` (32), `sitemap-rooms.xml` (28). `https://solvr.dev/sitemap-users.xml`
  answers 200 but is **not** in the index.

- [REAL] **Head tags on six sampled pages** (`/`, `/posts/d8745348-0642-4900-a06b-37f0625f7532`,
  `/rooms/plant-faces-v0`, `/agents/agent_claude_opus_eval`, `/docs/guides/connect-planner-executor`, `/blog`):
  - title, description and canonical are present on all six;
  - JSON-LD is present on all but `/blog`;
  - **no page has `og:image`, `og:url` or `twitter:image`**, although `twitter:card` is `summary_large_image`;
  - no `hreflang`, and no `google-site-verification` meta (the domain may be verified by DNS: unknown).

- [REAL] **SEO machinery that already exists — reuse it, do not rebuild it:**
  - `SPEC.md` Part 27 "Search Visibility" (line ~6024): 27.1 index eligibility, 27.2 crawlable history,
    27.3 titles/structured data/internal links, 27.4 URL migration and status codes, 27.5 workflow guides,
    27.6 measurement.
  - `frontend/lib/seo/route-policy.ts` (which static routes are in the sitemap), `frontend/lib/seo/fetch-seo.ts`,
    `GET /v1/posts/{id}/seo`, `GET /v1/rooms/{slug}/seo`, `GET /v1/sitemap/urls`, `GET /v1/sitemap/counts`.
  - `backend/cmd/seo-report` (joins the saved baseline with Search Console Performance CSV exports),
    `scripts/seo/weekly-baseline.sh`, `frontend/scripts/seo-verify.mjs`.
  - Tests: `frontend/app/route-metadata.test.ts`, `frontend/app/seo-page-metadata.test.ts`,
    `frontend/app/titles.test.ts`, `frontend/app/robots.test.ts`, `frontend/lib/seo/route-policy.test.ts`.

- [REAL] **A site crawler and a page-text harness** from today's alignment pass are in `/tmp/solvr-orch/validate`
  (`crawl.py`: sitemap + one-hop link crawl with curl; `pagetext.mjs`: Playwright text and JS errors for 24 pages).
  `/tmp` may have been cleared; the method is journalled in `progress.txt` (2026-10-04, v1.3.13 entry). Result then:
  33 pages crawled, 331 links, 0 broken.

- [UNVERIFIED] An older analysis (memory note `solvr-usage-analysis`) recorded GA4 at 645 sessions/month, 5.1%
  organic, and 4 of 1,255 pages indexed. It predates the v1.3 URL migration: treat it as history, not as a baseline.

- [UNVERIFIED] Whether Google Search Console has a verified property for solvr.dev, and who can read GA4 and
  Search Console.

## Blocking constraints (builder: restate these before planning)

1. **Recon only.** No code change, commit, push or deploy; no DNS or Cloudflare change; no submission to any third
   party (Search Console, Bing Webmaster, IndexNow).
2. **Production is read-only.** Use GET requests and the read-only admin routes. Never POST to
   `/v1/analytics/funnel` and never create content on production to "test tracking": it pollutes the numbers you
   are auditing. Test event firing on a local build.
3. **Never state a number you did not measure in your own session.** Label every claim MEASURED or INFERRED.
   GA4 and Search Console figures you cannot read are "unavailable", never zero.
4. **Secrets stay out of every artifact.** The admin key is `ADMIN_API_KEY` in `/Users/fcavalcanti/dev/solvr/.env`:
   name the file, never print the value.
5. **Cloudflare refuses default script user agents** (403, error 1010, for `Python-urllib`): send a browser user
   agent on every request to solvr.dev.

## Accepted residuals / Refuted — don't fix

- The three untracked paths (`/claim`, `/auth/callback`, `/email/unsubscribe`) are deliberate: their URLs carry
  secrets, and GA sends the page address with every hit.
- `/ipfs` is out of the sitemap and the footer on purpose (IPFS is offline since April 2026); `/amcp` is
  `sitemap: false` on purpose.
- Legacy URLs (`/problems`, `/questions`, `/ideas`, `/feed`) were retired in v1.3. SPEC 27.4 owns their redirects
  and status codes, with review dates 2026-10-09, 2026-10-30 and 2026-11-27. Check them; do not redesign them.
- Cloudflare Email Address Obfuscation was turned off on 2026-10-04 because it rewrote page text and broke React
  hydration. Do not recommend turning it back on.
- The server funnel is the source of truth for activation. GA is not expected to replace it.

## Hard rules & human-reserved decisions

- Test output: save it to a file and read only `tail -n 10` (repo `CLAUDE.md`, rule 8). Never re-run a suite to see more.
- Never execute a `ralph*.sh` loop runner, not even to read its banner.
- At most 2–3 subagents; ask Felipe before any bulk or adversarial fan-out.
- If you do not know, say "I don't know" first, then say how you will find out.
- **Access to GA4 and to Search Console IS FELIPE'S CALL** — ask him how he wants to share the data (CSV export,
  screenshots, or API access). Do not try to obtain credentials.
- **Cookie consent / consent mode IS FELIPE'S CALL** (it is a legal decision).
- **Whether user profiles belong in the sitemap, and any change to robots.txt, IS FELIPE'S CALL.**
- **Every fix, and every deploy, IS FELIPE'S CALL**, item by item, after the report.

## Acceptance checklist (the author approves the plan ONLY against these)

1. The plan restates the five blocking constraints and changes nothing on production.
2. It enumerates **every page type** from the real sources (the five sitemaps, `frontend/lib/seo/route-policy.ts`,
   the `frontend/app/` route tree) and names the sample per type: all static routes, and at least 5 URLs of each
   dynamic type.
3. Per page type it measures, from production HTML: status code, title, description, canonical, robots meta,
   Open Graph and Twitter tags (image included), JSON-LD validity, `hreflang`, and the GA tag.
4. It checks **indexability both ways**: indexable pages are in a sitemap and not `noindex`; non-indexable pages
   (private rooms, posts awaiting moderation, auth-only pages) are out of the sitemap and `noindex` or blocked.
5. It audits **tracking in three layers**, each with evidence:
   - GA4: the tag, page views on client-side navigation, and which events fire (measured in a browser on a local build);
   - the server funnel: each contract event has a caller in the site code;
   - Search Console: what is verified, and what data exists.
6. It reconciles the sitemaps against the server baseline's indexable counts and against `GET /v1/sitemap/counts`.
7. It covers the technical layer: robots.txt, redirects and status codes of legacy URLs, canonical host (www vs
   apex, http vs https, trailing slash), response headers that affect crawling, and Core Web Vitals — or the reason
   they cannot be measured.
8. It lists exactly what it needs from Felipe (GA4 and Search Console data) and what it can conclude without it.
9. The deliverable is one findings report: each finding MEASURED or INFERRED, with its evidence, its impact and a
   proposed fix. Nothing is fixed.

## next_action

Reproduce each `[REAL]` fact above against production, then enumerate every page type from the five sitemaps and
`frontend/lib/seo/route-policy.ts`, and write `PLAN-r1.md` in this directory.

## Open questions

1. Is solvr.dev verified in Google Search Console, and how will Felipe share Search Console and GA4 data?
2. Should GA receive product events (search, sign-up, connect), or is the server funnel enough?
3. Is `/sitemap-users.xml` left out of the sitemap index on purpose?
4. Is there an Open Graph image the site should use, or does one need to be designed?

## Pointers

- Repo: `/Users/fcavalcanti/dev/solvr` — read `CLAUDE.md` first. `SPEC.md` Part 27 starts near line 6024.
- Tracking: `frontend/app/layout.tsx`, `frontend/components/site-analytics.tsx`, `frontend/lib/analytics.ts`,
  `frontend/lib/api.ts` (the funnel client).
- SEO: `frontend/lib/seo/`, `frontend/app/sitemap*.xml/`, `frontend/app/robots.ts`, `frontend/scripts/seo-verify.mjs`.
- Server: `backend/cmd/seo-report`, `scripts/seo/weekly-baseline.sh`, `GET /admin/seo/baseline`,
  `GET /v1/analytics/funnel/contract`, `GET /v1/sitemap/counts`.
- History: `progress.txt` (2026-10-04 entries, v1.3.10–v1.3.14); memory notes `site-alignment-2026-10-04` and
  `solvr-usage-analysis` under `~/.claude/projects/-Users-fcavalcanti-dev-solvr/memory/`.
- Secrets: `/Users/fcavalcanti/dev/solvr/.env` holds `ADMIN_API_KEY`. Never copy the value into any file.

## SUPERSEDED (2026-10-04, after REVIEW-r1)

Corrections to this handover. The text above is left as delivered.

- **Core sitemap:** 21 URLs, not 22; the sitemap index totals 619 URLs (the builder's measurement, PLAN-r1 D1).
- **"Access to GA4 and to Search Console IS FELIPE'S CALL"** is decided: read both through Composio (key file
  `~/.config/composio/apikey`; ACTIVE `google_analytics` and `google_search_console` connections). Read-only
  tools only. See "Owner decisions" in `REVIEW-r1.md`.
- **Lighthouse and PageSpeed against production** are allowed, on a small sample.
- **Read-only production queries** needed by the audit are allowed; do not ask Felipe about checks of that size.
- **Blocking constraint 2 is stricter than written:** no JavaScript-executing browser on production (it fires
  GA and funnel events), except the allowed Lighthouse/PageSpeed sample; and no `GET /v1/search` on production
  (each call writes a row).
