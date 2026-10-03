import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

vi.mock('@/lib/api', () => ({ api: { getConnectStart: vi.fn() } }));

import { api } from '@/lib/api';
import { APIError } from '@/lib/api-error';
import { useConnectStart } from './use-connect-start';

// The /connect page carries what it was linked with to the API: the source
// (?from_room= / ?post=) and the preset (?preset=, from the homepage use cases).
// The API is the only judge of a preset: a value it refuses (old links still carry
// the retired planner-executor) is dropped once and the contract is read again
// with the API's own default. Nothing is mapped or guessed.

function setSearch(search: string) {
  window.history.replaceState(null, '', `/connect${search}`);
}

beforeEach(() => {
  vi.mocked(api.getConnectStart).mockReset();
  vi.mocked(api.getConnectStart).mockResolvedValue({ data: {} as never });
});
afterEach(() => setSearch(''));

describe('useConnectStart source from the page URL', () => {
  // Replaces 'sends from_room when the page reads its location', which asserted that
  // ?preset=planner-executor was never forwarded (calls[0][0] equal to { from_room }).
  // A linked preset is forwarded now; the refused value is dropped by the next test.
  it('sends from_room when the page reads its location', async () => {
    setSearch('?from_room=ttt-room');
    renderHook(() => useConnectStart({ readLocation: true }));
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalled());
    expect(vi.mocked(api.getConnectStart).mock.calls[0][0]).toEqual({ from_room: 'ttt-room' });
  });

  it('forwards a linked ?preset= with the source, so the API preselects it', async () => {
    setSearch('?from_room=ttt-room&preset=collaborate');
    renderHook(() => useConnectStart({ readLocation: true }));
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalled());
    expect(vi.mocked(api.getConnectStart).mock.calls[0][0]).toEqual({
      from_room: 'ttt-room',
      preset: 'collaborate',
    });
  });

  it('drops a linked preset the API refuses and reads the contract again with its default', async () => {
    setSearch('?from_room=ttt-room&preset=planner-executor');
    vi.mocked(api.getConnectStart)
      .mockRejectedValueOnce(new APIError('preset must be plan-and-build, build-and-review or collaborate', 400))
      .mockResolvedValue({ data: { selected: { preset: 'plan-and-build' } } as never });
    const { result } = renderHook(() => useConnectStart({ readLocation: true }));

    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.getConnectStart).mock.calls[0][0]).toEqual({
      from_room: 'ttt-room',
      preset: 'planner-executor',
    });
    expect(vi.mocked(api.getConnectStart).mock.calls[1][0]).toEqual({ from_room: 'ttt-room' });
    await waitFor(() => expect(result.current.start).not.toBeNull());
    expect(result.current.error).toBeNull();
  });

  it('reports any other failure instead of retrying', async () => {
    setSearch('?preset=collaborate');
    vi.mocked(api.getConnectStart).mockRejectedValue(new APIError('server error', 500));
    const { result } = renderHook(() => useConnectStart({ readLocation: true }));

    await waitFor(() => expect(result.current.error).toBe('server error'));
    expect(api.getConnectStart).toHaveBeenCalledTimes(1);
  });

  it('never reads ?preset= on the homepage panel', async () => {
    setSearch('?preset=collaborate');
    renderHook(() => useConnectStart());
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalled());
    expect(vi.mocked(api.getConnectStart).mock.calls[0][0]).toEqual({});
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
