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

  it('leads back to Posts when there is no such user', () => {
    const { container } = render(<UserProfileClient id="u1" />);

    expect(screen.getByText('User not found')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'BACK TO POSTS' })).toHaveAttribute('href', '/posts');
    expect(hrefs(container)).not.toContain('/feed');
  });
});
