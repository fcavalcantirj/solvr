"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, formatRelativeTime } from '@/lib/api';
import type { APIAuthoredReply } from '@/lib/api-types';

// A user's contributions are their replies, listed newest first by GET /v1/replies (idx 73
// step 3: it replaced the retired GET /v1/users/{id}/contributions).
export interface ContributionItem {
  id: string;
  postId: string;
  postTitle: string;
  legacyType: string | null;
  body: string;
  timestamp: string;
  createdAt: string;
}

function transformReply(reply: APIAuthoredReply): ContributionItem {
  return {
    id: reply.id,
    postId: reply.post_id,
    postTitle: reply.post.title,
    legacyType: reply.legacy_type ?? null,
    body: reply.body,
    timestamp: formatRelativeTime(reply.created_at),
    createdAt: reply.created_at,
  };
}

export interface UseContributionsResult {
  contributions: ContributionItem[];
  loading: boolean;
  error: string | null;
  total: number;
  hasMore: boolean;
  loadMore: () => void;
}

const PAGE_SIZE = 20;

export function useContributions(userId: string): UseContributionsResult {
  const [contributions, setContributions] = useState<ContributionItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);

  const fetchReplies = useCallback(async (cursor?: string) => {
    try {
      setLoading(true);
      setError(null);

      const response = await api.getRepliesByAuthor('human', userId, cursor ? { limit: PAGE_SIZE, cursor } : { limit: PAGE_SIZE });
      const page = response.data.map(transformReply);

      setContributions((prev) => (cursor ? [...prev, ...page] : page));
      setTotal(response.meta.total);
      setHasMore(response.meta.has_more);
      setNextCursor(response.meta.next_cursor);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch contributions');
      if (!cursor) {
        setContributions([]);
      }
    } finally {
      setLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    fetchReplies();
  }, [fetchReplies]);

  const loadMore = useCallback(() => {
    if (hasMore && !loading && nextCursor) {
      fetchReplies(nextCursor);
    }
  }, [hasMore, loading, nextCursor, fetchReplies]);

  return {
    contributions,
    loading,
    error,
    total,
    hasMore,
    loadMore,
  };
}
