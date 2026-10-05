import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';

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

import { PostsPageClient } from './posts-page-client';

beforeEach(() => {
  locationQuery = '';
  getPosts.mockReset();
  search.mockReset();
  getPosts.mockResolvedValue({ data: [], meta: { total: 0, page: 1, per_page: 20, has_more: false } });
  search.mockResolvedValue({
    data: [],
    meta: { query: '', total: 0, page: 1, per_page: 20, has_more: false, took_ms: 1, method: 'hybrid' },
  });
});

describe('PostsPageClient', () => {
  it('offers search and Recent/Top sorting but no type selector or status filter', () => {
    render(<PostsPageClient initialPosts={[]} />);
    expect(screen.getByRole('heading', { name: /^posts$/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/search posts/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /recent/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^top$/i })).toBeInTheDocument();

    // No legacy type selectors or problem-specific status filters.
    expect(screen.queryByRole('button', { name: /problems?/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /ideas?/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /questions?/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^(open|solved|closed|unsolved)$/i })).not.toBeInTheDocument();
  });
});

// The box used to hand every keystroke to the list, which called the search API each
// time, and the API logs each call as a search: typing "deadlock" counted as eight
// searches. The term now reaches the list only after typing pauses for 300 ms.
describe('PostsPageClient search box', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  const box = () => screen.getByLabelText(/search posts/i) as HTMLInputElement;
  const type = (value: string) => fireEvent.change(box(), { target: { value } });
  // Lets the clock run and the list's read settle.
  const wait = (ms: number) => act(async () => { await vi.advanceTimersByTimeAsync(ms); });

  it('searches once when typing settles, never once per keystroke', async () => {
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);
    expect(getPosts).toHaveBeenCalledTimes(1);

    for (const typed of ['d', 'de', 'dea', 'dead', 'deadl', 'deadlo', 'deadloc', 'deadlock']) {
      type(typed);
      await wait(50);
    }
    // What is typed shows at once; nothing was searched while the keys were still going.
    expect(box().value).toBe('deadlock');
    expect(search).not.toHaveBeenCalled();

    await wait(249);
    expect(search).not.toHaveBeenCalled();

    await wait(1);
    expect(search).toHaveBeenCalledTimes(1);
    expect(search.mock.calls[0][0]).toMatchObject({ q: 'deadlock', page: 1 });

    // And it stays at one.
    await wait(2000);
    expect(search).toHaveBeenCalledTimes(1);
    expect(getPosts).toHaveBeenCalledTimes(1);
  });

  it('searches at once for a term a link carried in (/posts?q=term)', async () => {
    locationQuery = 'q=mutex';
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);

    expect(box().value).toBe('mutex');
    expect(search).toHaveBeenCalledTimes(1);
    expect(search.mock.calls[0][0]).toMatchObject({ q: 'mutex' });
    expect(getPosts).not.toHaveBeenCalled();
  });

  it('does not search again for a space typed after the term', async () => {
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);

    type('mutex');
    await wait(300);
    expect(search).toHaveBeenCalledTimes(1);

    type('mutex ');
    await wait(1000);
    expect(search).toHaveBeenCalledTimes(1);
  });

  it('goes back to browsing, after the same pause, when the box is cleared', async () => {
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);
    type('mutex');
    await wait(300);
    expect(getPosts).toHaveBeenCalledTimes(1);

    type('');
    await wait(299);
    expect(getPosts).toHaveBeenCalledTimes(1);
    await wait(1);

    expect(getPosts).toHaveBeenCalledTimes(2);
    expect(search).toHaveBeenCalledTimes(1);
  });

  it('does not delay the Recent/Top switch', async () => {
    render(<PostsPageClient initialPosts={[]} />);
    await wait(0);
    expect(getPosts.mock.calls[0][0]).toMatchObject({ sort: 'new' });

    fireEvent.click(screen.getByRole('button', { name: /^top$/i }));
    await wait(0);

    expect(getPosts).toHaveBeenCalledTimes(2);
    expect(getPosts.mock.calls[1][0]).toMatchObject({ sort: 'top' });
  });
});
