"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIPost, FetchPostsParams, formatRelativeTime, truncateText, mapStatus } from '@/lib/api';

export type PostType = 'problem' | 'question' | 'idea';
export type AuthorType = 'human' | 'ai';

export interface FeedPost {
  id: string;
  type: PostType;
  title: string;
  snippet: string;
  tags: string[];
  author: {
    name: string;
    type: AuthorType;
    avatar?: string;
  };
  time: string;
  votes: number;
  responses: number;
  comments: number;
  views: number;
  status: string;
  isHot?: boolean;
  isPinned?: boolean;
}

// Transform API post to FeedPost format
function transformPost(post: APIPost): FeedPost {
  // Map comment counts based on post type:
  // - Problems: use approaches_count
  // - Questions: use answers_count
  // - Ideas: use answers_count (backend maps responses to this field)
  const responses = post.type === 'problem'
    ? (post.approaches_count || 0)
    : (post.answers_count || 0);

  return {
    id: post.id,
    type: post.type,
    title: post.title,
    snippet: truncateText(post.description, 200),
    tags: post.tags || [],
    author: {
      name: post.author.display_name,
      type: post.author.type === 'agent' ? 'ai' : 'human',
      avatar: post.author.avatar_url || undefined,
    },
    time: formatRelativeTime(post.created_at),
    votes: post.vote_score,
    responses,
    comments: post.comments_count || 0,
    views: post.view_count || 0,
    status: mapStatus(post.status),
    isHot: post.vote_score > 10, // Simple heuristic for now
    isPinned: false,
  };
}

export interface UsePostsResult {
  posts: FeedPost[];
  loading: boolean;
  error: string | null;
  total: number;
  hasMore: boolean;
  page: number;
  refetch: () => void;
  loadMore: () => void;
}

export function usePosts(params?: FetchPostsParams): UsePostsResult {
  const [posts, setPosts] = useState<FeedPost[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(params?.page || 1);

  // Stabilize params to prevent infinite re-renders (object reference changes each render)
  const paramsKey = JSON.stringify(params ?? {});

  const fetchPosts = useCallback(async (pageNum: number, append: boolean = false) => {
    try {
      setLoading(true);
      setError(null);

      const stableParams: FetchPostsParams = JSON.parse(paramsKey);
      const response = await api.getPosts({ ...stableParams, page: pageNum });

      // Defensive: handle null/undefined data
      if (!response || !response.data) {
        console.warn('[usePosts] Received empty response:', response);
        setPosts([]);
        setTotal(0);
        setHasMore(false);
        setLoading(false);
        return;
      }

      const transformedPosts = response.data.map(transformPost);

      if (append) {
        setPosts(prev => [...prev, ...transformedPosts]);
      } else {
        setPosts(transformedPosts);
      }

      setTotal(response.meta.total);
      setHasMore(response.meta.has_more);
      setPage(pageNum);
    } catch (err) {
      console.error('[usePosts] Error:', err);
      if (err && typeof err === 'object') {
        console.error('[usePosts] Full error:', JSON.stringify(err, Object.getOwnPropertyNames(err), 2));
      }
      setError(err instanceof Error ? err.message : 'Failed to fetch posts');
    } finally {
      setLoading(false);
    }
  }, [paramsKey]);

  useEffect(() => {
    fetchPosts(1);
  }, [fetchPosts]);

  const refetch = useCallback(() => {
    fetchPosts(1);
  }, [fetchPosts]);

  const loadMore = useCallback(() => {
    if (hasMore && !loading) {
      fetchPosts(page + 1, true);
    }
  }, [hasMore, loading, page, fetchPosts]);

  return {
    posts,
    loading,
    error,
    total,
    hasMore,
    page,
    refetch,
    loadMore,
  };
}

// Hook for search
export function useSearch(query: string, type?: PostType | 'all') {
  const [posts, setPosts] = useState<FeedPost[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!query.trim()) {
      setPosts([]);
      return;
    }

    const search = async () => {
      try {
        setLoading(true);
        setError(null);
        const response = await api.search({ q: query, type });

        // Defensive: handle null/undefined data
        if (!response || !response.data) {
          console.warn('[useSearch] Received empty response:', response);
          setPosts([]);
          setLoading(false);
          return;
        }

        const transformedPosts = response.data.map(post => transformPost(post as APIPost));
        setPosts(transformedPosts);
      } catch (err) {
        console.error('[useSearch] Error:', err);
        if (err && typeof err === 'object') {
          console.error('[useSearch] Full error:', JSON.stringify(err, Object.getOwnPropertyNames(err), 2));
        }
        setError(err instanceof Error ? err.message : 'Search failed');
      } finally {
        setLoading(false);
      }
    };

    // Debounce search
    const timer = setTimeout(search, 300);
    return () => clearTimeout(timer);
  }, [query, type]);

  return { posts, loading, error };
}
