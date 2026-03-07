"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useVerifyApproach } from './use-verify-approach';
import { api } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  api: {
    verifyApproach: vi.fn(),
  },
}));

describe('useVerifyApproach', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should have initial state', () => {
    const { result } = renderHook(() => useVerifyApproach(vi.fn()));

    expect(result.current.isVerifying).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('should verify approach and call onSuccess', async () => {
    const onSuccess = vi.fn();
    (api.verifyApproach as ReturnType<typeof vi.fn>).mockResolvedValue({
      message: 'Approach verified', verified: true
    });

    const { result } = renderHook(() => useVerifyApproach(onSuccess));

    await act(async () => {
      await result.current.verify('approach-123');
    });

    expect(api.verifyApproach).toHaveBeenCalledWith('approach-123');
    expect(result.current.isVerifying).toBe(false);
    expect(result.current.error).toBeNull();
    expect(onSuccess).toHaveBeenCalled();
  });

  it('should handle API errors', async () => {
    const onSuccess = vi.fn();
    (api.verifyApproach as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('Not authorized')
    );

    const { result } = renderHook(() => useVerifyApproach(onSuccess));

    await act(async () => {
      await result.current.verify('approach-123');
    });

    expect(result.current.error).toBe('Not authorized');
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('should set isVerifying during API call', async () => {
    let resolvePromise: (value: unknown) => void;
    const promise = new Promise((resolve) => {
      resolvePromise = resolve;
    });
    (api.verifyApproach as ReturnType<typeof vi.fn>).mockReturnValue(promise);

    const { result } = renderHook(() => useVerifyApproach(vi.fn()));

    act(() => {
      result.current.verify('approach-123');
    });

    expect(result.current.isVerifying).toBe(true);

    await act(async () => {
      resolvePromise!({ message: 'Approach verified', verified: true });
      await promise;
    });

    expect(result.current.isVerifying).toBe(false);
  });
});
