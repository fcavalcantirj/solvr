"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIIdeaResponseWithAuthor, IdeaResponseType, formatRelativeTime } from '@/lib/api';

export interface IdeaResponseData {
  id: string;
  ideaId: string;
  content: string;
  responseType: IdeaResponseType;
  author: {
    id: string;
    type: 'human' | 'ai';
    displayName: string;
    avatarUrl?: string;
  };
  upvotes: number;
  downvotes: number;
  voteScore: number;
  createdAt: string;
  time: string;
}

// Transform API response to frontend format
function transformResponse(response: APIIdeaResponseWithAuthor): IdeaResponseData {
  return {
    id: response.id,
    ideaId: response.idea_id,
    content: response.content,
    responseType: response.response_type,
    author: {
      id: response.author.id,
      type: response.author.type === 'agent' ? 'ai' : 'human',
      displayName: response.author.display_name,
      avatarUrl: response.author.avatar_url,
    },
    upvotes: response.upvotes,
    downvotes: response.downvotes,
    voteScore: response.vote_score,
    createdAt: response.created_at,
    time: formatRelativeTime(response.created_at),
  };
}

export interface UseIdeaResponsesOptions {
  page?: number;
  perPage?: number;
}

export interface UseIdeaResponsesResult {
  responses: IdeaResponseData[];
  loading: boolean;
  error: string | null;
  total: number;
  hasMore: boolean;
  page: number;
  refetch: () => void;
  loadMore: () => void;
}

export function useIdeaResponses(ideaId: string, options: UseIdeaResponsesOptions = {}): UseIdeaResponsesResult {
  const [responses, setResponses] = useState<IdeaResponseData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(options.page || 1);

  const fetchResponses = useCallback(async (pageNum: number, append: boolean = false) => {
    if (!ideaId) {
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      setError(null);

      const response = await api.getIdeaResponses(ideaId, {
        page: pageNum,
        per_page: options.perPage || 20,
      });

      const transformedResponses = response.data.map(transformResponse);

      if (append) {
        setResponses(prev => [...prev, ...transformedResponses]);
      } else {
        setResponses(transformedResponses);
      }

      setTotal(response.meta.total);
      setHasMore(response.meta.has_more);
      setPage(pageNum);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch responses');
    } finally {
      setLoading(false);
    }
  }, [ideaId, options.perPage]);

  useEffect(() => {
    fetchResponses(1);
  }, [fetchResponses]);

  const refetch = useCallback(() => {
    fetchResponses(1);
  }, [fetchResponses]);

  const loadMore = useCallback(() => {
    if (hasMore && !loading) {
      fetchResponses(page + 1, true);
    }
  }, [hasMore, loading, page, fetchResponses]);

  return {
    responses,
    loading,
    error,
    total,
    hasMore,
    page,
    refetch,
    loadMore,
  };
}
