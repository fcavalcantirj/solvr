import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import type { APIPost, APIReply, APIRoom } from '@/lib/api-types';

const getPost = vi.fn();
const getPostReplies = vi.fn();
const getRelatedRooms = vi.fn();
const recordView = vi.fn();
const postFunnelEvent = vi.fn();
const useAuthMock = vi.fn();

vi.mock('@/lib/api', () => ({
  api: {
    getPost: (...a: unknown[]) => getPost(...a),
    getPostReplies: (...a: unknown[]) => getPostReplies(...a),
    getRelatedRooms: (...a: unknown[]) => getRelatedRooms(...a),
    recordView: (...a: unknown[]) => recordView(...a),
    postFunnelEvent: (...a: unknown[]) => postFunnelEvent(...a),
  },
  formatRelativeTime: () => '2h ago',
}));

vi.mock('@/hooks/use-auth', () => ({ useAuth: () => useAuthMock() }));

import { PostDetail } from './post-detail';

// Who is looking at the page, as useAuth reports it once the stored session was read.
const anonymous = { user: null, isAuthenticated: false, isLoading: false };
const signedInAs = (id: string, type: 'human' | 'agent' = 'human') => ({
  user: { id, type, displayName: 'Someone' },
  isAuthenticated: true,
  isLoading: false,
});

function makePost(over: Partial<APIPost> = {}): APIPost {
  return {
    id: 'p1',
    type: 'post',
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
  recordView.mockReset();
  postFunnelEvent.mockReset();
  useAuthMock.mockReset();
  getPost.mockResolvedValue({ data: makePost() });
  getPostReplies.mockResolvedValue({ data: [], meta: { total: 0, page: 1 } });
  getRelatedRooms.mockResolvedValue({ data: [] });
  recordView.mockResolvedValue({ data: { view_count: 11 } });
  useAuthMock.mockReturnValue(anonymous);
  window.sessionStorage.clear();
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

  // The Edit link used to be on every post page for everyone: 467 crawlable links to
  // noindex editor pages. Only the post's author sees it now, the same comparison the
  // API makes before it accepts an edit (author type and author id).
  describe('edit link', () => {
    it('offers the author an edit link to the canonical edit route', async () => {
      useAuthMock.mockReturnValue(signedInAs('author-9'));
      render(<PostDetail postId="p1" />);
      await waitFor(() => expect(screen.getByRole('link', { name: /edit/i })).toHaveAttribute('href', '/posts/p1/edit'));
    });

    it.each([
      ['an anonymous visitor', anonymous],
      ['a visitor whose session is still being read', { user: null, isAuthenticated: false, isLoading: true }],
      ['another signed-in person', signedInAs('someone-else')],
      ['an agent that shares the id of a human author', signedInAs('author-9', 'agent')],
    ])('offers no edit link to %s', async (_label, auth) => {
      useAuthMock.mockReturnValue(auth);
      const { container } = render(<PostDetail postId="p1" />);
      await waitFor(() => expect(screen.getByText(/body of the post/i)).toBeInTheDocument());
      expect(screen.queryByRole('link', { name: /edit/i })).not.toBeInTheDocument();
      expect(container.querySelector('a[href$="/edit"]')).toBeNull();
    });

    it('offers an agent author its own edit link', async () => {
      getPost.mockResolvedValue({
        data: makePost({ author: { id: 'agent_planner', type: 'agent', display_name: 'planner' } }),
      });
      useAuthMock.mockReturnValue(signedInAs('agent_planner', 'agent'));
      render(<PostDetail postId="p1" />);
      await waitFor(() => expect(screen.getByRole('link', { name: /edit/i })).toHaveAttribute('href', '/posts/p1/edit'));
    });
  });

  // Post views were never recorded by the site (the hook had no caller).
  describe('view and share visit', () => {
    it('records one view once the post is shown', async () => {
      render(<PostDetail postId="p1" />);
      await waitFor(() => expect(recordView).toHaveBeenCalledTimes(1));
      expect(recordView.mock.calls[0][0]).toBe('p1');
    });

    it('records no view of a post that could not be loaded', async () => {
      getPost.mockRejectedValue(new Error('boom'));
      render(<PostDetail postId="p1" />);
      await waitFor(() => expect(screen.getByText(/could not load/i)).toBeInTheDocument());
      await new Promise((resolve) => setTimeout(resolve, 20));
      expect(recordView).not.toHaveBeenCalled();
    });

    it('reports a share visit for a post opened through a share link', async () => {
      window.history.replaceState(null, '', '/posts/p1?via=share');
      render(<PostDetail postId="p1" />);
      await waitFor(() =>
        expect(postFunnelEvent).toHaveBeenCalledWith({
          event: 'share_visit',
          entry_surface: 'post_page',
          source: { kind: 'post', ref: 'p1' },
        }),
      );
      expect(window.location.search).toBe('');
    });
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

  // Task 709 — Preserve saved knowledge and IPFS references as contextual features.
  describe('saved IPFS snapshot (task 709)', () => {
    it('renders the immutable snapshot as an external read-only IPFS link with its timestamp and a live-differs note (steps 1, 2)', async () => {
      getPost.mockResolvedValue({
        data: makePost({ crystallization_cid: 'bafycid123', crystallized_at: '2026-09-05T00:00:00Z' }),
      });
      render(<PostDetail postId="p1" />);
      const link = await screen.findByRole('link', { name: /bafycid123/i });
      // step 1: the snapshot link addresses the CID on the IPFS gateway and opens externally (immutable, read-only).
      expect(link).toHaveAttribute('href', 'https://ipfs.io/ipfs/bafycid123');
      expect(link).toHaveAttribute('target', '_blank');
      expect(link).toHaveAttribute('rel', 'noopener noreferrer');
      // step 1: the snapshot section carries its own timestamp (scoped so the header date is not matched).
      const section = link.closest('section');
      expect(section).not.toBeNull();
      expect(within(section!).getByText(/2h ago/i)).toBeInTheDocument();
      // step 2: it explains the live post may differ from the saved copy.
      expect(within(section!).getByText(/live post may differ/i)).toBeInTheDocument();
    });

    it('shows the snapshot for any canonical post carrying a CID, not only a problem type (step 3)', async () => {
      getPost.mockResolvedValue({ data: makePost({ type: 'post', crystallization_cid: 'bafyIdea' }) });
      render(<PostDetail postId="p1" />);
      expect(await screen.findByRole('link', { name: /bafyIdea/i })).toBeInTheDocument();
    });

    it('omits the snapshot section entirely when the post has no saved copy — never fabricates one (steps 2, 5)', async () => {
      getPost.mockResolvedValue({ data: makePost() }); // no crystallization_cid
      render(<PostDetail postId="p1" />);
      await waitFor(() => expect(screen.getByText(/body of the post/i)).toBeInTheDocument());
      expect(screen.queryByText(/saved snapshot/i)).not.toBeInTheDocument();
    });

    it('presents the snapshot as optional context, never as editable content (step 5)', async () => {
      getPost.mockResolvedValue({ data: makePost({ crystallization_cid: 'bafycid123' }) });
      // The post's author is looking: the one viewer who is offered an Edit link at all.
      useAuthMock.mockReturnValue(signedInAs('author-9'));
      render(<PostDetail postId="p1" />);
      const heading = await screen.findByText(/saved snapshot/i);
      const section = heading.closest('section');
      expect(section).not.toBeNull();
      // The snapshot is a read-only external reference: no edit control or editable field inside it.
      expect(section!.querySelector('button')).toBeNull();
      expect(section!.querySelector('input, textarea, [contenteditable="true"]')).toBeNull();
      // The only Edit affordance targets the LIVE post, not the immutable snapshot.
      expect(screen.getByRole('link', { name: /edit/i })).toHaveAttribute('href', '/posts/p1/edit');
    });
  });

  // idx 88: a post can seed a fresh room ("Try this workflow"). The API decides whether
  // the post is public enough to seed one; the link only names the post.
  it('offers Try this workflow, opening the start flow seeded from this post', async () => {
    render(<PostDetail postId="p1" />);
    const link = await screen.findByRole('link', { name: /try this workflow/i });
    expect(link).toHaveAttribute('href', '/connect?post=p1');
  });
});
