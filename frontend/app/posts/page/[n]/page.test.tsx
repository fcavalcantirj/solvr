import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render } from '@testing-library/react';
import { renderToStaticMarkup } from 'react-dom/server';
import { previewProblems, readPreview } from '@/lib/seo/preview-check';

// SPEC.md 27.2: the post archive. 446 of 467 posts the sitemap listed had no inbound link:
// /posts shows twenty and loads the rest in the browser. /posts/page/{n} is page n of the
// API's indexable list, fifty posts to a page, each a plain link in server HTML. Its own
// canonical, indexable, a real 404 past the last page, and a retryable 5xx when the API fails.

vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => {
    // What a link asks of the router (prefetch) is not an attribute of the element.
    const attributes = { ...props };
    delete attributes.prefetch;
    return <a href={href} {...attributes}>{children}</a>;
  },
}));
vi.mock('next/navigation', () => ({
  notFound: () => {
    throw new Error('NEXT_NOT_FOUND');
  },
}));

import ArchivePage, { dynamic, generateMetadata } from './page';

const post = (i: number) => ({
  id: `post-${i}`,
  title: `Post number ${i}`,
  created_at: '2026-09-01T10:00:00Z',
  author: i % 2 === 0 ? { id: `agent_${i}`, type: 'agent', display_name: `Agent ${i}` } : { id: `user-${i}`, type: 'human', display_name: `Person ${i}` },
});
// What GET /v1/posts?indexable=true&sort=new&page=N&per_page=50 answers for a list of `total` posts.
const list = (page: number, total: number) => {
  const pages = Math.ceil(total / 50);
  const first = (page - 1) * 50;
  const count = Math.max(0, Math.min(50, total - first));
  return {
    data: Array.from({ length: count }, (_, i) => post(first + i + 1)),
    meta: { total, page, per_page: 50, total_pages: pages, has_more: page < pages },
  };
};

const fetchMock = vi.fn();
const answer = (status: number, body: unknown) =>
  fetchMock.mockResolvedValue({ ok: status < 300, status, json: async () => body });
const params = (n: string) => ({ params: Promise.resolve({ n }) });
const hrefs = (container: HTMLElement) => [...container.querySelectorAll('a')].map((a) => a.getAttribute('href'));

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe('the post archive page', () => {
  it('asks the API on every request for page n of the indexable posts, newest first, fifty a page', async () => {
    answer(200, list(2, 467));
    await ArchivePage(params('2'));
    expect(dynamic).toBe('force-dynamic');
    const [url, init] = fetchMock.mock.calls[0];
    expect(new URL(String(url)).pathname + new URL(String(url)).search).toBe('/v1/posts?indexable=true&sort=new&page=2&per_page=50');
    expect(init).toMatchObject({ cache: 'no-store' });
  });

  it('server-renders every post of the page as a plain link with its title', async () => {
    answer(200, list(2, 467));
    const html = renderToStaticMarkup(await ArchivePage(params('2')));
    for (let i = 51; i <= 100; i++) {
      expect(html, `post ${i}`).toContain(`href="/posts/post-${i}"`);
    }
    expect(html).toMatch(/<a\b[^>]*href="\/posts\/post-51"[^>]*>Post number 51<\/a>/);
    expect(html.match(/href="\/posts\/post-\d+"/g)).toHaveLength(50);
  });

  it('gives each row its date and a link to its author, an agent or a person', async () => {
    answer(200, list(1, 3));
    const { container } = render(await ArchivePage(params('1')));
    const all = hrefs(container);
    expect(all).toEqual(expect.arrayContaining(['/users/user-1', '/agents/agent_2', '/users/user-3']));
    expect([...container.querySelectorAll('ol[data-testid="archive-posts"] time')].map((t) => t.textContent)).toEqual([
      '2026-09-01', '2026-09-01', '2026-09-01',
    ]);
  });

  it('names where it is: one h1 with the page and the number of pages, and the range of posts', async () => {
    answer(200, list(2, 467));
    const { container } = render(await ArchivePage(params('2')));
    const headings = container.querySelectorAll('h1');
    expect(headings).toHaveLength(1);
    expect(headings[0].textContent).toBe('Posts, page 2 of 10');
    expect(container.textContent).toContain('Posts 51 to 100 of 467');
  });

  it('links the previous and next pages, the first and the last, every page by number, and the collection', async () => {
    answer(200, list(4, 467));
    const { container } = render(await ArchivePage(params('4')));
    expect(container.querySelector('a[rel="prev"]')?.getAttribute('href')).toBe('/posts/page/3');
    expect(container.querySelector('a[rel="next"]')?.getAttribute('href')).toBe('/posts/page/5');
    const pager = container.querySelector('nav[aria-label="Archive pages"]')!;
    const named = Object.fromEntries([...pager.querySelectorAll('a')].map((a) => [a.textContent, a.getAttribute('href')]));
    expect(named['First page']).toBe('/posts/page/1');
    expect(named['Last page']).toBe('/posts/page/10');
    for (let page = 1; page <= 10; page++) {
      if (page === 4) continue;
      expect(named[`Page ${page}`], `page ${page}`).toBe(`/posts/page/${page}`);
    }
    // The page it is on is named, not linked.
    expect(pager.querySelector('[aria-current="page"]')?.textContent).toBe('Page 4');
    expect(named['Page 4']).toBeUndefined();
    expect(hrefs(container)).toContain('/posts');
  });

  it('has no previous or first link on the first page, and no next or last on the last', async () => {
    answer(200, list(1, 467));
    const first = render(await ArchivePage(params('1'))).container;
    expect(first.querySelector('a[rel="prev"]')).toBeNull();
    expect(first.querySelector('a[rel="next"]')?.getAttribute('href')).toBe('/posts/page/2');
    expect([...first.querySelectorAll('a')].map((a) => a.textContent)).not.toContain('First page');

    answer(200, list(10, 467));
    const last = render(await ArchivePage(params('10'))).container;
    expect(last.querySelector('a[rel="next"]')).toBeNull();
    expect(last.querySelector('a[rel="prev"]')?.getAttribute('href')).toBe('/posts/page/9');
    expect([...last.querySelectorAll('a')].map((a) => a.textContent)).not.toContain('Last page');
    expect(last.querySelectorAll('ol[data-testid="archive-posts"] li')).toHaveLength(17);
  });

  it('lists the ends of a long archive by number, and still links its neighbours', async () => {
    answer(200, list(12, 50 * 30));
    const { container } = render(await ArchivePage(params('12')));
    const pager = container.querySelector('nav[aria-label="Archive pages"]')!;
    const numbered = [...pager.querySelectorAll('a')].map((a) => a.textContent).filter((t) => /^Page \d+$/.test(t ?? ''));
    expect(numbered).toEqual(['Page 1', 'Page 2', 'Page 3', 'Page 28', 'Page 29', 'Page 30']);
    expect(container.querySelector('a[rel="prev"]')?.getAttribute('href')).toBe('/posts/page/11');
    expect(container.querySelector('a[rel="next"]')?.getAttribute('href')).toBe('/posts/page/13');
  });

  it('marks its links for the click listener', async () => {
    answer(200, list(2, 467));
    const { container } = render(await ArchivePage(params('2')));
    const pager = container.querySelector('nav[aria-label="Archive pages"]')!;
    const marks = Object.fromEntries(
      [...pager.querySelectorAll('a')].map((a) => [a.textContent, [a.getAttribute('data-track'), a.getAttribute('data-track-item'), a.getAttribute('data-track-location')]]),
    );
    expect(marks['First page']).toEqual(['nav', 'archive_first', 'page']);
    expect(marks['Newer posts']).toEqual(['nav', 'archive_newer', 'page']);
    expect(marks['Older posts']).toEqual(['nav', 'archive_older', 'page']);
    expect(marks['Last page']).toEqual(['nav', 'archive_last', 'page']);
    expect(marks['Page 7']).toEqual(['nav', 'archive_page', 'page']);
    for (const link of container.querySelectorAll('main a')) {
      expect(link.getAttribute('data-track'), link.outerHTML).toBe('nav');
    }
  });

  it('carries breadcrumbs from the posts collection to this page', async () => {
    answer(200, list(2, 467));
    const { container } = render(await ArchivePage(params('2')));
    const blocks = [...container.querySelectorAll('script[type="application/ld+json"]')].map((s) => JSON.parse(s.innerHTML));
    const crumbs = blocks.find((b) => b['@type'] === 'BreadcrumbList');
    expect(crumbs.itemListElement.map((i: { item: string }) => i.item)).toEqual([
      'https://solvr.dev/', 'https://solvr.dev/posts', 'https://solvr.dev/posts/page/2',
    ]);
  });
});

describe('the post archive page metadata', () => {
  it('is its own canonical, indexable, titled by its page and the number of pages', async () => {
    answer(200, list(2, 467));
    const metadata = await generateMetadata(params('2'));
    expect(metadata.title).toBe('Posts, page 2 of 10');
    expect(metadata.alternates?.canonical).toBe('/posts/page/2');
    // No robots directive is an indexable, followable page.
    expect(metadata.robots).toBeUndefined();
    expect(String(metadata.description)).toContain('Page 2 of 10');
    expect(String(metadata.description).length).toBeGreaterThan(50);
    expect(String(metadata.description).length).toBeLessThanOrEqual(160);
  });

  it('states its own link preview, under that title and address', async () => {
    answer(200, list(2, 467));
    const metadata = await generateMetadata(params('2'));
    expect(previewProblems(metadata, { homeTitle: 'the home page title' })).toEqual([]);
    expect(readPreview(metadata)).toMatchObject({ title: 'Posts, page 2 of 10 | Solvr', url: 'https://solvr.dev/posts/page/2', type: 'website' });
  });

  it('gives every page of the archive a title and a description of its own', async () => {
    const seen = new Set<string>();
    for (const n of ['1', '2', '10']) {
      answer(200, list(Number(n), 467));
      const metadata = await generateMetadata(params(n));
      seen.add(String(metadata.title));
      seen.add(String(metadata.description));
    }
    expect(seen.size).toBe(6);
  });
});

describe('a post archive page that does not exist', () => {
  it('answers a real 404 past the last page', async () => {
    answer(200, list(11, 467));
    await expect(ArchivePage(params('11'))).rejects.toThrow('NEXT_NOT_FOUND');
    await expect(generateMetadata(params('11'))).rejects.toThrow('NEXT_NOT_FOUND');
  });

  it.each(['0', '01', '-1', '1.5', 'x', '1e2', ' 1'])('answers a real 404 for %s without asking the API', async (n) => {
    await expect(ArchivePage(params(n))).rejects.toThrow('NEXT_NOT_FOUND');
    await expect(generateMetadata(params(n))).rejects.toThrow('NEXT_NOT_FOUND');
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('shows page 1 of an empty collection, and no page 2', async () => {
    answer(200, list(1, 0));
    const { container } = render(await ArchivePage(params('1')));
    expect(container.querySelector('h1')?.textContent).toBe('Posts, page 1 of 1');
    expect(container.textContent).toContain('No posts yet.');
    expect(container.querySelector('ol[data-testid="archive-posts"]')).toBeNull();

    answer(200, list(2, 0));
    await expect(ArchivePage(params('2'))).rejects.toThrow('NEXT_NOT_FOUND');
  });
});

describe('a post archive page the API cannot answer', () => {
  it.each([500, 502, 503, 429])('fails retryably, never as a 404 or an empty page, when the API answers %i', async (status) => {
    answer(status, null);
    await expect(ArchivePage(params('2'))).rejects.toThrow(new RegExp(`/v1/posts.*${status}`));
    await expect(generateMetadata(params('2'))).rejects.toThrow(new RegExp(`${status}`));
  });

  it('fails retryably when the API cannot be reached', async () => {
    fetchMock.mockRejectedValue(new TypeError('fetch failed'));
    await expect(ArchivePage(params('2'))).rejects.toThrow(/unreachable/);
  });

  it.each([400, 404])('fails retryably, never as a 404, when the API refuses the list with %i', async (status) => {
    answer(status, { error: { code: 'X' } });
    await expect(ArchivePage(params('2'))).rejects.toThrow(new RegExp(`${status}`));
    await expect(ArchivePage(params('2'))).rejects.not.toThrow('NEXT_NOT_FOUND');
  });

  // An API that does not know the filter yet ignores it and sends no total_pages: its list is
  // not the sitemap's, so the page fails instead of linking posts the sitemap does not list.
  it('fails rather than render a list that carries no total_pages', async () => {
    const old = list(1, 467);
    answer(200, { data: old.data, meta: { total: 467, page: 1, per_page: 50, has_more: true } });
    await expect(ArchivePage(params('1'))).rejects.toThrow(/total_pages/);
  });
});
