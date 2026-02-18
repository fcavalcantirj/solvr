"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIPost, FetchIdeasParams, formatRelativeTime, mapStatus } from '@/lib/api';

export type IdeaStage = 'spark' | 'developing' | 'mature' | 'realized' | 'archived';

export interface IdeaListItem {
  id: string;
  title: string;
  spark: string; // description snippet
  stage: IdeaStage;
  potential: 'high' | 'rising' | 'needs_validation';
  author: {
    name: string;
    type: 'human' | 'ai';
    avatar?: string;
  };
  support: number;
  comments: number;
  branches: number;
  viewCount: number;
  tags: string[];
  timestamp: string;
  supporters?: Array<{ name: string; type: 'human' | 'ai' }>;
  recentComment: {
    author: string;
    type: 'human' | 'ai';
    content: string;
  } | null;
}

// Map API status to IdeaStage
function mapStatusToStage(status: string): IdeaStage {
  const stageMap: Record<string, IdeaStage> = {
    'open': 'spark',
    'draft': 'spark',
    'active': 'developing',
    'dormant': 'mature',
    'evolved': 'realized',
  };
  return stageMap[status.toLowerCase()] || 'spark';
}

// Transform API post to IdeaListItem format
function transformIdea(post: APIPost): IdeaListItem {
  return {
    id: post.id,
    title: post.title,
    spark: post.description
      ? post.description.slice(0, 200) + (post.description.length > 200 ? '...' : '')
      : '',
    stage: mapStatusToStage(post.status),
    potential: post.vote_score > 50 ? 'high' : post.vote_score > 10 ? 'rising' : 'needs_validation',
    author: {
      name: post.author.display_name,
      type: post.author.type === 'agent' ? 'ai' : 'human',
      avatar: post.author.avatar_url || post.author.display_name.slice(0, 2).toUpperCase(),
    },
    support: post.upvotes,
    comments: post.comments_count || 0,
    branches: post.evolved_into?.length ?? 0,
    viewCount: post.view_count || 0,
    tags: post.tags || [],
    timestamp: formatRelativeTime(post.created_at),
    supporters: (post as any).supporters ?? [],
    recentComment: null,
  };
}

export interface UseIdeasOptions {
  status?: string;
  tags?: string[];
  page?: number;
  perPage?: number;
  sort?: 'newest' | 'trending' | 'most_support';
}

export interface UseIdeasResult {
  ideas: IdeaListItem[];
  loading: boolean;
  error: string | null;
  total: number;
  hasMore: boolean;
  page: number;
  refetch: () => void;
  loadMore: () => void;
}

export function useIdeas(options: UseIdeasOptions = {}): UseIdeasResult {
  const [ideas, setIdeas] = useState<IdeaListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(options.page || 1);

  // Stabilize options to prevent infinite re-renders
  const optionsKey = JSON.stringify(options);

  const fetchIdeas = useCallback(async (pageNum: number, append: boolean = false) => {
    try {
      setLoading(true);
      setError(null);

      const stableOptions: UseIdeasOptions = JSON.parse(optionsKey);
      const params: FetchIdeasParams = {
        status: stableOptions.status as FetchIdeasParams['status'],
        tags: stableOptions.tags,
        page: pageNum,
        per_page: stableOptions.perPage || 20,
        sort: stableOptions.sort,
      };

      const response = await api.getIdeas(params);

      // Defensive: handle null/undefined data
      if (!response || !response.data) {
        console.warn('[useIdeas] Received empty response:', response);
        setIdeas([]);
        setTotal(0);
        setHasMore(false);
        setLoading(false);
        return;
      }

      const transformedIdeas = response.data.map(transformIdea);

      if (append) {
        setIdeas(prev => [...prev, ...transformedIdeas]);
      } else {
        setIdeas(transformedIdeas);
      }

      setTotal(response.meta?.total || 0);
      setHasMore(response.meta?.has_more || false);
      setPage(pageNum);
    } catch (err) {
      console.error('[useIdeas] Error fetching ideas:', err);
      if (err && typeof err === 'object') {
        console.error('[useIdeas] Full error object:', JSON.stringify(err, Object.getOwnPropertyNames(err), 2));
      }
      setError(err instanceof Error ? err.message : 'Failed to fetch ideas');
    } finally {
      setLoading(false);
    }
  }, [optionsKey]);

  useEffect(() => {
    fetchIdeas(1);
  }, [fetchIdeas]);

  const refetch = useCallback(() => {
    fetchIdeas(1);
  }, [fetchIdeas]);

  const loadMore = useCallback(() => {
    if (hasMore && !loading) {
      fetchIdeas(page + 1, true);
    }
  }, [hasMore, loading, page, fetchIdeas]);

  return {
    ideas,
    loading,
    error,
    total,
    hasMore,
    page,
    refetch,
    loadMore,
  };
}
