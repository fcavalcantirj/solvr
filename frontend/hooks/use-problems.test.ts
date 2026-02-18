"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useProblems } from './use-problems';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getProblems: vi.fn(),
  },
  formatRelativeTime: (date: string) => '3d ago',
  mapStatus: (status: string) => status.toUpperCase(),
}));

const mockProblemsResponse = {
  data: [
    {
      id: 'p1',
      type: 'problem',
      title: 'Auth bug in login flow',
      description: 'Users cannot login with Google OAuth...',
      status: 'open',
      upvotes: 10,
      downvotes: 2,
      vote_score: 8,
      view_count: 150,
      author: { id: 'u1', type: 'human', display_name: 'Alice' },
      tags: ['auth', 'oauth'],
      created_at: '2026-01-15T10:00:00Z',
      updated_at: '2026-01-16T10:00:00Z',
      approaches_count: 3,
    },
    {
      id: 'p2',
      type: 'problem',
      title: 'Memory leak in worker',
      description: 'The background worker leaks memory over time...',
      status: 'in_progress',
      upvotes: 25,
      downvotes: 0,
      vote_score: 25,
      view_count: 300,
      author: { id: 'a1', type: 'agent', display_name: 'solver-bot' },
      tags: ['performance'],
      created_at: '2026-01-10T10:00:00Z',
      updated_at: '2026-01-14T10:00:00Z',
      approaches_count: 5,
    },
  ],
  meta: {
    total: 42,
    page: 1,
    per_page: 20,
    has_more: true,
  },
};

describe('useProblems', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns loading true initially', () => {
    (api.getProblems as ReturnType<typeof vi.fn>).mockImplementation(
      () => new Promise(() => {})
    );

    const { result } = renderHook(() => useProblems());

    expect(result.current.loading).toBe(true);
    expect(result.current.problems).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('fetches problems and returns transformed data', async () => {
    (api.getProblems as ReturnType<typeof vi.fn>).mockResolvedValue(mockProblemsResponse);

    const { result } = renderHook(() => useProblems());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.problems).toHaveLength(2);
    expect(result.current.problems[0].id).toBe('p1');
    expect(result.current.problems[0].title).toBe('Auth bug in login flow');
    expect(result.current.problems[0].voteScore).toBe(8);
    expect(result.current.problems[0].approachesCount).toBe(3);
    expect(result.current.problems[0].tags).toEqual(['auth', 'oauth']);
    expect(result.current.total).toBe(42);
    expect(result.current.hasMore).toBe(true);
  });

  it('passes filter params to API', async () => {
    (api.getProblems as ReturnType<typeof vi.fn>).mockResolvedValue(mockProblemsResponse);

    const { result } = renderHook(() => useProblems({
      status: 'open',
      tags: ['auth'],
      sort: 'votes',
    }));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getProblems).toHaveBeenCalledWith({
      status: 'open',
      tags: ['auth'],
      sort: 'votes',
      page: 1,
      per_page: 20,
    });
  });

  it('handles API error gracefully', async () => {
    (api.getProblems as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useProblems());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe('Network error');
    expect(result.current.problems).toEqual([]);
  });

  it('loads more problems when loadMore is called', async () => {
    (api.getProblems as ReturnType<typeof vi.fn>).mockResolvedValue(mockProblemsResponse);

    const { result } = renderHook(() => useProblems());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.problems).toHaveLength(2);

    const page2Response = {
      data: [
        {
          id: 'p3',
          type: 'problem',
          title: 'Slow query',
          description: 'Query takes too long...',
          status: 'open',
          upvotes: 5,
          downvotes: 1,
          vote_score: 4,
          view_count: 50,
          author: { id: 'u2', type: 'human', display_name: 'Bob' },
          tags: ['database'],
          created_at: '2026-01-20T10:00:00Z',
          updated_at: '2026-01-20T10:00:00Z',
          approaches_count: 1,
        },
      ],
      meta: { total: 42, page: 2, per_page: 20, has_more: false },
    };
    (api.getProblems as ReturnType<typeof vi.fn>).mockResolvedValue(page2Response);

    result.current.loadMore();

    await waitFor(() => {
      expect(result.current.problems).toHaveLength(3);
    });

    expect(result.current.problems[2].id).toBe('p3');
    expect(result.current.hasMore).toBe(false);
  });

  it('refetches and resets to page 1', async () => {
    (api.getProblems as ReturnType<typeof vi.fn>).mockResolvedValue(mockProblemsResponse);

    const { result } = renderHook(() => useProblems());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    vi.clearAllMocks();
    (api.getProblems as ReturnType<typeof vi.fn>).mockResolvedValue(mockProblemsResponse);

    result.current.refetch();

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getProblems).toHaveBeenCalledTimes(1);
  });

  it('returns empty array when no problems', async () => {
    (api.getProblems as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 20, has_more: false },
    });

    const { result } = renderHook(() => useProblems());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.problems).toEqual([]);
    expect(result.current.total).toBe(0);
    expect(result.current.hasMore).toBe(false);
  });

  it('handles null response.data gracefully', async () => {
    const mockResponse = {
      data: null as any,
      meta: { total: 0, page: 1, per_page: 20, has_more: false }
    };
    (api.getProblems as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const { result } = renderHook(() => useProblems());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe(null);
    expect(result.current.problems).toEqual([]);
    expect(result.current.total).toBe(0);
    expect(result.current.hasMore).toBe(false);
  });

  it('handles undefined response.data gracefully', async () => {
    const mockResponse = {
      meta: { total: 0, page: 1, per_page: 20, has_more: false }
    } as any;
    (api.getProblems as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const { result } = renderHook(() => useProblems());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe(null);
    expect(result.current.problems).toEqual([]);
    expect(result.current.total).toBe(0);
    expect(result.current.hasMore).toBe(false);
  });

  it('handles missing meta gracefully', async () => {
    const mockResponse = {
      data: []
    } as any;
    (api.getProblems as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const { result } = renderHook(() => useProblems());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe(null);
    expect(result.current.problems).toEqual([]);
  });

  it('handles null commentsCount from production API', async () => {
    // Production API may return null when comments table doesn't exist
    const mockProblem = {
      id: 'p1',
      type: 'problem' as const,
      title: 'Test Problem',
      description: 'Test description',
      status: 'open',
      upvotes: 5,
      downvotes: 1,
      vote_score: 4,
      view_count: 100,
      author: {
        id: 'u1',
        type: 'human' as const,
        display_name: 'Test User',
        avatar_url: ''
      },
      tags: ['test'],
      created_at: '2026-01-15T10:00:00Z',
      updated_at: '2026-01-15T10:00:00Z',
      approaches_count: 2,
      comments_count: null, // Production returns null
    };

    (api.getProblems as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [mockProblem],
      meta: { total: 1, page: 1, per_page: 20, has_more: false },
    });

    const { result } = renderHook(() => useProblems());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // null should be converted to 0
    expect(result.current.problems[0].commentsCount).toBe(0);
    expect(result.current.problems[0].commentsCount).not.toBeNull();
  });
});
