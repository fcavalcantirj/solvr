import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useBookmarks } from './use-bookmarks';
import { api, APIAddBookmarkResponse } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  api: {
    addBookmark: vi.fn(),
    removeBookmark: vi.fn(),
    isBookmarked: vi.fn(),
    getBookmarks: vi.fn(),
  },
}));

describe('useBookmarks', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should initialize with empty state', () => {
    const { result } = renderHook(() => useBookmarks());

    expect(result.current.bookmarkedPosts).toEqual(new Set());
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('should check if a post is bookmarked', async () => {
    vi.mocked(api.isBookmarked).mockResolvedValue({ data: { bookmarked: true } });

    const { result } = renderHook(() => useBookmarks());

    let isBookmarked: boolean | undefined;
    await act(async () => {
      isBookmarked = await result.current.checkBookmarked('post-123');
    });

    expect(isBookmarked).toBe(true);
    expect(api.isBookmarked).toHaveBeenCalledWith('post-123');
  });

  it('should toggle bookmark on (add)', async () => {
    vi.mocked(api.isBookmarked).mockResolvedValue({ data: { bookmarked: false } });
    vi.mocked(api.addBookmark).mockResolvedValue({ data: { id: 'bookmark-1', post_id: 'post-123', user_type: 'human', user_id: 'user-1', created_at: '2024-01-01T00:00:00Z' } });

    const { result } = renderHook(() => useBookmarks());

    await act(async () => {
      await result.current.toggleBookmark('post-123');
    });

    expect(api.addBookmark).toHaveBeenCalledWith('post-123');
    expect(result.current.bookmarkedPosts.has('post-123')).toBe(true);
  });

  it('should toggle bookmark off (remove)', async () => {
    vi.mocked(api.removeBookmark).mockResolvedValue(undefined);

    const { result } = renderHook(() => useBookmarks());

    // First, mark it as bookmarked
    act(() => {
      result.current.bookmarkedPosts.add('post-123');
    });

    await act(async () => {
      await result.current.toggleBookmark('post-123', true);
    });

    expect(api.removeBookmark).toHaveBeenCalledWith('post-123');
    expect(result.current.bookmarkedPosts.has('post-123')).toBe(false);
  });

  it('should handle API errors gracefully', async () => {
    vi.mocked(api.addBookmark).mockRejectedValue(new Error('Unauthorized'));

    const { result } = renderHook(() => useBookmarks());

    await act(async () => {
      await result.current.toggleBookmark('post-123');
    });

    expect(result.current.error).toBe('Unauthorized');
  });

  it('should track loading state during operations', async () => {
    let resolvePromise: (value: unknown) => void;
    const promise = new Promise((resolve) => {
      resolvePromise = resolve;
    });

    vi.mocked(api.addBookmark).mockReturnValue(promise as Promise<APIAddBookmarkResponse>);

    const { result } = renderHook(() => useBookmarks());

    act(() => {
      result.current.toggleBookmark('post-123');
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(true);
    });

    await act(async () => {
      resolvePromise!({ data: { id: 'bookmark-1' } });
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });
  });
});
