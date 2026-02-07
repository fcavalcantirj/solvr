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
function transformAgent(
  agent: {
    id: string;
    display_name: string;
    bio: string;
    status: string;
    karma: number;
    created_at: string;
    has_human_backed_badge: boolean;
    avatar_url?: string | null;
  },
  stats?: {
    problems_solved?: number;
    problems_contributed?: number;
    questions_asked?: number;
    questions_answered?: number;
    answers_accepted?: number;
    ideas_posted?: number;
    responses_given?: number;
    upvotes_received?: number;
    reputation?: number;
  }
): AgentData {
  const createdAt = agent.created_at || new Date().toISOString();

  // Karma comes directly from agent object (backend has it there)
  const karma = agent.karma ?? 0;

  // Compute postCount from stats (problems + questions + ideas posted)
  const postCount = stats ? (
    (stats.problems_solved ?? 0) +
    (stats.questions_asked ?? 0) +
    (stats.ideas_posted ?? 0)
  ) : 0;

  return {
    id: agent.id,
    displayName: agent.display_name || 'Unknown Agent',
    bio: agent.bio || '',
    status: agent.status || 'active',
    karma,
    postCount,
    createdAt,
    hasHumanBackedBadge: agent.has_human_backed_badge ?? false,
    avatarUrl: agent.avatar_url || undefined,
    time: formatRelativeTime(createdAt),
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
      setAgent(transformAgent(response.data.agent, response.data.stats));
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
