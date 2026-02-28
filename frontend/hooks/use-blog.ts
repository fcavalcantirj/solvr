"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, formatRelativeTime } from '@/lib/api';
import type { APIBlogPost, FetchBlogPostsParams } from '@/lib/api-types';

export interface BlogPost {
  slug: string;
  title: string;
  excerpt: string;
  body: string;
  tags: string[];
  coverImageUrl?: string;
  author: {
    name: string;
    type: 'human' | 'ai';
    avatar?: string;
  };
  readTime: string;
  publishedAt: string;
  voteScore: number;
  viewCount: number;
  userVote?: 'up' | 'down' | null;
}

export interface BlogTag {
  name: string;
  count: number;
}

function transformBlogPost(post: APIBlogPost): BlogPost {
  return {
    slug: post.slug,
    title: post.title,
    excerpt: post.excerpt || '',
    body: post.body,
    tags: post.tags || [],
    coverImageUrl: post.cover_image_url || undefined,
    author: {
      name: post.author.display_name,
      type: post.author.type === 'agent' ? 'ai' : 'human',
      avatar: post.author.avatar_url || undefined,
    },
    readTime: `${post.read_time_minutes} min read`,
    publishedAt: post.published_at ? formatRelativeTime(post.published_at) : formatRelativeTime(post.created_at),
    voteScore: post.vote_score,
    viewCount: post.view_count,
    userVote: post.user_vote,
  };
}

export interface UseBlogPostsResult {
  posts: BlogPost[];
  loading: boolean;
  error: string | null;
  total: number;
  hasMore: boolean;
  page: number;
  refetch: () => void;
  loadMore: () => void;
}

export function useBlogPosts(params?: FetchBlogPostsParams): UseBlogPostsResult {
  const [posts, setPosts] = useState<BlogPost[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(params?.page || 1);

  const paramsKey = JSON.stringify(params ?? {});

  const fetchPosts = useCallback(async (pageNum: number, append: boolean = false) => {
    try {
      setLoading(true);
      setError(null);

      const stableParams: FetchBlogPostsParams = JSON.parse(paramsKey);
      const response = await api.getBlogPosts({ ...stableParams, page: pageNum });

      if (!response || !response.data) {
        setPosts([]);
        setTotal(0);
        setHasMore(false);
        setLoading(false);
        return;
      }

      const transformed = response.data.map(transformBlogPost);

      if (append) {
        setPosts(prev => [...prev, ...transformed]);
      } else {
        setPosts(transformed);
      }

      setTotal(response.meta.total);
      setHasMore(response.meta.has_more);
      setPage(pageNum);
    } catch (err) {
      console.error('[useBlogPosts] Error:', err);
      setError(err instanceof Error ? err.message : 'Failed to fetch blog posts');
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

  return { posts, loading, error, total, hasMore, page, refetch, loadMore };
}

export interface UseBlogPostResult {
  post: BlogPost | null;
  loading: boolean;
  error: string | null;
}

export function useBlogPost(slug: string): UseBlogPostResult {
  const [post, setPost] = useState<BlogPost | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!slug) {
      setLoading(false);
      return;
    }

    const fetchPost = async () => {
      try {
        setLoading(true);
        setError(null);
        const response = await api.getBlogPost(slug);

        if (!response || !response.data) {
          setPost(null);
          setLoading(false);
          return;
        }

        setPost(transformBlogPost(response.data));
      } catch (err) {
        console.error('[useBlogPost] Error:', err);
        setError(err instanceof Error ? err.message : 'Failed to fetch blog post');
        setPost(null);
      } finally {
        setLoading(false);
      }
    };

    fetchPost();
  }, [slug]);

  return { post, loading, error };
}

export interface UseBlogFeaturedResult {
  post: BlogPost | null;
  loading: boolean;
  error: string | null;
}

export function useBlogFeatured(): UseBlogFeaturedResult {
  const [post, setPost] = useState<BlogPost | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchFeatured = async () => {
      try {
        setLoading(true);
        setError(null);
        const response = await api.getBlogFeatured();

        if (!response || !response.data) {
          setPost(null);
          setLoading(false);
          return;
        }

        setPost(transformBlogPost(response.data));
      } catch (err) {
        console.error('[useBlogFeatured] Error:', err);
        setError(err instanceof Error ? err.message : 'Failed to fetch featured post');
        setPost(null);
      } finally {
        setLoading(false);
      }
    };

    fetchFeatured();
  }, []);

  return { post, loading, error };
}

export interface UseBlogTagsResult {
  tags: BlogTag[];
  loading: boolean;
  error: string | null;
}

export function useBlogTags(): UseBlogTagsResult {
  const [tags, setTags] = useState<BlogTag[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchTags = async () => {
      try {
        setLoading(true);
        setError(null);
        const response = await api.getBlogTags();

        if (!response || !response.data) {
          setTags([]);
          setLoading(false);
          return;
        }

        setTags(response.data);
      } catch (err) {
        console.error('[useBlogTags] Error:', err);
        setError(err instanceof Error ? err.message : 'Failed to fetch blog tags');
      } finally {
        setLoading(false);
      }
    };

    fetchTags();
  }, []);

  return { tags, loading, error };
}
