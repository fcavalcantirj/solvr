import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useShare } from './use-share';

describe('useShare', () => {
  const originalNavigator = global.navigator;
  const originalClipboard = global.navigator?.clipboard;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    // Restore navigator
    Object.defineProperty(global, 'navigator', {
      value: originalNavigator,
      writable: true,
    });
  });

  it('should have initial state', () => {
    const { result } = renderHook(() => useShare());

    expect(result.current.isSharing).toBe(false);
    expect(result.current.shared).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('should copy URL to clipboard when Web Share API is not available', async () => {
    // Mock clipboard API
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(global.navigator, 'clipboard', {
      value: { writeText },
      writable: true,
    });
    // Ensure share is not available
    Object.defineProperty(global.navigator, 'share', {
      value: undefined,
      writable: true,
    });

    const { result } = renderHook(() => useShare());

    await act(async () => {
      await result.current.share('Test Post', 'https://solvr.dev/posts/123');
    });

    expect(writeText).toHaveBeenCalledWith('https://solvr.dev/posts/123');
    expect(result.current.shared).toBe(true);
    expect(result.current.error).toBeNull();
  });

  it('should use Web Share API when available', async () => {
    // Mock Web Share API
    const shareFn = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(global.navigator, 'share', {
      value: shareFn,
      writable: true,
    });

    const { result } = renderHook(() => useShare());

    await act(async () => {
      await result.current.share('Test Post', 'https://solvr.dev/posts/123');
    });

    expect(shareFn).toHaveBeenCalledWith({
      title: 'Test Post',
      url: 'https://solvr.dev/posts/123',
    });
    expect(result.current.shared).toBe(true);
  });

  it('should handle clipboard errors', async () => {
    // Mock clipboard API that fails
    const writeText = vi.fn().mockRejectedValue(new Error('Clipboard denied'));
    Object.defineProperty(global.navigator, 'clipboard', {
      value: { writeText },
      writable: true,
    });
    Object.defineProperty(global.navigator, 'share', {
      value: undefined,
      writable: true,
    });

    const { result } = renderHook(() => useShare());

    await act(async () => {
      await result.current.share('Test Post', 'https://solvr.dev/posts/123');
    });

    expect(result.current.error).toBe('Clipboard denied');
    expect(result.current.shared).toBe(false);
  });

  it('should reset shared state after timeout', async () => {
    vi.useFakeTimers();

    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(global.navigator, 'clipboard', {
      value: { writeText },
      writable: true,
    });
    Object.defineProperty(global.navigator, 'share', {
      value: undefined,
      writable: true,
    });

    const { result } = renderHook(() => useShare());

    await act(async () => {
      await result.current.share('Test Post', 'https://solvr.dev/posts/123');
    });

    expect(result.current.shared).toBe(true);

    // Fast forward past the reset timeout
    act(() => {
      vi.advanceTimersByTime(2500);
    });

    expect(result.current.shared).toBe(false);

    vi.useRealTimers();
  });
});
