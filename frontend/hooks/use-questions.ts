"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIPost, FetchQuestionsParams, formatRelativeTime } from '@/lib/api';

function mapQuestionStatus(status: string): string {
  const statusMap: Record<string, string> = {
    'open': 'Unanswered',
    'answered': 'Answered',
    'solved': 'Accepted',
  };
  return statusMap[status.toLowerCase()] || status.toUpperCase();
}

export interface QuestionListItem {
  id: string;
  title: string;
  snippet: string;
  status: string;
  displayStatus: string;
  voteScore: number;
  answersCount: number;
  author: {
    id: string;
    name: string;
    type: 'human' | 'agent';
  };
  tags: string[];
  timestamp: string;
}

function transformQuestion(post: APIPost): QuestionListItem {
  return {
    id: post.id,
    title: post.title,
    snippet: post.description.slice(0, 200) + (post.description.length > 200 ? '...' : ''),
    status: post.status,
    displayStatus: mapQuestionStatus(post.status),
    voteScore: post.vote_score,
    answersCount: post.answers_count || 0,
    author: {
      id: post.author.id,
      name: post.author.display_name,
      type: post.author.type,
    },
    tags: post.tags || [],
    timestamp: formatRelativeTime(post.created_at),
  };
}

export interface UseQuestionsOptions {
  status?: string;
  tags?: string[];
  sort?: 'newest' | 'votes' | 'answers';
  page?: number;
  perPage?: number;
}

export interface UseQuestionsResult {
  questions: QuestionListItem[];
  loading: boolean;
  error: string | null;
  total: number;
  hasMore: boolean;
  page: number;
  refetch: () => void;
  loadMore: () => void;
}

export function useQuestions(options: UseQuestionsOptions = {}): UseQuestionsResult {
  const [questions, setQuestions] = useState<QuestionListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(options.page || 1);

  const optionsKey = JSON.stringify(options);

  const fetchQuestions = useCallback(async (pageNum: number, append: boolean = false) => {
    try {
      setLoading(true);
      setError(null);

      const stableOptions: UseQuestionsOptions = JSON.parse(optionsKey);
      const params: FetchQuestionsParams = {
        status: stableOptions.status,
        tags: stableOptions.tags,
        sort: stableOptions.sort,
        page: pageNum,
        per_page: stableOptions.perPage || 20,
      };

      const response = await api.getQuestions(params);

      // Defensive: handle null/undefined data
      if (!response || !response.data) {
        console.warn('[useQuestions] Received empty response:', response);
        setQuestions([]);
        setTotal(0);
        setHasMore(false);
        setLoading(false);
        return;
      }

      const transformed = response.data.map(transformQuestion);

      if (append) {
        setQuestions(prev => [...prev, ...transformed]);
      } else {
        setQuestions(transformed);
      }

      setTotal(response.meta?.total || 0);
      setHasMore(response.meta?.has_more || false);
      setPage(pageNum);
    } catch (err) {
      console.error('[useQuestions] Error fetching questions:', err);
      if (err && typeof err === 'object') {
        console.error('[useQuestions] Full error object:', JSON.stringify(err, Object.getOwnPropertyNames(err), 2));
      }
      setError(err instanceof Error ? err.message : 'Failed to fetch questions');
    } finally {
      setLoading(false);
    }
  }, [optionsKey]);

  useEffect(() => {
    fetchQuestions(1);
  }, [fetchQuestions]);

  const refetch = useCallback(() => {
    fetchQuestions(1);
  }, [fetchQuestions]);

  const loadMore = useCallback(() => {
    if (hasMore && !loading) {
      fetchQuestions(page + 1, true);
    }
  }, [hasMore, loading, page, fetchQuestions]);

  return {
    questions,
    loading,
    error,
    total,
    hasMore,
    page,
    refetch,
    loadMore,
  };
}
