import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';

// The related posts the page read (app/posts/[id]/page.tsx) are plain links in the detail
// view, so the server HTML of every tagged post links other posts (SEO audit 2026-11-08).

vi.mock('@/lib/api', () => ({
  api: {
    getPost: vi.fn(),
    getPostReplies: vi.fn(),
    getRelatedRooms: vi.fn(),
    recordView: vi.fn().mockResolvedValue({ data: { view_count: 1 } }),
    postFunnelEvent: vi.fn(),
  },
  formatRelativeTime: () => '2h ago',
}));
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => ({ user: null, isAuthenticated: false, isLoading: false }),
}));

import { PostDetail } from './post-detail';

const author = { id: 'agent_planner', type: 'agent', display_name: 'planner' };
const post = {
  id: 'p1', title: 'Boot loop on an ESP32-S3', description: 'What fixed it.', author,
  created_at: '2026-09-01T00:00:00Z', vote_score: 2, reply_count: 0, tags: ['esp32'],
  moderation_state: 'approved', publication_state: 'published',
};
const related = [
  { id: 'p2', title: 'Deep sleep drains the battery', created_at: '2026-09-01T00:00:00Z', author, tags: ['esp32'] },
  { id: 'p3', title: 'USB serial goes mute after suspend', created_at: '2026-09-01T00:00:00Z', author, tags: ['esp32'] },
];

describe('PostDetail related posts', () => {
  it('links each related post by its title', () => {
    render(
      <PostDetail
        postId="p1"
        initial={{ post: post as never, replies: [], rooms: [], replyPages: 1 }}
        relatedPosts={related as never}
      />
    );
    const section = screen.getByRole('heading', { name: /related posts/i }).closest('section')!;
    const links = within(section).getAllByRole('link');
    expect(links.map((a) => [a.textContent, a.getAttribute('href')])).toEqual([
      ['Deep sleep drains the battery', '/posts/p2'],
      ['USB serial goes mute after suspend', '/posts/p3'],
    ]);
  });

  it('shows no related section when there is nothing related', () => {
    render(<PostDetail postId="p1" initial={{ post: post as never, replies: [], rooms: [], replyPages: 1 }} relatedPosts={[]} />);
    expect(screen.queryByRole('heading', { name: /related posts/i })).toBeNull();
  });
});
