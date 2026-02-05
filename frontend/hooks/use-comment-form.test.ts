"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useCommentForm } from './use-comment-form';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    createComment: vi.fn(),
  },
}));

describe('useCommentForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should have initial empty state', () => {
    // Act
    const { result } = renderHook(() => useCommentForm('answer', 'answer-123', vi.fn()));

    // Assert
    expect(result.current.content).toBe('');
    expect(result.current.isSubmitting).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('should update content when setContent is called', () => {
    // Act
    const { result } = renderHook(() => useCommentForm('answer', 'answer-123', vi.fn()));

    act(() => {
      result.current.setContent('My comment');
    });

    // Assert
    expect(result.current.content).toBe('My comment');
  });

  it('should submit comment and clear form on success', async () => {
    // Arrange
    const onSuccess = vi.fn();
    (api.createComment as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: {
        id: 'comment-456',
        content: 'My comment',
      }
    });

    // Act
    const { result } = renderHook(() => useCommentForm('answer', 'answer-123', onSuccess));

    act(() => {
      result.current.setContent('My comment');
    });

    await act(async () => {
      await result.current.submit();
    });

    // Assert
    expect(api.createComment).toHaveBeenCalledWith('answer', 'answer-123', 'My comment');
    expect(result.current.content).toBe(''); // Form cleared
    expect(result.current.isSubmitting).toBe(false);
    expect(result.current.error).toBeNull();
    expect(onSuccess).toHaveBeenCalled();
  });

  it('should handle API errors', async () => {
    // Arrange
    const onSuccess = vi.fn();
    (api.createComment as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('Auth required')
    );

    // Act
    const { result } = renderHook(() => useCommentForm('approach', 'approach-123', onSuccess));

    act(() => {
      result.current.setContent('My comment');
    });

    await act(async () => {
      await result.current.submit();
    });

    // Assert
    expect(result.current.error).toBe('Auth required');
    expect(result.current.content).toBe('My comment'); // Form not cleared
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('should not submit if content is empty', async () => {
    // Arrange
    const onSuccess = vi.fn();

    // Act
    const { result } = renderHook(() => useCommentForm('response', 'response-123', onSuccess));

    await act(async () => {
      await result.current.submit();
    });

    // Assert
    expect(api.createComment).not.toHaveBeenCalled();
    expect(result.current.error).toBe('Comment content is required');
  });

  it('should set isSubmitting during API call', async () => {
    // Arrange
    let resolvePromise: (value: unknown) => void;
    const promise = new Promise((resolve) => {
      resolvePromise = resolve;
    });
    (api.createComment as ReturnType<typeof vi.fn>).mockReturnValue(promise);

    // Act
    const { result } = renderHook(() => useCommentForm('answer', 'answer-123', vi.fn()));

    act(() => {
      result.current.setContent('My comment');
    });

    act(() => {
      result.current.submit();
    });

    // Assert - isSubmitting should be true during call
    expect(result.current.isSubmitting).toBe(true);

    // Resolve the promise
    await act(async () => {
      resolvePromise!({ data: { id: 'comment-456' } });
      await promise;
    });

    // Assert - isSubmitting should be false after call
    expect(result.current.isSubmitting).toBe(false);
  });
});
