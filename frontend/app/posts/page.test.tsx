import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// The Posts collection renders the newest posts the API lists. A post that is
// deleted or turns family-only leaves that list at once, so the page must ask the
// API on every request: a stored copy would keep showing its title and
// description to every visitor until the next regeneration.

// React's request-scoped cache() only exists in the server build; pass through here.
vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/posts/posts-page-client', () => ({ PostsPageClient: () => null }));

import * as postsPage from './page';

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => ({ data: [] }) });
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('posts collection page caching', () => {
  it('renders on every request instead of serving a stored copy', () => {
    expect(postsPage.dynamic).toBe('force-dynamic');
    expect((postsPage as Record<string, unknown>).revalidate).toBeUndefined();
  });

  it('fetches the newest posts from the API without the data cache', async () => {
    await postsPage.default();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/posts?sort=newest');
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(init?.next?.revalidate).toBeUndefined();
  });
});
