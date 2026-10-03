import { renderHook } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('@/lib/api', () => ({ api: { postFunnelEvent: vi.fn() } }));

import { api } from '@/lib/api';
import { useShareVisit } from './use-share-visit';

// A visit that arrives through a share link (?via=share) is counted once per tab as a
// share_visit attributed to the public room, and the marker is then removed so the
// address bar shows the clean room link.

function at(path: string) {
  window.history.replaceState(null, '', path);
}

beforeEach(() => {
  vi.mocked(api.postFunnelEvent).mockReset();
  window.sessionStorage.clear();
});

describe('useShareVisit', () => {
  it('reports one share_visit for a share link and cleans the URL', () => {
    at('/rooms/ttt-room?via=share&message=4#m');
    renderHook(() => useShareVisit({ kind: 'room', ref: 'ttt-room' }, 'room_page'));
    expect(api.postFunnelEvent).toHaveBeenCalledTimes(1);
    expect(api.postFunnelEvent).toHaveBeenCalledWith({
      event: 'share_visit',
      entry_surface: 'room_page',
      source: { kind: 'room', ref: 'ttt-room' },
    });
    expect(window.location.search).toBe('?message=4');
    expect(window.location.hash).toBe('#m');
  });

  it('counts the same tab once', () => {
    at('/rooms/ttt-room?via=share');
    renderHook(() => useShareVisit({ kind: 'room', ref: 'ttt-room' }, 'room_page'));
    at('/rooms/ttt-room?via=share');
    renderHook(() => useShareVisit({ kind: 'room', ref: 'ttt-room' }, 'room_page'));
    expect(api.postFunnelEvent).toHaveBeenCalledTimes(1);
  });

  it('does nothing without the share marker', () => {
    at('/rooms/ttt-room');
    renderHook(() => useShareVisit({ kind: 'room', ref: 'ttt-room' }, 'room_page'));
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
  });

  it('cleans the URL but reports nothing for a room that cannot be attributed', () => {
    at('/rooms/hidden?via=share');
    renderHook(() => useShareVisit(null, 'room_page'));
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
    expect(window.location.search).toBe('');
  });
});
