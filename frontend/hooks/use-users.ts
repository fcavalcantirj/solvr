"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIUserListItem, formatRelativeTime } from '@/lib/api';

export interface UserListItem {
  id: string;
  username: string;
  displayName: string;
  avatarUrl?: string;
  reputation: number;
  agentsCount: number;
  createdAt: string;
  initials: string;
}

// Transform API user to UserListItem format
function transformUser(user: APIUserListItem): UserListItem {
  return {
    id: user.id,
    username: user.username,
    displayName: user.display_name,
    avatarUrl: user.avatar_url || undefined,
    reputation: user.reputation,
    agentsCount: user.agents_count,
    createdAt: formatRelativeTime(user.created_at),
    initials: user.display_name.slice(0, 2).toUpperCase(),
  };
}

export interface UseUsersOptions {
  limit?: number;
  offset?: number;
  sort?: 'newest' | 'reputation' | 'agents';
}

export interface UseUsersResult {
  users: UserListItem[];
  loading: boolean;
  error: string | null;
  total: number;
  hasMore: boolean;
  page: number;
  refetch: () => void;
  loadMore: () => void;
}

export function useUsers(options: UseUsersOptions = {}): UseUsersResult {
  const [users, setUsers] = useState<UserListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(1);

  const limit = options.limit || 20;
  const sort = options.sort || 'reputation';

  // Stabilize options to prevent infinite re-renders
  const optionsKey = JSON.stringify({ limit, sort, offset: options.offset });

  const fetchUsers = useCallback(async (offset: number, append: boolean = false) => {
    try {
      setLoading(true);
      setError(null);

      const stableOptions: UseUsersOptions = JSON.parse(optionsKey);
      const params = {
        limit: stableOptions.limit || 20,
        offset: offset,
        sort: stableOptions.sort || 'reputation',
      };

      const response = await api.getUsers(params);
      const transformedUsers = response.data.map(transformUser);

      if (append) {
        setUsers(prev => [...prev, ...transformedUsers]);
      } else {
        setUsers(transformedUsers);
      }

      setTotal(response.meta.total);
      setHasMore(response.meta.has_more);
      setPage(Math.floor(offset / limit) + 1);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch users');
    } finally {
      setLoading(false);
    }
  }, [optionsKey, limit]);

  useEffect(() => {
    fetchUsers(options.offset || 0);
  }, [fetchUsers, options.offset]);

  const refetch = useCallback(() => {
    fetchUsers(0);
  }, [fetchUsers]);

  const loadMore = useCallback(() => {
    if (hasMore && !loading) {
      fetchUsers(users.length, true);
    }
  }, [hasMore, loading, users.length, fetchUsers]);

  return {
    users,
    loading,
    error,
    total,
    hasMore,
    page,
    refetch,
    loadMore,
  };
}
