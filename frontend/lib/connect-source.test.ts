import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { api } from './api';

// "Try this workflow" (idx 88): the client forwards the source it was linked with and
// reads the share contract; the API validates and decides everything.

describe('connect source and share requests', () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ data: {} }) });
    global.fetch = fetchMock as unknown as typeof global.fetch;
  });
  afterEach(() => vi.restoreAllMocks());

  it('forwards from_room to GET /v1/connect', async () => {
    await api.getConnectStart({ from_room: 'ttt-room' });
    expect(String(fetchMock.mock.calls[0][0])).toMatch(/\/v1\/connect\?from_room=ttt-room$/);
  });

  it('forwards post to GET /v1/connect', async () => {
    await api.getConnectStart({ post: '6f1b9a52-34d4-4c55-9d0e-0b6a8b0e2a11' });
    expect(String(fetchMock.mock.calls[0][0])).toMatch(/\/v1\/connect\?post=6f1b9a52-34d4-4c55-9d0e-0b6a8b0e2a11$/);
  });

  it('reads the share contract of a room', async () => {
    await api.getRoomShare('ttt-room');
    expect(String(fetchMock.mock.calls[0][0])).toMatch(/\/v1\/rooms\/ttt-room\/share$/);
  });
});
