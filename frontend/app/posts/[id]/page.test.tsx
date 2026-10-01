import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// The post page's title, description and tags come from the API. A post that is
// deleted or turns family-only is refused by the API from that moment on, so the
// page must ask the API on every request: a stored response would keep publishing
// the removed post's title and description in the page metadata for an hour.

// React's request-scoped cache() only exists in the server build; pass through here.
vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/posts/post-detail', () => ({ PostDetail: () => null }));

import * as postPage from './page';

const fetchMock = vi.fn();
const params = Promise.resolve({ id: 'post-1' });

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('post page caching', () => {
  it('renders on every request instead of serving a stored copy', () => {
    expect(postPage.dynamic).toBe('force-dynamic');
    expect((postPage as Record<string, unknown>).revalidate).toBeUndefined();
  });

  it('reads the post from the API without the data cache', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ data: { id: 'post-1', title: 'Listed title', description: 'Body' } }),
    });

    const metadata = await postPage.generateMetadata({ params });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/posts/post-1');
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(init?.next?.revalidate).toBeUndefined();
    expect(metadata.title).toBe('Listed title');
  });

  it('publishes no title or description once the API refuses the post', async () => {
    fetchMock.mockResolvedValue({
      ok: false,
      status: 404,
      json: async () => ({ error: { code: 'NOT_FOUND' } }),
    });

    const metadata = await postPage.generateMetadata({ params });

    expect(metadata.title).toBe('Post');
    expect(metadata.description).toBeUndefined();
    expect(metadata.openGraph).toBeUndefined();
  });
});
