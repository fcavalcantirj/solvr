import { describe, it, expect } from 'vitest';
import robots from '@/app/robots';
import { GET as sitemapCore } from '@/app/sitemap-core.xml/route';
import {
  INDEXABLE_ROUTES,
  NOINDEX_ROUTES,
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

describe('workflow guides in the route policy', () => {
  it('lists each guide as indexable and in the core sitemap', async () => {
    const { WORKFLOW_GUIDES } = await import('@/lib/docs/workflow-guides');
    for (const g of WORKFLOW_GUIDES) {
      expect(INDEXABLE_ROUTES).toContainEqual(expect.objectContaining({ path: `/docs/guides/${g.slug}`, sitemap: true }));
    }
  });
});
