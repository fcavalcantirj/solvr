"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useIdea } from './use-idea';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getPost: vi.fn(),
  },
  formatRelativeTime: (date: string) => 'just now',
  mapStatus: (status: string) => status.toUpperCase(),
}));

const mockIdea = {
  id: 'idea-123',
  type: 'idea' as const,
  title: 'Semantic diff for AI-generated code suggestions',
  description: 'What if instead of showing line-by-line diffs, we showed semantic changes?',
  status: 'developing',
  upvotes: 234,
  downvotes: 12,
  vote_score: 222,
  author: {
    id: 'user-1',
    type: 'human' as const,
    display_name: 'Alex Kumar',
  },
  tags: ['semantic-analysis', 'ux', 'ai-agents', 'developer-tools'],
  created_at: '2025-01-10T10:00:00Z',
  updated_at: '2025-01-15T12:00:00Z',
};

describe('useIdea', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should fetch idea data', async () => {
    // Arrange
    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockIdea });

    // Act
    const { result } = renderHook(() => useIdea('idea-123'));

    // Assert - initial loading state
    expect(result.current.loading).toBe(true);
    expect(result.current.idea).toBeNull();

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - idea loaded
    expect(result.current.idea).not.toBeNull();
    expect(result.current.idea?.id).toBe('idea-123');
    expect(result.current.idea?.title).toBe('Semantic diff for AI-generated code suggestions');
    expect(result.current.idea?.voteScore).toBe(222);
    expect(result.current.idea?.tags).toEqual(['semantic-analysis', 'ux', 'ai-agents', 'developer-tools']);

    // Assert - no error
    expect(result.current.error).toBeNull();
  });

  it('should handle API errors gracefully', async () => {
    // Arrange
    (api.getPost as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Network error'));

    // Act
    const { result } = renderHook(() => useIdea('idea-123'));

    // Wait for error
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.error).toBe('Network error');
    expect(result.current.idea).toBeNull();
  });

  it('should handle idea not found', async () => {
    // Arrange
    (api.getPost as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('API error: 404'));

    // Act
    const { result } = renderHook(() => useIdea('nonexistent-id'));

    // Wait for error
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.error).toBe('API error: 404');
    expect(result.current.idea).toBeNull();
  });

  it('should refetch data when refetch is called', async () => {
    // Arrange
    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockIdea });

    // Act
    const { result } = renderHook(() => useIdea('idea-123'));

    // Wait for initial load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Clear mocks and change the data
    vi.clearAllMocks();
    const updatedIdea = { ...mockIdea, upvotes: 250, vote_score: 238 };
    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: updatedIdea });

    // Refetch
    result.current.refetch();

    // Wait for refetch
    await waitFor(() => {
      expect(result.current.idea?.voteScore).toBe(238);
    });

    // Assert
    expect(api.getPost).toHaveBeenCalledTimes(1);
  });

  it('should not fetch when id is empty', async () => {
    // Act
    const { result } = renderHook(() => useIdea(''));

    // Assert - should not be loading
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // No API calls made
    expect(api.getPost).not.toHaveBeenCalled();
  });
});
