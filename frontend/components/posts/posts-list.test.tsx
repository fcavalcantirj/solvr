import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import type { APIPost } from '@/lib/api-types';

const getPosts = vi.fn();
const search = vi.fn();

vi.mock('@/lib/api', () => ({
  api: {
    getPosts: (...args: unknown[]) => getPosts(...args),
    search: (...args: unknown[]) => search(...args),
  },
  formatRelativeTime: () => '2h ago',
  truncateText: (t: string) => t,
}));

import { PostsList } from './posts-list';

function post(id: string, type: APIPost['type'], title: string, reply_count: number): APIPost {
  return {
    id,
    type,
    title,
    description: 'body',
    status: 'open',
    upvotes: 0,
    downvotes: 0,
    vote_score: 1,
    view_count: 0,
    author: { id: `u-${id}`, type: 'human', display_name: 'A' },
    tags: [],
    created_at: '2026-09-01T00:00:00Z',
    updated_at: '2026-09-01T00:00:00Z',
    reply_count,
  };
}

const mixed = [
  post('p1', 'problem', 'Prob one', 2),
  post('i1', 'idea', 'Idea one', 0),
  post('q1', 'question', 'Question one', 5),
];

beforeEach(() => {
  getPosts.mockReset();
  search.mockReset();
});

describe('PostsList', () => {
  it('renders every post type through one uniform card set linking to /posts/{id}', async () => {
    getPosts.mockResolvedValue({ data: mixed, meta: { total: 3, page: 1, per_page: 20, has_more: false } });
    const { container } = render(<PostsList searchQuery="" sort="new" />);
    await waitFor(() => expect(screen.getByRole('link', { name: /prob one/i })).toBeInTheDocument());
    expect(screen.getByRole('link', { name: /idea one/i })).toHaveAttribute('href', '/posts/i1');
    expect(screen.getByRole('link', { name: /question one/i })).toHaveAttribute('href', '/posts/q1');
    const hrefs = Array.from(container.querySelectorAll('a')).map((a) => a.getAttribute('href'));
    expect(
      hrefs.some(
        (h) => h?.startsWith('/problems/') || h?.startsWith('/ideas/') || h?.startsWith('/questions/'),
      ),
    ).toBe(false);
  });

  it('uses the search endpoint when a query is present', async () => {
    search.mockResolvedValue({
      data: [{ ...mixed[0], snippet: 's', score: 1 }],
      meta: { query: 'x', total: 1, page: 1, per_page: 20, has_more: false, took_ms: 1, method: 'hybrid' },
    });
    render(<PostsList searchQuery="deadlock" sort="new" />);
    await waitFor(() => expect(search).toHaveBeenCalled());
    expect(getPosts).not.toHaveBeenCalled();
    expect(search.mock.calls[0][0]).toMatchObject({ q: 'deadlock' });
  });

  it("shows each search result's matching replies on its card", async () => {
    search.mockResolvedValue({
      data: [
        {
          ...mixed[2],
          snippet: 's',
          score: 1,
          matched_replies: [
            {
              id: 'r9',
              post_id: 'q1',
              url: '/posts/q1#r9',
              snippet: 'use a <mark>mutex</mark>',
              author: { id: 'a9', type: 'agent', display_name: 'Helper' },
              score: 0.5,
              created_at: '2026-09-01T00:00:00Z',
            },
          ],
        },
      ],
      meta: { query: 'mutex', total: 1, page: 1, per_page: 20, has_more: false, took_ms: 1, method: 'fulltext' },
    });
    render(<PostsList searchQuery="mutex" sort="new" />);
    await waitFor(() => expect(screen.getByRole('link', { name: /use a mutex/i })).toHaveAttribute('href', '/posts/q1#r9'));
    expect(screen.getByLabelText(/5 replies/i)).toBeInTheDocument();
  });

  it('loads more pages and appends without dropping earlier posts', async () => {
    getPosts
      .mockResolvedValueOnce({ data: [mixed[0]], meta: { total: 2, page: 1, per_page: 1, has_more: true } })
      .mockResolvedValueOnce({ data: [mixed[1]], meta: { total: 2, page: 2, per_page: 1, has_more: false } });
    render(<PostsList searchQuery="" sort="new" />);
    await waitFor(() => expect(screen.getByRole('link', { name: /prob one/i })).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /load more/i }));
    await waitFor(() => expect(screen.getByRole('link', { name: /idea one/i })).toBeInTheDocument());
    expect(screen.getByRole('link', { name: /prob one/i })).toBeInTheDocument();
  });

  it('shows an empty state when there are no posts', async () => {
    getPosts.mockResolvedValue({ data: [], meta: { total: 0, page: 1, per_page: 20, has_more: false } });
    render(<PostsList searchQuery="" sort="new" />);
    await waitFor(() => expect(screen.getByText(/no posts found/i)).toBeInTheDocument());
  });

  it('shows an error state with a retry control when the request fails', async () => {
    getPosts.mockRejectedValue(new Error('boom'));
    render(<PostsList searchQuery="" sort="new" />);
    await waitFor(() => expect(screen.getByText(/could not load posts/i)).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });
});
