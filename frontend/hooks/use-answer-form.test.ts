"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useAnswerForm } from './use-answer-form';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    createAnswer: vi.fn(),
  },
}));

describe('useAnswerForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should have initial empty state', () => {
    // Act
    const { result } = renderHook(() => useAnswerForm('question-123', vi.fn()));

    // Assert
    expect(result.current.content).toBe('');
    expect(result.current.isSubmitting).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('should update content when setContent is called', () => {
    // Act
    const { result } = renderHook(() => useAnswerForm('question-123', vi.fn()));

    act(() => {
      result.current.setContent('This is my answer');
    });

    // Assert
    expect(result.current.content).toBe('This is my answer');
  });

  it('should submit answer and clear form on success', async () => {
    // Arrange
    const onSuccess = vi.fn();
    (api.createAnswer as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: {
        id: 'answer-456',
        content: 'This is my answer',
        question_id: 'question-123',
      }
    });

    // Act
    const { result } = renderHook(() => useAnswerForm('question-123', onSuccess));

    act(() => {
      result.current.setContent('This is my answer');
    });

    await act(async () => {
      await result.current.submit();
    });

    // Assert
    expect(api.createAnswer).toHaveBeenCalledWith('question-123', 'This is my answer');
    expect(result.current.content).toBe(''); // Form cleared
    expect(result.current.isSubmitting).toBe(false);
    expect(result.current.error).toBeNull();
    expect(onSuccess).toHaveBeenCalled();
  });

  it('should handle API errors', async () => {
    // Arrange
    const onSuccess = vi.fn();
    (api.createAnswer as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('Auth required')
    );

    // Act
    const { result } = renderHook(() => useAnswerForm('question-123', onSuccess));

    act(() => {
      result.current.setContent('This is my answer');
    });

    await act(async () => {
      await result.current.submit();
    });

    // Assert
    expect(result.current.error).toBe('Auth required');
    expect(result.current.content).toBe('This is my answer'); // Form not cleared
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('should not submit if content is empty', async () => {
    // Arrange
    const onSuccess = vi.fn();

    // Act
    const { result } = renderHook(() => useAnswerForm('question-123', onSuccess));

    await act(async () => {
      await result.current.submit();
    });

    // Assert
    expect(api.createAnswer).not.toHaveBeenCalled();
    expect(result.current.error).toBe('Answer content is required');
  });

  it('should set isSubmitting during API call', async () => {
    // Arrange
    let resolvePromise: (value: unknown) => void;
    const promise = new Promise((resolve) => {
      resolvePromise = resolve;
    });
    (api.createAnswer as ReturnType<typeof vi.fn>).mockReturnValue(promise);

    // Act
    const { result } = renderHook(() => useAnswerForm('question-123', vi.fn()));

    act(() => {
      result.current.setContent('My answer');
    });

    act(() => {
      result.current.submit();
    });

    // Assert - isSubmitting should be true during call
    expect(result.current.isSubmitting).toBe(true);

    // Resolve the promise
    await act(async () => {
      resolvePromise!({ data: { id: 'answer-456' } });
      await promise;
    });

    // Assert - isSubmitting should be false after call
    expect(result.current.isSubmitting).toBe(false);
  });
});
