"use client";

import { useState, useCallback } from 'react';
import { api } from '@/lib/api';
import { isUnauthorizedError } from '@/lib/api-error';

export interface UseAnswerVoteResult {
  score: number;
  userVote: 'up' | 'down' | null;
  isVoting: boolean;
  error: string | null;
  upvote: () => Promise<void>;
  downvote: () => Promise<void>;
}

export function useAnswerVote(answerId: string, initialScore: number): UseAnswerVoteResult {
  const [score, setScore] = useState(initialScore);
  const [isVoting, setIsVoting] = useState(false);
  const [userVote, setUserVote] = useState<'up' | 'down' | null>(null);
  const [error, setError] = useState<string | null>(null);

  const vote = useCallback(async (direction: 'up' | 'down') => {
    if (isVoting) return;

    setIsVoting(true);
    setError(null);

    const previousScore = score;
    const previousVote = userVote;
    setScore(direction === 'up' ? score + 1 : score - 1);
    setUserVote(direction);

    try {
      await api.voteOnAnswer(answerId, direction);
    } catch (err) {
      setScore(previousScore);
      setUserVote(previousVote);
      if (isUnauthorizedError(err)) {
        setError('Login required to vote');
        // Modal will be shown automatically by AuthContext
        return;
      }
      setError(err instanceof Error ? err.message : 'Failed to vote');
    } finally {
      setIsVoting(false);
    }
  }, [answerId, score, isVoting, userVote]);

  const upvote = useCallback(() => vote('up'), [vote]);
  const downvote = useCallback(() => vote('down'), [vote]);

  return { score, userVote, isVoting, error, upvote, downvote };
}
