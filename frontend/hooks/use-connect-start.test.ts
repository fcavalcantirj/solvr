import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

vi.mock('@/lib/api', () => ({ api: { getConnectStart: vi.fn() } }));

import { api } from '@/lib/api';
import { useConnectStart } from './use-connect-start';

// The /connect page carries the source it was linked with (?from_room= / ?post=) to the
// API. It does NOT forward ?preset=: links in the wild still carry the retired
// planner-executor value, and the API would refuse it.

function setSearch(search: string) {
  window.history.replaceState(null, '', `/connect${search}`);
}

beforeEach(() => {
  vi.mocked(api.getConnectStart).mockReset();
  vi.mocked(api.getConnectStart).mockResolvedValue({ data: {} as never });
});
afterEach(() => setSearch(''));

describe('useConnectStart source from the page URL', () => {
  it('sends from_room when the page reads its location', async () => {
    setSearch('?from_room=ttt-room&preset=planner-executor');
    renderHook(() => useConnectStart({ readLocation: true }));
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalled());
    expect(vi.mocked(api.getConnectStart).mock.calls[0][0]).toEqual({ from_room: 'ttt-room' });
  });

  it('sends post when the page reads its location', async () => {
    setSearch('?post=p-1');
    renderHook(() => useConnectStart({ readLocation: true }));
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalled());
    expect(vi.mocked(api.getConnectStart).mock.calls[0][0]).toEqual({ post: 'p-1' });
  });

  it('ignores the URL unless asked (the homepage panel)', async () => {
    setSearch('?from_room=ttt-room');
    renderHook(() => useConnectStart());
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalled());
    expect(vi.mocked(api.getConnectStart).mock.calls[0][0]).toEqual({});
  });
});
