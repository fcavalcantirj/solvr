"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useUser } from './use-user';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getUserProfile: vi.fn(),
    getUserPosts: vi.fn(),
  },
  formatRelativeTime: (date: string) => 'just now',
}));

const mockUserProfile = {
  id: 'user-123',
  username: 'johndoe',
  display_name: 'John Doe',
  avatar_url: 'https://example.com/avatar.png',
  bio: 'A passionate developer',
  stats: {
    posts_created: 10,
    contributions: 25,
    karma: 150,
  },
};

const mockUserPosts = [
  {
    id: 'post-1',
    type: 'question' as const,
    title: 'How to handle async errors?',
    description: 'I need help with error handling...',
    status: 'open',
    upvotes: 5,
    downvotes: 1,
    vote_score: 4,
    view_count: 100,
    author: {
      id: 'user-123',
      type: 'human' as const,
      display_name: 'John Doe',
    },
    tags: ['go', 'errors'],
    created_at: '2025-01-15T10:00:00Z',
    updated_at: '2025-01-15T12:00:00Z',
  },
  {
    id: 'post-2',
    type: 'problem' as const,
    title: 'Optimize database queries',
    description: 'Need to improve performance...',
    status: 'active',
    upvotes: 10,
    downvotes: 0,
    vote_score: 10,
    view_count: 250,
    author: {
      id: 'user-123',
      type: 'human' as const,
      display_name: 'John Doe',
    },
    tags: ['sql', 'performance'],
    created_at: '2025-01-10T08:00:00Z',
    updated_at: '2025-01-10T08:00:00Z',
  },
];

describe('useUser', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should fetch user profile and posts', async () => {
    // Arrange
    (api.getUserProfile as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockUserProfile });
    (api.getUserPosts as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: mockUserPosts,
      meta: { total: 2, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useUser('user-123'));

    // Assert - initial loading state
    expect(result.current.loading).toBe(true);
    expect(result.current.user).toBeNull();
    expect(result.current.posts).toEqual([]);

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - user loaded
    expect(result.current.user).not.toBeNull();
    expect(result.current.user?.id).toBe('user-123');
    expect(result.current.user?.username).toBe('johndoe');
    expect(result.current.user?.displayName).toBe('John Doe');
    expect(result.current.user?.avatarUrl).toBe('https://example.com/avatar.png');
    expect(result.current.user?.bio).toBe('A passionate developer');

    // Assert - stats loaded
    expect(result.current.user?.stats.postsCreated).toBe(10);
    expect(result.current.user?.stats.contributions).toBe(25);
    expect(result.current.user?.stats.karma).toBe(150);

    // Assert - posts loaded
    expect(result.current.posts).toHaveLength(2);
    expect(result.current.posts[0].id).toBe('post-1');
    expect(result.current.posts[1].id).toBe('post-2');

    // Assert - no error
    expect(result.current.error).toBeNull();
  });

  it('should handle API errors gracefully', async () => {
    // Arrange
    (api.getUserProfile as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Network error'));

    // Act
    const { result } = renderHook(() => useUser('user-123'));

    // Wait for error
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.error).toBe('Network error');
    expect(result.current.user).toBeNull();
  });

  it('should handle user not found (404)', async () => {
    // Arrange
    (api.getUserProfile as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('API error: 404'));

    // Act
    const { result } = renderHook(() => useUser('nonexistent-id'));

    // Wait for error
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.error).toBe('API error: 404');
    expect(result.current.user).toBeNull();
  });

  it('should refetch data when refetch is called', async () => {
    // Arrange
    (api.getUserProfile as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockUserProfile });
    (api.getUserPosts as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: mockUserPosts,
      meta: { total: 2, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useUser('user-123'));

    // Wait for initial load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Clear mocks and change the data
    vi.clearAllMocks();
    const updatedProfile = { ...mockUserProfile, stats: { ...mockUserProfile.stats, karma: 200 } };
    (api.getUserProfile as ReturnType<typeof vi.fn>).mockResolvedValue({ data: updatedProfile });
    (api.getUserPosts as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: mockUserPosts,
      meta: { total: 2, page: 1, per_page: 20, has_more: false },
    });

    // Refetch
    result.current.refetch();

    // Wait for refetch
    await waitFor(() => {
      expect(result.current.user?.stats.karma).toBe(200);
    });

    // Assert
    expect(api.getUserProfile).toHaveBeenCalledTimes(1);
    expect(api.getUserPosts).toHaveBeenCalledTimes(1);
  });

  it('should not fetch when id is empty', async () => {
    // Act
    const { result } = renderHook(() => useUser(''));

    // Assert - should not be loading
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // No API calls made
    expect(api.getUserProfile).not.toHaveBeenCalled();
    expect(api.getUserPosts).not.toHaveBeenCalled();
  });

  it('should handle null stats gracefully', async () => {
    // Arrange
    const profileWithNullStats = {
      ...mockUserProfile,
      stats: null,
    };
    (api.getUserProfile as ReturnType<typeof vi.fn>).mockResolvedValue({ data: profileWithNullStats });
    (api.getUserPosts as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useUser('user-123'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - should default to zeros
    expect(result.current.user?.stats.postsCreated).toBe(0);
    expect(result.current.user?.stats.contributions).toBe(0);
    expect(result.current.user?.stats.karma).toBe(0);
    expect(result.current.error).toBeNull();
  });

  it('should handle posts API returning undefined data', async () => {
    // Arrange
    (api.getUserProfile as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockUserProfile });
    (api.getUserPosts as ReturnType<typeof vi.fn>).mockResolvedValue({
      // Missing data property
      meta: { total: 0, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useUser('user-123'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - should return empty array
    expect(result.current.posts).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('should handle stats with undefined property values', async () => {
    // Arrange - API returns stats object but with undefined/null values
    const profileWithPartialStats = {
      ...mockUserProfile,
      stats: {
        posts_created: undefined,
        contributions: null,
        karma: undefined,
      },
    };
    (api.getUserProfile as ReturnType<typeof vi.fn>).mockResolvedValue({ data: profileWithPartialStats });
    (api.getUserPosts as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useUser('user-123'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - should default undefined/null values to 0
    expect(result.current.user?.stats.postsCreated).toBe(0);
    expect(result.current.user?.stats.contributions).toBe(0);
    expect(result.current.user?.stats.karma).toBe(0);
    expect(result.current.error).toBeNull();
  });
});
