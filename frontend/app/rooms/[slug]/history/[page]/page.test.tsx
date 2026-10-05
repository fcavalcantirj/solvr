import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render } from '@testing-library/react';
import { NOINDEX } from '@/lib/seo/route-policy';

// Task idx 81: each transcript segment is its own server-rendered page with an
// immutable sequence range, its own canonical, ordinary links to its neighbours,
// its authors and the room, and a real 404 for a range that does not exist.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('next/navigation', () => ({
  notFound: () => {
    throw new Error('NEXT_NOT_FOUND');
  },
}));

import HistoryPage, { generateMetadata } from './page';

const room = (indexable = true) => ({
  data: {
    room: { slug: 'kestrel', display_name: 'Kestrel build' },
    history: { page_size: 100, total_pages: 3 },
  },
  // What GET /v1/rooms/{slug}/seo answers for this room (task idx 80).
  seo: { data: { indexable, title: 'Kestrel build', description: 'Plan and ship kestrel' } },
});
const history = (overrides: Record<string, unknown> = {}) => ({
  data: {
    page: 2,
    page_size: 100,
    from_sequence: 101,
    to_sequence: 200,
    total_pages: 3,
    prev_page: 1,
    next_page: 3,
    messages: [
      { id: 7, author_type: 'agent', author_id: 'agent_planner', agent_name: 'planner', content: 'Plan **step** one', sequence_num: 101, created_at: '2026-09-01T10:00:00Z' },
      { id: 8, author_type: 'human', agent_name: 'reviewer-human', content: 'Looks good', sequence_num: 102, created_at: '2026-09-01T10:05:00Z' },
    ],
    ...overrides,
  },
});

const fetchMock = vi.fn();
// seoStatus, when given, is what the verdict read answers on its own (else it answers like the room).
function api(roomAnswer: [number, unknown], historyAnswer: [number, unknown], seoStatus?: number) {
  fetchMock.mockImplementation(async (url: string) => {
    const u = String(url);
    const [roomStatus, body] = u.includes('/history/') ? historyAnswer : roomAnswer;
    const status = u.endsWith('/seo') && seoStatus !== undefined ? seoStatus : roomStatus;
    const seo = (body as { seo?: unknown } | null)?.seo;
    return { ok: status < 300, status, json: async () => (u.endsWith('/seo') ? seo : body) };
  });
}
const params = (page = '2') => ({ params: Promise.resolve({ slug: 'kestrel', page }) });

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe('room transcript page', () => {
  it('is rendered on every request', async () => {
    expect((await import('./page')).dynamic).toBe('force-dynamic');
  });

  it('server-renders its messages and links its neighbours, authors and room', async () => {
    api([200, room()], [200, history()]);
    const { container } = render(await HistoryPage(params()));
    expect(container.textContent).toContain('Plan step one');
    expect(container.textContent).toContain('Looks good');
    const hrefs = [...container.querySelectorAll('a')].map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual(
      expect.arrayContaining([
        '/rooms/kestrel',
        '/rooms/kestrel/history/1',
        '/rooms/kestrel/history/3',
        '/agents/agent_planner',
      ])
    );
    const [url, init] = fetchMock.mock.calls.find(([u]) => String(u).includes('/history/'))!;
    expect(String(url)).toContain('/v1/rooms/kestrel/history/2');
    expect(init).toMatchObject({ cache: 'no-store' });
  });

  it('has its own canonical and a title naming its range', async () => {
    api([200, room()], [200, history()]);
    const metadata = await generateMetadata(params());
    expect(metadata.alternates?.canonical).toBe('/rooms/kestrel/history/2');
    expect(String(metadata.title)).toContain('Kestrel build');
    expect(String(metadata.title)).toContain('101');
    expect(metadata.robots).toBeUndefined();
  });

  it('is noindex when the room is not indexable or the range holds no message', async () => {
    api([200, room(false)], [200, history()]);
    expect((await generateMetadata(params())).robots).toEqual(NOINDEX);
    api([200, room()], [200, history({ messages: [] })]);
    expect((await generateMetadata(params())).robots).toEqual(NOINDEX);
  });

  // SPEC 27.4: a failed verdict read is a retryable failure, never a "not indexable".
  it('fails retryably, never as noindex, when the verdict read fails', async () => {
    api([200, room()], [200, history()], 503);
    await expect(generateMetadata(params())).rejects.toThrow(/\/v1\/rooms\/kestrel\/seo.*503/);
  });

  it('is noindex when the API refuses the verdict', async () => {
    api([200, room()], [200, history()], 403);
    expect((await generateMetadata(params())).robots).toEqual(NOINDEX);
  });

  it('answers a real 404 for a range that does not exist', async () => {
    api([200, room()], [404, null]);
    await expect(HistoryPage(params('9'))).rejects.toThrow('NEXT_NOT_FOUND');
  });

  it('answers a real 404 for a non-canonical page number without asking the API', async () => {
    for (const page of ['0', '01', '-1', 'abc', '1.5']) {
      await expect(HistoryPage(params(page))).rejects.toThrow('NEXT_NOT_FOUND');
    }
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('never serves a private room transcript', async () => {
    api([403, null], [403, null]);
    await expect(HistoryPage(params())).rejects.toThrow('NEXT_NOT_FOUND');
  });

  it('fails retryably when the API fails', async () => {
    api([200, room()], [503, null]);
    await expect(HistoryPage(params())).rejects.toThrow(/503/);
  });
});

describe('room transcript page structured data', () => {
  it('carries breadcrumbs from the rooms list through the room to this page', async () => {
    api([200, room()], [200, history()]);
    const { container } = render(await HistoryPage(params()));
    const blocks = [...container.querySelectorAll('script[type="application/ld+json"]')].map((s) => JSON.parse(s.innerHTML));
    const crumbs = blocks.find((b) => b['@type'] === 'BreadcrumbList');
    expect(crumbs.itemListElement.map((i: { item: string }) => i.item)).toEqual([
      'https://solvr.dev/', 'https://solvr.dev/rooms', 'https://solvr.dev/rooms/kestrel', 'https://solvr.dev/rooms/kestrel/history/2',
    ]);
  });
});
