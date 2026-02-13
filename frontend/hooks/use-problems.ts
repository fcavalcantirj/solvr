"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIPost, FetchProblemsParams, formatRelativeTime, mapStatus } from '@/lib/api';

export interface ProblemListItem {
  id: string;
  title: string;
  snippet: string;
  status: string;
  displayStatus: string;
  voteScore: number;
  viewCount: number;
  approachesCount: number;
  author: {
    id: string;
    name: string;
    type: 'human' | 'agent';
  };
  tags: string[];
  timestamp: string;
}

function transformProblem(post: APIPost): ProblemListItem {
  return {
    id: post.id,
    title: post.title,
    snippet: post.description.slice(0, 200) + (post.description.length > 200 ? '...' : ''),
    status: post.status,
    displayStatus: mapStatus(post.status),
    voteScore: post.vote_score,
    viewCount: post.view_count,
    approachesCount: post.approaches_count || 0,
    author: {
      id: post.author.id,
      name: post.author.display_name,
      type: post.author.type,
    },
    tags: post.tags || [],
    timestamp: formatRelativeTime(post.created_at),
  };
}

export interface UseProblemsOptions {
  status?: string;
  tags?: string[];
  sort?: 'newest' | 'votes' | 'approaches';
  page?: number;
  perPage?: number;
}

export interface UseProblemsResult {
  problems: ProblemListItem[];
  loading: boolean;
  error: string | null;
  total: number;
  hasMore: boolean;
  page: number;
  refetch: () => void;
  loadMore: () => void;
}

export function useProblems(options: UseProblemsOptions = {}): UseProblemsResult {
  const [problems, setProblems] = useState<ProblemListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(options.page || 1);

  const optionsKey = JSON.stringify(options);

  const fetchProblems = useCallback(async (pageNum: number, append: boolean = false) => {
    try {
      setLoading(true);
      setError(null);

      const stableOptions: UseProblemsOptions = JSON.parse(optionsKey);
      const params: FetchProblemsParams = {
        status: stableOptions.status,
        tags: stableOptions.tags,
        sort: stableOptions.sort,
        page: pageNum,
        per_page: stableOptions.perPage || 20,
      };

      const response = await api.getProblems(params);
      const transformed = response.data.map(transformProblem);

      if (append) {
        setProblems(prev => [...prev, ...transformed]);
      } else {
        setProblems(transformed);
      }

      setTotal(response.meta.total);
      setHasMore(response.meta.has_more);
      setPage(pageNum);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch problems');
    } finally {
      setLoading(false);
    }
  }, [optionsKey]);

  useEffect(() => {
    fetchProblems(1);
  }, [fetchProblems]);

  const refetch = useCallback(() => {
    fetchProblems(1);
  }, [fetchProblems]);

  const loadMore = useCallback(() => {
    if (hasMore && !loading) {
      fetchProblems(page + 1, true);
    }
  }, [hasMore, loading, page, fetchProblems]);

  return {
    problems,
    loading,
    error,
    total,
    hasMore,
    page,
    refetch,
    loadMore,
  };
}
