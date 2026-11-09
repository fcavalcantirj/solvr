import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';

// A profile that cannot be shown offers a way back. It pointed at /feed, a page retired with
// the typed lists (it answers 404): the way back is the Posts collection.

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const state: { user: unknown; posts: unknown[]; loading: boolean; error: string | null } = {
  user: null,
  posts: [],
  loading: false,
  error: null,
};
vi.mock('@/hooks/use-user', () => ({ useUser: () => state }));
vi.mock('@/lib/api', () => ({
  api: { getUserAgents: vi.fn().mockResolvedValue({ data: [] }) },
  truncateText: (text: string) => text,
}));
vi.mock('@/components/users/user-posts-list', () => ({ UserPostsList: () => null }));
vi.mock('@/components/users/contributions-list', () => ({ ContributionsList: () => null }));
vi.mock('@/components/follow-button', () => ({ FollowButton: () => null }));
vi.mock('@/components/badges-display', () => ({ BadgesDisplay: () => null }));

import { UserProfileClient } from './user-profile-client';

const hrefs = (container: HTMLElement) => [...container.querySelectorAll('a')].map((a) => a.getAttribute('href'));

beforeEach(() => {
  state.user = null;
  state.posts = [];
  state.loading = false;
  state.error = null;
});

describe('a user profile that cannot be shown', () => {
  it('leads back to Posts when the profile failed to load', () => {
    state.error = 'Failed to fetch user';
    const { container } = render(<UserProfileClient id="u1" />);

    expect(screen.getByText('Failed to load profile')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'BACK TO POSTS' })).toHaveAttribute('href', '/posts');
    expect(hrefs(container)).not.toContain('/feed');
  });

  // Search Console reported a profile as a Soft 404: when the browser's posts read fails during
  // Google's render, the whole profile was replaced by "Failed to load profile" although the server
  // HTML already carried the person. The profile stays; only the posts section says it could not load.
  it('keeps the server-rendered profile when only the posts read failed', () => {
    state.user = {
      id: 'u1',
      username: 'listed',
      displayName: 'Listed User',
      bio: 'Bio',
      avatarUrl: null,
      stats: { postsCreated: 3, contributions: 2, reputation: 10 },
    };
    state.error = 'Failed to fetch user';
    render(<UserProfileClient id="u1" initialUserData={{ id: 'u1' }} />);

    expect(screen.getByText('Listed User')).toBeInTheDocument();
    expect(screen.queryByText('Failed to load profile')).not.toBeInTheDocument();
    expect(screen.getByText(/posts could not be loaded/i)).toBeInTheDocument();
  });

  it('leads back to Posts when there is no such user', () => {
    const { container } = render(<UserProfileClient id="u1" />);

    expect(screen.getByText('User not found')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'BACK TO POSTS' })).toHaveAttribute('href', '/posts');
    expect(hrefs(container)).not.toContain('/feed');
  });
});
