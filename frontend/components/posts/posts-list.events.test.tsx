import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';

// What a visitor does with the Posts collection, as Google Analytics events (SPEC.md 27.7):
// search, sort_change and load_more. Each is sent once, after the list it asked for arrived.
// search carries the term the visitor settled on and how many posts matched; it is never
// sent per keystroke, for a term of one character, or for the term a link brought.

const track = vi.hoisted(() => vi.fn());
vi.mock('@/lib/analytics', () => ({ track, trackOnNextPage: vi.fn() }));

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

const post = (id: string) => ({ id, title: `Post ${id}` });
const page = (ids: string[], has_more = false, total = ids.length) => ({
  data: ids.map(post),
  meta: { total, page: 1, per_page: 20, has_more },
});
const found = (total: number, has_more = false) => ({
  data: Array.from({ length: Math.min(total, 20) }, (_, i) => post(`s${i}`)),
  meta: { query: '', total, page: 1, per_page: 20, has_more, took_ms: 1, method: 'hybrid' },
});

const box = () => screen.getByLabelText(/search posts/i) as HTMLInputElement;
const type = (value: string) => fireEvent.change(box(), { target: { value } });
// Lets the clock run and the list's read settle.
const wait = (ms: number) =>
  act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
const eventsNamed = (name: string) => track.mock.calls.filter(([event]) => event === name);

beforeEach(() => {
  vi.useFakeTimers();
  locationQuery = '';
  track.mockReset();
  getPosts.mockReset();
  search.mockReset();
  getPosts.mockResolvedValue(page(['a', 'b']));
  search.mockResolvedValue(found(0));
});

afterEach(() => {
  vi.useRealTimers();
});

describe('opening /posts', () => {
  it('sends nothing: the first list is not something the visitor did', async () => {
    render(<PostsPageClient initialPosts={[]} />);
    await wait(400);
    expect(getPosts).toHaveBeenCalledTimes(1);
    expect(track).not.toHaveBeenCalled();
  });
});

describe('search on /posts', () => {
  it('is sent once when typing settles and the answer arrived, with the term and the number of results', async () => {
    search.mockResolvedValue(found(7));
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);

    for (const typed of ['d', 'de', 'dea', 'dead', 'deadl', 'deadlo', 'deadloc', 'deadlock']) {
      type(typed);
      await wait(50);
    }
    expect(track).not.toHaveBeenCalled();

    await wait(300);
    expect(search).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('search', { search_term: 'deadlock', results: 7, list: 'posts' });
  });

  it('waits for the answer: nothing is sent while the search is still out', async () => {
    let answer: (value: unknown) => void = () => {};
    search.mockReturnValue(new Promise((resolve) => (answer = resolve)));
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);

    type('race condition');
    await wait(400);
    expect(search).toHaveBeenCalledTimes(1);
    expect(track).not.toHaveBeenCalled();

    await act(async () => {
      answer(found(3));
    });
    expect(track).toHaveBeenCalledWith('search', { search_term: 'race condition', results: 3, list: 'posts' });
  });

  it('counts each settled term once: a second term is a second search', async () => {
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);

    search.mockResolvedValue(found(4));
    type('postgres');
    await wait(400);
    search.mockResolvedValue(found(0));
    type('postgres pool');
    await wait(400);

    expect(eventsNamed('search').map(([, params]) => params)).toEqual([
      { search_term: 'postgres', results: 4, list: 'posts' },
      { search_term: 'postgres pool', results: 0, list: 'posts' },
    ]);
  });

  it('is not sent for a term of one character, nor for clearing the box', async () => {
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);

    type('d');
    await wait(400);
    expect(search).toHaveBeenCalledWith(expect.objectContaining({ q: 'd' }));
    type('');
    await wait(400);

    expect(eventsNamed('search')).toEqual([]);
  });

  it('is not sent when the search failed', async () => {
    search.mockRejectedValue(new Error('down'));
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);

    type('deadlock');
    await wait(400);
    expect(screen.getByText('Could not load posts.')).toBeInTheDocument();
    expect(track).not.toHaveBeenCalled();

    // Retry works: the term the visitor settled on got its answer, once.
    search.mockResolvedValue(found(2));
    fireEvent.click(screen.getByRole('button', { name: /retry/i }));
    await wait(0);
    expect(eventsNamed('search').map(([, params]) => params)).toEqual([{ search_term: 'deadlock', results: 2, list: 'posts' }]);
  });

  it('is not sent for the term a link brought, and is for the first term typed after it', async () => {
    locationQuery = 'q=connection+pool';
    search.mockResolvedValue(found(5));
    render(<PostsPageClient initialPosts={[]} />);
    await wait(400);
    expect(search).toHaveBeenCalledWith(expect.objectContaining({ q: 'connection pool' }));
    expect(track).not.toHaveBeenCalled();

    type('connection pool exhausted');
    await wait(400);
    expect(eventsNamed('search').map(([, params]) => params.search_term)).toEqual(['connection pool exhausted']);

    // Typing the link's own term again is the visitor's search now.
    type('connection pool');
    await wait(400);
    expect(eventsNamed('search').map(([, params]) => params.search_term)).toEqual(['connection pool exhausted', 'connection pool']);
  });

  it('ignores the answer of a term that is no longer the one searched for', async () => {
    const answers: Array<(value: unknown) => void> = [];
    search.mockImplementation(() => new Promise((resolve) => answers.push(resolve)));
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);

    type('first term');
    await wait(400);
    type('second term');
    await wait(400);
    expect(answers).toHaveLength(2);

    // The older search answers last.
    await act(async () => {
      answers[1](found(9));
    });
    await act(async () => {
      answers[0](found(1));
    });
    expect(eventsNamed('search').map(([, params]) => params)).toEqual([{ search_term: 'second term', results: 9, list: 'posts' }]);
  });
});

describe('sort_change on /posts', () => {
  it('is sent once the list in the new order arrived', async () => {
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);
    let answer: (value: unknown) => void = () => {};
    getPosts.mockReturnValueOnce(new Promise((resolve) => (answer = resolve)));

    fireEvent.click(screen.getByRole('button', { name: /^top$/i }));
    await wait(0);
    expect(getPosts).toHaveBeenLastCalledWith(expect.objectContaining({ sort: 'top' }));
    expect(track).not.toHaveBeenCalled();

    await act(async () => {
      answer(page(['c']));
    });
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('sort_change', { list: 'posts', sort: 'top' });
  });

  it('is not sent for pressing the order already showing, and says each change once', async () => {
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);

    fireEvent.click(screen.getByRole('button', { name: /recent/i }));
    await wait(0);
    expect(track).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: /^top$/i }));
    await wait(0);
    fireEvent.click(screen.getByRole('button', { name: /^top$/i }));
    await wait(0);
    fireEvent.click(screen.getByRole('button', { name: /recent/i }));
    await wait(0);
    expect(eventsNamed('sort_change').map(([, params]) => params.sort)).toEqual(['top', 'new']);
  });

  it('is not sent when the list could not be loaded', async () => {
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);
    getPosts.mockRejectedValueOnce(new Error('down'));

    fireEvent.click(screen.getByRole('button', { name: /^top$/i }));
    await wait(0);
    expect(screen.getByText('Could not load posts.')).toBeInTheDocument();
    expect(track).not.toHaveBeenCalled();
  });

  it('does not count as a new search while a term is showing', async () => {
    search.mockResolvedValue(found(6));
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);
    type('deadlock');
    await wait(400);
    expect(eventsNamed('search')).toHaveLength(1);

    fireEvent.click(screen.getByRole('button', { name: /^top$/i }));
    await wait(0);
    expect(eventsNamed('search')).toHaveLength(1);
    expect(eventsNamed('sort_change').map(([, params]) => params)).toEqual([{ list: 'posts', sort: 'top' }]);
  });
});

describe('load_more on /posts', () => {
  it('is sent with the page that was added, once it arrived', async () => {
    getPosts.mockResolvedValue(page(['a', 'b'], true));
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);
    expect(track).not.toHaveBeenCalled();

    getPosts.mockResolvedValue({ ...page(['c', 'd'], true), meta: { total: 6, page: 2, per_page: 20, has_more: true } });
    fireEvent.click(screen.getByRole('button', { name: /load more/i }));
    await wait(0);
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('load_more', { list: 'posts', page: 2 });

    fireEvent.click(screen.getByRole('button', { name: /load more/i }));
    await wait(0);
    expect(eventsNamed('load_more').map(([, params]) => params.page)).toEqual([2, 3]);
  });

  it('is not sent when the next page could not be loaded', async () => {
    getPosts.mockResolvedValue(page(['a', 'b'], true));
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);
    getPosts.mockRejectedValueOnce(new Error('down'));

    fireEvent.click(screen.getByRole('button', { name: /load more/i }));
    await wait(0);
    expect(track).not.toHaveBeenCalled();
  });
});
