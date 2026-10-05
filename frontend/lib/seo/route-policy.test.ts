import { describe, it, expect } from 'vitest';
import { existsSync } from 'node:fs';
import { join } from 'node:path';
import robots from '@/app/robots';
import { GET as sitemapCore } from '@/app/sitemap-core.xml/route';
import {
  INDEXABLE_PATTERNS,
  INDEXABLE_ROUTES,
  NOINDEX_ROUTES,
  POSTS_ARCHIVE,
  NOINDEX,
  collectionRobots,
  indexableMetadata,
  noindexMetadata,
  TITLE_TEMPLATE,
} from './route-policy';

// Task idx 80: one table decides which static routes may be indexed; robots.txt and
// the core sitemap are derived from it and must agree with it.

function disallowedFor(agent: string): string[] {
  const result = robots();
  const rules = Array.isArray(result.rules) ? result.rules : [result.rules];
  const rule = rules.find((r) => r.userAgent === agent);
  if (!rule?.disallow) return [];
  return Array.isArray(rule.disallow) ? rule.disallow : [rule.disallow];
}

describe('route policy', () => {
  it('lets crawlers fetch every noindex page, or they could never read its noindex', () => {
    const disallowed = disallowedFor('*');
    for (const route of NOINDEX_ROUTES) {
      for (const url of [route, `${route}/x`]) {
        const blockedBy = disallowed.filter((d) => url.startsWith(d));
        expect(blockedBy, `${url} is disallowed by robots.txt`).toEqual([]);
      }
    }
  });

  it('never blocks an indexable route in robots.txt', () => {
    const disallowed = disallowedFor('*');
    for (const { path } of INDEXABLE_ROUTES) {
      expect(disallowed.filter((d) => path.startsWith(d)), path).toEqual([]);
    }
  });

  // /notifications is the signed-in inbox: it was in neither table, so it was served
  // with the home page's title and no robots directive.
  it('keeps the signed-in inbox out of the index and out of the sitemap', () => {
    expect(NOINDEX_ROUTES).toContain('/notifications');
    expect(INDEXABLE_ROUTES.map((r) => r.path)).not.toContain('/notifications');
  });

  it('keeps indexable and noindex routes apart', () => {
    for (const { path } of INDEXABLE_ROUTES) {
      expect(NOINDEX_ROUTES.some((n) => path === n || path.startsWith(`${n}/`)), path).toBe(false);
    }
  });

  it('lists exactly the sitemap routes in the core sitemap, as absolute URLs', async () => {
    const xml = await (await sitemapCore()).text();
    const locs = [...xml.matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1]).sort();
    const want = INDEXABLE_ROUTES.filter((r) => r.sitemap)
      .map((r) => (r.path === '/' ? 'https://solvr.dev/' : `https://solvr.dev${r.path}`))
      .sort();
    expect(locs).toEqual(want);
    for (const route of NOINDEX_ROUTES) {
      expect(xml).not.toContain(`<loc>https://solvr.dev${route}</loc>`);
    }
  });

  it('lists the docs, the protocol and the connect page', async () => {
    const xml = await (await sitemapCore()).text();
    expect(xml).toContain('<loc>https://solvr.dev/docs</loc>');
    expect(xml).toContain('<loc>https://solvr.dev/docs/protocol</loc>');
    expect(xml).toContain('<loc>https://solvr.dev/connect</loc>');
  });
});

describe('collectionRobots', () => {
  it('indexes the bare collection', () => {
    expect(collectionRobots({})).toBeUndefined();
    expect(collectionRobots(undefined)).toBeUndefined();
  });

  it('noindexes internal search results and uncurated filters', () => {
    expect(collectionRobots({ q: 'planner' })).toEqual(NOINDEX);
    expect(collectionRobots({ sort: 'top', tag: 'go' })).toEqual(NOINDEX);
  });

  it('treats a tracking-only query as the canonical page', () => {
    expect(collectionRobots({ utm_source: 'news', utm_campaign: 'x', ref: 'abc' })).toBeUndefined();
  });
});

// Next passes a layout's title.template only to segments below it, and a layout whose
// title is a plain string passes none, so /docs/guides lost " | Solvr" under /docs.
describe('route layout titles', () => {
  it('keep the root title template for the pages nested below them', () => {
    expect(TITLE_TEMPLATE).toBe('%s | Solvr');
    expect(indexableMetadata('/docs', 'Docs', 'x'.repeat(50)).title).toEqual({ default: 'Docs', template: TITLE_TEMPLATE });
    expect(noindexMetadata('Settings').title).toEqual({ default: 'Settings', template: TITLE_TEMPLATE });
  });
});

// Link previews (recon finding F04): a route that stated only a title inherited the home
// page's whole preview. The two route helpers now state each route's own.
describe('route metadata helpers and the link preview', () => {
  it('an indexable route previews under its own title, description and canonical', () => {
    const metadata = indexableMetadata('/docs', 'Docs', 'x'.repeat(50));
    expect(metadata.openGraph).toMatchObject({
      title: 'Docs | Solvr',
      description: 'x'.repeat(50),
      url: 'https://solvr.dev/docs',
      siteName: 'Solvr',
      type: 'website',
    });
    expect(metadata.twitter).toMatchObject({ card: 'summary_large_image', title: 'Docs | Solvr', description: 'x'.repeat(50) });
    expect(metadata.alternates?.canonical).toBe('/docs');
  });

  it('a noindex route previews under its own title and names no address', () => {
    const metadata = noindexMetadata('Settings');
    expect(metadata.openGraph).toMatchObject({ title: 'Settings | Solvr', siteName: 'Solvr' });
    expect(metadata.openGraph).not.toHaveProperty('url');
    expect(metadata.twitter).toMatchObject({ card: 'summary_large_image', title: 'Settings | Solvr' });
    expect(metadata.robots).toEqual(NOINDEX);
    expect(metadata.alternates).toBeUndefined();
  });
});

describe('workflow guides in the route policy', () => {
  it('lists each guide as indexable and in the core sitemap', async () => {
    const { WORKFLOW_GUIDES } = await import('@/lib/docs/workflow-guides');
    for (const g of WORKFLOW_GUIDES) {
      expect(INDEXABLE_ROUTES).toContainEqual(expect.objectContaining({ path: `/docs/guides/${g.slug}`, sitemap: true }));
    }
  });

  // SPEC.md 27.5: nine guides, the five per-agent ones included, each one URL in the sitemap.
  it('puts all nine guide URLs in the core sitemap, and no guide the app does not have', async () => {
    const { WORKFLOW_GUIDES } = await import('@/lib/docs/workflow-guides');
    const xml = await (await sitemapCore()).text();
    const guideLocs = [...xml.matchAll(/<loc>https:\/\/solvr\.dev(\/docs\/guides\/[^<]+)<\/loc>/g)].map((m) => m[1]);
    expect(guideLocs.sort()).toEqual(WORKFLOW_GUIDES.map((g) => `/docs/guides/${g.slug}`).sort());
    expect(guideLocs).toHaveLength(9);
    for (const slug of ['claude-code', 'codex', 'kimi-code', 'hermes', 'openclaw']) {
      expect(guideLocs).toContain(`/docs/guides/${slug}`);
    }
  });
});

// SPEC.md 27.2: the post archive (/posts/page/{n}) is indexable, and its pages are counted by
// the API, so the policy names its pattern instead of each path.
describe('indexable patterns', () => {
  it('names the post archive, and no other', () => {
    expect(INDEXABLE_PATTERNS).toEqual([POSTS_ARCHIVE]);
    expect(POSTS_ARCHIVE.route).toBe('/posts/page/[n]');
  });

  it('each has its page in the app directory', () => {
    for (const { route } of INDEXABLE_PATTERNS) {
      expect(existsSync(join(process.cwd(), 'app', ...route.split('/').filter(Boolean), 'page.tsx')), route).toBe(true);
    }
  });

  it('writes a page number the one canonical way', () => {
    expect(POSTS_ARCHIVE.path(1)).toBe('/posts/page/1');
    expect(POSTS_ARCHIVE.path(12)).toBe('/posts/page/12');
    for (const path of ['/posts/page/1', '/posts/page/12', '/posts/page/467']) {
      expect(POSTS_ARCHIVE.matches(path), path).toBe(true);
    }
    for (const path of ['/posts/page/0', '/posts/page/01', '/posts/page/x', '/posts/page/1/', '/posts/page', '/posts/page/1.5', '/posts/page/-1', '/posts/p1']) {
      expect(POSTS_ARCHIVE.matches(path), path).toBe(false);
    }
  });

  it('is never blocked in robots.txt and never under a noindex route', () => {
    const disallowed = disallowedFor('*');
    for (const pattern of INDEXABLE_PATTERNS) {
      const path = pattern.path(2);
      expect(disallowed.filter((d) => path.startsWith(d)), path).toEqual([]);
      expect(NOINDEX_ROUTES.some((n) => path === n || path.startsWith(`${n}/`)), path).toBe(false);
    }
  });

  it('is no static route of the table, and is not in the core sitemap', async () => {
    const xml = await (await sitemapCore()).text();
    const locs = [...xml.matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1].replace('https://solvr.dev', ''));
    for (const pattern of INDEXABLE_PATTERNS) {
      expect(INDEXABLE_ROUTES.filter((r) => pattern.matches(r.path))).toEqual([]);
      expect(locs.filter((loc) => pattern.matches(loc))).toEqual([]);
    }
  });
});
