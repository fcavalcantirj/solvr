import { describe, it, expect, vi, afterEach } from 'vitest';
import * as route from './route';

// The API lists only users that still exist; one that deletes itself or is banned
// leaves that list at once. A stored copy of this index would keep naming it to
// crawlers for hours (data cache + shared cache).

describe('sitemap-users.xml', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function stub(rows: { id: string; updated_at: string }[]) {
    const fetchMock = vi.fn().mockResolvedValue({ json: async () => ({ data: { users: rows } }) });
    vi.stubGlobal('fetch', fetchMock);
    return fetchMock;
  }

  it('lists exactly the users the API returns', async () => {
    stub([{ id: 'listed-1', updated_at: '2026-10-01T00:00:00Z' }]);

    const xml = await (await route.GET()).text();

    expect(xml).toContain('<loc>https://solvr.dev/users/listed-1</loc>');
    expect(xml.match(/<url>/g)).toHaveLength(1);
  });

  it('asks the API for users on every request, without the data cache', async () => {
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

  it('returns an empty urlset when the API call fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('boom')));

    const res = await route.GET();

    expect(await res.text()).toContain('<urlset');
    expect(res.headers.get('Cache-Control') ?? '').toContain('no-store');
  });
});
