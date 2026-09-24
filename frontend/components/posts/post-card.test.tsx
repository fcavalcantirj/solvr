import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { PostCard } from './post-card';
import type { APIPost } from '@/lib/api-types';

function makePost(overrides: Partial<APIPost> = {}): APIPost {
  return {
    id: 'post-123',
    type: 'problem',
    title: 'How to debug a deadlock',
    description: 'A long description about a deadlock in the scheduler.',
    status: 'open',
    upvotes: 5,
    downvotes: 1,
    vote_score: 4,
    view_count: 10,
    author: { id: 'user-9', type: 'human', display_name: 'Ada' },
    tags: ['go', 'concurrency'],
    created_at: '2026-09-01T00:00:00Z',
    updated_at: '2026-09-01T00:00:00Z',
    reply_count: 3,
    ...overrides,
  };
}

describe('PostCard', () => {
  it('links the title to the canonical /posts/{id} detail', () => {
    render(<PostCard post={makePost()} />);
    const titleLink = screen.getByRole('link', { name: /how to debug a deadlock/i });
    expect(titleLink).toHaveAttribute('href', '/posts/post-123');
  });

  it('never links to a legacy /problems, /ideas, or /questions detail route', () => {
    const { container } = render(<PostCard post={makePost({ type: 'idea' })} />);
    const hrefs = Array.from(container.querySelectorAll('a')).map((a) => a.getAttribute('href'));
    expect(
      hrefs.some(
        (h) =>
          h?.startsWith('/problems/') ||
          h?.startsWith('/ideas/') ||
          h?.startsWith('/questions/'),
      ),
    ).toBe(false);
  });

  it('shows the author link and the unified reply count from the API', () => {
    render(<PostCard post={makePost({ reply_count: 3 })} />);
    expect(screen.getByRole('link', { name: /ada/i })).toHaveAttribute('href', '/users/user-9');
    expect(screen.getByLabelText(/3 replies/i)).toBeInTheDocument();
  });

  it('renders each tag once', () => {
    render(<PostCard post={makePost()} />);
    expect(screen.getByText('#go')).toBeInTheDocument();
    expect(screen.getByText('#concurrency')).toBeInTheDocument();
  });
});
