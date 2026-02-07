"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, formatRelativeTime, APIActivityItem } from '@/lib/api';

// Activity item for frontend use
export interface ActivityItem {
  id: string;
  type: string;  // 'post' | 'answer' | 'approach' | 'response'
  action: string;
  title: string;
  postType?: string;
  status?: string;
  createdAt: string;
  time: string;
  targetId?: string;
  targetTitle?: string;
}

export interface UseAgentActivityResult {
  items: ActivityItem[];
  loading: boolean;
  error: string | null;
  total: number;
  hasMore: boolean;
  page: number;
  loadMore: () => void;
  refetch: () => void;
}

// Transform API activity item to frontend format
function transformActivityItem(item: APIActivityItem): ActivityItem {
  return {
    id: item.id,
    type: item.type,
    action: item.action,
    title: item.title,
    postType: item.post_type,
    status: item.status,
    createdAt: item.created_at,
    time: formatRelativeTime(item.created_at),
    targetId: item.target_id,
    targetTitle: item.target_title,
  };
}

/**
 * Hook to fetch agent activity from the API.
 * @param agentId - The agent ID to fetch activity for
 * @param perPage - Items per page (default 10)
 * @returns Activity items, loading state, error, pagination controls
 */
export function useAgentActivity(agentId: string, perPage = 10): UseAgentActivityResult {
  const [items, setItems] = useState<ActivityItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(1);

  const fetchActivity = useCallback(async (pageNum: number, append: boolean = false) => {
    // Don't fetch if no agentId provided
    if (!agentId) {
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      setError(null);

      const response = await api.getAgentActivity(agentId, pageNum, perPage);
      const transformedItems = (response.data || []).map(transformActivityItem);

      if (append) {
        setItems(prev => [...prev, ...transformedItems]);
      } else {
        setItems(transformedItems);
      }

      setTotal(response.meta.total);
      setHasMore(response.meta.has_more);
      setPage(pageNum);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch activity');
      if (!append) {
        setItems([]);
      }
    } finally {
      setLoading(false);
    }
  }, [agentId, perPage]);

  useEffect(() => {
    fetchActivity(1);
  }, [fetchActivity]);

  const loadMore = useCallback(() => {
    if (hasMore && !loading) {
      fetchActivity(page + 1, true);
    }
  }, [hasMore, loading, page, fetchActivity]);

  const refetch = useCallback(() => {
    setItems([]);
    fetchActivity(1);
  }, [fetchActivity]);

  return {
    items,
    loading,
    error,
    total,
    hasMore,
    page,
    loadMore,
    refetch,
  };
}
