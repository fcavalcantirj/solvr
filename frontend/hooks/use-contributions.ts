"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, formatRelativeTime } from '@/lib/api';
import type { APIContribution } from '@/lib/api-types';

export interface ContributionItem {
  type: 'answer' | 'approach' | 'response';
  id: string;
  parentId: string;
  parentTitle: string;
  parentType: string;
  contentPreview: string;
  status: string;
  timestamp: string;
  createdAt: string;
}

function transformContribution(item: APIContribution): ContributionItem {
  return {
    type: item.type,
    id: item.id,
    parentId: item.parent_id,
    parentTitle: item.parent_title,
    parentType: item.parent_type,
    contentPreview: item.content_preview,
    status: item.status || '',
    timestamp: formatRelativeTime(item.created_at),
    createdAt: item.created_at,
  };
}

export interface UseContributionsOptions {
  type?: 'answers' | 'approaches' | 'responses';
}

export interface UseContributionsResult {
  contributions: ContributionItem[];
  loading: boolean;
  error: string | null;
  total: number;
  hasMore: boolean;
  loadMore: () => void;
}

export function useContributions(
  userId: string,
  options: UseContributionsOptions = {}
): UseContributionsResult {
  const [contributions, setContributions] = useState<ContributionItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(1);

  const typeFilter = options.type;

  const fetchContributions = useCallback(async (pageNum: number, append: boolean = false) => {
    try {
      setLoading(true);
      setError(null);

      const response = await api.getUserContributions(userId, {
        type: typeFilter,
        page: pageNum,
        per_page: 20,
      });

      // Defensive: handle null/undefined data
      if (!response || !response.data) {
        console.warn('[useContributions] Received empty response:', response);
        setContributions([]);
        setTotal(0);
        setHasMore(false);
        setLoading(false);
        return;
      }

      const transformed = response.data.map(transformContribution);

      if (append) {
        setContributions(prev => [...prev, ...transformed]);
      } else {
        setContributions(transformed);
      }

      setTotal(response.meta.total);
      setHasMore(response.meta.has_more);
      setPage(pageNum);
    } catch (err) {
      console.error('[useContributions] Error:', err);
      if (err && typeof err === 'object') {
        console.error('[useContributions] Full error:', JSON.stringify(err, Object.getOwnPropertyNames(err), 2));
      }
      setError(err instanceof Error ? err.message : 'Failed to fetch contributions');
      if (!append) {
        setContributions([]);
      }
    } finally {
      setLoading(false);
    }
  }, [userId, typeFilter]);

  useEffect(() => {
    setPage(1);
    fetchContributions(1);
  }, [fetchContributions]);

  const loadMore = useCallback(() => {
    if (hasMore && !loading) {
      fetchContributions(page + 1, true);
    }
  }, [hasMore, loading, page, fetchContributions]);

  return {
    contributions,
    loading,
    error,
    total,
    hasMore,
    loadMore,
  };
}
