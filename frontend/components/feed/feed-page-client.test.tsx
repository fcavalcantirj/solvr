import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { FeedPageClient } from './feed-page-client';

// The destination of a search link.
//
// The homepage publishes canonical Posts search links as /posts?q=<term>, and
// /posts resolves to this collection today. A link that lands here and then
// ignores its own query parameter is a dead link with a 200 status, so the
// collection reads q= from the URL and searches with it. No credential is
// involved: an anonymous visitor follows the link and gets results.

const searchParams = { value: new URLSearchParams() };

vi.mock('next/navigation', () => ({
  useSearchParams: () => searchParams.value,
}));

vi.mock('@/hooks/use-stats', () => ({
  useStats: () => ({ stats: null }),
}));

const feedListProps: Record<string, unknown>[] = [];

vi.mock('./feed-list', () => ({
  FeedList: (props: Record<string, unknown>) => {
    feedListProps.push(props);
    return <div data-testid="feed-list">{String(props.searchQuery ?? '')}</div>;
  },
}));

vi.mock('./feed-sidebar', () => ({
  FeedSidebar: () => <div data-testid="feed-sidebar" />,
}));

const filterProps: Record<string, unknown>[] = [];

vi.mock('./feed-filters', () => ({
  FeedFilters: (props: Record<string, unknown>) => {
    filterProps.push(props);
    return (
      <div>
        <button onClick={() => (props.onFiltersChange as (f: unknown) => void)({ searchQuery: 'typed by hand' })}>
          change filters
        </button>
        <button onClick={props.onToggleSidebar as () => void}>toggle sidebar</button>
        <button onClick={props.onToggleFilters as () => void}>toggle filters</button>
      </div>
    );
  },
}));

beforeEach(() => {
  feedListProps.length = 0;
  filterProps.length = 0;
  searchParams.value = new URLSearchParams();
});

describe('FeedPageClient', () => {
  it('searches with the q parameter the link carried', () => {
    searchParams.value = new URLSearchParams('q=postgres connection pool');
    render(<FeedPageClient initialPosts={[]} />);

    expect(screen.getByTestId('feed-list')).toHaveTextContent('postgres connection pool');
    expect(feedListProps[0].searchQuery).toBe('postgres connection pool');
  });

  it('applies no legacy type filter when the link carried only a query', () => {
    searchParams.value = new URLSearchParams('q=vitest mock hoisting');
    render(<FeedPageClient initialPosts={[]} />);

    expect(feedListProps[0].type).toBe('all');
  });

  it('shows the whole collection when no query was passed', () => {
    render(<FeedPageClient initialPosts={[]} />);
    expect(feedListProps[0].searchQuery).toBe('');
  });

  it('trims a padded query rather than searching for whitespace', () => {
    searchParams.value = new URLSearchParams('q=  rate limit 429  ');
    render(<FeedPageClient initialPosts={[]} />);
    expect(feedListProps[0].searchQuery).toBe('rate limit 429');
  });

  it('lets the visitor replace the query the link carried', () => {
    searchParams.value = new URLSearchParams('q=from the link');
    render(<FeedPageClient initialPosts={[]} />);
    expect(feedListProps[0].searchQuery).toBe('from the link');

    fireEvent.click(screen.getByRole('button', { name: 'change filters' }));
    expect(feedListProps[feedListProps.length - 1].searchQuery).toBe('typed by hand');
  });

  it('opens and closes the mobile sidebar', () => {
    render(<FeedPageClient initialPosts={[]} />);
    expect(screen.queryByRole('button', { name: 'CLOSE' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'toggle sidebar' }));
    const close = screen.getByRole('button', { name: 'CLOSE' });
    expect(close).toBeInTheDocument();

    fireEvent.click(close);
    expect(screen.queryByRole('button', { name: 'CLOSE' })).not.toBeInTheDocument();
  });

  it('toggles the filter panel through the API the filters bar was given', () => {
    render(<FeedPageClient initialPosts={[]} />);
    expect(filterProps[0].showFilters).toBe(false);

    fireEvent.click(screen.getByRole('button', { name: 'toggle filters' }));
    expect(filterProps[filterProps.length - 1].showFilters).toBe(true);
  });
});
