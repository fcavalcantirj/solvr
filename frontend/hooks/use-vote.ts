"use client";

import { useState, useCallback, useEffect } from 'react';
import { api } from '@/lib/api';
import { isUnauthorizedError } from '@/lib/api-error';
import { useAuth } from '@/hooks/use-auth';

export interface UseVoteResult {
  score: number;
  userVote: 'up' | 'down' | null;
  isVoting: boolean;
  error: string | null;
  upvote: () => Promise<void>;
  downvote: () => Promise<void>;
}

/**
 * Hook to handle voting on posts.
 * Provides optimistic updates with rollback on error.
 * Fetches user's current vote on mount.
 * @param postId - The post ID to vote on
 * @param initialScore - The initial vote score
 * @returns Vote state and actions
 */
export function useVote(postId: string, initialScore: number): UseVoteResult {
  const { isAuthenticated } = useAuth();
  const [score, setScore] = useState(initialScore);
  const [isVoting, setIsVoting] = useState(false);
  const [userVote, setUserVote] = useState<'up' | 'down' | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Fetch user's vote on mount (only when authenticated)
  useEffect(() => {
    if (!isAuthenticated) return;

    const fetchUserVote = async () => {
      try {
        const response = await api.getMyVote(postId);
        setUserVote(response.data.vote);
      } catch (err) {
        // Silently fail if unauthorized (user not logged in)
        if (!isUnauthorizedError(err)) {
          console.error('Failed to fetch user vote:', err);
        }
      }
    };

    fetchUserVote();
  }, [postId, isAuthenticated]);

  const vote = useCallback(async (direction: 'up' | 'down') => {
    if (isVoting) return;

    setIsVoting(true);
    setError(null);

    // Optimistic update
    const previousScore = score;
    const previousVote = userVote;
    setScore(direction === 'up' ? score + 1 : score - 1);
    setUserVote(direction);

    try {
      const response = await api.voteOnPost(postId, direction);
      // Update with actual server score and vote direction
      setScore(response.data.vote_score);
      setUserVote(response.data.user_vote);
    } catch (err) {
      // Rollback on error
      setScore(previousScore);
      setUserVote(previousVote);

      if (isUnauthorizedError(err)) {
        setError('Login required to vote');
        // Modal will be shown automatically by AuthContext
        return;
      }

      // Handle duplicate vote error (409)
      const errorMessage = err instanceof Error ? err.message : 'Failed to vote';
      if (errorMessage.includes('409') || errorMessage.includes('DUPLICATE_VOTE')) {
        setError('You have already voted on this post');
      } else {
        setError(errorMessage);
      }
    } finally {
      setIsVoting(false);
    }
  }, [postId, score, isVoting, userVote]);

  const upvote = useCallback(() => vote('up'), [vote]);
  const downvote = useCallback(() => vote('down'), [vote]);

  return {
    score,
    userVote,
    isVoting,
    error,
    upvote,
    downvote,
  };
}
