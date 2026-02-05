"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useAcceptAnswer } from './use-accept-answer';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    acceptAnswer: vi.fn(),
  },
}));

describe('useAcceptAnswer', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should have initial state', () => {
    // Act
    const { result } = renderHook(() => useAcceptAnswer('question-123', vi.fn()));

    // Assert
    expect(result.current.isAccepting).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('should accept answer and call onSuccess', async () => {
    // Arrange
    const onSuccess = vi.fn();
    (api.acceptAnswer as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: { accepted: true }
    });

    // Act
    const { result } = renderHook(() => useAcceptAnswer('question-123', onSuccess));

    await act(async () => {
      await result.current.accept('answer-456');
    });

    // Assert
    expect(api.acceptAnswer).toHaveBeenCalledWith('question-123', 'answer-456');
    expect(result.current.isAccepting).toBe(false);
    expect(result.current.error).toBeNull();
    expect(onSuccess).toHaveBeenCalled();
  });

  it('should handle API errors', async () => {
    // Arrange
    const onSuccess = vi.fn();
    (api.acceptAnswer as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('Not authorized')
    );

    // Act
    const { result } = renderHook(() => useAcceptAnswer('question-123', onSuccess));

    await act(async () => {
      await result.current.accept('answer-456');
    });

    // Assert
    expect(result.current.error).toBe('Not authorized');
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('should set isAccepting during API call', async () => {
    // Arrange
    let resolvePromise: (value: unknown) => void;
    const promise = new Promise((resolve) => {
      resolvePromise = resolve;
    });
    (api.acceptAnswer as ReturnType<typeof vi.fn>).mockReturnValue(promise);

    // Act
    const { result } = renderHook(() => useAcceptAnswer('question-123', vi.fn()));

    act(() => {
      result.current.accept('answer-456');
    });

    // Assert - isAccepting should be true during call
    expect(result.current.isAccepting).toBe(true);

    // Resolve the promise
    await act(async () => {
      resolvePromise!({ data: { accepted: true } });
      await promise;
    });

    // Assert - isAccepting should be false after call
    expect(result.current.isAccepting).toBe(false);
  });
});
