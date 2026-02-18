"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useProblem } from './use-problem';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getPost: vi.fn(),
    getProblemApproaches: vi.fn(),
  },
  formatRelativeTime: (date: string) => 'just now',
  mapStatus: (status: string) => status.toUpperCase(),
}));

const mockProblem = {
  id: 'problem-123',
  type: 'problem' as const,
  title: 'Connection pool exhaustion under load',
  description: 'When running high traffic, the connection pool becomes exhausted...',
  status: 'open',
  upvotes: 25,
  downvotes: 3,
  vote_score: 22,
  author: {
    id: 'user-1',
    type: 'human' as const,
    display_name: 'Sarah Dev',
  },
  tags: ['postgresql', 'performance', 'connection-pool'],
  created_at: '2025-01-10T10:00:00Z',
  updated_at: '2025-01-15T12:00:00Z',
  approaches_count: 3,
};

const mockApproaches = [
  {
    id: 'approach-1',
    problem_id: 'problem-123',
    author_type: 'human' as const,
    author_id: 'user-2',
    angle: 'Connection pooling optimization',
    method: 'Implement PgBouncer as connection pooler',
    assumptions: ['Traffic spikes are temporary', 'Current pool size is too small'],
    status: 'working',
    outcome: null,
    solution: null,
    created_at: '2025-01-11T10:00:00Z',
    updated_at: '2025-01-12T10:00:00Z',
    author: {
      type: 'human' as const,
      id: 'user-2',
      display_name: 'John Expert',
    },
  },
  {
    id: 'approach-2',
    problem_id: 'problem-123',
    author_type: 'agent' as const,
    author_id: 'claude-1',
    angle: 'Query optimization',
    method: 'Reduce long-running queries that hold connections',
    assumptions: ['Some queries are holding connections too long'],
    status: 'succeeded',
    outcome: 'Identified 3 slow queries causing connection hold',
    solution: 'Added indexes and query optimization reduced connection time by 60%',
    created_at: '2025-01-12T10:00:00Z',
    updated_at: '2025-01-14T10:00:00Z',
    author: {
      type: 'agent' as const,
      id: 'claude-1',
      display_name: 'Claude Assistant',
    },
  },
];

describe('useProblem', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should fetch problem data and approaches', async () => {
    // Arrange
    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockProblem });
    (api.getProblemApproaches as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: mockApproaches,
      meta: { total: 2, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useProblem('problem-123'));

    // Assert - initial loading state
    expect(result.current.loading).toBe(true);
    expect(result.current.problem).toBeNull();
    expect(result.current.approaches).toEqual([]);

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - problem loaded
    expect(result.current.problem).not.toBeNull();
    expect(result.current.problem?.id).toBe('problem-123');
    expect(result.current.problem?.title).toBe('Connection pool exhaustion under load');
    expect(result.current.problem?.voteScore).toBe(22);
    expect(result.current.problem?.tags).toEqual(['postgresql', 'performance', 'connection-pool']);

    // Assert - approaches loaded
    expect(result.current.approaches).toHaveLength(2);
    expect(result.current.approaches[0].id).toBe('approach-1');
    expect(result.current.approaches[0].status).toBe('working');
    expect(result.current.approaches[1].id).toBe('approach-2');
    expect(result.current.approaches[1].status).toBe('succeeded');

    // Assert - no error
    expect(result.current.error).toBeNull();
  });

  it('should handle API errors gracefully', async () => {
    // Arrange
    (api.getPost as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Network error'));

    // Act
    const { result } = renderHook(() => useProblem('problem-123'));

    // Wait for error
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.error).toBe('Network error');
    expect(result.current.problem).toBeNull();
  });

  it('should handle problem not found', async () => {
    // Arrange
    (api.getPost as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('API error: 404'));

    // Act
    const { result } = renderHook(() => useProblem('nonexistent-id'));

    // Wait for error
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.error).toBe('API error: 404');
    expect(result.current.problem).toBeNull();
  });

  it('should refetch data when refetch is called', async () => {
    // Arrange
    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockProblem });
    (api.getProblemApproaches as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: mockApproaches,
      meta: { total: 2, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useProblem('problem-123'));

    // Wait for initial load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Clear mocks and change the data
    vi.clearAllMocks();
    const updatedProblem = { ...mockProblem, upvotes: 30, vote_score: 27 };
    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: updatedProblem });
    (api.getProblemApproaches as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: mockApproaches,
      meta: { total: 2, page: 1, per_page: 20, has_more: false },
    });

    // Refetch
    result.current.refetch();

    // Wait for refetch
    await waitFor(() => {
      expect(result.current.problem?.voteScore).toBe(27);
    });

    // Assert
    expect(api.getPost).toHaveBeenCalledTimes(1);
    expect(api.getProblemApproaches).toHaveBeenCalledTimes(1);
  });

  it('should not fetch when id is empty', async () => {
    // Act
    const { result } = renderHook(() => useProblem(''));

    // Assert - should not be loading
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // No API calls made
    expect(api.getPost).not.toHaveBeenCalled();
    expect(api.getProblemApproaches).not.toHaveBeenCalled();
  });

  // FE-021: Edge cases for defensive handling

  it('should handle problem with null tags and missing optional fields', async () => {
    // Arrange - API returns null for tags and missing optional fields
    const problemWithNulls = {
      id: 'problem-123',
      type: 'problem' as const,
      title: 'Test problem',
      description: 'Description',
      status: 'open',
      upvotes: 5,
      downvotes: 1,
      vote_score: 4,
      author: {
        id: 'user-1',
        type: 'human' as const,
        display_name: 'Test User',
      },
      tags: null, // null instead of array
      created_at: '2025-01-10T10:00:00Z',
      updated_at: '2025-01-10T10:00:00Z',
      // Missing: approaches_count, view_count
    };

    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: problemWithNulls });
    (api.getProblemApproaches as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useProblem('problem-123'));

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - should handle nulls gracefully
    expect(result.current.problem).not.toBeNull();
    expect(result.current.problem?.tags).toEqual([]);
    expect(result.current.problem?.approachesCount).toBe(0);
    expect(result.current.problem?.views).toBe(0);
    expect(result.current.error).toBeNull();
  });

  it('should handle approaches with null assumptions and method', async () => {
    // Arrange - Approach with null values
    const approachWithNulls = {
      id: 'approach-1',
      problem_id: 'problem-123',
      author_type: 'agent' as const,
      author_id: 'claude-1',
      angle: 'Test approach',
      method: null, // null instead of string
      assumptions: null, // null instead of array
      status: 'starting',
      outcome: null,
      solution: null,
      created_at: '2025-01-11T10:00:00Z',
      updated_at: '2025-01-11T10:00:00Z',
      author: {
        type: 'agent' as const,
        id: 'claude-1',
        display_name: 'Claude',
      },
    };

    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockProblem });
    (api.getProblemApproaches as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [approachWithNulls],
      meta: { total: 1, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useProblem('problem-123'));

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - should handle nulls gracefully
    expect(result.current.approaches).toHaveLength(1);
    expect(result.current.approaches[0].method).toBe('');
    expect(result.current.approaches[0].assumptions).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('should handle empty approaches response', async () => {
    // Arrange - Empty data array
    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockProblem });
    (api.getProblemApproaches as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useProblem('problem-123'));

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - should return empty array, not error
    expect(result.current.approaches).toEqual([]);
    expect(result.current.problem).not.toBeNull();
    expect(result.current.error).toBeNull();
  });

  it('should handle approaches API returning undefined data', async () => {
    // Arrange - Malformed API response without data property
    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockProblem });
    (api.getProblemApproaches as ReturnType<typeof vi.fn>).mockResolvedValue({
      // Missing data property
      meta: { total: 0, page: 1, per_page: 20, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useProblem('problem-123'));

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - should return empty array when data is undefined
    expect(result.current.approaches).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('should include crystallization fields when present', async () => {
    const crystallizedProblem = {
      ...mockProblem,
      status: 'solved',
      crystallization_cid: 'QmTestCid123',
      crystallized_at: '2026-02-15T10:30:00Z',
    };

    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: crystallizedProblem });
    (api.getProblemApproaches as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 20, has_more: false },
    });

    const { result } = renderHook(() => useProblem('problem-123'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.problem?.crystallizationCid).toBe('QmTestCid123');
    expect(result.current.problem?.crystallizedAt).toBe('2026-02-15T10:30:00Z');
  });

  it('should have undefined crystallization fields when not present', async () => {
    (api.getPost as ReturnType<typeof vi.fn>).mockResolvedValue({ data: mockProblem });
    (api.getProblemApproaches as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 20, has_more: false },
    });

    const { result } = renderHook(() => useProblem('problem-123'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.problem?.crystallizationCid).toBeUndefined();
    expect(result.current.problem?.crystallizedAt).toBeUndefined();
  });
});
