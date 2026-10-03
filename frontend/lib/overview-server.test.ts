import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import { getInitialOverview } from './overview-server';
import { OVERVIEW, STALE_META } from '@/components/homepage/overview-fixture';

// The index reads the overview on the server so its first HTML already carries
// the hero numbers. Production images run `next build` against the live API, so
// this read also runs at BUILD time: it must never throw and never hang — any
// failure is null, and the page falls back to reading the overview in the browser.

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const okResponse = (body: unknown) => ({ ok: true, status: 200, json: async () => body });

describe('getInitialOverview', () => {
  it('reads GET /v1/overview, revalidated every 60 seconds and bounded by a timeout', async () => {
    fetchMock.mockResolvedValue(okResponse({ data: OVERVIEW, meta: STALE_META }));

    const result = await getInitialOverview();

    expect(result).toEqual({ data: OVERVIEW, meta: STALE_META });
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toMatch(/\/v1\/overview$/);
    expect(init).toMatchObject({ next: { revalidate: 60 } });
    expect(init.signal).toBeInstanceOf(AbortSignal);
  });

  it('answers null for a refused read', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 503, json: async () => ({}) });
    await expect(getInitialOverview()).resolves.toBeNull();
  });

  it('answers null, without throwing, when the API cannot be reached', async () => {
    fetchMock.mockRejectedValue(new TypeError('fetch failed'));
    await expect(getInitialOverview()).resolves.toBeNull();
  });

  it('answers null, without throwing, when the read times out', async () => {
    fetchMock.mockRejectedValue(new DOMException('The operation was aborted due to timeout', 'TimeoutError'));
    await expect(getInitialOverview()).resolves.toBeNull();
  });

  it('answers null for a body that is not JSON', async () => {
    fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => { throw new SyntaxError('bad json'); } });
    await expect(getInitialOverview()).resolves.toBeNull();
  });

  it('answers null for a body without data', async () => {
    fetchMock.mockResolvedValue(okResponse({ error: { code: 'X' } }));
    await expect(getInitialOverview()).resolves.toBeNull();
  });
});
