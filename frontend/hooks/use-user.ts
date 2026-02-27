"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIPost, formatRelativeTime } from '@/lib/api';

// User stats for frontend use
export interface UserStats {
  postsCreated: number;
  contributions: number;
  reputation: number;
}

// User data for frontend use
export interface UserData {
  id: string;
  username: string;
  displayName: string;
  avatarUrl?: string;
  bio?: string;
  stats: UserStats;
}

// Post data for frontend use in user profile
export interface UserPostData {
  id: string;
  type: 'problem' | 'question' | 'idea';
  title: string;
  description: string;
  status: string;
  voteScore: number;
  upvotes: number;
  downvotes: number;
  views: number;
  tags: string[];
  createdAt: string;
  time: string;
}

export interface UseUserResult {
  user: UserData | null;
  posts: UserPostData[];
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

// Transform API user profile to frontend format
function transformUser(data: {
  id: string;
  username: string;
  display_name: string;
  avatar_url?: string;
  bio?: string;
  stats: { posts_created?: number | null; contributions?: number | null; reputation?: number | null } | null;
}): UserData {
  const stats = data.stats || { posts_created: 0, contributions: 0, reputation: 0 };
  return {
    id: data.id,
    username: data.username,
    displayName: data.display_name,
    avatarUrl: data.avatar_url,
    bio: data.bio,
    stats: {
      postsCreated: stats.posts_created ?? 0,
      contributions: stats.contributions ?? 0,
      reputation: stats.reputation ?? 0,
    },
  };
}

// Transform API post to frontend format
function transformPost(post: APIPost): UserPostData {
  return {
    id: post.id,
    type: post.type,
    title: post.title,
    description: post.description,
    status: post.status,
    voteScore: post.vote_score,
    upvotes: post.upvotes,
    downvotes: post.downvotes,
    views: post.view_count || 0,
    tags: post.tags || [],
    createdAt: post.created_at,
    time: formatRelativeTime(post.created_at),
  };
}

/**
 * Hook to fetch a user profile and their posts from the API.
 * @param id - The user ID to fetch
 * @param initialUserData - Optional server-side fetched user data (for SSR/SEO)
 * @returns User data, posts, loading state, error, and refetch function
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function useUser(id: string, initialUserData?: any): UseUserResult {
  const [user, setUser] = useState<UserData | null>(
    initialUserData ? transformUser(initialUserData) : null
  );
  const [posts, setPosts] = useState<UserPostData[]>([]);
  const [loading, setLoading] = useState(!initialUserData);
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

      if (initialUserData) {
        // SSR already provided user data — only fetch posts
        const postsResponse = await api.getUserPosts(id);
        const postsData = postsResponse?.data ?? [];
        setPosts(postsData.map(transformPost));
      } else {
        // Fetch user profile and posts in parallel
        const [profileResponse, postsResponse] = await Promise.all([
          api.getUserProfile(id),
          api.getUserPosts(id),
        ]);
        setUser(transformUser(profileResponse.data));
        const postsData = postsResponse?.data ?? [];
        setPosts(postsData.map(transformPost));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch user');
      if (!initialUserData) {
        setUser(null);
      }
      setPosts([]);
    } finally {
      setLoading(false);
    }
  }, [id, initialUserData]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const refetch = useCallback(() => {
    fetchData();
  }, [fetchData]);

  return {
    user,
    posts,
    loading,
    error,
    refetch,
  };
}
