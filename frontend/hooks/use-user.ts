"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIPost, formatRelativeTime } from '@/lib/api';

// User stats for frontend use
export interface UserStats {
  postsCreated: number;
  contributions: number;
  karma: number;
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
  stats: { posts_created?: number | null; contributions?: number | null; karma?: number | null } | null;
}): UserData {
  const stats = data.stats || { posts_created: 0, contributions: 0, karma: 0 };
  return {
    id: data.id,
    username: data.username,
    displayName: data.display_name,
    avatarUrl: data.avatar_url,
    bio: data.bio,
    stats: {
      postsCreated: stats.posts_created ?? 0,
      contributions: stats.contributions ?? 0,
      karma: stats.karma ?? 0,
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
 * @returns User data, posts, loading state, error, and refetch function
 */
export function useUser(id: string): UseUserResult {
  const [user, setUser] = useState<UserData | null>(null);
  const [posts, setPosts] = useState<UserPostData[]>([]);
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

      // Fetch user profile and posts in parallel
      const [profileResponse, postsResponse] = await Promise.all([
        api.getUserProfile(id),
        api.getUserPosts(id),
      ]);

      // Transform and set user data
      setUser(transformUser(profileResponse.data));

      // Defensive check - handle undefined/null data array
      const postsData = postsResponse?.data ?? [];
      const transformedPosts = postsData.map(transformPost);
      setPosts(transformedPosts);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch user');
      setUser(null);
      setPosts([]);
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
    user,
    posts,
    loading,
    error,
    refetch,
  };
}
