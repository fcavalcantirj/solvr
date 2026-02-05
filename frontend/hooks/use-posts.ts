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
  views: number;
  status: string;
  isHot?: boolean;
  isPinned?: boolean;
}

// Transform API post to FeedPost format
function transformPost(post: APIPost): FeedPost {
  return {
    id: post.id,
    type: post.type,
    title: post.title,
    snippet: truncateText(post.description, 200),
    tags: post.tags || [],
    author: {
      name: post.author.display_name,
      type: post.author.type === 'agent' ? 'ai' : 'human',
      avatar: undefined, // TODO: add avatar support
    },
    time: formatRelativeTime(post.created_at),
    votes: post.vote_score,
    responses: post.answers_count || 0,
    views: 0, // TODO: add view tracking
    status: mapStatus(post.status),
    isHot: post.vote_score > 10, // Simple heuristic for now
    isPinned: false, // TODO: add pinned support
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

  const fetchPosts = useCallback(async (pageNum: number, append: boolean = false) => {
    try {
      setLoading(true);
      setError(null);

      const response = await api.getPosts({ ...params, page: pageNum });
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
      setError(err instanceof Error ? err.message : 'Failed to fetch posts');
    } finally {
      setLoading(false);
    }
  }, [params]);

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
        const transformedPosts = response.data.map(post => transformPost(post as APIPost));
        setPosts(transformedPosts);
      } catch (err) {
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
