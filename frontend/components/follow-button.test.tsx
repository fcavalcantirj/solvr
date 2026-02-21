import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock useAuth hook
let mockUser: { id: string; type: 'human' | 'agent' } | null = { id: 'user-1', type: 'human' };
let mockIsAuthenticated = true;

vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => ({
    user: mockUser,
    isAuthenticated: mockIsAuthenticated,
    isLoading: false,
  }),
}));

// Mock api methods
const mockFollow = vi.fn().mockResolvedValue({ id: 'follow-1', follower_type: 'human', follower_id: 'user-1', followed_type: 'agent', followed_id: 'agent-1', created_at: '2026-01-01T00:00:00Z' });
const mockUnfollow = vi.fn().mockResolvedValue({ status: 'unfollowed' });
const mockIsFollowing = vi.fn().mockResolvedValue(false);

vi.mock('@/lib/api', () => ({
  api: {
    follow: (...args: unknown[]) => mockFollow(...args),
    unfollow: (...args: unknown[]) => mockUnfollow(...args),
    isFollowing: (...args: unknown[]) => mockIsFollowing(...args),
  },
}));

import { FollowButton } from './follow-button';

describe('FollowButton', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUser = { id: 'user-1', type: 'human' };
    mockIsAuthenticated = true;
    mockIsFollowing.mockResolvedValue(false);
  });

  it('renders Follow button for unfollowed entity', async () => {
    mockIsFollowing.mockResolvedValue(false);
    render(<FollowButton targetType="agent" targetId="agent-1" />);

    await waitFor(() => {
      expect(screen.getByRole('button')).toHaveTextContent('FOLLOW');
    });
    expect(mockIsFollowing).toHaveBeenCalledWith('agent', 'agent-1');
  });

  it('renders Following button for already followed entity', async () => {
    mockIsFollowing.mockResolvedValue(true);
    render(<FollowButton targetType="agent" targetId="agent-1" />);

    await waitFor(() => {
      expect(screen.getByRole('button')).toHaveTextContent('FOLLOWING');
    });
  });

  it('clicking Follow calls API and shows Following', async () => {
    mockIsFollowing.mockResolvedValue(false);
    render(<FollowButton targetType="agent" targetId="agent-1" />);

    await waitFor(() => {
      expect(screen.getByRole('button')).toHaveTextContent('FOLLOW');
    });

    fireEvent.click(screen.getByRole('button'));

    // Optimistic UI: should immediately show FOLLOWING
    expect(screen.getByRole('button')).toHaveTextContent('FOLLOWING');
    expect(mockFollow).toHaveBeenCalledWith('agent', 'agent-1');
  });

  it('clicking Following calls unfollow API and shows Follow', async () => {
    mockIsFollowing.mockResolvedValue(true);
    render(<FollowButton targetType="agent" targetId="agent-1" />);

    await waitFor(() => {
      expect(screen.getByRole('button')).toHaveTextContent('FOLLOWING');
    });

    fireEvent.click(screen.getByRole('button'));

    // Optimistic UI: should immediately show FOLLOW
    expect(screen.getByRole('button')).toHaveTextContent('FOLLOW');
    expect(mockUnfollow).toHaveBeenCalledWith('agent', 'agent-1');
  });

  it('hides button when viewing own profile (human)', async () => {
    mockUser = { id: 'user-1', type: 'human' };
    const { container } = render(
      <FollowButton targetType="human" targetId="user-1" />
    );

    // Should render nothing for own profile
    await waitFor(() => {
      expect(container.querySelector('button')).toBeNull();
    });
  });

  it('hides button when viewing own profile (agent)', async () => {
    mockUser = { id: 'agent-1', type: 'agent' };
    const { container } = render(
      <FollowButton targetType="agent" targetId="agent-1" />
    );

    await waitFor(() => {
      expect(container.querySelector('button')).toBeNull();
    });
  });

  it('hides button when not authenticated', async () => {
    mockUser = null;
    mockIsAuthenticated = false;
    const { container } = render(
      <FollowButton targetType="agent" targetId="agent-1" />
    );

    await waitFor(() => {
      expect(container.querySelector('button')).toBeNull();
    });
  });

  it('reverts to Follow on follow API error', async () => {
    mockIsFollowing.mockResolvedValue(false);
    mockFollow.mockRejectedValue(new Error('Network error'));
    render(<FollowButton targetType="agent" targetId="agent-1" />);

    await waitFor(() => {
      expect(screen.getByRole('button')).toHaveTextContent('FOLLOW');
    });

    fireEvent.click(screen.getByRole('button'));

    // Optimistic: shows FOLLOWING immediately
    expect(screen.getByRole('button')).toHaveTextContent('FOLLOWING');

    // After API failure, reverts to FOLLOW
    await waitFor(() => {
      expect(screen.getByRole('button')).toHaveTextContent('FOLLOW');
    });
  });

  it('reverts to Following on unfollow API error', async () => {
    mockIsFollowing.mockResolvedValue(true);
    mockUnfollow.mockRejectedValue(new Error('Network error'));
    render(<FollowButton targetType="agent" targetId="agent-1" />);

    await waitFor(() => {
      expect(screen.getByRole('button')).toHaveTextContent('FOLLOWING');
    });

    fireEvent.click(screen.getByRole('button'));

    // Optimistic: shows FOLLOW immediately
    expect(screen.getByRole('button')).toHaveTextContent('FOLLOW');

    // After API failure, reverts to FOLLOWING
    await waitFor(() => {
      expect(screen.getByRole('button')).toHaveTextContent('FOLLOWING');
    });
  });
});
