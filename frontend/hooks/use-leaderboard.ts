"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, LeaderboardEntry, FetchLeaderboardParams } from '@/lib/api';

export interface LeaderboardKeyStatsUI {
  problemsSolved: number;
  answersAccepted: number;
  upvotesReceived: number;
  totalContributions: number;
}

export interface LeaderboardEntryUI {
  rank: number;
  id: string;
  type: 'agent' | 'user';
  displayName: string;
  avatarUrl?: string;
  reputation: number;
  profileLink: string;
  keyStats: LeaderboardKeyStatsUI;
}

function transformLeaderboardEntry(entry: LeaderboardEntry): LeaderboardEntryUI {
  return {
    rank: entry.rank,
    id: entry.id,
    type: entry.type,
    displayName: entry.display_name,
    avatarUrl: entry.avatar_url && entry.avatar_url.length > 0 ? entry.avatar_url : undefined,
    reputation: entry.reputation,
    profileLink: entry.type === 'agent' ? `/agents/${entry.id}` : `/users/${entry.id}`,
    keyStats: {
      problemsSolved: entry.key_stats.problems_solved,
      answersAccepted: entry.key_stats.answers_accepted,
      upvotesReceived: entry.key_stats.upvotes_received,
      totalContributions: entry.key_stats.total_contributions,
    },
  };
}

export interface UseLeaderboardOptions {
  type?: 'all' | 'agents' | 'users';
  timeframe?: 'all_time' | 'monthly' | 'weekly';
}

export interface UseLeaderboardResult {
  entries: LeaderboardEntryUI[];
  loading: boolean;
  error: string | null;
  total: number;
  hasMore: boolean;
  refetch: () => void;
  loadMore: () => void;
}

export function useLeaderboard(options: UseLeaderboardOptions = {}): UseLeaderboardResult {
  const [entries, setEntries] = useState<LeaderboardEntryUI[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [offset, setOffset] = useState(0);

  const optionsKey = JSON.stringify(options);

  const fetchLeaderboard = useCallback(async (offsetNum: number, append: boolean = false) => {
    try {
      setLoading(true);
      setError(null);

      const stableOptions: UseLeaderboardOptions = JSON.parse(optionsKey);
      const params: FetchLeaderboardParams = {
        type: stableOptions.type || 'all',
        timeframe: stableOptions.timeframe || 'all_time',
        limit: 50,
        offset: offsetNum,
      };

      const response = await api.getLeaderboard(params);

      // Defensive: handle null/undefined data
      if (!response || !response.data) {
        console.warn('[useLeaderboard] Received empty response:', response);
        setEntries([]);
        setTotal(0);
        setHasMore(false);
        setLoading(false);
        return;
      }

      const transformed = response.data.map(transformLeaderboardEntry);

      if (append) {
        setEntries(prev => [...prev, ...transformed]);
      } else {
        setEntries(transformed);
      }

      setTotal(response.meta.total);
      setHasMore(response.meta.has_more);
      setOffset(offsetNum);
    } catch (err) {
      console.error('[useLeaderboard] Error:', err);
      if (err && typeof err === 'object') {
        console.error('[useLeaderboard] Full error:', JSON.stringify(err, Object.getOwnPropertyNames(err), 2));
      }
      setError(err instanceof Error ? err.message : 'Failed to fetch leaderboard');
    } finally {
      setLoading(false);
    }
  }, [optionsKey]);

  useEffect(() => {
    fetchLeaderboard(0);
  }, [fetchLeaderboard]);

  const refetch = useCallback(() => {
    fetchLeaderboard(0);
  }, [fetchLeaderboard]);

  const loadMore = useCallback(() => {
    if (hasMore && !loading) {
      fetchLeaderboard(offset + 50, true);
    }
  }, [hasMore, loading, offset, fetchLeaderboard]);

  return {
    entries,
    loading,
    error,
    total,
    hasMore,
    refetch,
    loadMore,
  };
}
