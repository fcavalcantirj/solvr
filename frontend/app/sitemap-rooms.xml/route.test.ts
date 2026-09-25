import { describe, it, expect, vi, afterEach } from 'vitest';
import * as route from './route';

// The rooms sitemap is an index crawlers read. The API lists only public,
// non-deleted rooms; a stored copy would keep a room that turned private or was
// deleted in the index (the shared sitemap cache keeps it for hours).

describe('sitemap-rooms.xml', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function stubRooms(rooms: { slug: string; last_active_at: string }[]) {
    const fetchMock = vi.fn().mockResolvedValue({ json: async () => ({ data: { rooms } }) });
    vi.stubGlobal('fetch', fetchMock);
    return fetchMock;
  }

  it('lists exactly the rooms the API returns', async () => {
    stubRooms([{ slug: 'open-room', last_active_at: '2026-09-25T00:00:00Z' }]);

    const xml = await (await route.GET()).text();

    expect(xml).toContain('<loc>https://solvr.dev/rooms/open-room</loc>');
  });

  it('asks the API on every request instead of the data cache', async () => {
    const fetchMock = stubRooms([]);

    await route.GET();

    expect(route.dynamic).toBe('force-dynamic');
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/sitemap/urls?type=rooms');
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(init?.next?.revalidate).toBeUndefined();
  });

  it('never lets a shared cache store the rooms index', async () => {
    stubRooms([]);

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
