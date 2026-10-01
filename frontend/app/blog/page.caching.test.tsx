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
    await blogPage.default();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/blog?');
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(init?.next?.revalidate).toBeUndefined();
  });
});
