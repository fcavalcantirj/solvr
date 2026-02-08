import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useCreatePost } from './use-create-post';
import { api, APICreatePostResponse } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  api: {
    createPost: vi.fn(),
  },
}));

describe('useCreatePost', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should initialize with empty form state', () => {
    const { result } = renderHook(() => useCreatePost());

    expect(result.current.form).toEqual({
      type: 'question',
      title: '',
      description: '',
      tags: [],
    });
    expect(result.current.isSubmitting).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('should update form fields', () => {
    const { result } = renderHook(() => useCreatePost());

    act(() => {
      result.current.updateForm({ title: 'Test Title' });
    });
    expect(result.current.form.title).toBe('Test Title');

    act(() => {
      result.current.updateForm({ type: 'problem', description: 'Test description' });
    });
    expect(result.current.form.type).toBe('problem');
    expect(result.current.form.description).toBe('Test description');
  });

  it('should validate required fields before submit', async () => {
    const { result } = renderHook(() => useCreatePost());

    await act(async () => {
      await result.current.submit();
    });

    expect(result.current.error).toBe('Title is required');
    expect(api.createPost).not.toHaveBeenCalled();
  });

  it('should validate title length', async () => {
    const { result } = renderHook(() => useCreatePost());

    act(() => {
      result.current.updateForm({ title: 'Short' });
    });

    await act(async () => {
      await result.current.submit();
    });

    expect(result.current.error).toBe('Title must be at least 10 characters');
    expect(api.createPost).not.toHaveBeenCalled();
  });

  it('should validate description is required', async () => {
    const { result } = renderHook(() => useCreatePost());

    act(() => {
      result.current.updateForm({ title: 'A valid title that is long enough' });
    });

    await act(async () => {
      await result.current.submit();
    });

    expect(result.current.error).toBe('Description is required');
    expect(api.createPost).not.toHaveBeenCalled();
  });

  it('should validate description length', async () => {
    const { result } = renderHook(() => useCreatePost());

    act(() => {
      result.current.updateForm({
        title: 'A valid title that is long enough',
        description: 'Too short',
      });
    });

    await act(async () => {
      await result.current.submit();
    });

    expect(result.current.error).toBe('Description must be at least 50 characters');
    expect(api.createPost).not.toHaveBeenCalled();
  });

  it('should submit valid form and return created post', async () => {
    const mockPost = {
      id: 'post-123',
      type: 'question' as const,
      title: 'A valid question title here',
      description: 'A description that is long enough to meet the minimum requirement of 50 characters.',
      tags: ['go', 'testing'],
      status: 'open',
      posted_by_type: 'human' as const,
      posted_by_id: 'user-1',
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    };

    vi.mocked(api.createPost).mockResolvedValue({ data: mockPost });

    const { result } = renderHook(() => useCreatePost());

    act(() => {
      result.current.updateForm({
        type: 'question',
        title: 'A valid question title here',
        description: 'A description that is long enough to meet the minimum requirement of 50 characters.',
        tags: ['go', 'testing'],
      });
    });

    let createdPost: unknown;
    await act(async () => {
      createdPost = await result.current.submit();
    });

    expect(api.createPost).toHaveBeenCalledWith({
      type: 'question',
      title: 'A valid question title here',
      description: 'A description that is long enough to meet the minimum requirement of 50 characters.',
      tags: ['go', 'testing'],
    });
    expect(createdPost).toEqual(mockPost);
    expect(result.current.error).toBeNull();
  });

  it('should handle API errors', async () => {
    vi.mocked(api.createPost).mockRejectedValue(new Error('Unauthorized'));

    const { result } = renderHook(() => useCreatePost());

    act(() => {
      result.current.updateForm({
        type: 'question',
        title: 'A valid question title here',
        description: 'A description that is long enough to meet the minimum requirement of 50 characters.',
      });
    });

    await act(async () => {
      await result.current.submit();
    });

    expect(result.current.error).toBe('Unauthorized');
    expect(result.current.isSubmitting).toBe(false);
  });

  it('should set isSubmitting during API call', async () => {
    let resolvePromise: (value: unknown) => void;
    const promise = new Promise((resolve) => {
      resolvePromise = resolve;
    });

    vi.mocked(api.createPost).mockReturnValue(promise as Promise<APICreatePostResponse>);

    const { result } = renderHook(() => useCreatePost());

    act(() => {
      result.current.updateForm({
        type: 'question',
        title: 'A valid question title here',
        description: 'A description that is long enough to meet the minimum requirement of 50 characters.',
      });
    });

    act(() => {
      result.current.submit();
    });

    await waitFor(() => {
      expect(result.current.isSubmitting).toBe(true);
    });

    await act(async () => {
      resolvePromise!({ data: { id: 'post-123' } });
    });

    await waitFor(() => {
      expect(result.current.isSubmitting).toBe(false);
    });
  });
});
