import { act, renderHook, waitFor } from '@testing-library/react';
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

// One visit is one flow (SPEC.md 25.6). The API mints a flow id with every answer unless it
// is handed one back, so the hook echoes the id of its first answer on every later read:
// connection_started and starter_prompt_copied then carry the same id. The hook only
// echoes what the API issued; it never makes, checks or changes a flow id.
describe('useConnectStart keeps one flow for the visit', () => {
  const answer = (flowId?: string) =>
    ({ data: { selected: { preset: 'plan-and-build', ...(flowId ? { flow_id: flowId } : {}) }, presets: [] } }) as never;

  it('sends the flow id of its first answer back on every later read', async () => {
    vi.mocked(api.getConnectStart)
      .mockResolvedValueOnce(answer('k7m2p9xq'))
      // An API that answered with another id anyway: the first one is still what is echoed.
      .mockResolvedValueOnce(answer('zzzzzzz2'))
      .mockResolvedValue(answer('k7m2p9xq'));
    const { result } = renderHook(() => useConnectStart());

    await waitFor(() => expect(result.current.start?.selected.flow_id).toBe('k7m2p9xq'));
    expect(vi.mocked(api.getConnectStart).mock.calls[0][0]).toEqual({});

    act(() => result.current.setVisibility('private'));
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.getConnectStart).mock.calls[1][0]).toEqual({ visibility: 'private', flow: 'k7m2p9xq' });

    act(() => result.current.setVisibility('public'));
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalledTimes(3));
    expect(vi.mocked(api.getConnectStart).mock.calls[2][0]).toEqual({ visibility: 'public', flow: 'k7m2p9xq' });
  });

  it('echoes the flow with a typed intent and with the source it was linked with', async () => {
    setSearch('?from_room=ttt-room');
    vi.mocked(api.getConnectStart).mockResolvedValue(answer('k7m2p9xq'));
    const { result } = renderHook(() => useConnectStart({ readLocation: true }));
    await waitFor(() => expect(result.current.start).not.toBeNull());

    act(() => result.current.setIntent('ship the signup page'));
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.getConnectStart).mock.calls[1][0]).toEqual({
      from_room: 'ttt-room',
      intent: 'ship the signup page',
      flow: 'k7m2p9xq',
    });
  });

  it('adopts the flow of the first answer it KEEPS, not of one that arrived too late', async () => {
    let releaseFirst: (value: never) => void = () => {};
    vi.mocked(api.getConnectStart)
      .mockReturnValueOnce(new Promise((resolve) => { releaseFirst = resolve; }))
      .mockResolvedValue(answer('bbbbbbb2'));
    const { result } = renderHook(() => useConnectStart());
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalledTimes(1));

    // The visitor flips the visibility before the first answer arrives: that read is
    // abandoned, and the second one (sent with no flow yet) is the first answer kept.
    act(() => result.current.setVisibility('private'));
    await waitFor(() => expect(result.current.start?.selected.flow_id).toBe('bbbbbbb2'));
    expect(vi.mocked(api.getConnectStart).mock.calls[1][0]).toEqual({ visibility: 'private' });

    await act(async () => { releaseFirst(answer('aaaaaaa2')); });
    expect(result.current.start?.selected.flow_id).toBe('bbbbbbb2');

    act(() => result.current.setVisibility('public'));
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalledTimes(3));
    expect(vi.mocked(api.getConnectStart).mock.calls[2][0]).toEqual({ visibility: 'public', flow: 'bbbbbbb2' });
  });

  it('sends no flow while the API has issued none', async () => {
    vi.mocked(api.getConnectStart).mockResolvedValueOnce(answer()).mockResolvedValue(answer('k7m2p9xq'));
    const { result } = renderHook(() => useConnectStart());
    await waitFor(() => expect(result.current.start).not.toBeNull());

    act(() => result.current.setVisibility('private'));
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.getConnectStart).mock.calls[1][0]).toEqual({ visibility: 'private' });

    // The second answer carried one: from now on it is echoed.
    act(() => result.current.setVisibility('public'));
    await waitFor(() => expect(api.getConnectStart).toHaveBeenCalledTimes(3));
    expect(vi.mocked(api.getConnectStart).mock.calls[2][0]).toEqual({ visibility: 'public', flow: 'k7m2p9xq' });
  });
});
