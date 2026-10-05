import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import type { ReactElement } from 'react';

// SPEC.md 27.4: a collection page that read its list on the server answered 200 with an empty
// list when the API could not be read, which told a crawler the collection was empty. Its read
// now fails the page like a page's own read does (lib/seo/read-for-page.ts): a 5xx or 429, any
// other non-2xx, an unreachable API or a body that is not a list throws, and the page answers a
// retryable 5xx. A real empty list is still a page, with its empty state.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('next/link', () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));
vi.mock('next/navigation', () => ({
  notFound: () => {
    throw new Error('NEXT_NOT_FOUND');
  },
}));
vi.mock('@/components/header', () => ({ Header: () => null }));

// Each page's browser-rendered list receives what the server read; record it.
const { received, capture } = vi.hoisted(() => {
  const received = { list: undefined as unknown };
  const capture = (key: string) => (props: Record<string, unknown>) => {
    received.list = props[key];
    return null;
  };
  return { received, capture };
});
vi.mock('@/components/posts/posts-page-client', () => ({ PostsPageClient: capture('initialPosts') }));
vi.mock('@/components/rooms/rooms-browser', () => ({ RoomsBrowser: capture('initialRooms') }));
vi.mock('@/components/rooms/recently-viewed-rooms', () => ({ RecentlyViewedRooms: () => null }));
vi.mock('@/components/rooms/create-room-dialog', () => ({ CreateRoomDialog: () => null }));
vi.mock('@/components/agents/agents-page-client', () => ({ AgentsPageClient: capture('initialAgentData') }));
vi.mock('@/components/users/users-page-client', () => ({ UsersPageClient: capture('initialUserData') }));
vi.mock('@/components/blog/blog-page-client', () => ({ BlogPageClient: capture('initialBlogPosts') }));
vi.mock('@/components/leaderboard/leaderboard-page-client', () => ({ LeaderboardPageClient: capture('initialEntries') }));
vi.mock('@/components/posts/post-link-list', () => ({ PostLinkList: capture('posts') }));

import PostsPage from './posts/page';
import RoomsPage from './rooms/page';
import AgentsPage from './agents/page';
import UsersPage from './users/page';
import BlogPage from './blog/page';
import LeaderboardPage from './leaderboard/page';
import ArchivePage from './posts/page/[n]/page';

const fetchMock = vi.fn();
const answer = (status: number, body: unknown) =>
  fetchMock.mockResolvedValue({ ok: status >= 200 && status < 300, status, json: async () => body });

beforeEach(() => {
  fetchMock.mockReset();
  received.list = undefined;
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

const meta = (total: number) => ({ total, page: 1, per_page: 50, total_pages: Math.ceil(total / 50), has_more: false });

// Every collection page that reads its list on the server, with the path it reads and a real
// answer. The archive reads the indexable list (SPEC.md 27.2), which also names its last page.
const PAGES: Record<string, { render: () => Promise<ReactElement>; path: string; list: unknown[]; body?: (list: unknown[]) => unknown }> = {
  '/posts': { render: () => PostsPage(), path: '/v1/posts?sort=newest&per_page=20', list: [{ id: 'p1', title: 'A post' }] },
  '/posts/page/1': {
    render: () => ArchivePage({ params: Promise.resolve({ n: '1' }) }),
    path: '/v1/posts?indexable=true&sort=new&page=1&per_page=50',
    list: [{ id: 'p1', title: 'A post', created_at: '2026-09-01T10:00:00Z', author: { id: 'a1', type: 'agent', display_name: 'A' } }],
    body: (list) => ({ data: list, meta: meta(list.length) }),
  },
  '/rooms': { render: () => RoomsPage(), path: '/v1/rooms?limit=20&offset=0&sort=recent', list: [{ id: 'r1', slug: 'a-room' }] },
  '/agents': { render: () => AgentsPage(), path: '/v1/agents?sort=reputation&per_page=20', list: [{ id: 'agent_a' }] },
  '/users': { render: () => UsersPage(), path: '/v1/users?sort=reputation&limit=20', list: [{ id: 'u1' }] },
  '/blog': { render: () => BlogPage(), path: '/v1/blog?per_page=20', list: [{ slug: 'a-blog-post' }] },
  '/leaderboard': { render: () => LeaderboardPage(), path: '/v1/leaderboard?timeframe=all_time&per_page=50', list: [{ id: 'agent_a', rank: 1 }] },
};

const NOT_A_LIST = { data: { posts: [] } };

const FAILURES: [string, () => void][] = [
  ['the API answers 500', () => answer(500, { error: { code: 'INTERNAL_ERROR' } })],
  ['the API answers 503', () => answer(503, { error: { code: 'SERVICE_UNAVAILABLE' } })],
  ['the API answers 429', () => answer(429, { error: { code: 'RATE_LIMITED' } })],
  ['the API answers another non-2xx (400)', () => answer(400, { error: { code: 'BAD_REQUEST' } })],
  ['the API cannot be reached', () => fetchMock.mockRejectedValue(new TypeError('fetch failed'))],
  ['the body is not JSON', () => fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => { throw new SyntaxError('Unexpected token <'); } })],
  ['the body carries no list', () => answer(200, NOT_A_LIST)],
];

describe('a collection page whose list read fails', () => {
  for (const [route, page] of Object.entries(PAGES)) {
    for (const [failure, arrange] of FAILURES) {
      it(`${route}: ${failure} → the page fails (a retryable 5xx), never an empty list`, async () => {
        arrange();
        await expect(page.render()).rejects.toThrow();
        expect(received.list).toBeUndefined();
      });
    }
  }
});

describe('a collection page whose list read answers', () => {
  for (const [route, page] of Object.entries(PAGES)) {
    const body = page.body ?? ((list: unknown[]) => ({ data: list, meta: meta(list.length) }));

    it(`${route}: reads ${page.path} without the data cache, and renders what it lists`, async () => {
      answer(200, body(page.list));
      renderToStaticMarkup(await page.render());
      const [url, init] = fetchMock.mock.calls[0];
      const read = new URL(String(url));
      expect(read.pathname + read.search).toBe(page.path);
      expect(init).toMatchObject({ cache: 'no-store' });
      expect(received.list).toEqual(page.list);
    });

    it(`${route}: a real empty list is a page with an empty list`, async () => {
      answer(200, body([]));
      renderToStaticMarkup(await page.render());
      expect(received.list ?? []).toEqual([]);
    });
  }
});
