"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useApproachForm } from './use-approach-form';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    createApproach: vi.fn(),
  },
}));

describe('useApproachForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should have initial empty state', () => {
    // Act
    const { result } = renderHook(() => useApproachForm('problem-123', vi.fn()));

    // Assert
    expect(result.current.angle).toBe('');
    expect(result.current.method).toBe('');
    expect(result.current.assumptions).toEqual([]);
    expect(result.current.isSubmitting).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('should update fields when setters are called', () => {
    // Act
    const { result } = renderHook(() => useApproachForm('problem-123', vi.fn()));

    act(() => {
      result.current.setAngle('My approach angle');
      result.current.setMethod('My method');
      result.current.setAssumptions(['Assumption 1', 'Assumption 2']);
    });

    // Assert
    expect(result.current.angle).toBe('My approach angle');
    expect(result.current.method).toBe('My method');
    expect(result.current.assumptions).toEqual(['Assumption 1', 'Assumption 2']);
  });

  it('should submit approach and clear form on success', async () => {
    // Arrange
    const onSuccess = vi.fn();
    (api.createApproach as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: {
        id: 'approach-456',
        problem_id: 'problem-123',
        angle: 'My approach angle',
      }
    });

    // Act
    const { result } = renderHook(() => useApproachForm('problem-123', onSuccess));

    act(() => {
      result.current.setAngle('My approach angle');
      result.current.setMethod('My method');
    });

    await act(async () => {
      await result.current.submit();
    });

    // Assert
    expect(api.createApproach).toHaveBeenCalledWith('problem-123', {
      angle: 'My approach angle',
      method: 'My method',
      assumptions: [],
    });
    expect(result.current.angle).toBe(''); // Form cleared
    expect(result.current.method).toBe('');
    expect(result.current.isSubmitting).toBe(false);
    expect(result.current.error).toBeNull();
    expect(onSuccess).toHaveBeenCalled();
  });

  it('should handle API errors', async () => {
    // Arrange
    const onSuccess = vi.fn();
    (api.createApproach as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('Auth required')
    );

    // Act
    const { result } = renderHook(() => useApproachForm('problem-123', onSuccess));

    act(() => {
      result.current.setAngle('My approach angle');
    });

    await act(async () => {
      await result.current.submit();
    });

    // Assert
    expect(result.current.error).toBe('Auth required');
    expect(result.current.angle).toBe('My approach angle'); // Form not cleared
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('should not submit if angle is empty', async () => {
    // Arrange
    const onSuccess = vi.fn();

    // Act
    const { result } = renderHook(() => useApproachForm('problem-123', onSuccess));

    await act(async () => {
      await result.current.submit();
    });

    // Assert
    expect(api.createApproach).not.toHaveBeenCalled();
    expect(result.current.error).toBe('Approach angle is required');
  });

  it('should set isSubmitting during API call', async () => {
    // Arrange
    let resolvePromise: (value: unknown) => void;
    const promise = new Promise((resolve) => {
      resolvePromise = resolve;
    });
    (api.createApproach as ReturnType<typeof vi.fn>).mockReturnValue(promise);

    // Act
    const { result } = renderHook(() => useApproachForm('problem-123', vi.fn()));

    act(() => {
      result.current.setAngle('My approach');
    });

    act(() => {
      result.current.submit();
    });

    // Assert - isSubmitting should be true during call
    expect(result.current.isSubmitting).toBe(true);

    // Resolve the promise
    await act(async () => {
      resolvePromise!({ data: { id: 'approach-456' } });
      await promise;
    });

    // Assert - isSubmitting should be false after call
    expect(result.current.isSubmitting).toBe(false);
  });
});
