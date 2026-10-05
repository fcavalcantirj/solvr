import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { readdirSync } from 'node:fs';
import { join } from 'node:path';
import * as index from './sitemap.xml/route';
import * as posts from './sitemap-posts.xml/route';
import * as agents from './sitemap-agents.xml/route';
import * as users from './sitemap-users.xml/route';
import * as blog from './sitemap-blog.xml/route';
import * as rooms from './sitemap-rooms.xml/route';

// SPEC.md 27.1 (Sitemaps): a crawler that read a sitemap while the API could not answer was
// told the site had no pages (200, empty url set). Every sitemap that reads the API now answers
// such a read with a retryable 503 (Retry-After: 120, no-store, no url set); only a real answer
// is a sitemap, and a real empty answer is still an empty url set at 200.

const fetchMock = vi.fn();
beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const answer = (status: number, body: unknown) => ({ ok: status >= 200 && status < 300, status, json: async () => body });

const NO_DATES = { posts: null, agents: null, users: null, blog_posts: null, rooms: null };

// Every sitemap that reads the API, with the body of a real answer that lists nothing.
const SITEMAPS: Record<string, { GET: () => Promise<Response>; empty: unknown }> = {
  '/sitemap.xml': { GET: index.GET, empty: { data: { posts: 0, agents: 0, users: 0, blog_posts: 0, rooms: 0, lastmod: NO_DATES } } },
  '/sitemap-posts.xml': { GET: posts.GET, empty: { data: { posts: [] } } },
  '/sitemap-agents.xml': { GET: agents.GET, empty: { data: { agents: [] } } },
  '/sitemap-users.xml': { GET: users.GET, empty: { data: { users: [] } } },
  '/sitemap-blog.xml': { GET: blog.GET, empty: { data: { blog_posts: [] } } },
  '/sitemap-rooms.xml': { GET: rooms.GET, empty: { data: { rooms: [] } } },
};

const NOT_A_LIST = { posts: 'x', agents: 'x', users: 'x', blog_posts: 'x', rooms: 'x', lastmod: 'x' };

// Every way a read is not an answer.
const FAILURES: [string, () => void][] = [
  ['the API answers 500', () => fetchMock.mockResolvedValue(answer(500, { error: { code: 'INTERNAL_ERROR' } }))],
  ['the API answers 503', () => fetchMock.mockResolvedValue(answer(503, { error: { code: 'SERVICE_UNAVAILABLE' } }))],
  ['the API answers 429', () => fetchMock.mockResolvedValue(answer(429, { error: { code: 'RATE_LIMITED' } }))],
  ['the API answers another non-2xx (404)', () => fetchMock.mockResolvedValue(answer(404, { error: { code: 'NOT_FOUND' } }))],
  ['the API cannot be reached', () => fetchMock.mockRejectedValue(new TypeError('fetch failed'))],
  ['the body is not JSON', () => fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => { throw new SyntaxError('Unexpected token <'); } })],
  ['the body carries no data', () => fetchMock.mockResolvedValue(answer(200, { error: { code: 'INTERNAL_ERROR' } }))],
  ['the list is not a list', () => fetchMock.mockResolvedValue(answer(200, { data: NOT_A_LIST }))],
];

describe('a sitemap whose API read fails', () => {
  for (const [path, sitemap] of Object.entries(SITEMAPS)) {
    for (const [failure, arrange] of FAILURES) {
      it(`${path}: ${failure} → 503, Retry-After 120, no-store, no url set`, async () => {
        arrange();
        const res = await sitemap.GET();
        const body = await res.text();
        expect(res.status).toBe(503);
        expect(res.headers.get('Retry-After')).toBe('120');
        expect(res.headers.get('Cache-Control') ?? '').toContain('no-store');
        expect(res.headers.get('CDN-Cache-Control')).toBeNull();
        expect(body).not.toMatch(/<urlset|<sitemapindex|<url>|<sitemap>/);
      });
    }
  }
});

describe('a sitemap whose API read lists nothing', () => {
  for (const [path, sitemap] of Object.entries(SITEMAPS)) {
    it(`${path}: a real empty answer is a 200`, async () => {
      fetchMock.mockResolvedValue(answer(200, sitemap.empty));
      const res = await sitemap.GET();
      const body = await res.text();
      expect(res.status).toBe(200);
      expect(res.headers.get('Retry-After')).toBeNull();
      if (path === '/sitemap.xml') {
        // The index still names every sub-sitemap; it only has no date for them.
        expect(body).toContain('<sitemapindex');
        expect(body).not.toContain('<lastmod>');
      } else {
        expect(body).toContain('<urlset');
        expect(body).not.toContain('<url>');
      }
    });
  }
});

// The orphan /sitemap-users.xml existed for months without being named by the index: no
// crawler read it. Every sitemap route is named by the index, and the index names no other.
describe('the sitemap index and the sitemap routes', () => {
  it('name each other exactly', async () => {
    fetchMock.mockResolvedValue(answer(200, SITEMAPS['/sitemap.xml'].empty));
    const xml = await (await index.GET()).text();
    const named = [...xml.matchAll(/<loc>https:\/\/solvr\.dev\/([^<]+)<\/loc>/g)].map((m) => m[1]).sort();
    const routes = readdirSync(join(process.cwd(), 'app')).filter((name) => /^sitemap-.+\.xml$/.test(name)).sort();
    expect(named).toEqual(routes);
    // Every route that reads the API is in the failure matrix above.
    expect(Object.keys(SITEMAPS).sort()).toEqual(['/sitemap.xml', ...routes.filter((r) => r !== 'sitemap-core.xml').map((r) => `/${r}`)].sort());
  });
});
