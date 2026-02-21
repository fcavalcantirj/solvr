"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useUsers } from './use-users';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getUsers: vi.fn(),
  },
  formatRelativeTime: (date: string) => '5d ago',
}));

const mockUsersResponse = {
  data: [
    {
      id: 'user-1',
      username: 'johndoe',
      display_name: 'John Doe',
      avatar_url: 'https://example.com/avatar1.png',
      reputation: 1500,
      agents_count: 3,
      created_at: '2025-01-01T10:00:00Z',
    },
    {
      id: 'user-2',
      username: 'janedoe',
      display_name: 'Jane Doe',
      avatar_url: null,
      reputation: 800,
      agents_count: 1,
      created_at: '2025-01-15T10:00:00Z',
    },
  ],
  meta: {
    total: 50,
    page: 1,
    per_page: 20,
    has_more: true,
    total_backed_agents: 157,
  },
};

describe('useUsers', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns loading true initially', () => {
    // Arrange - make the API hang
    (api.getUsers as ReturnType<typeof vi.fn>).mockImplementation(
      () => new Promise(() => {}) // Never resolves
    );

    // Act
    const { result } = renderHook(() => useUsers());

    // Assert
    expect(result.current.loading).toBe(true);
    expect(result.current.users).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('fetches users and transforms to camelCase', async () => {
    // Arrange
    (api.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue(mockUsersResponse);

    // Act
    const { result } = renderHook(() => useUsers());

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - camelCase transformation
    expect(result.current.users).toHaveLength(2);
    expect(result.current.users[0].id).toBe('user-1');
    expect(result.current.users[0].username).toBe('johndoe');
    expect(result.current.users[0].displayName).toBe('John Doe');
    expect(result.current.users[0].avatarUrl).toBe('https://example.com/avatar1.png');
    expect(result.current.users[0].reputation).toBe(1500);
    expect(result.current.users[0].agentsCount).toBe(3);
    expect(result.current.users[0].createdAt).toBe('5d ago');
    expect(result.current.total).toBe(50);
    expect(result.current.hasMore).toBe(true);
    expect(result.current.error).toBeNull();
  });

  it('handles API error gracefully', async () => {
    // Arrange
    (api.getUsers as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Network error'));

    // Act
    const { result } = renderHook(() => useUsers());

    // Wait for error
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.error).toBe('Network error');
    expect(result.current.users).toEqual([]);
  });

  it('passes sort option to API', async () => {
    // Arrange
    (api.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue(mockUsersResponse);

    // Act
    const { result } = renderHook(() => useUsers({ sort: 'reputation' }));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(api.getUsers).toHaveBeenCalledWith({
      limit: 20,
      offset: 0,
      sort: 'reputation',
    });
  });

  it('passes limit and offset to API', async () => {
    // Arrange
    (api.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue(mockUsersResponse);

    // Act
    const { result } = renderHook(() => useUsers({ limit: 10, offset: 20 }));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - default sort is 'reputation' when not explicitly provided
    expect(api.getUsers).toHaveBeenCalledWith({
      limit: 10,
      offset: 20,
      sort: 'reputation',
    });
  });

  it('defaults to reputation sort when no sort option provided', async () => {
    // Arrange
    (api.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue(mockUsersResponse);

    // Act
    const { result } = renderHook(() => useUsers());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(api.getUsers).toHaveBeenCalledWith({
      limit: 20,
      offset: 0,
      sort: 'reputation',
    });
  });

  it('loads more users when loadMore is called', async () => {
    // Arrange
    (api.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue(mockUsersResponse);

    // Act
    const { result } = renderHook(() => useUsers({ limit: 20 }));

    // Wait for initial load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.users).toHaveLength(2);

    // Prepare second page
    const page2Response = {
      data: [
        {
          id: 'user-3',
          username: 'bobsmith',
          display_name: 'Bob Smith',
          avatar_url: null,
          reputation: 500,
          agents_count: 0,
          created_at: '2025-02-01T10:00:00Z',
        },
      ],
      meta: {
        total: 50,
        page: 2,
        per_page: 20,
        has_more: false,
        total_backed_agents: 157,
      },
    };
    (api.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue(page2Response);

    // Load more
    result.current.loadMore();

    await waitFor(() => {
      expect(result.current.users).toHaveLength(3);
    });

    // Assert - users appended
    expect(result.current.users[2].id).toBe('user-3');
    expect(result.current.hasMore).toBe(false);
  });

  it('refetches when refetch is called', async () => {
    // Arrange
    (api.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue(mockUsersResponse);

    // Act
    const { result } = renderHook(() => useUsers());

    // Wait for initial load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Clear mocks
    vi.clearAllMocks();
    (api.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue(mockUsersResponse);

    // Refetch
    result.current.refetch();

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(api.getUsers).toHaveBeenCalledTimes(1);
  });

  it('handles users with null avatar_url', async () => {
    // Arrange
    (api.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue(mockUsersResponse);

    // Act
    const { result } = renderHook(() => useUsers());

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - second user has null avatar
    expect(result.current.users[1].avatarUrl).toBeUndefined();
  });

  it('computes initials from display_name', async () => {
    // Arrange
    (api.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue(mockUsersResponse);

    // Act
    const { result } = renderHook(() => useUsers());

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - initials computed
    expect(result.current.users[0].initials).toBe('JO');
    expect(result.current.users[1].initials).toBe('JA');
  });

  it('returns totalBackedAgents from API meta', async () => {
    // Arrange
    (api.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue(mockUsersResponse);

    // Act
    const { result } = renderHook(() => useUsers());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - totalBackedAgents comes from API meta, not client-side sum
    expect(result.current.totalBackedAgents).toBe(157);
  });

  it('returns empty array when no users', async () => {
    // Arrange
    (api.getUsers as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 20, has_more: false, total_backed_agents: 0 },
    });

    // Act
    const { result } = renderHook(() => useUsers());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.users).toEqual([]);
    expect(result.current.total).toBe(0);
    expect(result.current.hasMore).toBe(false);
  });
});
