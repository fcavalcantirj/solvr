import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { renderToString } from 'react-dom/server';

// The Edit link of a post is decided in the browser, from the session the auth provider
// reads there. The server knows no session, so the HTML it sends (to a visitor, to a
// crawler, and to the author alike) carries no link to /posts/{id}/edit: every post page
// used to carry one, 467 crawlable links to noindex editor pages.
//
// This file renders through the REAL auth provider, not a stand-in for it.

vi.mock('@/lib/api', () => ({
  api: {
    getMe: vi.fn(),
    setAuthToken: vi.fn(),
    clearAuthToken: vi.fn(),
    onAuthError: vi.fn(),
    offAuthError: vi.fn(),
    getPost: vi.fn(),
    getPostReplies: vi.fn(),
    getRelatedRooms: vi.fn(),
    recordView: vi.fn(),
    postFunnelEvent: vi.fn(),
  },
  formatRelativeTime: () => 'just now',
}));
vi.mock('@/components/ui/auth-required-modal', () => ({ AuthRequiredModal: () => null }));

import { api } from '@/lib/api';
import { AuthProvider } from '@/hooks/use-auth';
import { PostDetail, type PostDetailInitial } from './post-detail';

const initial = {
  post: {
    id: 'p1',
    title: 'Hand a plan to an executor',
    description: 'The executor reads the plan.',
    author: { id: 'author-9', type: 'human', display_name: 'Dev Nine' },
    created_at: '2026-09-01T00:00:00Z',
    vote_score: 2,
    reply_count: 0,
    tags: [],
    moderation_state: 'approved',
    publication_state: 'published',
  },
  replies: [],
  rooms: [],
  replyPages: 1,
} as unknown as PostDetailInitial;

const page = (
  <AuthProvider>
    <PostDetail postId="p1" initial={initial} />
  </AuthProvider>
);

const me = (id: string, type: 'human' | 'agent' = 'human') => ({
  data: { id, type, display_name: 'Dev Nine', email: 'dev@example.test' },
});

beforeEach(() => {
  vi.mocked(api.getMe).mockReset();
  vi.mocked(api.recordView).mockReset();
  vi.mocked(api.recordView).mockResolvedValue({ data: { view_count: 1 } });
  window.localStorage.clear();
  window.sessionStorage.clear();
});

describe('PostDetail edit link, through the real auth provider', () => {
  it('is absent from the server HTML, even when the author holds a session', () => {
    window.localStorage.setItem('auth_token', 'jwt-of-the-author');
    vi.mocked(api.getMe).mockResolvedValue(me('author-9') as never);

    const html = renderToString(page);

    // The post itself is in the HTML; the way into its editor is not.
    expect(html).toContain('Hand a plan to an executor');
    expect(html).not.toContain('/posts/p1/edit');
    expect(html).not.toMatch(/>\s*Edit\s*</);
  });

  it('appears for the author in the browser, once the stored session was read', async () => {
    window.localStorage.setItem('auth_token', 'jwt-of-the-author');
    vi.mocked(api.getMe).mockResolvedValue(me('author-9') as never);

    render(page);

    const link = await screen.findByRole('link', { name: /edit/i });
    expect(link).toHaveAttribute('href', '/posts/p1/edit');
  });

  it('never appears for another signed-in person', async () => {
    window.localStorage.setItem('auth_token', 'jwt-of-someone-else');
    vi.mocked(api.getMe).mockResolvedValue(me('someone-else') as never);

    const { container } = render(page);

    // The view goes out only after the session was read: by then the link would be there.
    await waitFor(() => expect(api.recordView).toHaveBeenCalledTimes(1));
    expect(api.getMe).toHaveBeenCalledTimes(1);
    expect(container.querySelector('a[href$="/edit"]')).toBeNull();
  });

  it('never appears for an anonymous visitor', async () => {
    const { container } = render(page);

    await waitFor(() => expect(api.recordView).toHaveBeenCalledTimes(1));
    expect(api.getMe).not.toHaveBeenCalled();
    expect(container.querySelector('a[href$="/edit"]')).toBeNull();
  });
});
