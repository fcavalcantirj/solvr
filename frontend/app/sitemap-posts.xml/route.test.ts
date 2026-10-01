import { describe, it, expect, vi, afterEach } from 'vitest';
import { GET } from './route';
import * as route from './route';

describe('sitemap-posts.xml', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('emits canonical /posts/{id} for every eligible post regardless of legacy type', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        json: async () => ({
          data: {
            posts: [
              { id: 'p1', type: 'problem', updated_at: '2026-09-01T00:00:00Z' },
              { id: 'i1', type: 'idea', updated_at: '2026-09-02T00:00:00Z' },
              { id: 'q1', type: 'question', updated_at: '2026-09-03T00:00:00Z' },
            ],
          },
        }),
      })
    );

    const res = await GET();
    const xml = await res.text();

    // Every post resolves to the canonical /posts/{id} URL (no legacy type paths).
    expect(xml).toContain('https://solvr.dev/posts/p1');
    expect(xml).toContain('https://solvr.dev/posts/i1');
    expect(xml).toContain('https://solvr.dev/posts/q1');
    // No redirect chain: legacy type-specific URLs must never appear.
    expect(xml).not.toContain('/problems/');
    expect(xml).not.toContain('/ideas/');
  });

  it('returns an empty urlset when the API call fails, without throwing', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('boom')));

    const res = await GET();
    const xml = await res.text();

    expect(xml).toContain('<urlset');
    expect(xml).not.toContain('<url>');
  });
});

// The API lists only public, live, non-rejected posts; a post deleted or made
// family-only leaves that list at once. A stored copy of this index would keep
// naming it to crawlers for hours (data cache + shared cache).
describe('sitemap-posts.xml caching', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function stubPosts() {
    const fetchMock = vi.fn().mockResolvedValue({ json: async () => ({ data: { posts: [] } }) });
    vi.stubGlobal('fetch', fetchMock);
    return fetchMock;
  }

  it('asks the API on every request instead of the data cache', async () => {
    const fetchMock = stubPosts();

    await route.GET();

    expect(route.dynamic).toBe('force-dynamic');
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/sitemap/urls?type=posts');
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(init?.next?.revalidate).toBeUndefined();
  });

  it('never lets a shared cache store the posts index', async () => {
    stubPosts();

    const res = await route.GET();

    const value = res.headers.get('Cache-Control') ?? '';
    expect(value).toContain('no-store');
    expect(value).not.toMatch(/public|s-maxage|stale-while-revalidate/);
    expect(res.headers.get('CDN-Cache-Control')).toBeNull();
  });

  it('keeps a failed API call out of shared caches too', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('boom')));

    const res = await route.GET();

    expect(res.headers.get('Cache-Control') ?? '').toContain('no-store');
  });
});
