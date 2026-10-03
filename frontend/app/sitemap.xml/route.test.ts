import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { GET } from './route';

const fetchMock = vi.fn();
beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const counts = {
  data: {
    posts: 3, agents: 2, users: 0, blog_posts: 1, rooms: 2,
    lastmod: { posts: '2026-09-03T10:00:00Z', agents: '2026-09-02T10:00:00Z', blog_posts: null, rooms: '2026-09-04T10:00:00Z' },
  },
};

function entries(xml: string) {
  return [...xml.matchAll(/<sitemap>\s*<loc>([^<]+)<\/loc>(?:\s*<lastmod>([^<]+)<\/lastmod>)?/g)].map((m) => [m[1], m[2] ?? null]);
}

describe('sitemap.xml index', () => {
  it('references the canonical posts sitemap, not legacy type-specific sitemaps', async () => {
    fetchMock.mockResolvedValue({ ok: true, json: async () => counts });
    const xml = await (await GET()).text();

    expect(xml).toContain('https://solvr.dev/sitemap-posts.xml');
    expect(xml).toContain('https://solvr.dev/sitemap-rooms.xml');
    expect(xml).not.toContain('sitemap-problems.xml');
    expect(xml).not.toContain('sitemap-ideas.xml');
  });

  // Task idx 83: lastmod is each sub-sitemap's newest material change, from the API,
  // never "today" on every request.
  it('dates each sub-sitemap by its newest material change', async () => {
    fetchMock.mockResolvedValue({ ok: true, json: async () => counts });
    const res = await GET();
    expect(entries(await res.text())).toEqual([
      ['https://solvr.dev/sitemap-core.xml', null],
      ['https://solvr.dev/sitemap-posts.xml', '2026-09-03T10:00:00Z'],
      ['https://solvr.dev/sitemap-agents.xml', '2026-09-02T10:00:00Z'],
      ['https://solvr.dev/sitemap-blog.xml', null],
      ['https://solvr.dev/sitemap-rooms.xml', '2026-09-04T10:00:00Z'],
    ]);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/sitemap/counts');
    expect(init).toMatchObject({ cache: 'no-store' });
  });

  it('names no lastmod rather than a made-up one when the API fails', async () => {
    fetchMock.mockRejectedValue(new TypeError('fetch failed'));
    const xml = await (await GET()).text();
    expect(xml).not.toContain('<lastmod>');
    expect(entries(xml)).toHaveLength(5);
  });
});
