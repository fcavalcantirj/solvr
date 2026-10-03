import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// The room page must ask the API on every request: the API decides whether the
// caller may still read the room (visibility, deletion), and a cached payload
// would keep showing a room after it turned private or was deleted.

// React's request-scoped cache() only exists in the server build; pass through here.
vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/rooms/room-detail-client', () => ({ RoomDetailClient: () => null }));
vi.mock('@/components/rooms/private-room-view', () => ({ PrivateRoomView: () => null }));
vi.mock('@/components/seo/json-ld', () => ({ JsonLd: () => null, roomJsonLd: () => ({}), breadcrumbJsonLd: () => ({}) }));
vi.mock('next/navigation', () => ({
  notFound: () => {
    throw new Error('NEXT_NOT_FOUND');
  },
}));

import * as roomPage from './page';

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockResolvedValue({ ok: false, status: 403, json: async () => null });
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('room page caching', () => {
  it('renders on every request instead of serving a stored copy', () => {
    expect(roomPage.dynamic).toBe('force-dynamic');
    expect((roomPage as Record<string, unknown>).revalidate).toBeUndefined();
  });

  it('fetches the room from the API without the data cache', async () => {
    await roomPage.generateMetadata({ params: Promise.resolve({ slug: 'turned-private' }) });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/rooms/turned-private');
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(init?.next?.revalidate).toBeUndefined();
  });
});
