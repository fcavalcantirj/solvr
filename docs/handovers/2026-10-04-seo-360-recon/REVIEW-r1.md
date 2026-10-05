---
verdict: APPROVED
round: 1
---

# Review r1

Judged only against the nine checklist items and the five blocking constraints of `HANDOVER.md`.

## Checklist verdicts

1. **Constraints restated, production unchanged: satisfied.** All five are restated correctly, and the plan
   tightens constraint 2 with two rules I had missed: no JavaScript-executing browser on production, and no
   `GET /v1/search` there (each call writes a `search_queries` row).
2. **Every page type, with its sample: satisfied.** 39 static routes, all sampled; 9 dynamic patterns with
   named URLs. Guides have only 4 instances and reply pages may have none: taking every instance plus negatives
   meets the item.
3. **Per page type from production HTML: satisfied.** Status, title, description, canonical, robots meta,
   Open Graph and Twitter tags with image, JSON-LD validity, `hreflang` and the GA tag, on every sitemap URL
   rather than only the sample.
4. **Indexability both ways: satisfied.** Listed-and-clean for the 619 indexed URLs; unlisted-and-`noindex`/404
   for policy routes, private and one-sided rooms, account pages, deleted accounts and not-found URLs. Posts
   awaiting moderation depend on Question 1, answered below.
5. **Tracking in three layers: satisfied.** GA4 (tag from prod HTML; page views and events in a local browser
   behind a default-deny route), the server funnel (static callers, local firing, production rows read-only),
   Search Console (what is knowable without access, and the exact data asked from Felipe).
6. **Sitemap reconciliation: satisfied.** Sitemap files vs `GET /v1/sitemap/urls` vs `GET /v1/sitemap/counts`
   vs baseline `indexable`, by count and by id set.
7. **Technical layer: satisfied.** robots.txt, legacy redirects, canonical host matrix, crawl-relevant headers,
   and Core Web Vitals with the reason field data is unavailable.
8. **Needs from Felipe: satisfied.** The list is exact, the three ways to share are left to him, and the plan
   says what it can and cannot conclude without the data.
9. **One findings report, nothing fixed: satisfied.** `FINDINGS.md` with MEASURED/INFERRED, evidence, impact,
   proposed fix and decision owner per finding; cleanup by exact name and path.

## Handover discrepancies: accepted

All nine are accepted as corrections to the handover. Two matter beyond wording:

- **D1:** the core sitemap has 21 URLs and the index 619. My "22" was counted before v1.3.13 removed `/ipfs`.
- **D9 is correct, and larger than you measured.** My session loaded production in a JavaScript browser many
  times on 2026-10-04:
  - `pagetext.mjs` ran three times (24 pages each), so `/` and `/connect` each fired `connection_started` at
    least three times;
  - separate Playwright runs opened `/status`, `/mcp`, `/api-docs`, room pages and agent profiles for
    screenshots and hydration checks.

  I cannot give an exact count. State beside every 30-day funnel and GA number that it includes this test traffic.

## Answers to the builder's questions

1. **Pending or rejected post id on production:** I don't know one. Prove that row on the local build and label
   it "local only". `POST /admin/query` on production stays off-limits: it IS FELIPE'S CALL, not mine.
2. **Server-emitted funnel events:** agreed. Mapping `room_created`, `participant_joined` and
   `first_two_way_exchange` to their backend emitters satisfies item 5; "site code" in my wording should have
   said "the code".
3. **Report, evidence and harness under the handover directory, untracked and uncommitted:** agreed. Keep the
   admin responses reduced to counts and ids as you planned: this repository is public.
4. **Worktree, database and ports:** no clash, checked now. `~/dev/solvr-lanes` holds only `lane-f`, `lane-v`,
   `lane-v-astra` and `lane-v-astra2`; nothing listens on 18400 or 18401; no database named
   `solvr_lane_seo_recon` exists in the `solvr-postgres` container (port 5435). Release smoke tests use ports
   18763–18769, so stay off those.
5. **No Lighthouse or PageSpeed run against production:** agreed as the default; it follows from constraint 1.
   Whether to accept the extra events for production lab numbers IS FELIPE'S CALL: put it in your list of
   questions for him.
6. **Cloudflare 403/1010:** I don't know whether a rule changed. It was observed on 2026-10-03 with a
   `Python-urllib` user agent; the only Cloudflare change I know of on 2026-10-04 is Felipe turning Email
   Address Obfuscation off. Keep the browser user agent, as you planned.

## Notes for the run (not conditions of approval)

1. **Page fetches are not free of server-side records.** Each server render calls the API, and the API logs
   requests in `api_request_events`, which feeds the "API usage" figures on `/data`. About 1,500 page GETs will
   show there. Say so in the report and do the full crawl once.
2. **Timeout-guard every step that can hang:** `--max-time` on each curl, an explicit timeout on each Playwright
   step, and a time limit on the local build and on `migrate up`. A hang does not fail a run; it wedges it.
3. **The real GA ID in the local browser run** is acceptable only because of the default-deny route and the
   fake-ID self-test. If one request to a Google collect endpoint reaches the network, stop and report it.
4. **A local `solvr-ipfs` container is listening on 5001.** If your local API starts without `IPFS_API_URL`, it
   will reach that container and IPFS features will appear to work. Production has no node: do not report
   local IPFS behaviour as production behaviour.

## Closing

Builder is now the primary session. First move, from the handover's `next_action` (the reproduction of the
`[REAL]` facts is already done in your plan): run P0 and P1, the guard rails and the inventory, then the P2
production crawl.

Bring Felipe the "What I need from Felipe" list at the start, so his exports can arrive while the crawl runs.

## Owner decisions (Felipe, 2026-10-04, after this review)

These replace the three "IS FELIPE'S CALL" items above. Do not ask him again.

1. **GA4 and Search Console data: read it yourself through Composio.** Felipe's agents get this data through
   composio.dev. Verified on this Mac on 2026-10-04:
   - the API key is in `~/.config/composio/apikey` (mode 0600; never print it, pass it as the `x-api-key` header
     read from the file);
   - `GET https://backend.composio.dev/api/v3/connected_accounts?limit=100` lists the connections. ACTIVE today:
     `google_analytics` (2 accounts) and `google_search_console` (1 account). There are also EXPIRED ones:
     use only ACTIVE;
   - a working call pattern (a read-only Gmail fetch) is in
     `/Users/fcavalcanti/dev/dasbrow-hermes-coder/watchers/mail-watch.sh` (tool execution is
     `POST https://backend.composio.dev/api/v3/tools/execute/<TOOL_NAME>` with a `connected_account_id` and
     `user_id`).

   Not verified by me: which of the two Google Analytics accounts holds the property of `G-HS74SKKSQY`, and
   whether the Search Console account has a property for solvr.dev. Find out first (list the properties and
   sites). Use **read-only tools only**: no setting, filter, sitemap submission or URL inspection request that
   changes anything. So "exports and screenshots" are no longer needed, and the Search Console and GA4 parts of
   the report are expected to carry real numbers, not "unavailable".
2. **Lighthouse and PageSpeed runs against production: allowed.** Run them, on a small sample (one URL per page
   type), and state in the report how many page loads they added to GA and to the funnel.
3. **The pending-post question was noise: do not bring Felipe questions of that size.** He asked for a 360° SEO
   audit of the site. For a corner like "a post awaiting moderation", decide yourself: a read-only `SELECT`
   through the admin query route is allowed when the audit needs it, and proving it on the local build is also
   fine. Spend the effort on the audit, not on permissions for read-only checks.

Everything else in the handover stands: still recon only, still nothing written to production, still no fix
without Felipe.
