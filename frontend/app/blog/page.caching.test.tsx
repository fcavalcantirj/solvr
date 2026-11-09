import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// The blog index renders the published posts the API lists. A blog post that is
// deleted or unpublished leaves that list at once, so the page must ask the API on
// every request: a stored copy would keep showing its title and excerpt.

// React's request-scoped cache() only exists in the server build; pass through here.
vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/blog/blog-page-client', () => ({ BlogPageClient: () => null }));

import * as blogPage from './page';

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => ({ data: [] }) });
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('blog index page caching', () => {
  it('renders on every request instead of serving a stored copy', () => {
    expect(blogPage.dynamic).toBe('force-dynamic');
    expect((blogPage as Record<string, unknown>).revalidate).toBeUndefined();
  });

  it('fetches the published posts from the API without the data cache', async () => {
    await blogPage.default({ searchParams: Promise.resolve({}) });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/blog?');
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(init?.next?.revalidate).toBeUndefined();
  });
});

// SEO audit 2026-11-08: the tag links of the blog pointed at /blog?tag=<name>, but the page read
// no query, so every tag landed on the unfiltered list; and every query variant was indexable
// under the bare /blog canonical. The page now filters by tag and pages through the API, and
// a query variant stays usable but out of the index, like /posts (lib/seo/route-policy.ts).
describe('blog index page query variants', () => {
  it('asks the API for the tag and page the URL names', async () => {
    await blogPage.default({ searchParams: Promise.resolve({ tag: 'esp32', page: '2' }) });

    const [url] = fetchMock.mock.calls[0];
    const u = new URL(String(url));
    expect(u.pathname).toBe('/v1/blog');
    expect(u.searchParams.get('tags')).toBe('esp32');
    expect(u.searchParams.get('page')).toBe('2');
    expect(u.searchParams.get('per_page')).toBe('20');
  });

  it('indexes the bare blog and keeps a tag or page variant out of the index, canonical unchanged', async () => {
    const bare = await blogPage.generateMetadata({ searchParams: Promise.resolve({}) });
    const tagged = await blogPage.generateMetadata({ searchParams: Promise.resolve({ tag: 'esp32' }) });
    const paged = await blogPage.generateMetadata({ searchParams: Promise.resolve({ page: '2' }) });
    const tracked = await blogPage.generateMetadata({ searchParams: Promise.resolve({ utm_source: 'x' }) });

    expect(bare.robots).toBeUndefined();
    expect(tracked.robots).toBeUndefined();
    expect(tagged.robots).toEqual({ index: false, follow: true });
    expect(paged.robots).toEqual({ index: false, follow: true });
    expect(tagged.alternates?.canonical).toBe('/blog');
  });
});
