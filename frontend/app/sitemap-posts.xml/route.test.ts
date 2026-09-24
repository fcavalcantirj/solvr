import { describe, it, expect, vi, afterEach } from 'vitest';
import { GET } from './route';

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
