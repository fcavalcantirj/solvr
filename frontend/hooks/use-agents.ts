"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIAgent, FetchAgentsParams, formatRelativeTime } from '@/lib/api';

export interface AgentListItem {
  id: string;
  displayName: string;
  bio: string;
  status: 'active' | 'pending';
  karma: number;
  postCount: number;
  hasHumanBackedBadge: boolean;
  avatarUrl?: string;
  initials: string;
  createdAt: string;
}

// Transform API agent to AgentListItem format
function transformAgent(agent: APIAgent): AgentListItem {
  return {
    id: agent.id,
    displayName: agent.display_name,
    bio: agent.bio || '',
    status: agent.status === 'active' ? 'active' : 'pending',
    karma: agent.karma,
    postCount: agent.post_count,
    hasHumanBackedBadge: agent.has_human_backed_badge,
    avatarUrl: agent.avatar_url,
    initials: agent.display_name.slice(0, 2).toUpperCase(),
    createdAt: formatRelativeTime(agent.created_at),
  };
}

export interface UseAgentsOptions {
  page?: number;
  perPage?: number;
  sort?: 'newest' | 'oldest' | 'karma' | 'posts';
  status?: 'active' | 'pending' | 'all';
}

export interface UseAgentsResult {
  agents: AgentListItem[];
  loading: boolean;
  error: string | null;
  total: number;
  hasMore: boolean;
  page: number;
  refetch: () => void;
  loadMore: () => void;
}

export function useAgents(options: UseAgentsOptions = {}): UseAgentsResult {
  const [agents, setAgents] = useState<AgentListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(options.page || 1);

  // Stabilize options to prevent infinite re-renders
  const optionsKey = JSON.stringify(options);

  const fetchAgents = useCallback(async (pageNum: number, append: boolean = false) => {
    try {
      setLoading(true);
      setError(null);

      const stableOptions: UseAgentsOptions = JSON.parse(optionsKey);
      const params: FetchAgentsParams = {
        page: pageNum,
        per_page: stableOptions.perPage || 20,
        sort: stableOptions.sort,
        status: stableOptions.status,
      };

      const response = await api.getAgents(params);
      const transformedAgents = response.data.map(transformAgent);

      if (append) {
        setAgents(prev => [...prev, ...transformedAgents]);
      } else {
        setAgents(transformedAgents);
      }

      setTotal(response.meta.total);
      setHasMore(response.meta.has_more);
      setPage(pageNum);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch agents');
    } finally {
      setLoading(false);
    }
  }, [optionsKey]);

  useEffect(() => {
    fetchAgents(1);
  }, [fetchAgents]);

  const refetch = useCallback(() => {
    fetchAgents(1);
  }, [fetchAgents]);

  const loadMore = useCallback(() => {
    if (hasMore && !loading) {
      fetchAgents(page + 1, true);
    }
  }, [hasMore, loading, page, fetchAgents]);

  return {
    agents,
    loading,
    error,
    total,
    hasMore,
    page,
    refetch,
    loadMore,
  };
}
