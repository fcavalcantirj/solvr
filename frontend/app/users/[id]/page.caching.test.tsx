import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// The user profile page renders the user's name and bio from the API. A user who
// deletes their account or is banned is refused by the API at once, so the page must
// ask the API on every request: a stored response would keep publishing the removed
// profile for an hour.

// React's request-scoped cache() only exists in the server build; pass through here.
vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
const notFound = vi.fn(() => {
  throw new Error('NEXT_NOT_FOUND');
});
vi.mock('next/navigation', () => ({ notFound: () => notFound() }));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/users/user-profile-client', () => ({ UserProfileClient: () => null }));
vi.mock('@/components/seo/json-ld', () => ({ JsonLd: () => null, userJsonLd: () => ({}) }));

import * as userPage from './page';

const fetchMock = vi.fn();
const params = Promise.resolve({ id: 'user-1' });

beforeEach(() => {
  fetchMock.mockReset();
  notFound.mockClear();
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('user profile page caching', () => {
  it('renders on every request instead of serving a stored copy', () => {
    expect(userPage.dynamic).toBe('force-dynamic');
    expect((userPage as Record<string, unknown>).revalidate).toBeUndefined();
  });

  it('reads the user from the API without the data cache', async () => {
    fetchMock.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ data: { id: 'user-1', username: 'listed', display_name: 'Listed User', bio: 'Bio' } }),
    });

    const metadata = await userPage.generateMetadata({ params });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/users/user-1');
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(init?.next?.revalidate).toBeUndefined();
    expect(metadata.title).toBe('Listed User');
  });

  it('publishes nothing and answers 404 once the API refuses the user', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 404, json: async () => ({}) });

    expect(await userPage.generateMetadata({ params })).toEqual({});
    await expect(userPage.default({ params })).rejects.toThrow('NEXT_NOT_FOUND');
    expect(notFound).toHaveBeenCalledTimes(1);
  });

  // Task idx 83: an API failure is not a missing user. A real 404 tells crawlers to
  // drop the page; a failure must stay a retryable 5xx (an error the server renders).
  it('fails retryably, never as a 404, when the API fails', async () => {
    fetchMock.mockResolvedValue({ ok: false, status: 503, json: async () => ({}) });
    await expect(userPage.default({ params })).rejects.toThrow(/503/);
    expect(notFound).not.toHaveBeenCalled();
  });

  it('fails retryably, never as a 404, when the API is unreachable', async () => {
    fetchMock.mockRejectedValue(new TypeError('fetch failed'));
    await expect(userPage.default({ params })).rejects.toThrow(/unreachable/);
    expect(notFound).not.toHaveBeenCalled();
  });
});
