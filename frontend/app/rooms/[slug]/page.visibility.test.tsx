import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render } from '@testing-library/react';

// The room page's HTML and its social metadata (title, description, Open Graph, JSON-LD)
// are built only from what GET /v1/rooms/{slug} returned to the server render, which
// carries no credentials. The API decides visibility, deletion and expiry; the page must
// never show a room the API refused, even if the refusal carried a body.

// React's request-scoped cache() only exists in the server build; pass through here.
vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/rooms/room-detail-client', () => ({
  RoomDetailClient: ({ room, initialMessages }: { room: { display_name: string }; initialMessages: { content: string }[] }) => (
    <div data-testid="room-detail">
      {room.display_name}
      {initialMessages.map((m) => m.content).join(' ')}
    </div>
  ),
}));
vi.mock('@/components/rooms/private-room-view', () => ({
  PrivateRoomView: ({ slug }: { slug: string }) => <div data-testid="private-gate">{slug}</div>,
}));
vi.mock('@/components/seo/json-ld', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/components/seo/json-ld')>()),
  JsonLd: ({ data }: { data: unknown }) => <script data-testid="jsonld">{JSON.stringify(data)}</script>,
}));
vi.mock('next/navigation', () => ({
  notFound: () => {
    throw new Error('NEXT_NOT_FOUND');
  },
}));

import RoomDetailPage, { generateMetadata } from './page';
import { NOINDEX } from '@/lib/seo/route-policy';

const NAME = 'Room Name MARKER-NAME-7';
const DESCRIPTION = 'Room description MARKER-DESC-7';
const MESSAGE = 'Transcript MARKER-MSG-7';

function roomPayload() {
  return {
    data: {
      room: {
        id: 'r1',
        slug: 'the-room',
        display_name: NAME,
        description: DESCRIPTION,
        message_count: 1,
        created_at: '2026-09-01T00:00:00Z',
        last_active_at: '2026-09-02T00:00:00Z',
      },
      agents: [],
      recent_messages: [{ id: 1, content: MESSAGE }],
    },
  };
}

const fetchMock = vi.fn();

function apiAnswers(status: number) {
  // Even a refusal carries the room here: the page must not read a refused body. The
  // room's search verdict (GET /v1/rooms/{slug}/seo, task idx 80) answers like the room.
  fetchMock.mockImplementation(async (url: string) => ({
    ok: status >= 200 && status < 300,
    status,
    json: async () =>
      String(url).endsWith('/seo')
        ? { data: { indexable: true, title: NAME, description: DESCRIPTION } }
        : roomPayload(),
  }));
}

const params = () => ({ params: Promise.resolve({ slug: 'the-room' }) });

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('room page follows the API for HTML and social metadata', () => {
  it('a room the API serves: metadata and HTML carry what the API returned', async () => {
    apiAnswers(200);

    const meta = await generateMetadata(params());
    // The root template appends " | Solvr"; the page adds no second suffix.
    expect(meta.title).toBe(NAME);
    expect(meta.description).toBe(DESCRIPTION);
    expect(meta.openGraph).toMatchObject({ title: NAME, description: DESCRIPTION });

    const { getByTestId, getAllByTestId, queryByTestId } = render(await RoomDetailPage(params()));
    expect(getByTestId('room-detail').textContent).toContain(MESSAGE);
    // Two structured-data blocks (task idx 82): the room and its breadcrumbs.
    const blocks = getAllByTestId('jsonld').map((el) => el.textContent ?? '');
    expect(blocks).toHaveLength(2);
    expect(blocks[0]).toContain(NAME);
    expect(blocks[1]).toContain('BreadcrumbList');
    expect(queryByTestId('private-gate')).toBeNull();
  });

  it.each([401, 403])(
    'a room the API refuses (%i): no metadata, no transcript, no structured data, only the authenticated gate',
    async (status) => {
      apiAnswers(status);

      // Nothing about the room, only the directive to keep the gate out of search.
      const meta = await generateMetadata(params());
      expect(meta).toEqual({ robots: NOINDEX });

      const { container, getByTestId, queryByTestId } = render(await RoomDetailPage(params()));
      expect(getByTestId('private-gate').textContent).toBe('the-room');
      expect(queryByTestId('room-detail')).toBeNull();
      expect(queryByTestId('jsonld')).toBeNull();
      for (const marker of [NAME, DESCRIPTION, MESSAGE]) {
        expect(container.innerHTML).not.toContain(marker);
      }
    },
  );

  it('a room the API no longer has (deleted or expired, 404): no metadata and a real 404', async () => {
    apiAnswers(404);

    expect(await generateMetadata(params())).toEqual({ robots: NOINDEX });
    await expect(RoomDetailPage(params())).rejects.toThrow('NEXT_NOT_FOUND');
  });
});
