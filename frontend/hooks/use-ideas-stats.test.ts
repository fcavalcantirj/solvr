import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';

vi.mock('@/lib/api', () => ({
  api: {
    getIdeasStats: vi.fn().mockResolvedValue({
      data: {
        counts_by_status: { total: 0 },
        fresh_sparks: [],
        ready_to_develop: [],
        top_sparklers: [],
        trending_tags: [],
        pipeline_stats: {
          spark_to_developing: 0,
          developing_to_mature: 0,
          mature_to_realized: 0,
          avg_days_to_realization: 0,
        },
        recently_realized: [],
      },
    }),
  },
}));

import { api } from '@/lib/api';
import { useIdeasStats } from './use-ideas-stats';

describe('useIdeasStats', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('polls every 120 seconds', async () => {
    renderHook(() => useIdeasStats());

    // Initial fetch on mount
    expect(api.getIdeasStats).toHaveBeenCalledTimes(1);

    // Advance 60 seconds — should NOT have polled yet (old interval was 60s)
    await act(async () => {
      vi.advanceTimersByTime(60000);
    });
    expect(api.getIdeasStats).toHaveBeenCalledTimes(1);

    // Advance to 120 seconds total — should poll now
    await act(async () => {
      vi.advanceTimersByTime(60000);
    });
    expect(api.getIdeasStats).toHaveBeenCalledTimes(2);

    // Advance another 120 seconds — should poll again
    await act(async () => {
      vi.advanceTimersByTime(120000);
    });
    expect(api.getIdeasStats).toHaveBeenCalledTimes(3);
  });

  it('cleans up interval on unmount', async () => {
    const { unmount } = renderHook(() => useIdeasStats());

    expect(api.getIdeasStats).toHaveBeenCalledTimes(1);
    unmount();

    await act(async () => {
      vi.advanceTimersByTime(240000);
    });

    // Should only have the initial call
    expect(api.getIdeasStats).toHaveBeenCalledTimes(1);
  });
});
