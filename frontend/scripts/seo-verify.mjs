#!/usr/bin/env node
// seo-verify: checks what a crawler without JavaScript sees on a running Solvr web
// app (SPEC.md Part 27). It fetches server HTML only, follows no redirect on its
// own, and exits non-zero when any expectation fails.
//
//   node scripts/seo-verify.mjs check --base http://127.0.0.1:3400 \
//        --index /,/posts --noindex /login,/posts?q=x
//   node scripts/seo-verify.mjs crawl --base http://127.0.0.1:3400 --room <slug> [--expect-messages N]
//   node scripts/seo-verify.mjs structured --base http://127.0.0.1:3400 --index /,/posts/<id>
//   node scripts/seo-verify.mjs legacy --base http://127.0.0.1:3400 --file legacy-urls.txt
//   node scripts/seo-verify.mjs sample --base http://127.0.0.1:3400 [--per 5] [--noindex /login,...]
//
// --site is the canonical origin pages declare (default https://solvr.dev).

// parseHead extracts what a crawler reads from a page's server HTML.
export function parseHead(html) {
  const metaContent = (name) => {
    const re = new RegExp(`<meta[^>]*name="${name}"[^>]*>`, 'i');
    const tag = html.match(re)?.[0];
    return tag ? decode(tag.match(/content="([^"]*)"/i)?.[1] ?? '') : undefined;
  };
  const canonicalTag = html.match(/<link[^>]*rel="canonical"[^>]*>/i)?.[0];
  const jsonLd = [...html.matchAll(/<script[^>]*type="application\/ld\+json"[^>]*>([\s\S]*?)<\/script>/gi)].map(
    (m) => m[1]
  );
  return {
    title: decode(html.match(/<title[^>]*>([\s\S]*?)<\/title>/i)?.[1] ?? ''),
    description: metaContent('description'),
    robots: metaContent('robots'),
    canonical: canonicalTag ? decode(canonicalTag.match(/href="([^"]*)"/i)?.[1] ?? '') : undefined,
    jsonLd,
  };
}

// extractLinks returns the same-site paths of the page's ordinary <a href> anchors,
// in document order, without duplicates. Script contents are ignored: a crawler
// without JavaScript never sees links that only scripts would render.
export function extractLinks(html) {
  const body = html.replace(/<script[\s\S]*?<\/script>/gi, '');
  const out = [];
  for (const m of body.matchAll(/<a\b[^>]*\bhref="([^"]+)"/gi)) {
    const href = decode(m[1]);
    if (!href.startsWith('/') || href.startsWith('//')) continue;
    if (!out.includes(href)) out.push(href);
  }
  return out;
}

export function isNoindex(robots) {
  return /(^|[\s,])noindex([\s,]|$)/i.test(robots ?? '');
}

function decode(s) {
  return s
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#x27;|&#39;/g, "'");
}

// fetchPage GETs one URL without following redirects, with a timeout.
export async function fetchPage(url, timeoutMs = 20000) {
  const res = await fetch(url, { redirect: 'manual', signal: AbortSignal.timeout(timeoutMs) });
  const html = res.status >= 300 && res.status < 400 ? '' : await res.text();
  return { status: res.status, location: res.headers.get('location'), html };
}

// checkPage compares one page with its expectation and returns the problems found.
export function checkPage(path, expect, page, site) {
  const problems = [];
  const head = parseHead(page.html);
  if (page.status !== 200) problems.push(`${path}: status ${page.status}, want 200`);
  if (expect === 'index') {
    if (isNoindex(head.robots)) problems.push(`${path}: robots "${head.robots}" on an indexable page`);
    const want = site + (path === '/' ? '/' : path.split('?')[0]);
    const got = head.canonical === site ? `${site}/` : head.canonical;
    if (got !== want) problems.push(`${path}: canonical ${head.canonical}, want ${want}`);
  } else if (!isNoindex(head.robots)) {
    problems.push(`${path}: robots "${head.robots ?? '(none)'}", want noindex`);
  }
  return problems;
}

// relLink returns the href of the page's first anchor with the given rel (prev/next).
export function relLink(html, rel) {
  const body = html.replace(/<script[\s\S]*?<\/script>/gi, '');
  for (const m of body.matchAll(/<a\b[^>]*>/gi)) {
    const tag = m[0];
    if (new RegExp(`\\brel="${rel}"`, 'i').test(tag)) return decode(tag.match(/href="([^"]+)"/i)?.[1] ?? '');
  }
  return undefined;
}

// messageAnchors counts the transcript messages a page renders (id="message-<seq>").
export function messageAnchors(html) {
  return [...html.matchAll(/\bid="message-(\d+)"/g)].map((m) => Number(m[1]));
}

// runCrawl walks what a crawler without JavaScript can reach for one room: the rooms
// list links the room; the room links its transcript pages and outcome posts; the
// transcript pages chain by rel=prev/next, link back to the room and to agent authors;
// every live message is reached; out-of-range and non-canonical pages are 404.
async function runCrawl(opts) {
  const problems = [];
  const rows = [];
  const slug = opts.room;
  const roomPath = `/rooms/${slug}`;
  const get = async (path) => {
    const page = await fetchPage(opts.base + path);
    rows.push({ path, status: page.status, location: page.location ?? null });
    return page;
  };
  const rooms = await get('/rooms');
  if (!extractLinks(rooms.html).includes(roomPath)) problems.push(`/rooms does not link ${roomPath}`);
  const room = await get(roomPath);
  const roomLinks = extractLinks(room.html);
  const historyLinks = roomLinks.filter((l) => l.startsWith(`${roomPath}/history/`));
  if (!historyLinks.includes(`${roomPath}/history/1`)) problems.push(`${roomPath} does not link its first transcript page`);
  const outcomes = roomLinks.filter((l) => /^\/posts\/[0-9a-f-]{36}$/.test(l));

  let path = `${roomPath}/history/1`;
  const seen = new Set();
  const authors = new Set();
  let messages = 0;
  let pages = 0;
  while (path && !seen.has(path)) {
    seen.add(path);
    const page = await get(path);
    pages++;
    const head = parseHead(page.html);
    if (page.status !== 200) problems.push(`${path}: status ${page.status}`);
    if (head.canonical !== `${opts.site}${path}`) problems.push(`${path}: canonical ${head.canonical}`);
    const links = extractLinks(page.html);
    if (!links.includes(roomPath)) problems.push(`${path}: no link back to ${roomPath}`);
    links.filter((l) => l.startsWith('/agents/')).forEach((l) => authors.add(l));
    messages += messageAnchors(page.html).length;
    path = relLink(page.html, 'next');
  }
  const lastPage = pages;
  for (const bad of [`${roomPath}/history/${lastPage + 1}`, `${roomPath}/history/01`, `${roomPath}/history/0`]) {
    const page = await get(bad);
    if (page.status !== 404) problems.push(`${bad}: status ${page.status}, want 404`);
  }
  for (const link of [...authors].slice(0, 3).concat(outcomes)) {
    const page = await get(link);
    if (page.status !== 200) problems.push(`${link}: status ${page.status}`);
  }
  if (opts.expectMessages && messages !== Number(opts.expectMessages)) {
    problems.push(`reached ${messages} messages, want ${opts.expectMessages}`);
  }
  rows.push({ summary: { room: slug, transcriptPages: pages, messages, authors: authors.size, outcomes: outcomes.length } });
  return { rows, problems };
}

// structuredProblems checks what a page's structured data and title tell a crawler
// (task idx 82): every JSON-LD block parses, every URL it names is absolute on the
// site, every date is a real timestamp, and the title names the brand once.
export function structuredProblems(path, html, site) {
  const head = parseHead(html);
  const problems = [];
  const types = [];
  const brand = (head.title.match(/solvr/gi) ?? []).length;
  if (brand !== 1) problems.push(`${path}: title "${head.title}" names Solvr ${brand} times`);
  for (const raw of head.jsonLd) {
    let data;
    try {
      data = JSON.parse(raw);
    } catch {
      problems.push(`${path}: a JSON-LD block does not parse`);
      continue;
    }
    types.push(data['@type']);
    const walk = (node, key) => {
      if (Array.isArray(node)) return node.forEach((n) => walk(n, key));
      if (node && typeof node === 'object') return Object.entries(node).forEach(([k, v]) => walk(v, k));
      if (typeof node !== 'string') return;
      if (['url', 'item', '@id', 'mainEntityOfPage', 'logo'].includes(key) && !node.startsWith(site)) {
        problems.push(`${path}: ${key} ${node} is not on ${site}`);
      }
      if (/^date/.test(key) && Number.isNaN(Date.parse(node))) problems.push(`${path}: ${key} "${node}" is not a date`);
    };
    walk(data, '');
  }
  return { problems, types };
}

// legacyProblems classifies one legacy URL (task idx 83): it must take exactly one
// permanent redirect to the post with the same id, and that post page must answer 200
// (still published) or 404 (purged or deleted) itself, never another redirect.
export function legacyProblems(path, first, second) {
  const id = path.split('/')[2];
  const want = `/posts/${id}`;
  if (first.status !== 308) return [`${path}: status ${first.status}, want 308`];
  const target = first.location ? new URL(first.location, 'http://x').pathname : '';
  if (target !== want) return [`${path}: redirects to ${target || '(none)'}, want ${want}`];
  if (second.status !== 200 && second.status !== 404) return [`${want}: status ${second.status}, want 200 or 404`];
  return [];
}

async function runLegacy(opts) {
  const { readFileSync } = await import('node:fs');
  const paths = readFileSync(opts.file, 'utf8').split('\n').map((l) => l.trim()).filter(Boolean);
  const problems = [];
  const tally = { checked: 0, live: 0, gone: 0 };
  for (const path of paths) {
    const first = await fetchPage(opts.base + path);
    const target = first.location ? new URL(first.location, 'http://x').pathname : '';
    const second = target ? await fetchPage(opts.base + target) : { status: 0 };
    const found = legacyProblems(path, first, second);
    problems.push(...found);
    tally.checked++;
    if (!found.length) tally[second.status === 200 ? 'live' : 'gone']++;
  }
  return { rows: [{ summary: tally }], problems };
}

// runSample checks that the sitemaps and the pages agree (task idx 83): every sampled
// sitemap URL answers 200 with itself as canonical and no noindex, the internal links
// on those pages resolve, and no noindex route is listed anywhere.
async function runSample(opts) {
  const per = Number(opts.per || 5);
  const local = (loc) => loc.replace(opts.site, opts.base);
  const locsOf = (xml) => [...xml.matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1]);
  const problems = [];
  const rows = [];
  const index = await fetchPage(`${opts.base}/sitemap.xml`);
  const listed = [];
  const links = new Set();
  for (const sub of locsOf(index.html)) {
    const subPage = await fetchPage(local(sub));
    const locs = locsOf(subPage.html);
    listed.push(...locs);
    for (const loc of locs.slice(0, per)) {
      const page = await fetchPage(local(loc));
      const path = loc.replace(opts.site, '') || '/';
      const head = parseHead(page.html);
      rows.push({ path, status: page.status, canonical: head.canonical ?? null, robots: head.robots ?? null });
      problems.push(...checkPage(path, 'index', page, opts.site));
      extractLinks(page.html).slice(0, 8).forEach((l) => links.add(l));
    }
  }
  for (const link of [...links].slice(0, 60)) {
    const page = await fetchPage(opts.base + link);
    if (page.status >= 400) problems.push(`internal link ${link}: status ${page.status}`);
  }
  for (const route of opts.noindex) {
    const hit = listed.find((l) => l.replace(opts.site, '') === route);
    if (hit) problems.push(`${route} is noindex but listed in a sitemap`);
  }
  rows.push({ summary: { sitemapUrls: listed.length, sampled: rows.length, linksChecked: Math.min(links.size, 60) } });
  return { rows, problems };
}

function parseArgs(argv) {
  const [mode, ...rest] = argv;
  const opts = { mode, base: 'http://127.0.0.1:3400', site: 'https://solvr.dev', index: [], noindex: [] };
  for (let i = 0; i < rest.length; i += 2) {
    const key = rest[i].replace(/^--/, '');
    const value = rest[i + 1] ?? '';
    opts[key.replace(/-([a-z])/g, (_, c) => c.toUpperCase())] =
      key === 'index' || key === 'noindex' ? value.split(',').filter(Boolean) : value;
  }
  return opts;
}

async function runCheck(opts) {
  const problems = [];
  const rows = [];
  for (const [expect, paths] of [
    ['index', opts.index],
    ['noindex', opts.noindex],
  ]) {
    for (const path of paths) {
      const page = await fetchPage(opts.base + path);
      const head = parseHead(page.html);
      rows.push({ path, expect, status: page.status, robots: head.robots ?? null, canonical: head.canonical ?? null });
      problems.push(...checkPage(path, expect, page, opts.site));
    }
  }
  return { rows, problems };
}

async function runStructured(opts) {
  const rows = [];
  const problems = [];
  for (const path of opts.index) {
    const page = await fetchPage(opts.base + path);
    const head = parseHead(page.html);
    const result = structuredProblems(path, page.html, opts.site);
    rows.push({ path, status: page.status, title: head.title, types: result.types, canonical: head.canonical ?? null });
    if (page.status !== 200) problems.push(`${path}: status ${page.status}`);
    problems.push(...result.problems);
  }
  return { rows, problems };
}

async function main() {
  const opts = parseArgs(process.argv.slice(2));
  const modes = { check: runCheck, crawl: runCrawl, structured: runStructured, legacy: runLegacy, sample: runSample };
  const run = modes[opts.mode];
  if (!run) {
    console.error(`usage: seo-verify.mjs <${Object.keys(modes).join('|')}> [--base URL] ...`);
    process.exit(2);
  }
  const { rows, problems } = await run(opts);
  for (const row of rows) console.log(JSON.stringify(row));
  console.log(problems.length === 0 ? `OK ${rows.length} checked` : `FAIL ${problems.length} problem(s)`);
  for (const p of problems) console.log(`  - ${p}`);
  process.exit(problems.length === 0 ? 0 : 1);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main().catch((err) => {
    console.error(err);
    process.exit(2);
  });
}
