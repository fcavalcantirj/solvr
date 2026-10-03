import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render } from '@testing-library/react';
import { NOINDEX } from '@/lib/seo/route-policy';

// Task idx 81: a long post discussion continues on server-rendered reply pages with
// their own canonical, links to the post and their neighbours, and a real 404 past
// the last page. Page 1 is the post itself.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('next/navigation', () => ({
  notFound: () => {
    throw new Error('NEXT_NOT_FOUND');
  },
  permanentRedirect: (to: string) => {
    throw new Error(`NEXT_REDIRECT ${to}`);
  },
}));

import RepliesPage, { generateMetadata } from './page';

const post = (indexable = true) => ({
  data: { id: 'p1', title: 'Hand a plan to an executor' },
  // What GET /v1/posts/{id}/seo answers for this post (task idx 80).
  seo: { data: { indexable, description: 'x' } },
});
const replies = (page: number, totalPages: number) => ({
  data: [
    { id: `r${page}a`, body: `reply on page ${page}`, author: { id: 'agent_x', type: 'agent', display_name: 'executor' }, created_at: '2026-09-01T00:00:00Z' },
  ],
  meta: { total: 250, page, per_page: 100, total_pages: totalPages, has_more: page < totalPages },
});

const fetchMock = vi.fn();
function api(postAnswer: [number, unknown], repliesAnswer: [number, unknown]) {
  fetchMock.mockImplementation(async (url: string) => {
    const u = String(url);
    const [status, body] = u.includes('/replies') ? repliesAnswer : postAnswer;
    const seo = (body as { seo?: unknown } | null)?.seo;
    return { ok: status < 300, status, json: async () => (u.endsWith('/seo') ? seo : body) };
  });
}
const params = (page = '2') => ({ params: Promise.resolve({ id: 'p1', page }) });

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe('post reply pages', () => {
  it('server-renders the page and links the post and its neighbours', async () => {
    api([200, post()], [200, replies(2, 3)]);
    const { container } = render(await RepliesPage(params()));
    expect(container.textContent).toContain('reply on page 2');
    const hrefs = [...container.querySelectorAll('a')].map((a) => a.getAttribute('href'));
    expect(hrefs).toEqual(expect.arrayContaining(['/posts/p1', '/posts/p1/replies/3', '/agents/agent_x']));
    const [url, init] = fetchMock.mock.calls.find(([u]) => String(u).includes('/replies'))!;
    expect(String(url)).toContain('/v1/posts/p1/replies?page=2');
    expect(init).toMatchObject({ cache: 'no-store' });
  });

  it('links back to the post as the first page', async () => {
    api([200, post()], [200, replies(2, 2)]);
    const { container } = render(await RepliesPage(params()));
    const prev = container.querySelector('a[rel="prev"]');
    expect(prev?.getAttribute('href')).toBe('/posts/p1');
  });

  it('has its own canonical and follows the post page index verdict', async () => {
    api([200, post()], [200, replies(2, 3)]);
    const metadata = await generateMetadata(params());
    expect(metadata.alternates?.canonical).toBe('/posts/p1/replies/2');
    expect(metadata.robots).toBeUndefined();
    api([200, post(false)], [200, replies(2, 3)]);
    expect((await generateMetadata(params())).robots).toEqual(NOINDEX);
  });

  it('sends page 1 to the post itself', async () => {
    await expect(RepliesPage(params('1'))).rejects.toThrow('NEXT_REDIRECT /posts/p1');
  });

  it('answers a real 404 past the last page or for a non-canonical number', async () => {
    api([200, post()], [404, null]);
    await expect(RepliesPage(params('9'))).rejects.toThrow('NEXT_NOT_FOUND');
    for (const page of ['0', '02', 'x']) {
      await expect(RepliesPage(params(page))).rejects.toThrow('NEXT_NOT_FOUND');
    }
  });

  it('answers a real 404 when the post is gone and fails retryably when the API fails', async () => {
    api([404, null], [404, null]);
    await expect(RepliesPage(params())).rejects.toThrow('NEXT_NOT_FOUND');
    api([200, post()], [502, null]);
    await expect(RepliesPage(params())).rejects.toThrow(/502/);
  });
});

describe('post reply page structured data', () => {
  it('carries breadcrumbs from the posts list through the post to this page', async () => {
    api([200, post()], [200, replies(2, 3)]);
    const { container } = render(await RepliesPage(params()));
    const blocks = [...container.querySelectorAll('script[type="application/ld+json"]')].map((s) => JSON.parse(s.innerHTML));
    const crumbs = blocks.find((b) => b['@type'] === 'BreadcrumbList');
    expect(crumbs.itemListElement.map((i: { item: string }) => i.item)).toEqual([
      'https://solvr.dev/', 'https://solvr.dev/posts', 'https://solvr.dev/posts/p1', 'https://solvr.dev/posts/p1/replies/2',
    ]);
  });
});
