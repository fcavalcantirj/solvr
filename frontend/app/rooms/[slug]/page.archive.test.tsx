import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render } from '@testing-library/react';
import type { ReactNode } from 'react';

// Task idx 81: the live room page links its transcript archive and its outcome posts
// in server HTML, beside the live view.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/rooms/room-detail-client', () => ({
  RoomDetailClient: ({ archive }: { archive?: ReactNode }) => <div data-testid="room-detail">{archive}</div>,
}));
vi.mock('@/components/rooms/private-room-view', () => ({ PrivateRoomView: () => null }));
vi.mock('@/components/seo/json-ld', () => ({ JsonLd: () => null, roomJsonLd: () => ({}), breadcrumbJsonLd: () => ({}) }));
vi.mock('next/navigation', () => ({
  notFound: () => {
    throw new Error('NEXT_NOT_FOUND');
  },
}));

import RoomDetailPage from './page';

const fetchMock = vi.fn();
beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const roomBody = {
  data: {
    room: { id: 'r1', slug: 'kestrel', display_name: 'Kestrel', message_count: 150, created_at: '2026-09-01T00:00:00Z', last_active_at: '2026-09-02T00:00:00Z' },
    agents: [],
    recent_messages: [],
    seo: { indexable: true, title: 'Kestrel', description: 'Plan kestrel' },
    history: { page_size: 100, total_pages: 2 },
  },
};

describe('room page archive links', () => {
  it('links every transcript page and each outcome post', async () => {
    fetchMock.mockImplementation(async (url: string) => {
      if (String(url).endsWith('/v1/rooms/kestrel/posts')) {
        return { ok: true, status: 200, json: async () => ({ data: [{ id: 'p9', title: 'Kestrel shipped' }] }) };
      }
      return { ok: true, status: 200, json: async () => roomBody };
    });
    const { container } = render(await RoomDetailPage({ params: Promise.resolve({ slug: 'kestrel' }) }));
    const hrefs = [...container.querySelectorAll('a')].map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual(['/rooms/kestrel/history/1', '/rooms/kestrel/history/2', '/posts/p9']);
    const outcomeCall = fetchMock.mock.calls.find(([u]) => String(u).endsWith('/posts'));
    expect(outcomeCall?.[1]).toMatchObject({ cache: 'no-store' });
  });

  it('still renders the room when the outcome list fails', async () => {
    fetchMock.mockImplementation(async (url: string) => {
      if (String(url).endsWith('/posts')) return { ok: false, status: 500, json: async () => null };
      return { ok: true, status: 200, json: async () => roomBody };
    });
    const { container } = render(await RoomDetailPage({ params: Promise.resolve({ slug: 'kestrel' }) }));
    const hrefs = [...container.querySelectorAll('a')].map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual(['/rooms/kestrel/history/1', '/rooms/kestrel/history/2']);
  });
});
