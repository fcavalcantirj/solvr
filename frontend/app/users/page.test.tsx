import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// The Users page renders the accounts the API lists. An account that deletes itself or is
// banned leaves that list at once, so the page must ask the API on every request: a
// stored copy would keep showing the removed account to every visitor until the
// next regeneration.

// React's request-scoped cache() only exists in the server build; pass through here.
vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/users/users-page-client', () => ({ UsersPageClient: () => null }));

import * as listPage from './page';

const fetchMock = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  fetchMock.mockResolvedValue({ ok: true, status: 200, json: async () => ({ data: [] }) });
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('The Users page caching', () => {
  it('renders on every request instead of serving a stored copy', () => {
    expect(listPage.dynamic).toBe('force-dynamic');
    expect((listPage as Record<string, unknown>).revalidate).toBeUndefined();
  });

  it('fetches the list from the API without the data cache', async () => {
    await listPage.default();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toContain('/v1/users?sort=reputation');
    expect(init).toMatchObject({ cache: 'no-store' });
    expect(init?.next?.revalidate).toBeUndefined();
  });
});
