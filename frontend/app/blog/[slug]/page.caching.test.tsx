import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// A blog post page renders the post's full body. A post that is deleted or
// unpublished is refused by the API at once, so the page must ask the API on every
// request: a stored response would keep serving the removed body for an hour.

// React's request-scoped cache() only exists in the server build; pass through here.
vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
const notFound = vi.fn(() => {
  throw new Error('NEXT_NOT_FOUND');
});
vi.mock('next/navigation', () => ({ notFound: () => notFound() }));
vi.mock('./blog-post-content', () => ({ BlogPostContent: () => null }));
vi.mock('@/components/seo/json-ld', () => ({ JsonLd: () => null, blogPostJsonLd: () => ({}) }));

import * as blogPostPage from './page';

const fetchMock = vi.fn();
const params = Promise.resolve({ slug: 'a-post' });

beforeEach(() => {
  fetchMock.mockReset();
  notFound.mockClear();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('blog post page caching', () => {
  it('renders on every request instead of serving a stored copy', () => {
    expect(blogPostPage.dynamic).toBe('force-dynamic');
    expect((blogPostPage as Record<string, unknown>).revalidate).toBeUndefined();
  });

  it('reads the post from the API without the data cache', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ data: { slug: 'a-post', title: 'Listed', body: 'Body', excerpt: 'Ex' } }),
    });

    const metadata = await blogPostPage.generateMetadata({ params });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/blog/a-post');
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(init?.next?.revalidate).toBeUndefined();
    expect(metadata.title).toBe('Listed');
  });

  it('answers 404 once the API refuses the post', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 404, json: async () => ({}) });

    await expect(blogPostPage.default({ params })).rejects.toThrow('NEXT_NOT_FOUND');
    expect(notFound).toHaveBeenCalledTimes(1);
  });
});
