"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, formatRelativeTime } from '@/lib/api';

// Agent stats from API
export interface AgentStats {
  reputation: number;
  problemsSolved: number;
  problemsContributed: number;
  ideasPosted: number;
  responsesGiven: number;
}

// Agent data for frontend use
export interface AgentData {
  id: string;
  displayName: string;
  bio: string;
  status: string;
  reputation: number;
  createdAt: string;
  hasHumanBackedBadge: boolean;
  avatarUrl?: string;
  email?: string;
  externalLinks?: string[];
  model?: string;
  time: string;
  stats: AgentStats;
}

export interface UseAgentResult {
  agent: AgentData | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

// Transform API agent to frontend format
// Minimal transformation - just pass through API data
function transformAgent(
  agent: {
    id: string;
    display_name: string;
    bio: string;
    status: string;
    reputation: number;
    created_at: string;
    has_human_backed_badge: boolean;
    avatar_url?: string | null;
    email?: string | null;
    external_links?: string[] | null;
    model?: string | null;
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

  return {
    id: agent.id,
    displayName: agent.display_name || 'Unknown Agent',
    bio: agent.bio || '',
    status: agent.status || 'active',
    reputation: agent.reputation ?? 0,
    createdAt,
    hasHumanBackedBadge: agent.has_human_backed_badge ?? false,
    avatarUrl: agent.avatar_url || undefined,
    email: agent.email || undefined,
    externalLinks: agent.external_links || undefined,
    model: agent.model || undefined,
    time: formatRelativeTime(createdAt),
    stats: {
      reputation: stats?.reputation ?? 0,
      problemsSolved: stats?.problems_solved ?? 0,
      problemsContributed: stats?.problems_contributed ?? 0,
      ideasPosted: stats?.ideas_posted ?? 0,
      responsesGiven: stats?.responses_given ?? 0,
    },
  };
}

/**
 * Hook to fetch a single agent profile from the API.
 * @param id - The agent ID to fetch
 * @param initialAgentData - Optional server-side fetched agent data (for SSR/SEO)
 * @returns Agent data, loading state, error, and refetch function
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function useAgent(id: string, initialAgentData?: any): UseAgentResult {
  const [agent, setAgent] = useState<AgentData | null>(
    initialAgentData ? transformAgent(initialAgentData.agent, initialAgentData.stats) : null
  );
  const [loading, setLoading] = useState(!initialAgentData);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
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
      if (!initialAgentData) {
        setAgent(null);
      }
    } finally {
      setLoading(false);
    }
  }, [id, initialAgentData]);

  useEffect(() => {
    if (initialAgentData) {
      setLoading(false);
      return;
    }
    fetchData();
  }, [fetchData, initialAgentData]);

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
