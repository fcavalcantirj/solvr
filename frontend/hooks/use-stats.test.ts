import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';

vi.mock('@/lib/api', () => ({
  api: {
    getStats: vi.fn().mockResolvedValue({ data: { total_problems: 10, total_approaches: 5 } }),
    getTrending: vi.fn().mockResolvedValue({ data: { trending_problems: [], trending_tags: [] } }),
  },
}));

import { api } from '@/lib/api';
import { useStats, useTrending } from './use-stats';

describe('useStats', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('polls every 120 seconds', async () => {
    vi.mocked(api.getStats).mockResolvedValue({ data: { total_problems: 10, total_approaches: 5 } } as any);

    renderHook(() => useStats());

    // Initial fetch on mount
    expect(api.getStats).toHaveBeenCalledTimes(1);

    // Advance 60 seconds — should NOT have polled yet (old interval was 30s)
    await act(async () => {
      vi.advanceTimersByTime(60000);
    });
    expect(api.getStats).toHaveBeenCalledTimes(1);

    // Advance to 120 seconds total — should poll now
    await act(async () => {
      vi.advanceTimersByTime(60000);
    });
    expect(api.getStats).toHaveBeenCalledTimes(2);

    // Advance another 120 seconds — should poll again
    await act(async () => {
      vi.advanceTimersByTime(120000);
    });
    expect(api.getStats).toHaveBeenCalledTimes(3);
  });

  it('cleans up interval on unmount', async () => {
    vi.mocked(api.getStats).mockResolvedValue({ data: { total_problems: 10 } } as any);

    const { unmount } = renderHook(() => useStats());

    expect(api.getStats).toHaveBeenCalledTimes(1);
    unmount();

    await act(async () => {
      vi.advanceTimersByTime(240000);
    });

    // Should only have the initial call
    expect(api.getStats).toHaveBeenCalledTimes(1);
  });
});

describe('useTrending', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('polls every 120 seconds', async () => {
    vi.mocked(api.getTrending).mockResolvedValue({ data: { trending_problems: [] } } as any);

    renderHook(() => useTrending());

    // Initial fetch on mount
    expect(api.getTrending).toHaveBeenCalledTimes(1);

    // Advance 60 seconds — should NOT have polled yet (old interval was 60s)
    await act(async () => {
      vi.advanceTimersByTime(60000);
    });
    expect(api.getTrending).toHaveBeenCalledTimes(1);

    // Advance to 120 seconds total — should poll now
    await act(async () => {
      vi.advanceTimersByTime(60000);
    });
    expect(api.getTrending).toHaveBeenCalledTimes(2);

    // Advance another 120 seconds — should poll again
    await act(async () => {
      vi.advanceTimersByTime(120000);
    });
    expect(api.getTrending).toHaveBeenCalledTimes(3);
  });

  it('cleans up interval on unmount', async () => {
    vi.mocked(api.getTrending).mockResolvedValue({ data: { trending_problems: [] } } as any);

    const { unmount } = renderHook(() => useTrending());

    expect(api.getTrending).toHaveBeenCalledTimes(1);
    unmount();

    await act(async () => {
      vi.advanceTimersByTime(240000);
    });

    // Should only have the initial call
    expect(api.getTrending).toHaveBeenCalledTimes(1);
  });
});
