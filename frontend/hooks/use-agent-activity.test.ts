"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useAgentActivity } from './use-agent-activity';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getAgentActivity: vi.fn(),
  },
  formatRelativeTime: (date: string) => '5d ago',
}));

const mockActivityItems = [
  {
    id: 'post_1',
    type: 'post',
    action: 'created',
    title: 'How to fix memory leaks?',
    post_type: 'question',
    status: 'open',
    created_at: '2025-02-01T10:00:00Z',
  },
  {
    id: 'answer_1',
    type: 'answer',
    action: 'answered',
    title: 'Memory leaks occur when...',
    created_at: '2025-02-02T10:00:00Z',
    target_id: 'post_2',
    target_title: 'Understanding memory management',
  },
  {
    id: 'approach_1',
    type: 'approach',
    action: 'started_approach',
    title: 'Using profiler to track allocations',
    created_at: '2025-02-03T10:00:00Z',
    target_id: 'post_3',
    target_title: 'Fix the database connection issue',
  },
];

describe('useAgentActivity', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns loading true initially', () => {
    // Arrange - make the API hang
    (api.getAgentActivity as ReturnType<typeof vi.fn>).mockImplementation(
      () => new Promise(() => {}) // Never resolves
    );

    // Act
    const { result } = renderHook(() => useAgentActivity('agent_test'));

    // Assert
    expect(result.current.loading).toBe(true);
    expect(result.current.items).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('fetches activity items and transforms to frontend format', async () => {
    // Arrange
    (api.getAgentActivity as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: mockActivityItems,
      meta: { total: 3, page: 1, per_page: 10, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useAgentActivity('agent_test'));

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.items).toHaveLength(3);
    expect(result.current.items[0].id).toBe('post_1');
    expect(result.current.items[0].type).toBe('post');
    expect(result.current.items[0].title).toBe('How to fix memory leaks?');
    expect(result.current.items[0].time).toBe('5d ago');
    expect(result.current.hasMore).toBe(false);
    expect(result.current.total).toBe(3);
    expect(result.current.error).toBeNull();
  });

  it('handles API error gracefully', async () => {
    // Arrange
    (api.getAgentActivity as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Network error'));

    // Act
    const { result } = renderHook(() => useAgentActivity('agent_test'));

    // Wait for error
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.error).toBe('Network error');
    expect(result.current.items).toEqual([]);
  });

  it('does not fetch when agentId is empty', async () => {
    // Act
    const { result } = renderHook(() => useAgentActivity(''));

    // Wait for state to settle
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert - no API call made
    expect(api.getAgentActivity).not.toHaveBeenCalled();
    expect(result.current.items).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it('handles empty activity list', async () => {
    // Arrange
    (api.getAgentActivity as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [],
      meta: { total: 0, page: 1, per_page: 10, has_more: false },
    });

    // Act
    const { result } = renderHook(() => useAgentActivity('agent_test'));

    // Wait for data to load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Assert
    expect(result.current.items).toEqual([]);
    expect(result.current.total).toBe(0);
    expect(result.current.hasMore).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('supports pagination with loadMore', async () => {
    // Arrange - page 1
    (api.getAgentActivity as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [mockActivityItems[0]],
      meta: { total: 3, page: 1, per_page: 1, has_more: true },
    });

    // Act
    const { result } = renderHook(() => useAgentActivity('agent_test'));

    // Wait for initial load
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.items).toHaveLength(1);
    expect(result.current.hasMore).toBe(true);

    // Arrange - page 2
    vi.clearAllMocks();
    (api.getAgentActivity as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: [mockActivityItems[1]],
      meta: { total: 3, page: 2, per_page: 1, has_more: true },
    });

    // Load more
    result.current.loadMore();

    // Wait for second page
    await waitFor(() => {
      expect(result.current.items).toHaveLength(2);
    });

    // Assert
    expect(result.current.items[0].id).toBe('post_1');
    expect(result.current.items[1].id).toBe('answer_1');
    expect(api.getAgentActivity).toHaveBeenCalledWith('agent_test', 2, 10);
  });
});
