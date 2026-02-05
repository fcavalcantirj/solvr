import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { usePolling } from './use-polling';

describe('usePolling', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('should call callback immediately on mount', async () => {
    const callback = vi.fn().mockResolvedValue(undefined);
    renderHook(() => usePolling(callback, 5000));

    expect(callback).toHaveBeenCalledTimes(1);
  });

  it('should call callback at specified interval', async () => {
    const callback = vi.fn().mockResolvedValue(undefined);
    renderHook(() => usePolling(callback, 5000));

    expect(callback).toHaveBeenCalledTimes(1);

    await act(async () => {
      vi.advanceTimersByTime(5000);
    });
    expect(callback).toHaveBeenCalledTimes(2);

    await act(async () => {
      vi.advanceTimersByTime(5000);
    });
    expect(callback).toHaveBeenCalledTimes(3);
  });

  it('should not poll when interval is 0', async () => {
    const callback = vi.fn().mockResolvedValue(undefined);
    renderHook(() => usePolling(callback, 0));

    expect(callback).toHaveBeenCalledTimes(1);

    await act(async () => {
      vi.advanceTimersByTime(10000);
    });
    expect(callback).toHaveBeenCalledTimes(1);
  });

  it('should stop polling when enabled is false', async () => {
    const callback = vi.fn().mockResolvedValue(undefined);
    const { rerender } = renderHook(
      ({ enabled }) => usePolling(callback, 5000, { enabled }),
      { initialProps: { enabled: true } }
    );

    expect(callback).toHaveBeenCalledTimes(1);

    await act(async () => {
      vi.advanceTimersByTime(5000);
    });
    expect(callback).toHaveBeenCalledTimes(2);

    rerender({ enabled: false });

    await act(async () => {
      vi.advanceTimersByTime(5000);
    });
    expect(callback).toHaveBeenCalledTimes(2);
  });

  it('should handle callback errors gracefully', async () => {
    const callback = vi.fn().mockRejectedValue(new Error('Network error'));
    const { result } = renderHook(() => usePolling(callback, 5000));

    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(result.current.error).toBe('Network error');
    expect(callback).toHaveBeenCalledTimes(1);

    // Should continue polling despite error
    await act(async () => {
      vi.advanceTimersByTime(5000);
    });
    expect(callback).toHaveBeenCalledTimes(2);
  });

  it('should expose isPolling state', async () => {
    let resolvePromise: () => void;
    const callback = vi.fn().mockImplementation(() => {
      return new Promise<void>((resolve) => {
        resolvePromise = resolve;
      });
    });

    const { result } = renderHook(() => usePolling(callback, 5000));

    expect(result.current.isPolling).toBe(true);

    await act(async () => {
      resolvePromise!();
    });

    expect(result.current.isPolling).toBe(false);
  });

  it('should cleanup interval on unmount', async () => {
    const callback = vi.fn().mockResolvedValue(undefined);
    const { unmount } = renderHook(() => usePolling(callback, 5000));

    expect(callback).toHaveBeenCalledTimes(1);
    unmount();

    await act(async () => {
      vi.advanceTimersByTime(10000);
    });
    expect(callback).toHaveBeenCalledTimes(1);
  });
});
