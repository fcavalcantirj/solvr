"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useLeaderboard } from './use-leaderboard';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getLeaderboard: vi.fn(),
  },
}));

const mockLeaderboardResponse = {
  data: [
    {
      rank: 1,
      id: 'agent-123',
      type: 'agent',
      display_name: 'SolverBot',
      avatar_url: 'https://example.com/avatar1.jpg',
      reputation: 1250,
      key_stats: {
        problems_solved: 15,
        answers_accepted: 28,
        upvotes_received: 150,
        total_contributions: 193,
      },
    },
    {
      rank: 2,
      id: 'user-456',
      type: 'user',
      display_name: 'Alice Dev',
      avatar_url: '',
      reputation: 980,
      key_stats: {
        problems_solved: 8,
        answers_accepted: 42,
        upvotes_received: 120,
        total_contributions: 170,
      },
    },
  ],
  meta: {
    total: 125,
    page: 1,
    per_page: 50,
    has_more: true,
  },
};

describe('useLeaderboard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('fetches from API on mount with correct params', async () => {
    (api.getLeaderboard as ReturnType<typeof vi.fn>).mockResolvedValue(mockLeaderboardResponse);

    const { result } = renderHook(() => useLeaderboard());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getLeaderboard).toHaveBeenCalledWith({
      type: 'all',
      timeframe: 'all_time',
      limit: 50,
      offset: 0,
    });

    expect(result.current.entries).toHaveLength(2);
    expect(result.current.total).toBe(125);
    expect(result.current.hasMore).toBe(true);
  });

  it('transforms LeaderboardEntry data correctly (adds profileLink based on type)', async () => {
    (api.getLeaderboard as ReturnType<typeof vi.fn>).mockResolvedValue(mockLeaderboardResponse);

    const { result } = renderHook(() => useLeaderboard());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Check agent entry
    const agentEntry = result.current.entries[0];
    expect(agentEntry.rank).toBe(1);
    expect(agentEntry.id).toBe('agent-123');
    expect(agentEntry.type).toBe('agent');
    expect(agentEntry.displayName).toBe('SolverBot');
    expect(agentEntry.avatarUrl).toBe('https://example.com/avatar1.jpg');
    expect(agentEntry.reputation).toBe(1250);
    expect(agentEntry.profileLink).toBe('/agents/agent-123');
    expect(agentEntry.keyStats.problemsSolved).toBe(15);
    expect(agentEntry.keyStats.answersAccepted).toBe(28);
    expect(agentEntry.keyStats.upvotesReceived).toBe(150);
    expect(agentEntry.keyStats.totalContributions).toBe(193);

    // Check user entry
    const userEntry = result.current.entries[1];
    expect(userEntry.rank).toBe(2);
    expect(userEntry.id).toBe('user-456');
    expect(userEntry.type).toBe('user');
    expect(userEntry.displayName).toBe('Alice Dev');
    expect(userEntry.avatarUrl).toBeUndefined(); // Empty string maps to undefined
    expect(userEntry.reputation).toBe(980);
    expect(userEntry.profileLink).toBe('/users/user-456');
    expect(userEntry.keyStats.problemsSolved).toBe(8);
  });

  it('type filter change resets to offset 0 and refetches', async () => {
    (api.getLeaderboard as ReturnType<typeof vi.fn>).mockResolvedValue(mockLeaderboardResponse);

    const { result, rerender } = renderHook(
      ({ type }: { type?: 'all' | 'agents' | 'users' }) => useLeaderboard({ type }),
      { initialProps: { type: 'all' as 'all' | 'agents' | 'users' } }
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getLeaderboard).toHaveBeenCalledWith({
      type: 'all',
      timeframe: 'all_time',
      limit: 50,
      offset: 0,
    });

    vi.clearAllMocks();
    (api.getLeaderboard as ReturnType<typeof vi.fn>).mockResolvedValue(mockLeaderboardResponse);

    // Change type filter
    rerender({ type: 'agents' });

    await waitFor(() => {
      expect(api.getLeaderboard).toHaveBeenCalledWith({
        type: 'agents',
        timeframe: 'all_time',
        limit: 50,
        offset: 0,
      });
    });

    expect(result.current.entries).toHaveLength(2);
  });

  it('timeframe filter change resets to offset 0 and refetches', async () => {
    (api.getLeaderboard as ReturnType<typeof vi.fn>).mockResolvedValue(mockLeaderboardResponse);

    const { result, rerender } = renderHook(
      ({ timeframe }: { timeframe?: 'all_time' | 'monthly' | 'weekly' }) => useLeaderboard({ timeframe }),
      { initialProps: { timeframe: 'all_time' as 'all_time' | 'monthly' | 'weekly' } }
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getLeaderboard).toHaveBeenCalledWith({
      type: 'all',
      timeframe: 'all_time',
      limit: 50,
      offset: 0,
    });

    vi.clearAllMocks();
    (api.getLeaderboard as ReturnType<typeof vi.fn>).mockResolvedValue(mockLeaderboardResponse);

    // Change timeframe filter
    rerender({ timeframe: 'monthly' });

    await waitFor(() => {
      expect(api.getLeaderboard).toHaveBeenCalledWith({
        type: 'all',
        timeframe: 'monthly',
        limit: 50,
        offset: 0,
      });
    });

    expect(result.current.entries).toHaveLength(2);
  });

  it('loadMore() increments offset and appends results', async () => {
    (api.getLeaderboard as ReturnType<typeof vi.fn>).mockResolvedValue(mockLeaderboardResponse);

    const { result } = renderHook(() => useLeaderboard());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.entries).toHaveLength(2);

    // Mock page 2 response
    const page2Response = {
      data: [
        {
          rank: 3,
          id: 'agent-789',
          type: 'agent',
          display_name: 'CodeHelper',
          avatar_url: 'https://example.com/avatar2.jpg',
          reputation: 850,
          key_stats: {
            problems_solved: 12,
            answers_accepted: 20,
            upvotes_received: 90,
            total_contributions: 122,
          },
        },
      ],
      meta: { total: 125, page: 2, per_page: 50, has_more: true },
    };
    (api.getLeaderboard as ReturnType<typeof vi.fn>).mockResolvedValue(page2Response);

    result.current.loadMore();

    await waitFor(() => {
      expect(result.current.entries).toHaveLength(3);
    });

    expect(api.getLeaderboard).toHaveBeenCalledWith({
      type: 'all',
      timeframe: 'all_time',
      limit: 50,
      offset: 50,
    });

    expect(result.current.entries[2].id).toBe('agent-789');
    expect(result.current.entries[2].rank).toBe(3);
    expect(result.current.hasMore).toBe(true);
  });

  it('error handling displays error state', async () => {
    (api.getLeaderboard as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useLeaderboard());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe('Network error');
    expect(result.current.entries).toEqual([]);
  });
});
