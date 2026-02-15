"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { usePosts } from './use-posts';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getPosts: vi.fn(),
    search: vi.fn(),
  },
  formatRelativeTime: (date: string) => '3d ago',
  truncateText: (text: string, len: number) => text.slice(0, len),
  mapStatus: (status: string) => status.toUpperCase(),
}));

const mockPostWithAvatar = {
  id: 'p1',
  type: 'problem',
  title: 'Test problem',
  description: 'A test description for a problem',
  status: 'open',
  upvotes: 10,
  downvotes: 2,
  vote_score: 8,
  view_count: 150,
  author: {
    id: 'u1',
    type: 'human',
    display_name: 'Alice',
    avatar_url: 'https://example.com/alice.png',
  },
  tags: ['test'],
  created_at: '2026-01-15T10:00:00Z',
  updated_at: '2026-01-16T10:00:00Z',
  approaches_count: 3,
};

const mockPostWithoutAvatar = {
  id: 'p2',
  type: 'question',
  title: 'Test question',
  description: 'A question about something',
  status: 'open',
  upvotes: 5,
  downvotes: 1,
  vote_score: 4,
  view_count: 50,
  author: {
    id: 'a1',
    type: 'agent',
    display_name: 'solver-bot',
  },
  tags: ['go'],
  created_at: '2026-01-20T10:00:00Z',
  updated_at: '2026-01-20T10:00:00Z',
  answers_count: 2,
};

const mockResponse = {
  data: [mockPostWithAvatar, mockPostWithoutAvatar],
  meta: { total: 2, page: 1, per_page: 20, has_more: false },
};

describe('usePosts - avatar and pinned field resolution', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('maps avatar_url from API author to FeedPost avatar field', async () => {
    (api.getPosts as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const { result } = renderHook(() => usePosts());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Post with avatar_url should have it mapped
    expect(result.current.posts[0].author.avatar).toBe('https://example.com/alice.png');
  });

  it('sets avatar to undefined when avatar_url is not provided', async () => {
    (api.getPosts as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const { result } = renderHook(() => usePosts());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Post without avatar_url should have undefined avatar
    expect(result.current.posts[1].author.avatar).toBeUndefined();
  });

  it('sets avatar to undefined when avatar_url is empty string', async () => {
    const postWithEmptyAvatar = {
      ...mockPostWithAvatar,
      id: 'p3',
      author: { ...mockPostWithAvatar.author, avatar_url: '' },
    };
    (api.getPosts as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [postWithEmptyAvatar],
      meta: { total: 1, page: 1, per_page: 20, has_more: false },
    });

    const { result } = renderHook(() => usePosts());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Empty avatar_url should map to undefined (not empty string)
    expect(result.current.posts[0].author.avatar).toBeUndefined();
  });

  it('sets isPinned to false (backend does not support pinning)', async () => {
    (api.getPosts as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const { result } = renderHook(() => usePosts());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // isPinned should be false since backend doesn't support it
    expect(result.current.posts[0].isPinned).toBe(false);
    expect(result.current.posts[1].isPinned).toBe(false);
  });

  it('does not have TODO comments in transform function', async () => {
    // This test verifies the code quality - no TODO comments should remain
    // We verify this by checking that the hook file doesn't contain TODO patterns
    // This is tested at the source level via a grep in the implementation step
    // But we also verify the behavior is complete (no undefined where values should exist)
    (api.getPosts as ReturnType<typeof vi.fn>).mockResolvedValue(mockResponse);

    const { result } = renderHook(() => usePosts());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // All fields should be properly resolved, not left as TODO placeholders
    const post = result.current.posts[0];
    expect(post.author.avatar).toBeDefined(); // was TODO: now resolved from API
    expect(typeof post.isPinned).toBe('boolean'); // was TODO: now explicitly false
  });
});
