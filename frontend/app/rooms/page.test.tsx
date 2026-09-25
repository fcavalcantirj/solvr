import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// The rooms list must ask the API on every request: a room that turned private
// or was deleted leaves the API's public list at once, and a stored copy of the
// page would keep advertising its name and description.

// React's request-scoped cache() only exists in the server build; pass through here.
vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/rooms/rooms-browser', () => ({ RoomsBrowser: () => null }));
vi.mock('@/components/rooms/recently-viewed-rooms', () => ({ RecentlyViewedRooms: () => null }));
vi.mock('@/components/rooms/create-room-dialog', () => ({ CreateRoomDialog: () => null }));

import * as roomsPage from './page';

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => ({ data: [] }) });
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('rooms list page caching', () => {
  it('renders on every request instead of serving a stored copy', () => {
    expect(roomsPage.dynamic).toBe('force-dynamic');
    expect((roomsPage as Record<string, unknown>).revalidate).toBeUndefined();
  });

  it('fetches the room list from the API without the data cache', async () => {
    await roomsPage.default();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/rooms?');
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(init?.next?.revalidate).toBeUndefined();
  });
});
