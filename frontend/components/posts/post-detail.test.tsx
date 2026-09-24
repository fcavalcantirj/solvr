import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import type { APIPost, APIReply, APIRoom } from '@/lib/api-types';

const getPost = vi.fn();
const getPostReplies = vi.fn();
const getRelatedRooms = vi.fn();

vi.mock('@/lib/api', () => ({
  api: {
    getPost: (...a: unknown[]) => getPost(...a),
    getPostReplies: (...a: unknown[]) => getPostReplies(...a),
    getRelatedRooms: (...a: unknown[]) => getRelatedRooms(...a),
  },
  formatRelativeTime: () => '2h ago',
}));

import { PostDetail } from './post-detail';

function makePost(over: Partial<APIPost> = {}): APIPost {
  return {
    id: 'p1',
    type: 'problem',
    title: 'How to fix the deadlock',
    description: 'This is the body of the post.',
    status: 'open',
    upvotes: 3,
    downvotes: 0,
    vote_score: 3,
    view_count: 10,
    author: { id: 'author-9', type: 'human', display_name: 'Dev Nine' },
    tags: ['golang', 'concurrency'],
    created_at: '2026-09-01T00:00:00Z',
    updated_at: '2026-09-01T00:00:00Z',
    moderation_state: 'approved',
    publication_state: 'published',
    ...over,
  };
}

const reply: APIReply = {
  id: 'r1',
  post_id: 'p1',
  author: { id: 'a1', type: 'agent', display_name: 'Helper Bot' },
  body: 'Try a mutex ordering rule.',
  score: 2,
  upvotes: 2,
  downvotes: 0,
  created_at: '2026-09-02T00:00:00Z',
  updated_at: '2026-09-02T00:00:00Z',
};

const relatedRoom: APIRoom = {
  id: 'room-1',
  slug: 'deadlock-collab',
  display_name: 'Deadlock collab',
  tags: [],
  is_private: false,
  message_count: 4,
  created_at: '2026-09-03T00:00:00Z',
  updated_at: '2026-09-03T00:00:00Z',
  last_active_at: '2026-09-03T00:00:00Z',
};

beforeEach(() => {
  getPost.mockReset();
  getPostReplies.mockReset();
  getRelatedRooms.mockReset();
  getPost.mockResolvedValue({ data: makePost() });
  getPostReplies.mockResolvedValue({ data: [], meta: { total: 0, page: 1 } });
  getRelatedRooms.mockResolvedValue({ data: [] });
});

describe('PostDetail', () => {
  it('shows body, author link, date, vote score, and tags', async () => {
    render(<PostDetail postId="p1" />);
    await waitFor(() => expect(screen.getByText(/body of the post/i)).toBeInTheDocument());
    expect(screen.getByRole('link', { name: /dev nine/i })).toHaveAttribute('href', '/users/author-9');
    expect(screen.getByText(/2h ago/i)).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /golang/i })).toBeInTheDocument();
  });

  it('lists replies fetched from the canonical replies endpoint', async () => {
    getPostReplies.mockResolvedValue({ data: [reply], meta: { total: 1, page: 1 } });
    render(<PostDetail postId="p1" />);
    await waitFor(() => expect(screen.getByText(/mutex ordering rule/i)).toBeInTheDocument());
    expect(screen.getByText(/helper bot/i)).toBeInTheDocument();
  });

  it('shows a related room link and a saved snapshot link when present', async () => {
    getRelatedRooms.mockResolvedValue({ data: [relatedRoom] });
    getPost.mockResolvedValue({ data: makePost({ crystallization_cid: 'bafycid123' }) });
    render(<PostDetail postId="p1" />);
    await waitFor(() => expect(screen.getByRole('link', { name: /deadlock collab/i })).toHaveAttribute('href', '/rooms/deadlock-collab'));
    expect(screen.getByText(/snapshot/i)).toBeInTheDocument();
  });

  it('warns that public discovery waits for approved moderation', async () => {
    getPost.mockResolvedValue({ data: makePost({ moderation_state: 'pending', publication_state: 'draft' }) });
    render(<PostDetail postId="p1" />);
    await waitFor(() => expect(screen.getByText(/moderation/i)).toBeInTheDocument());
  });

  it('offers an edit link to the canonical edit route', async () => {
    render(<PostDetail postId="p1" />);
    await waitFor(() => expect(screen.getByRole('link', { name: /edit/i })).toHaveAttribute('href', '/posts/p1/edit'));
  });

  it('gives each reply a stable element id so legacy deep links can anchor to it', async () => {
    getPostReplies.mockResolvedValue({ data: [reply], meta: { total: 1, page: 1 } });
    const { container } = render(<PostDetail postId="p1" />);
    await waitFor(() => expect(screen.getByText(/mutex ordering rule/i)).toBeInTheDocument());
    expect(container.querySelector('#r1')).not.toBeNull();
  });

  it('shows an error state with retry when the post cannot load', async () => {
    getPost.mockRejectedValue(new Error('boom'));
    render(<PostDetail postId="p1" />);
    await waitFor(() => expect(screen.getByText(/could not load/i)).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });
});
