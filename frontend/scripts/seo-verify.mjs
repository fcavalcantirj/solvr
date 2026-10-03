#!/usr/bin/env node
// seo-verify: checks what a crawler without JavaScript sees on a running Solvr web
// app (SPEC.md Part 27). It fetches server HTML only, follows no redirect on its
// own, and exits non-zero when any expectation fails.
//
//   node scripts/seo-verify.mjs check --base http://127.0.0.1:3400 \
//        --index /,/posts --noindex /login,/posts?q=x
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

function parseArgs(argv) {
  const [mode, ...rest] = argv;
  const opts = { mode, base: 'http://127.0.0.1:3400', site: 'https://solvr.dev', index: [], noindex: [] };
  for (let i = 0; i < rest.length; i += 2) {
    const key = rest[i].replace(/^--/, '');
    const value = rest[i + 1] ?? '';
    opts[key] = key === 'index' || key === 'noindex' ? value.split(',').filter(Boolean) : value;
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

async function main() {
  const opts = parseArgs(process.argv.slice(2));
  const modes = { check: runCheck };
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
