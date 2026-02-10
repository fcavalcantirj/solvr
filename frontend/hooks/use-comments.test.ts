"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useComments } from './use-comments';
import { api } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  api: {
    getComments: vi.fn(),
  },
  formatRelativeTime: vi.fn((date: string) => '5m ago'),
}));

const mockCommentsResponse = {
  data: [
    {
      id: 'comment-1',
      target_type: 'post',
      target_id: 'post-123',
      author_type: 'agent' as const,
      author_id: 'agent-1',
      content: 'Agent comment here',
      created_at: '2026-02-09T12:50:10Z',
      author: {
        id: 'agent-1',
        type: 'agent' as const,
        display_name: 'agent_Phil',
        avatar_url: null,
      },
    },
    {
      id: 'comment-2',
      target_type: 'post',
      target_id: 'post-123',
      author_type: 'human' as const,
      author_id: 'user-1',
      content: 'Human comment here',
      created_at: '2026-02-09T13:00:00Z',
      author: {
        id: 'user-1',
        type: 'human' as const,
        display_name: 'Felipe',
        avatar_url: 'https://example.com/avatar.jpg',
      },
    },
  ],
  meta: {
    total: 2,
    page: 1,
    per_page: 20,
    has_more: false,
  },
};

describe('useComments', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should start with loading=true and empty comments', () => {
    (api.getComments as ReturnType<typeof vi.fn>).mockReturnValue(new Promise(() => {}));

    const { result } = renderHook(() => useComments('post', 'post-123'));

    expect(result.current.comments).toEqual([]);
    expect(result.current.loading).toBe(true);
    expect(result.current.error).toBeNull();
  });

  it('should fetch comments on mount and transform API data', async () => {
    (api.getComments as ReturnType<typeof vi.fn>).mockResolvedValue(mockCommentsResponse);

    const { result } = renderHook(() => useComments('post', 'post-123'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getComments).toHaveBeenCalledWith('post', 'post-123', { page: 1, per_page: 20 });
    expect(result.current.comments).toHaveLength(2);

    // Check agent -> ai transformation
    expect(result.current.comments[0].author.type).toBe('ai');
    expect(result.current.comments[0].author.displayName).toBe('agent_Phil');
    expect(result.current.comments[0].content).toBe('Agent comment here');
    expect(result.current.comments[0].time).toBe('5m ago');

    // Check human stays human
    expect(result.current.comments[1].author.type).toBe('human');
    expect(result.current.comments[1].author.displayName).toBe('Felipe');

    expect(result.current.total).toBe(2);
    expect(result.current.hasMore).toBe(false);
  });

  it('should handle API errors gracefully', async () => {
    (api.getComments as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useComments('post', 'post-123'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe('Network error');
    expect(result.current.comments).toEqual([]);
  });

  it('should handle empty comments array', async () => {
    (api.getComments as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 20, has_more: false },
    });

    const { result } = renderHook(() => useComments('answer', 'answer-123'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.comments).toEqual([]);
    expect(result.current.total).toBe(0);
  });

  it('should not fetch when targetId is empty', async () => {
    const { result } = renderHook(() => useComments('post', ''));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getComments).not.toHaveBeenCalled();
  });

  it('should refetch when refetch is called', async () => {
    (api.getComments as ReturnType<typeof vi.fn>).mockResolvedValue(mockCommentsResponse);

    const { result } = renderHook(() => useComments('post', 'post-123'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getComments).toHaveBeenCalledTimes(1);

    await act(async () => {
      result.current.refetch();
    });

    await waitFor(() => {
      expect(api.getComments).toHaveBeenCalledTimes(2);
    });
  });

  it('should load more comments in append mode', async () => {
    const page1Response = {
      data: [mockCommentsResponse.data[0]],
      meta: { total: 2, page: 1, per_page: 1, has_more: true },
    };
    const page2Response = {
      data: [mockCommentsResponse.data[1]],
      meta: { total: 2, page: 2, per_page: 1, has_more: false },
    };

    (api.getComments as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce(page1Response)
      .mockResolvedValueOnce(page2Response);

    const { result } = renderHook(() => useComments('post', 'post-123', { perPage: 1 }));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.comments).toHaveLength(1);
    expect(result.current.hasMore).toBe(true);

    await act(async () => {
      result.current.loadMore();
    });

    await waitFor(() => {
      expect(result.current.comments).toHaveLength(2);
    });

    expect(result.current.hasMore).toBe(false);
  });
});
