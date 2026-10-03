import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';

// Task idx 81: a post's body and first replies page are in the server HTML, so a
// crawler without JavaScript reads the discussion; later reply pages are ordinary links.

const getPost = vi.fn();
vi.mock('@/lib/api', () => ({
  api: {
    getPost: (...a: unknown[]) => getPost(...a),
    getPostReplies: vi.fn(),
    getRelatedRooms: vi.fn(),
  },
  formatRelativeTime: () => 'just now',
}));

import { PostDetail } from './post-detail';

const post = {
  id: 'p1',
  title: 'Hand a plan to an executor',
  description: 'The **executor** reads the plan.',
  author: { id: 'agent_planner', type: 'agent', display_name: 'planner' },
  created_at: '2026-09-01T00:00:00Z',
  vote_score: 2,
  reply_count: 150,
  tags: [],
  moderation_state: 'approved',
  publication_state: 'published',
};
const reply = (i: number) => ({
  id: `r${i}`,
  body: `reply body ${i}`,
  author: { id: 'agent_executor', type: 'agent', display_name: 'executor' },
  created_at: '2026-09-01T00:00:00Z',
});

beforeEach(() => getPost.mockReset());

describe('PostDetail with server data', () => {
  it('renders the post and its first replies without fetching', () => {
    render(
      <PostDetail
        postId="p1"
        initial={{
          post: post as never,
          replies: [reply(1), reply(2)] as never,
          rooms: [],
          replyPages: 1,
        }}
      />
    );
    expect(screen.getByRole('heading', { level: 1 }).textContent).toBe('Hand a plan to an executor');
    expect(screen.getByText('reply body 2')).toBeTruthy();
    expect(screen.queryByText('Loading…')).toBeNull();
    expect(getPost).not.toHaveBeenCalled();
  });

  it('links the later reply pages with ordinary anchors', () => {
    const { container } = render(
      <PostDetail
        postId="p1"
        initial={{ post: post as never, replies: [reply(1)] as never, rooms: [], replyPages: 2 }}
      />
    );
    const hrefs = [...container.querySelectorAll('a')].map((a) => a.getAttribute('href'));
    expect(hrefs).toContain('/posts/p1/replies/2');
  });
});
