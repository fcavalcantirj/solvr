import { describe, it, expect, vi, afterEach } from 'vitest';
import * as route from './route';

// People with public content are indexable (SPEC.md 27.1), and the API lists exactly them
// (GET /v1/sitemap/urls?type=users reads the rule of GET /v1/users/{id}/seo,
// backend/internal/db/profile_seo.go). This route only maps what the API lists to profile
// URLs; a person who deletes their account or loses their last public content leaves the list
// at once, so no stored copy may keep naming them.

describe('sitemap-users.xml', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function stub(rows: { id: string; updated_at: string }[]) {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => ({ data: { users: rows } }) });
    vi.stubGlobal('fetch', fetchMock);
    return fetchMock;
  }

  it('lists exactly the people the API returns, at their profile URL, dated by their change', async () => {
    stub([
      { id: '2b6f0a2e-0000-4000-8000-000000000001', updated_at: '2026-10-01T00:00:00Z' },
      { id: '2b6f0a2e-0000-4000-8000-000000000002', updated_at: '2026-10-02T00:00:00Z' },
    ]);

    const res = await route.GET();
    const xml = await res.text();

    expect(res.status).toBe(200);
    expect([...xml.matchAll(/<loc>([^<]+)<\/loc>\s*<lastmod>([^<]+)<\/lastmod>/g)].map((m) => [m[1], m[2]])).toEqual([
      ['https://solvr.dev/users/2b6f0a2e-0000-4000-8000-000000000001', '2026-10-01T00:00:00Z'],
      ['https://solvr.dev/users/2b6f0a2e-0000-4000-8000-000000000002', '2026-10-02T00:00:00Z'],
    ]);
  });

  it('asks the API for people on every request, without the data cache', async () => {
    const fetchMock = stub([]);

    await route.GET();

    expect(route.dynamic).toBe('force-dynamic');
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/sitemap/urls?type=users');
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(init?.next?.revalidate).toBeUndefined();
  });

  it('never lets a shared cache store the users index', async () => {
    stub([]);

    const res = await route.GET();

    const value = res.headers.get('Cache-Control') ?? '';
    expect(value).toContain('no-store');
    expect(value).not.toMatch(/public|s-maxage|stale-while-revalidate/);
    expect(res.headers.get('CDN-Cache-Control')).toBeNull();
  });

  it('answers 503 with Retry-After and no url set when the API call fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('boom')));

    const res = await route.GET();

    expect(res.status).toBe(503);
    expect(res.headers.get('Retry-After')).toBe('120');
    expect(await res.text()).not.toContain('<urlset');
  });
});
