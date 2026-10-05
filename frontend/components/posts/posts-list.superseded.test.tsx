import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';

// A late answer must not overwrite a newer list. Type fast, or change the order while a
// search is still out, and two reads are in flight: the answers can land in either order,
// and the older one landing last used to replace the list the visitor had asked for since.
// Only the newest read may change what is shown: the posts, the "load more" control, the
// error and the loading state. The events were already guarded this way (SPEC.md 27.7).

vi.mock('@/lib/analytics', () => ({ track: vi.fn(), trackOnNextPage: vi.fn() }));

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

import { PostsList } from './posts-list';

const list = (titles: string[], has_more = false) => ({
  data: titles.map((title) => ({ id: title, title })),
  meta: { total: titles.length, page: 1, per_page: 20, has_more },
});

// A read that stays out until the test answers or fails it.
interface Pending {
  answer: (value: unknown) => Promise<void>;
  fail: () => Promise<void>;
}
function pending(mock: ReturnType<typeof vi.fn>): Pending {
  let resolve: (value: unknown) => void = () => {};
  let reject: (reason: unknown) => void = () => {};
  mock.mockReturnValueOnce(
    new Promise((res, rej) => {
      resolve = res;
      reject = rej;
    }),
  );
  return {
    answer: (value) => act(async () => resolve(value)),
    fail: () => act(async () => reject(new Error('down'))),
  };
}

const shown = () => screen.queryAllByTestId('post-card').map((card) => card.textContent);
const settle = () => act(async () => {});

beforeEach(() => {
  getPosts.mockReset();
  search.mockReset();
});

describe('a superseded read of the posts list', () => {
  it('does not replace the list of the term typed after it', async () => {
    getPosts.mockResolvedValue(list(['browsed']));
    const { rerender } = render(<PostsList searchQuery="" sort="new" />);
    await settle();

    const first = pending(search);
    rerender(<PostsList searchQuery="first term" sort="new" />);
    const second = pending(search);
    rerender(<PostsList searchQuery="second term" sort="new" />);
    expect(search).toHaveBeenCalledTimes(2);

    await second.answer(list(['second A', 'second B']));
    expect(shown()).toEqual(['second A', 'second B']);

    // The older search answers last. It asked for a list nobody is waiting for any more.
    await first.answer(list(['first A']));
    expect(shown()).toEqual(['second A', 'second B']);
  });

  it('does not replace the list in the order chosen after it', async () => {
    const recent = pending(getPosts);
    const { rerender } = render(<PostsList searchQuery="" sort="new" />);
    const top = pending(getPosts);
    rerender(<PostsList searchQuery="" sort="top" />);

    await top.answer(list(['top 1', 'top 2']));
    await recent.answer(list(['recent 1']));
    expect(shown()).toEqual(['top 1', 'top 2']);
  });

  it('does not replace a search with the browse list that was still out when the visitor typed', async () => {
    const browse = pending(getPosts);
    const { rerender } = render(<PostsList searchQuery="" sort="new" />);
    const found = pending(search);
    rerender(<PostsList searchQuery="deadlock" sort="new" />);

    await found.answer(list(['deadlock fix']));
    await browse.answer(list(['newest post'], true));
    expect(shown()).toEqual(['deadlock fix']);
    // Nor does its has_more: the search had no second page.
    expect(screen.queryByRole('button', { name: /load more/i })).toBeNull();
  });

  it('does not show its failure over the newer list', async () => {
    getPosts.mockResolvedValue(list(['browsed']));
    const { rerender } = render(<PostsList searchQuery="" sort="new" />);
    await settle();

    const first = pending(search);
    rerender(<PostsList searchQuery="first term" sort="new" />);
    const second = pending(search);
    rerender(<PostsList searchQuery="second term" sort="new" />);

    await second.answer(list(['second A']));
    await first.fail();
    expect(screen.queryByText('Could not load posts.')).toBeNull();
    expect(shown()).toEqual(['second A']);
  });

  it('does not end the wait for the newer read: no empty state while it is still out', async () => {
    const first = pending(search);
    const { rerender } = render(<PostsList searchQuery="first term" sort="new" />);
    const second = pending(search);
    rerender(<PostsList searchQuery="second term" sort="new" />);

    // The superseded search comes back empty while the current one is still out.
    await first.answer(list([]));
    expect(screen.queryByText('No posts found.')).toBeNull();

    await second.answer(list(['second A']));
    expect(shown()).toEqual(['second A']);
  });

  it('does not append its page to the list that replaced the one it was extending', async () => {
    getPosts.mockResolvedValueOnce(list(['recent 1', 'recent 2'], true));
    const { rerender } = render(<PostsList searchQuery="" sort="new" />);
    await settle();
    expect(shown()).toEqual(['recent 1', 'recent 2']);

    // The next page of the recent list is asked for, and before it answers the order changes.
    const more = pending(getPosts);
    fireEvent.click(screen.getByRole('button', { name: /load more/i }));
    const top = pending(getPosts);
    rerender(<PostsList searchQuery="" sort="top" />);

    await top.answer(list(['top 1']));
    await more.answer(list(['recent 3', 'recent 4'], true));
    expect(shown()).toEqual(['top 1']);
    expect(screen.queryByRole('button', { name: /load more/i })).toBeNull();
  });

  it('still shows the newest read when it is the one that answers last', async () => {
    const first = pending(search);
    const { rerender } = render(<PostsList searchQuery="first term" sort="new" />);
    const second = pending(search);
    rerender(<PostsList searchQuery="second term" sort="new" />);

    await first.answer(list(['first A']));
    expect(shown()).toEqual([]);
    await second.answer(list(['second A'], true));
    expect(shown()).toEqual(['second A']);
    expect(screen.getByRole('button', { name: /load more/i })).toBeEnabled();
  });

  it('still adds the next page to the list it extends', async () => {
    getPosts.mockResolvedValueOnce(list(['recent 1'], true));
    render(<PostsList searchQuery="" sort="new" />);
    await settle();

    const more = pending(getPosts);
    fireEvent.click(screen.getByRole('button', { name: /load more/i }));
    await more.answer(list(['recent 2']));
    expect(shown()).toEqual(['recent 1', 'recent 2']);
  });

  it('still shows the failure of the newest read', async () => {
    const first = pending(search);
    const { rerender } = render(<PostsList searchQuery="first term" sort="new" />);
    const second = pending(search);
    rerender(<PostsList searchQuery="second term" sort="new" />);

    await first.answer(list(['first A']));
    await second.fail();
    expect(screen.getByText('Could not load posts.')).toBeInTheDocument();
  });
});
