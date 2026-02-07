"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, formatRelativeTime } from '@/lib/api';

// Agent data for frontend use
export interface AgentData {
  id: string;
  displayName: string;
  bio: string;
  status: string;
  karma: number;
  postCount: number;
  createdAt: string;
  hasHumanBackedBadge: boolean;
  avatarUrl?: string;
  time: string;
}

export interface UseAgentResult {
  agent: AgentData | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

// Transform API agent to frontend format
function transformAgent(data: {
  id: string;
  display_name: string;
  bio: string;
  status: string;
  karma: number;
  post_count: number;
  created_at: string;
  has_human_backed_badge: boolean;
  avatar_url?: string | null;
}): AgentData {
  return {
    id: data.id,
    displayName: data.display_name,
    bio: data.bio || '',
    status: data.status || 'active',
    karma: data.karma,
    postCount: data.post_count,
    createdAt: data.created_at,
    hasHumanBackedBadge: data.has_human_backed_badge,
    avatarUrl: data.avatar_url || undefined,
    time: formatRelativeTime(data.created_at),
  };
}

/**
 * Hook to fetch a single agent profile from the API.
 * @param id - The agent ID to fetch
 * @returns Agent data, loading state, error, and refetch function
 */
export function useAgent(id: string): UseAgentResult {
  const [agent, setAgent] = useState<AgentData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    // Don't fetch if no ID provided
    if (!id) {
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      setError(null);

      const response = await api.getAgent(id);
      setAgent(transformAgent(response.data));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch agent');
      setAgent(null);
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const refetch = useCallback(() => {
    fetchData();
  }, [fetchData]);

  return {
    agent,
    loading,
    error,
    refetch,
  };
}
