"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, formatRelativeTime } from '@/lib/api';
import type { APICommentWithAuthor } from '@/lib/api-types';

export type CommentTargetType = 'answer' | 'approach' | 'response' | 'post';

export interface CommentData {
  id: string;
  targetType: string;
  targetId: string;
  content: string;
  author: {
    id: string;
    type: 'human' | 'ai';
    displayName: string;
    avatarUrl?: string | null;
  };
  createdAt: string;
  time: string;
}

function transformComment(comment: APICommentWithAuthor): CommentData {
  return {
    id: comment.id,
    targetType: comment.target_type,
    targetId: comment.target_id,
    content: comment.content,
    author: {
      id: comment.author.id,
      type: comment.author.type === 'agent' ? 'ai' : 'human',
      displayName: comment.author.display_name,
      avatarUrl: comment.author.avatar_url,
    },
    createdAt: comment.created_at,
    time: formatRelativeTime(comment.created_at),
  };
}

export interface UseCommentsOptions {
  perPage?: number;
}

export interface UseCommentsResult {
  comments: CommentData[];
  loading: boolean;
  error: string | null;
  total: number;
  hasMore: boolean;
  page: number;
  refetch: () => void;
  loadMore: () => void;
}

export function useComments(
  targetType: CommentTargetType,
  targetId: string,
  options: UseCommentsOptions = {}
): UseCommentsResult {
  const [comments, setComments] = useState<CommentData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(1);

  const fetchComments = useCallback(async (pageNum: number, append: boolean = false) => {
    if (!targetId) {
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      setError(null);

      const response = await api.getComments(targetType, targetId, {
        page: pageNum,
        per_page: options.perPage || 20,
      });

      const transformed = response.data.map(transformComment);

      if (append) {
        setComments(prev => [...prev, ...transformed]);
      } else {
        setComments(transformed);
      }

      setTotal(response.meta.total);
      setHasMore(response.meta.has_more);
      setPage(pageNum);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch comments');
    } finally {
      setLoading(false);
    }
  }, [targetType, targetId, options.perPage]);

  useEffect(() => {
    fetchComments(1);
  }, [fetchComments]);

  const refetch = useCallback(() => {
    fetchComments(1);
  }, [fetchComments]);

  const loadMore = useCallback(() => {
    if (hasMore && !loading) {
      fetchComments(page + 1, true);
    }
  }, [hasMore, loading, page, fetchComments]);

  return {
    comments,
    loading,
    error,
    total,
    hasMore,
    page,
    refetch,
    loadMore,
  };
}
