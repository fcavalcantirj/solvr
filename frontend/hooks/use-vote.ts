"use client";

import { useState, useCallback } from 'react';
import { api } from '@/lib/api';

export interface UseVoteResult {
  score: number;
  isVoting: boolean;
  error: string | null;
  upvote: () => Promise<void>;
  downvote: () => Promise<void>;
}

/**
 * Hook to handle voting on posts.
 * Provides optimistic updates with rollback on error.
 * @param postId - The post ID to vote on
 * @param initialScore - The initial vote score
 * @returns Vote state and actions
 */
export function useVote(postId: string, initialScore: number): UseVoteResult {
  const [score, setScore] = useState(initialScore);
  const [isVoting, setIsVoting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const vote = useCallback(async (direction: 'up' | 'down') => {
    if (isVoting) return;

    setIsVoting(true);
    setError(null);

    // Optimistic update
    const previousScore = score;
    setScore(direction === 'up' ? score + 1 : score - 1);

    try {
      const response = await api.voteOnPost(postId, direction);
      // Update with actual server score
      setScore(response.data.vote_score);
    } catch (err) {
      // Rollback on error
      setScore(previousScore);
      setError(err instanceof Error ? err.message : 'Failed to vote');
    } finally {
      setIsVoting(false);
    }
  }, [postId, score, isVoting]);

  const upvote = useCallback(() => vote('up'), [vote]);
  const downvote = useCallback(() => vote('down'), [vote]);

  return {
    score,
    isVoting,
    error,
    upvote,
    downvote,
  };
}
