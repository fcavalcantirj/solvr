"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useVote } from './use-vote';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    voteOnPost: vi.fn(),
  },
}));

describe('useVote', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should upvote and update score optimistically', async () => {
    // Arrange - API returns new score
    (api.voteOnPost as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: { vote_score: 25, upvotes: 26, downvotes: 1 }
    });

    // Act
    const { result } = renderHook(() => useVote('post-123', 24));

    // Assert - initial state
    expect(result.current.score).toBe(24);
    expect(result.current.isVoting).toBe(false);

    // Trigger upvote
    await act(async () => {
      await result.current.upvote();
    });

    // Assert - after vote
    expect(api.voteOnPost).toHaveBeenCalledWith('post-123', 'up');
    expect(result.current.score).toBe(25);
  });

  it('should downvote and update score optimistically', async () => {
    // Arrange
    (api.voteOnPost as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: { vote_score: 23, upvotes: 24, downvotes: 1 }
    });

    // Act
    const { result } = renderHook(() => useVote('post-123', 24));

    // Trigger downvote
    await act(async () => {
      await result.current.downvote();
    });

    // Assert
    expect(api.voteOnPost).toHaveBeenCalledWith('post-123', 'down');
    expect(result.current.score).toBe(23);
  });

  it('should handle API errors gracefully', async () => {
    // Arrange - API returns error
    (api.voteOnPost as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Auth required'));

    // Act
    const { result } = renderHook(() => useVote('post-123', 24));

    // Trigger upvote
    await act(async () => {
      await result.current.upvote();
    });

    // Assert - score should revert to original
    expect(result.current.score).toBe(24);
    expect(result.current.error).toBe('Auth required');
  });

  it('should handle auth required error', async () => {
    // Arrange - API returns 401
    (api.voteOnPost as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('API error: 401')
    );

    // Act
    const { result } = renderHook(() => useVote('post-123', 24));

    // Trigger upvote
    await act(async () => {
      await result.current.upvote();
    });

    // Assert
    expect(result.current.error).toBe('API error: 401');
    expect(result.current.score).toBe(24);
  });

  it('should redirect to login on 401 APIError', async () => {
    // Arrange - API returns 401 APIError
    const { APIError } = await import('@/lib/api-error');
    (api.voteOnPost as ReturnType<typeof vi.fn>).mockRejectedValue(
      new APIError('authentication required', 401)
    );

    // Mock window.location
    const originalHref = window.location.href;
    const hrefSetter = vi.fn();
    Object.defineProperty(window, 'location', {
      writable: true,
      value: {
        ...window.location,
        pathname: '/feed',
        get href() { return ''; },
        set href(val: string) { hrefSetter(val); },
      },
    });

    // Act
    const { result } = renderHook(() => useVote('post-123', 24));
    await act(async () => {
      await result.current.upvote();
    });

    // Assert - score rolled back, redirect triggered
    expect(result.current.score).toBe(24);
    expect(hrefSetter).toHaveBeenCalledWith(
      expect.stringContaining('/v1/auth/google')
    );
  });

  it('should track userVote after successful vote', async () => {
    // Arrange - API returns user_vote
    (api.voteOnPost as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: { vote_score: 25, upvotes: 26, downvotes: 1, user_vote: 'up' }
    });

    // Act
    const { result } = renderHook(() => useVote('post-123', 24));

    // Assert - initial state has no userVote
    expect(result.current.userVote).toBeNull();

    // Trigger upvote
    await act(async () => {
      await result.current.upvote();
    });

    // Assert - userVote should be 'up' after successful vote
    expect(result.current.userVote).toBe('up');
  });

  it('should reset userVote on error rollback', async () => {
    // Arrange - API fails
    (api.voteOnPost as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Server error'));

    // Act
    const { result } = renderHook(() => useVote('post-123', 24));

    await act(async () => {
      await result.current.upvote();
    });

    // Assert - userVote should be null after rollback
    expect(result.current.userVote).toBeNull();
  });

  it('should set isVoting during API call', async () => {
    // Arrange - slow API call
    let resolvePromise: (value: unknown) => void;
    const promise = new Promise((resolve) => {
      resolvePromise = resolve;
    });
    (api.voteOnPost as ReturnType<typeof vi.fn>).mockReturnValue(promise);

    // Act
    const { result } = renderHook(() => useVote('post-123', 24));

    // Start voting
    act(() => {
      result.current.upvote();
    });

    // Assert - isVoting should be true during call
    expect(result.current.isVoting).toBe(true);

    // Resolve the promise
    await act(async () => {
      resolvePromise!({ data: { vote_score: 25 } });
      await promise;
    });

    // Assert - isVoting should be false after call
    expect(result.current.isVoting).toBe(false);
  });
});
