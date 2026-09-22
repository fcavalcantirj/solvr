import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { useHomepageOverview } from './use-homepage-overview';
import { OVERVIEW, STALE_META } from '@/components/homepage/overview-fixture';

const mockGetOverview = vi.fn();
vi.mock('@/lib/api', () => ({
  api: {
    getOverview: () => mockGetOverview(),
  },
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe('useHomepageOverview', () => {
  it('returns whatever the API served', async () => {
    mockGetOverview.mockResolvedValue({ data: OVERVIEW, meta: STALE_META });
    const { result } = renderHook(() => useHomepageOverview());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.overview).toEqual(OVERVIEW);
    expect(result.current.meta).toEqual(STALE_META);
    expect(result.current.error).toBeNull();
  });

  it('starts out loading with nothing', () => {
    mockGetOverview.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useHomepageOverview());
    expect(result.current.loading).toBe(true);
    expect(result.current.overview).toBeNull();
  });

  it('reports the failure message rather than an empty page', async () => {
    mockGetOverview.mockRejectedValue(new Error('overview unavailable'));
    const { result } = renderHook(() => useHomepageOverview());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('overview unavailable');
    expect(result.current.overview).toBeNull();
  });

  it('reports a non-Error rejection too', async () => {
    mockGetOverview.mockRejectedValue('something odd');
    const { result } = renderHook(() => useHomepageOverview());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBeTruthy();
  });

  it('does not set state after unmount', async () => {
    let resolve: ((v: unknown) => void) | undefined;
    mockGetOverview.mockReturnValue(
      new Promise((r) => {
        resolve = r;
      }),
    );
    const { unmount } = renderHook(() => useHomepageOverview());
    unmount();
    resolve?.({ data: OVERVIEW, meta: { generated_at: new Date().toISOString() } });
    await new Promise((r) => setTimeout(r, 0));
    // No act() warning and no throw is the assertion here.
    expect(mockGetOverview).toHaveBeenCalledTimes(1);
  });

  it('swallows a rejection that lands after unmount', async () => {
    let reject: ((e: unknown) => void) | undefined;
    mockGetOverview.mockReturnValue(
      new Promise((_, r) => {
        reject = r;
      }),
    );
    const { unmount } = renderHook(() => useHomepageOverview());
    unmount();
    reject?.(new Error('too late'));
    await new Promise((r) => setTimeout(r, 0));
    expect(mockGetOverview).toHaveBeenCalledTimes(1);
  });
});
