"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIPost, formatRelativeTime, mapStatus } from '@/lib/api';

// Progress note type for frontend use
export interface ProgressNote {
  id: string;
  approachId: string;
  content: string;
  createdAt: string;
  time: string;
}

// Approach type for frontend use
export interface ProblemApproach {
  id: string;
  angle: string;
  method: string;
  assumptions: string[];
  status: string;
  outcome: string | null;
  solution: string | null;
  progressNotes: ProgressNote[];
  author: {
    id: string;
    type: 'human' | 'ai';
    displayName: string;
  };
  createdAt: string;
  updatedAt: string;
  time: string;
}

// Problem data for frontend use
export interface ProblemData {
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
  approachesCount: number;
  views: number;
}

export interface UseProblemResult {
  problem: ProblemData | null;
  approaches: ProblemApproach[];
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

// Transform API problem to frontend format
function transformProblem(post: APIPost): ProblemData {
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
    approachesCount: post.approaches_count || 0,
    views: post.view_count || 0,
  };
}

// Transform API progress note to frontend format
function transformProgressNote(note: APIProgressNote): ProgressNote {
  return {
    id: note.id,
    approachId: note.approach_id,
    content: note.content,
    createdAt: note.created_at,
    time: formatRelativeTime(note.created_at),
  };
}

// Transform API approach to frontend format
function transformApproach(approach: APIApproachWithAuthor): ProblemApproach {
  return {
    id: approach.id,
    angle: approach.angle,
    method: approach.method || '',
    assumptions: approach.assumptions || [],
    status: approach.status,
    outcome: approach.outcome,
    solution: approach.solution,
    progressNotes: (approach.progress_notes || []).map(transformProgressNote),
    author: {
      id: approach.author.id,
      type: approach.author.type === 'agent' ? 'ai' : 'human',
      displayName: approach.author.display_name,
    },
    createdAt: approach.created_at,
    updatedAt: approach.updated_at,
    time: formatRelativeTime(approach.created_at),
  };
}

// API response types for approaches
interface APIApproachAuthor {
  type: 'agent' | 'human';
  id: string;
  display_name: string;
}

interface APIProgressNote {
  id: string;
  approach_id: string;
  content: string;
  created_at: string;
}

interface APIApproachWithAuthor {
  id: string;
  problem_id: string;
  author_type: 'agent' | 'human';
  author_id: string;
  angle: string;
  method: string;
  assumptions: string[];
  status: string;
  outcome: string | null;
  solution: string | null;
  progress_notes?: APIProgressNote[];
  created_at: string;
  updated_at: string;
  author: APIApproachAuthor;
}

interface APIApproachesResponse {
  data: APIApproachWithAuthor[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

/**
 * Hook to fetch a problem and its approaches from the API.
 * @param id - The problem ID to fetch
 * @returns Problem data, approaches, loading state, error, and refetch function
 */
export function useProblem(id: string): UseProblemResult {
  const [problem, setProblem] = useState<ProblemData | null>(null);
  const [approaches, setApproaches] = useState<ProblemApproach[]>([]);
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

      // Fetch problem and approaches in parallel
      const [problemResponse, approachesResponse] = await Promise.all([
        api.getPost(id),
        api.getProblemApproaches(id),
      ]);

      // Transform and set problem data
      setProblem(transformProblem(problemResponse.data));

      // FE-021: Defensive check - handle undefined/null data array
      const approachesData = approachesResponse?.data ?? [];
      const transformedApproaches = approachesData.map(transformApproach);
      setApproaches(transformedApproaches);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch problem');
      setProblem(null);
      setApproaches([]);
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
    problem,
    approaches,
    loading,
    error,
    refetch,
  };
}
