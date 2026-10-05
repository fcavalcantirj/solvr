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
    posts: 3, agents: 2, users: 1, blog_posts: 1, rooms: 2,
    lastmod: {
      posts: '2026-09-03T10:00:00Z', agents: '2026-09-02T10:00:00Z', users: '2026-09-05T10:00:00Z',
      blog_posts: null, rooms: '2026-09-04T10:00:00Z',
    },
  },
};

function entries(xml: string) {
  return [...xml.matchAll(/<sitemap>\s*<loc>([^<]+)<\/loc>(?:\s*<lastmod>([^<]+)<\/lastmod>)?/g)].map((m) => [m[1], m[2] ?? null]);
}

describe('sitemap.xml index', () => {
  it('references the canonical posts sitemap, not legacy type-specific sitemaps', async () => {
    fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => counts });
    const xml = await (await GET()).text();

    expect(xml).toContain('https://solvr.dev/sitemap-posts.xml');
    expect(xml).toContain('https://solvr.dev/sitemap-rooms.xml');
    expect(xml).not.toContain('sitemap-problems.xml');
    expect(xml).not.toContain('sitemap-ideas.xml');
  });

  // Task idx 83: lastmod is each sub-sitemap's newest material change, from the API,
  // never "today" on every request. People with content are indexable (SPEC.md 27.1), so
  // their sub-sitemap is named and dated like the agents one.
  it('dates each sub-sitemap by its newest material change', async () => {
    fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => counts });
    const res = await GET();
    expect(res.status).toBe(200);
    expect(entries(await res.text())).toEqual([
      ['https://solvr.dev/sitemap-core.xml', null],
      ['https://solvr.dev/sitemap-posts.xml', '2026-09-03T10:00:00Z'],
      ['https://solvr.dev/sitemap-agents.xml', '2026-09-02T10:00:00Z'],
      ['https://solvr.dev/sitemap-users.xml', '2026-09-05T10:00:00Z'],
      ['https://solvr.dev/sitemap-blog.xml', null],
      ['https://solvr.dev/sitemap-rooms.xml', '2026-09-04T10:00:00Z'],
    ]);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/sitemap/counts');
    expect(init).toMatchObject({ cache: 'no-store' });
  });

  // An API that names no users lastmod (deployed before the web) dates the users entry with
  // nothing, never with a made-up time.
  it('names no lastmod for a type the answer leaves out', async () => {
    const older: Record<string, string | null> = { ...counts.data.lastmod };
    delete older.users;
    fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => ({ data: { ...counts.data, lastmod: older } }) });
    const res = await GET();
    expect(res.status).toBe(200);
    expect(entries(await res.text())).toContainEqual(['https://solvr.dev/sitemap-users.xml', null]);
  });

  // SPEC.md 27.1 (Sitemaps): when the API cannot answer, the index is a retryable 503 with no
  // sitemap list, not a list without dates.
  it('answers 503 with Retry-After and no sitemap list when the API fails', async () => {
    fetchMock.mockRejectedValue(new TypeError('fetch failed'));
    const res = await GET();
    const body = await res.text();
    expect(res.status).toBe(503);
    expect(res.headers.get('Retry-After')).toBe('120');
    expect(res.headers.get('Cache-Control') ?? '').toContain('no-store');
    expect(body).not.toContain('<sitemap');
    expect(entries(body)).toHaveLength(0);
  });
});
