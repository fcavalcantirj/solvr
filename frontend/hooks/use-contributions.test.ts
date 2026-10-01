import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useContributions } from './use-contributions';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getRepliesByAuthor: vi.fn(),
  },
  formatRelativeTime: vi.fn(() => '2d ago'),
}));

import { api } from '@/lib/api';
import type { APIAuthoredRepliesResponse } from '@/lib/api-types';

// idx 73 step 3: a user's contributions are their replies, listed by the canonical
// GET /v1/replies?author_type=human&author_id={id} (newest first, limit + cursor).
const author = { id: 'user-123', type: 'human' as const, display_name: 'Ada' };

const firstPage: APIAuthoredRepliesResponse = {
  data: [
    {
      id: 'reply-3',
      post_id: 'post-1',
      author,
      body: 'A reply written since the cutover',
      score: 0,
      upvotes: 0,
      downvotes: 0,
      created_at: '2026-02-10T10:00:00Z',
      updated_at: '2026-02-10T10:00:00Z',
      post: { id: 'post-1', type: 'post', title: 'How to drain a Go server?' },
    },
    {
      id: 'reply-2',
      post_id: 'post-2',
      author,
      body: 'You can use useState and useEffect...',
      score: 2,
      upvotes: 2,
      downvotes: 0,
      created_at: '2026-02-09T10:00:00Z',
      updated_at: '2026-02-09T10:00:00Z',
      legacy_type: 'answer',
      legacy_id: 'answer-1',
      post: { id: 'post-2', type: 'question', title: 'How to use React hooks?' },
    },
  ],
  meta: { total: 3, has_more: true, next_cursor: 'cursor-1' },
};

const lastPage: APIAuthoredRepliesResponse = {
  data: [
    {
      id: 'reply-1',
      post_id: 'post-3',
      author,
      body: '**Angle:** use mutex locks',
      score: 0,
      upvotes: 0,
      downvotes: 0,
      created_at: '2026-02-08T10:00:00Z',
      updated_at: '2026-02-08T10:00:00Z',
      legacy_type: 'approach',
      legacy_id: 'approach-1',
      post: { id: 'post-3', type: 'problem', title: 'Fix async race condition' },
    },
  ],
  meta: { total: 3, has_more: false },
};

describe('useContributions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches the user's replies from GET /v1/replies", async () => {
    vi.mocked(api.getRepliesByAuthor).mockResolvedValueOnce(firstPage);

    const { result } = renderHook(() => useContributions('user-123'));

    expect(result.current.loading).toBe(true);
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getRepliesByAuthor).toHaveBeenCalledWith('human', 'user-123', { limit: 20 });
    expect(result.current.contributions).toHaveLength(2);
    expect(result.current.total).toBe(3);
    expect(result.current.hasMore).toBe(true);
    expect(result.current.error).toBeNull();
  });

  it('maps a migrated reply to its post, body and legacy type', async () => {
    vi.mocked(api.getRepliesByAuthor).mockResolvedValueOnce(firstPage);

    const { result } = renderHook(() => useContributions('user-123'));
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    const answer = result.current.contributions.find((c) => c.id === 'reply-2');
    expect(answer).toEqual({
      id: 'reply-2',
      postId: 'post-2',
      postTitle: 'How to use React hooks?',
      legacyType: 'answer',
      body: 'You can use useState and useEffect...',
      timestamp: '2d ago',
      createdAt: '2026-02-09T10:00:00Z',
    });
  });

  it('gives a reply written since the cutover no legacy type', async () => {
    vi.mocked(api.getRepliesByAuthor).mockResolvedValueOnce(firstPage);

    const { result } = renderHook(() => useContributions('user-123'));
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.contributions[0].legacyType).toBeNull();
    expect(result.current.contributions[0].postTitle).toBe('How to drain a Go server?');
  });

  it('loads the next page with the cursor from meta.next_cursor', async () => {
    vi.mocked(api.getRepliesByAuthor).mockResolvedValueOnce(firstPage).mockResolvedValueOnce(lastPage);

    const { result } = renderHook(() => useContributions('user-123'));
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    act(() => {
      result.current.loadMore();
    });

    await waitFor(() => {
      expect(result.current.contributions).toHaveLength(3);
    });
    expect(api.getRepliesByAuthor).toHaveBeenLastCalledWith('human', 'user-123', { limit: 20, cursor: 'cursor-1' });
    expect(result.current.contributions.map((c) => c.id)).toEqual(['reply-3', 'reply-2', 'reply-1']);
    expect(result.current.hasMore).toBe(false);
  });

  it('handles API errors gracefully', async () => {
    vi.mocked(api.getRepliesByAuthor).mockRejectedValueOnce(new Error('Network error'));

    const { result } = renderHook(() => useContributions('user-123'));
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe('Network error');
    expect(result.current.contributions).toHaveLength(0);
  });
});
