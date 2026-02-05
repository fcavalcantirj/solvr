"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIPost, formatRelativeTime, mapStatus } from '@/lib/api';

// Idea data for frontend use
export interface IdeaData {
  id: string;
  title: string;
  description: string;
  status: string;
  voteScore: number;
  upvotes: number;
  downvotes: number;
  author: {
    id: string;
    type: 'human' | 'ai';
    displayName: string;
  };
  tags: string[];
  createdAt: string;
  updatedAt: string;
  time: string;
  views: number;
}

export interface UseIdeaResult {
  idea: IdeaData | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

// Transform API idea to frontend format
function transformIdea(post: APIPost): IdeaData {
  return {
    id: post.id,
    title: post.title,
    description: post.description,
    status: mapStatus(post.status),
    voteScore: post.vote_score,
    upvotes: post.upvotes,
    downvotes: post.downvotes,
    author: {
      id: post.author.id,
      type: post.author.type === 'agent' ? 'ai' : 'human',
      displayName: post.author.display_name,
    },
    tags: post.tags || [],
    createdAt: post.created_at,
    updatedAt: post.updated_at,
    time: formatRelativeTime(post.created_at),
    views: post.view_count || 0,
  };
}

/**
 * Hook to fetch an idea from the API.
 * @param id - The idea ID to fetch
 * @returns Idea data, loading state, error, and refetch function
 */
export function useIdea(id: string): UseIdeaResult {
  const [idea, setIdea] = useState<IdeaData | null>(null);
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

      // Fetch idea
      const response = await api.getPost(id);

      // Transform and set idea data
      setIdea(transformIdea(response.data));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch idea');
      setIdea(null);
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
    idea,
    loading,
    error,
    refetch,
  };
}
