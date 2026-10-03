import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { PostCard } from './post-card';
import type { APIPost, APISearchReplyMatch } from '@/lib/api-types';

function makePost(overrides: Partial<APIPost> = {}): APIPost {
  return {
    id: 'post-123',
    type: 'post',
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
    const { container } = render(<PostCard post={makePost({ type: 'post' })} />);
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

  // Task idx 53: a search result found through its replies lists them as anchors. The card
  // renders what the API returns: the anchor link, the excerpt with the matched words the API
  // marked, the original author and time, and the origin of a migrated contribution.
  it('lists the replies that matched a search, each linking to its anchor', () => {
    const match: APISearchReplyMatch = {
      id: 'reply-1',
      post_id: 'post-123',
      url: '/posts/post-123#reply-1',
      snippet: 'the scheduler <mark>deadlock</mark> went away after pinning',
      author: { id: 'agent-7', type: 'agent', display_name: 'Fixer Bot' },
      legacy_type: 'approach',
      legacy_status: 'succeeded',
      score: 0.4,
      created_at: '2026-09-02T00:00:00Z',
    };
    const { container } = render(<PostCard post={makePost()} matchedReplies={[match]} />);
    const list = screen.getByRole('list', { name: /matching replies/i });
    const anchor = within(list).getByRole('link');
    expect(anchor).toHaveAttribute('href', '/posts/post-123#reply-1');
    const marks = container.querySelectorAll('mark');
    expect(marks).toHaveLength(1);
    expect(marks[0]).toHaveTextContent('deadlock');
    expect(anchor).toHaveTextContent('the scheduler deadlock went away after pinning');
    expect(container.textContent).not.toContain('<mark>');
    expect(within(list).getByText(/fixer bot/i)).toBeInTheDocument();
    expect(within(list).getByText(/approach succeeded/i)).toBeInTheDocument();
  });

  it('renders an excerpt as text, never as HTML', () => {
    const match: APISearchReplyMatch = {
      id: 'reply-2',
      post_id: 'post-123',
      url: '/posts/post-123#reply-2',
      snippet: '<img src=x onerror=alert(1)> <mark>deadlock</mark>',
      author: { id: 'user-2', type: 'human', display_name: 'Bea' },
      score: 0.1,
      created_at: '2026-09-02T00:00:00Z',
    };
    const { container } = render(<PostCard post={makePost()} matchedReplies={[match]} />);
    expect(container.querySelector('img')).toBeNull();
    expect(container.textContent).toContain('<img src=x onerror=alert(1)>');
    expect(screen.queryByText(/approach/i)).toBeNull();
  });

  it('shows no reply list for a post that matched by its own text', () => {
    render(<PostCard post={makePost()} />);
    expect(screen.queryByRole('list', { name: /matching replies/i })).toBeNull();
  });

  it('renders each tag once', () => {
    render(<PostCard post={makePost()} />);
    expect(screen.getByText('#go')).toBeInTheDocument();
    expect(screen.getByText('#concurrency')).toBeInTheDocument();
  });
});
