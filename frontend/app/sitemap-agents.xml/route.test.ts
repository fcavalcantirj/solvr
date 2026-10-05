import { describe, it, expect, vi, afterEach } from 'vitest';
import * as route from './route';

// The API lists only agents that still exist; one that deletes itself or is banned
// leaves that list at once. A stored copy of this index would keep naming it to
// crawlers for hours (data cache + shared cache).

describe('sitemap-agents.xml', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function stub(rows: { id: string; updated_at: string }[]) {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => ({ data: { agents: rows } }) });
    vi.stubGlobal('fetch', fetchMock);
    return fetchMock;
  }

  it('lists exactly the agents the API returns', async () => {
    stub([{ id: 'listed-1', updated_at: '2026-10-01T00:00:00Z' }]);

    const xml = await (await route.GET()).text();

    expect(xml).toContain('<loc>https://solvr.dev/agents/listed-1</loc>');
    expect(xml.match(/<url>/g)).toHaveLength(1);
  });

  it('asks the API for agents on every request, without the data cache', async () => {
    const fetchMock = stub([]);

    await route.GET();

    expect(route.dynamic).toBe('force-dynamic');
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/sitemap/urls?type=agents');
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(init?.next?.revalidate).toBeUndefined();
  });

  it('never lets a shared cache store the agents index', async () => {
    stub([]);

    const res = await route.GET();

    const value = res.headers.get('Cache-Control') ?? '';
    expect(value).toContain('no-store');
    expect(value).not.toMatch(/public|s-maxage|stale-while-revalidate/);
    expect(res.headers.get('CDN-Cache-Control')).toBeNull();
  });

  // SPEC.md 27.1 (Sitemaps): a failed read is a retryable 503, never an empty url set.
  it('answers 503 with Retry-After and no url set when the API call fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('boom')));

    const res = await route.GET();

    expect(res.status).toBe(503);
    expect(res.headers.get('Retry-After')).toBe('120');
    expect(await res.text()).not.toContain('<urlset');
    expect(res.headers.get('Cache-Control') ?? '').toContain('no-store');
  });

  // The rule is the API's (GET /v1/agents/{id}/seo, backend/internal/db/profile_seo.go): the
  // route only maps what it lists to profile URLs.
  it('maps every listed agent to its profile URL and adds none', async () => {
    stub([
      { id: 'agent_a', updated_at: '2026-10-01T00:00:00Z' },
      { id: 'agent_b', updated_at: '2026-10-02T00:00:00Z' },
    ]);

    const xml = await (await route.GET()).text();

    expect([...xml.matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1])).toEqual([
      'https://solvr.dev/agents/agent_a',
      'https://solvr.dev/agents/agent_b',
    ]);
  });
});
