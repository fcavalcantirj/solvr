import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';

// The search says how its term was entered (SPEC.md 27.6): link for the term the page opened
// with, until the visitor types; typed for a term the visitor typed. The API stores it and
// counts a link-brought term (the tag links a crawler walks) as no search. The list judges
// nothing else: it only says where the term came from.

vi.mock('@/lib/analytics', () => ({ track: vi.fn(), trackOnNextPage: vi.fn() }));

let locationQuery = '';
vi.mock('next/navigation', () => ({
  useSearchParams: () => new URLSearchParams(locationQuery),
}));

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
vi.mock('./post-card', () => ({
  PostCard: ({ post }: { post: { id: string; title: string } }) => <div data-testid="post-card">{post.title}</div>,
}));

import { PostsPageClient } from './posts-page-client';

const found = (total: number, has_more = false) => ({
  data: Array.from({ length: Math.min(total, 20) }, (_, i) => ({ id: `s${i}`, title: `Post s${i}` })),
  meta: { query: '', total, page: 1, per_page: 20, has_more, took_ms: 1, method: 'hybrid' },
});
const type = (value: string) =>
  fireEvent.change(screen.getByLabelText(/search posts/i) as HTMLInputElement, { target: { value } });
const wait = (ms: number) =>
  act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
const entries = () => search.mock.calls.map(([params]) => [params.q, params.entry]);

beforeEach(() => {
  vi.useFakeTimers();
  locationQuery = '';
  getPosts.mockReset();
  search.mockReset();
  getPosts.mockResolvedValue({ data: [], meta: { total: 0, page: 1, per_page: 20, has_more: false } });
  search.mockResolvedValue(found(3));
});

afterEach(() => {
  vi.useRealTimers();
});

describe('the entry of a search', () => {
  it('is link for the term the page opened with, and typed once the visitor types', async () => {
    locationQuery = 'q=scraping';
    render(<PostsPageClient initialPosts={[]} />);
    await wait(400);
    type('scraping proxies');
    await wait(400);
    // The link's own term, typed again, is the visitor's.
    type('scraping');
    await wait(400);
    expect(entries()).toEqual([
      ['scraping', 'link'],
      ['scraping proxies', 'typed'],
      ['scraping', 'typed'],
    ]);
  });

  it('is typed for a term typed on a page that opened without one', async () => {
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);
    type('deadlock');
    await wait(400);
    expect(entries()).toEqual([['deadlock', 'typed']]);
  });

  it('keeps the entry of the search a next page extends', async () => {
    locationQuery = 'q=symlink';
    search.mockResolvedValue(found(25, true));
    render(<PostsPageClient initialPosts={[]} />);
    await wait(400);
    fireEvent.click(screen.getByRole('button', { name: /load more/i }));
    await wait(0);
    expect(entries()).toEqual([
      ['symlink', 'link'],
      ['symlink', 'link'],
    ]);
  });
});
