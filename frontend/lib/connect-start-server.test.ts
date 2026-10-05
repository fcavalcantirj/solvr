import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import { getDefaultConnectStart } from './connect-start-server';
import { CONNECT_START, CONNECT_START_NO_FLOW } from '@/components/connect/connect-fixture';

// /connect carries its default sentence in the HTML the server sends (SPEC.md 25.6). The
// server reads it as GET /v1/connect?flow=none and nothing else: the page is cached and
// shared, so it must never carry a flow code. The read also runs at BUILD time, so it never
// throws and never hangs: any failure is null, and the page shows the panel's loading state.

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const okResponse = (body: unknown) => ({ ok: true, status: 200, json: async () => body });

describe('getDefaultConnectStart', () => {
  it('reads GET /v1/connect?flow=none alone, revalidated every 300 seconds and bounded by a timeout', async () => {
    fetchMock.mockResolvedValue(okResponse({ data: CONNECT_START_NO_FLOW }));

    await expect(getDefaultConnectStart()).resolves.toEqual(CONNECT_START_NO_FLOW);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    const read = new URL(String(url));
    expect(read.pathname).toBe('/v1/connect');
    expect([...read.searchParams.entries()]).toEqual([['flow', 'none']]);
    expect(init).toMatchObject({ next: { revalidate: 300 } });
    expect(init.signal).toBeInstanceOf(AbortSignal);
  });

  // The API is asked for no flow; an answer that carries one anyway (an API that does not
  // know flow=none) must not reach a cached page.
  it('answers null for an answer that carries a flow code', async () => {
    fetchMock.mockResolvedValue(okResponse({ data: CONNECT_START }));
    await expect(getDefaultConnectStart()).resolves.toBeNull();
  });

  it('answers null for a refused read', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 503, json: async () => ({}) });
    await expect(getDefaultConnectStart()).resolves.toBeNull();
  });

  it('answers null, without throwing, when the API cannot be reached', async () => {
    fetchMock.mockRejectedValue(new TypeError('fetch failed'));
    await expect(getDefaultConnectStart()).resolves.toBeNull();
  });

  it('answers null, without throwing, when the read times out', async () => {
    fetchMock.mockRejectedValue(new DOMException('The operation was aborted due to timeout', 'TimeoutError'));
    await expect(getDefaultConnectStart()).resolves.toBeNull();
  });

  it('answers null for a body that is not JSON', async () => {
    fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => { throw new SyntaxError('bad json'); } });
    await expect(getDefaultConnectStart()).resolves.toBeNull();
  });

  it('answers null for a body without a contract to show', async () => {
    for (const body of [null, { error: { code: 'X' } }, { data: { ...CONNECT_START_NO_FLOW, presets: [] } }, { data: { presets: CONNECT_START_NO_FLOW.presets } }]) {
      fetchMock.mockResolvedValueOnce(okResponse(body));
      await expect(getDefaultConnectStart(), JSON.stringify(body).slice(0, 60)).resolves.toBeNull();
    }
  });
});
