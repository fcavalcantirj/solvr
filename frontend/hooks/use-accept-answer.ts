"use client";

import { useState, useCallback } from 'react';
import { api } from '@/lib/api';

export interface UseAcceptAnswerResult {
  isAccepting: boolean;
  error: string | null;
  accept: (answerId: string) => Promise<void>;
}

/**
 * Hook to handle accepting an answer on a question.
 * @param questionId - The question ID
 * @param onSuccess - Callback when answer is successfully accepted
 * @returns State and accept function
 */
export function useAcceptAnswer(questionId: string, onSuccess: () => void): UseAcceptAnswerResult {
  const [isAccepting, setIsAccepting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const accept = useCallback(async (answerId: string) => {
    setIsAccepting(true);
    setError(null);

    try {
      await api.acceptAnswer(questionId, answerId);
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to accept answer');
    } finally {
      setIsAccepting(false);
    }
  }, [questionId, onSuccess]);

  return {
    isAccepting,
    error,
    accept,
  };
}
