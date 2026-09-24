import { describe, it, expect, beforeEach } from 'vitest';
import {
  getRecentRooms,
  recordRoomView,
  removeRecentRoom,
  clearRecentRooms,
  RECENT_ROOMS_MAX,
} from './recently-viewed-rooms';

const STORAGE_KEY = 'solvr:recently-viewed-rooms';

describe('recently-viewed-rooms store', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it('returns an empty list when nothing has been viewed', () => {
    expect(getRecentRooms()).toEqual([]);
  });

  it('records a viewed room and reads it back', () => {
    recordRoomView({ slug: 'tic-tac-toe', displayName: 'Tic Tac Toe' });
    const rooms = getRecentRooms();
    expect(rooms).toHaveLength(1);
    expect(rooms[0].slug).toBe('tic-tac-toe');
    expect(rooms[0].displayName).toBe('Tic Tac Toe');
    expect(typeof rooms[0].viewedAt).toBe('string');
  });

  it('deduplicates by slug and moves the most recent view to the front', () => {
    recordRoomView({ slug: 'a', displayName: 'A' });
    recordRoomView({ slug: 'b', displayName: 'B' });
    recordRoomView({ slug: 'a', displayName: 'A again' });
    const rooms = getRecentRooms();
    expect(rooms.map((r) => r.slug)).toEqual(['a', 'b']);
    expect(rooms[0].displayName).toBe('A again');
  });

  it('caps the list at RECENT_ROOMS_MAX, dropping the oldest', () => {
    for (let i = 0; i < RECENT_ROOMS_MAX + 3; i++) {
      recordRoomView({ slug: `room-${i}`, displayName: `Room ${i}` });
    }
    const rooms = getRecentRooms();
    expect(rooms).toHaveLength(RECENT_ROOMS_MAX);
    // Most recent first: the last-recorded slug is at the front.
    expect(rooms[0].slug).toBe(`room-${RECENT_ROOMS_MAX + 2}`);
  });

  it('persists only non-secret public fields — never a token or other extras', () => {
    recordRoomView({
      slug: 'private-ish',
      displayName: 'Nope',
      // Extra fields a caller might accidentally pass must NOT be persisted.
      token: 'secret-room-token',
      is_private: true,
    } as unknown as { slug: string; displayName: string });
    const raw = window.localStorage.getItem(STORAGE_KEY) ?? '';
    expect(raw).not.toContain('secret-room-token');
    expect(raw).not.toContain('is_private');
    const rooms = getRecentRooms();
    expect(Object.keys(rooms[0]).sort()).toEqual(['displayName', 'slug', 'viewedAt']);
  });

  it('ignores an empty or missing slug', () => {
    recordRoomView({ slug: '', displayName: 'Empty' });
    expect(getRecentRooms()).toEqual([]);
  });

  it('tolerates malformed stored JSON and returns an empty list', () => {
    window.localStorage.setItem(STORAGE_KEY, '{not valid json');
    expect(getRecentRooms()).toEqual([]);
  });

  it('drops malformed entries inside a stored array', () => {
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify([
        { slug: 'good', displayName: 'Good', viewedAt: '2026-09-24T00:00:00Z' },
        { displayName: 'no slug', viewedAt: 'x' },
        42,
      ]),
    );
    const rooms = getRecentRooms();
    expect(rooms.map((r) => r.slug)).toEqual(['good']);
  });

  it('removes a single entry by slug', () => {
    recordRoomView({ slug: 'a', displayName: 'A' });
    recordRoomView({ slug: 'b', displayName: 'B' });
    removeRecentRoom('a');
    expect(getRecentRooms().map((r) => r.slug)).toEqual(['b']);
  });

  it('clears the whole list', () => {
    recordRoomView({ slug: 'a', displayName: 'A' });
    clearRecentRooms();
    expect(getRecentRooms()).toEqual([]);
    expect(window.localStorage.getItem(STORAGE_KEY)).toBeNull();
  });
});
