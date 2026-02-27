"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIPost, formatRelativeTime, mapStatus } from '@/lib/api';

// Answer type for frontend use
export interface QuestionAnswer {
  id: string;
  content: string;
  isAccepted: boolean;
  voteScore: number;
  upvotes: number;
  downvotes: number;
  author: {
    id: string;
    type: 'human' | 'ai';
    displayName: string;
  };
  createdAt: string;
  time: string;
}

// Question data for frontend use
export interface QuestionData {
  id: string;
  title: string;
  description: string;
  status: string;
  voteScore: number;
  upvotes: number;
  downvotes: number;
  author: {
    id: string;
    type: 'human' | 'ai';
    displayName: string;
  };
  tags: string[];
  createdAt: string;
  updatedAt: string;
  time: string;
  answersCount: number;
  views: number;
}

export interface UseQuestionResult {
  question: QuestionData | null;
  answers: QuestionAnswer[];
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

// Transform API question to frontend format
function transformQuestion(post: APIPost): QuestionData {
  return {
    id: post.id,
    title: post.title,
    description: post.description,
    status: mapStatus(post.status),
    voteScore: post.vote_score,
    upvotes: post.upvotes,
    downvotes: post.downvotes,
    author: {
      id: post.author.id,
      type: post.author.type === 'agent' ? 'ai' : 'human',
      displayName: post.author.display_name,
    },
    tags: post.tags || [],
    createdAt: post.created_at,
    updatedAt: post.updated_at,
    time: formatRelativeTime(post.created_at),
    answersCount: post.answers_count || 0,
    views: post.view_count || 0,
  };
}

// Transform API answer to frontend format
function transformAnswer(answer: APIAnswerWithAuthor): QuestionAnswer {
  return {
    id: answer.id,
    content: answer.content,
    isAccepted: answer.is_accepted,
    voteScore: answer.vote_score,
    upvotes: answer.upvotes,
    downvotes: answer.downvotes,
    author: {
      id: answer.author.id,
      type: answer.author.type === 'agent' ? 'ai' : 'human',
      displayName: answer.author.display_name,
    },
    createdAt: answer.created_at,
    time: formatRelativeTime(answer.created_at),
  };
}

// API response types for answers
interface APIAnswerAuthor {
  type: 'agent' | 'human';
  id: string;
  display_name: string;
}

interface APIAnswerWithAuthor {
  id: string;
  question_id: string;
  author_type: 'agent' | 'human';
  author_id: string;
  content: string;
  is_accepted: boolean;
  upvotes: number;
  downvotes: number;
  vote_score: number;
  created_at: string;
  author: APIAnswerAuthor;
}

interface APIAnswersResponse {
  data: APIAnswerWithAuthor[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

/**
 * Hook to fetch a question and its answers from the API.
 * @param id - The question ID to fetch
 * @param initialPost - Optional server-side fetched post data (for SSR/SEO)
 * @returns Question data, answers, loading state, error, and refetch function
 */
export function useQuestion(id: string, initialPost?: APIPost): UseQuestionResult {
  const [question, setQuestion] = useState<QuestionData | null>(
    initialPost ? transformQuestion(initialPost) : null
  );
  const [answers, setAnswers] = useState<QuestionAnswer[]>([]);
  const [loading, setLoading] = useState(!initialPost);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    if (!id) {
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      setError(null);

      if (initialPost) {
        // Only fetch answers (question already loaded from SSR)
        const answersResponse = await api.getQuestionAnswers(id);
        const answersData = answersResponse?.data ?? [];
        setAnswers(answersData.map(transformAnswer));
      } else {
        // Fetch question and answers in parallel
        const [questionResponse, answersResponse] = await Promise.all([
          api.getPost(id),
          api.getQuestionAnswers(id),
        ]);

        setQuestion(transformQuestion(questionResponse.data));

        // FE-021: Defensive check - handle undefined/null data array
        const answersData = answersResponse?.data ?? [];
        setAnswers(answersData.map(transformAnswer));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch question');
      if (!initialPost) {
        setQuestion(null);
      }
      setAnswers([]);
    } finally {
      setLoading(false);
    }
  }, [id, initialPost]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const refetch = useCallback(() => {
    fetchData();
  }, [fetchData]);

  return {
    question,
    answers,
    loading,
    error,
    refetch,
  };
}
