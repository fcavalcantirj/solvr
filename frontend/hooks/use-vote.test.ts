"use client";

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useVote } from './use-vote';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    voteOnPost: vi.fn(),
    getMyVote: vi.fn(),
  },
}));

// Mock the useAuth hook
vi.mock('@/hooks/use-auth', () => ({
  useAuth: vi.fn(() => ({
    isAuthenticated: false,
    user: null,
    isLoading: false,
    showAuthModal: false,
    authModalMessage: '',
    setShowAuthModal: vi.fn(),
    setToken: vi.fn(),
    logout: vi.fn(),
    loginWithGitHub: vi.fn(),
    loginWithGoogle: vi.fn(),
    loginWithEmail: vi.fn(),
    register: vi.fn(),
  })),
}));

import { useAuth } from '@/hooks/use-auth';

const mockUseAuth = vi.mocked(useAuth);

describe('useVote', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default: user is NOT authenticated
    mockUseAuth.mockReturnValue({
      isAuthenticated: false,
      user: null,
      isLoading: false,
      showAuthModal: false,
      authModalMessage: '',
      setShowAuthModal: vi.fn(),
      setToken: vi.fn(),
      logout: vi.fn(),
      loginWithGitHub: vi.fn(),
      loginWithGoogle: vi.fn(),
      loginWithEmail: vi.fn(),
      register: vi.fn(),
    });
    // Mock getMyVote to return null vote by default (user not voted yet or not logged in)
    (api.getMyVote as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Not authenticated'));
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should NOT call getMyVote when user is not authenticated', async () => {
    // Arrange - user is not authenticated (default mock)

    // Act
    const { result } = renderHook(() => useVote('post-123', 24));

    // Wait for effects to settle
    await waitFor(() => {
      expect(result.current.score).toBe(24);
    });

    // Assert - getMyVote should NOT have been called
    expect(api.getMyVote).not.toHaveBeenCalled();
    expect(result.current.userVote).toBeNull();
  });

  it('should call getMyVote when user IS authenticated', async () => {
    // Arrange - user is authenticated
    mockUseAuth.mockReturnValue({
      isAuthenticated: true,
      user: { id: 'user-1', type: 'human', displayName: 'Test User' },
      isLoading: false,
      showAuthModal: false,
      authModalMessage: '',
      setShowAuthModal: vi.fn(),
      setToken: vi.fn(),
      logout: vi.fn(),
      loginWithGitHub: vi.fn(),
      loginWithGoogle: vi.fn(),
      loginWithEmail: vi.fn(),
      register: vi.fn(),
    });
    (api.getMyVote as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: { vote: 'up' },
    });

    // Act
    const { result } = renderHook(() => useVote('post-123', 24));

    // Assert - getMyVote should have been called
    await waitFor(() => {
      expect(api.getMyVote).toHaveBeenCalledWith('post-123');
      expect(result.current.userVote).toBe('up');
    });
  });

  it('should upvote and update score optimistically', async () => {
    // Arrange - user must be authenticated to vote
    mockUseAuth.mockReturnValue({
      isAuthenticated: true,
      user: { id: 'user-1', type: 'human', displayName: 'Test User' },
      isLoading: false,
      showAuthModal: false,
      authModalMessage: '',
      setShowAuthModal: vi.fn(),
      setToken: vi.fn(),
      logout: vi.fn(),
      loginWithGitHub: vi.fn(),
      loginWithGoogle: vi.fn(),
      loginWithEmail: vi.fn(),
      register: vi.fn(),
    });
    (api.getMyVote as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Not authenticated'));
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
    mockUseAuth.mockReturnValue({
      isAuthenticated: true,
      user: { id: 'user-1', type: 'human', displayName: 'Test User' },
      isLoading: false,
      showAuthModal: false,
      authModalMessage: '',
      setShowAuthModal: vi.fn(),
      setToken: vi.fn(),
      logout: vi.fn(),
      loginWithGitHub: vi.fn(),
      loginWithGoogle: vi.fn(),
      loginWithEmail: vi.fn(),
      register: vi.fn(),
    });
    (api.getMyVote as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Not authenticated'));
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

  it('should set error state on 401 APIError without redirecting', async () => {
    // Arrange - API returns 401 APIError
    const { APIError } = await import('@/lib/api-error');
    (api.voteOnPost as ReturnType<typeof vi.fn>).mockRejectedValue(
      new APIError('authentication required', 401)
    );

    // Act
    const { result } = renderHook(() => useVote('post-123', 24));
    await act(async () => {
      await result.current.upvote();
    });

    // Assert - score rolled back, error message set, no redirect
    expect(result.current.score).toBe(24);
    expect(result.current.error).toBe('Login required to vote');
    // Modal will be shown automatically by AuthContext via event system
  });

  it('should track userVote after successful vote', async () => {
    // Arrange - user is authenticated, API returns user_vote
    mockUseAuth.mockReturnValue({
      isAuthenticated: true,
      user: { id: 'user-1', type: 'human', displayName: 'Test User' },
      isLoading: false,
      showAuthModal: false,
      authModalMessage: '',
      setShowAuthModal: vi.fn(),
      setToken: vi.fn(),
      logout: vi.fn(),
      loginWithGitHub: vi.fn(),
      loginWithGoogle: vi.fn(),
      loginWithEmail: vi.fn(),
      register: vi.fn(),
    });
    (api.getMyVote as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Not authenticated'));
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

  it('should use initialUserVote and skip getMyVote call', async () => {
    // Arrange - user is authenticated
    mockUseAuth.mockReturnValue({
      isAuthenticated: true,
      user: { id: 'user-1', type: 'human', displayName: 'Test User' },
      isLoading: false,
      showAuthModal: false,
      authModalMessage: '',
      setShowAuthModal: vi.fn(),
      setToken: vi.fn(),
      logout: vi.fn(),
      loginWithGitHub: vi.fn(),
      loginWithGoogle: vi.fn(),
      loginWithEmail: vi.fn(),
      register: vi.fn(),
    });

    // Act - pass initialUserVote as third arg
    const { result } = renderHook(() => useVote('post-123', 24, 'up'));

    // Assert - getMyVote should NOT be called (vote data provided by parent)
    await waitFor(() => {
      expect(api.getMyVote).not.toHaveBeenCalled();
    });
    // userVote should be 'up' immediately
    expect(result.current.userVote).toBe('up');
  });

  it('should use null initialUserVote and skip getMyVote (user has not voted)', async () => {
    // Arrange - user is authenticated
    mockUseAuth.mockReturnValue({
      isAuthenticated: true,
      user: { id: 'user-1', type: 'human', displayName: 'Test User' },
      isLoading: false,
      showAuthModal: false,
      authModalMessage: '',
      setShowAuthModal: vi.fn(),
      setToken: vi.fn(),
      logout: vi.fn(),
      loginWithGitHub: vi.fn(),
      loginWithGoogle: vi.fn(),
      loginWithEmail: vi.fn(),
      register: vi.fn(),
    });

    // Act - explicitly null means 'no vote, but skip fetch'
    const { result } = renderHook(() => useVote('post-123', 24, null));

    // Assert - getMyVote should NOT be called
    await waitFor(() => {
      expect(api.getMyVote).not.toHaveBeenCalled();
    });
    // userVote should be null
    expect(result.current.userVote).toBeNull();
  });

  it('should call getMyVote when initialUserVote is undefined (backward compat for detail pages)', async () => {
    // Arrange - user is authenticated
    mockUseAuth.mockReturnValue({
      isAuthenticated: true,
      user: { id: 'user-1', type: 'human', displayName: 'Test User' },
      isLoading: false,
      showAuthModal: false,
      authModalMessage: '',
      setShowAuthModal: vi.fn(),
      setToken: vi.fn(),
      logout: vi.fn(),
      loginWithGitHub: vi.fn(),
      loginWithGoogle: vi.fn(),
      loginWithEmail: vi.fn(),
      register: vi.fn(),
    });
    (api.getMyVote as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: { vote: 'down' },
    });

    // Act - no third arg (undefined) = backward compat, should fetch
    const { result } = renderHook(() => useVote('post-123', 24));

    // Assert - getMyVote WAS called
    await waitFor(() => {
      expect(api.getMyVote).toHaveBeenCalledWith('post-123');
      expect(result.current.userVote).toBe('down');
    });
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
