import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';

vi.mock('next/navigation', () => ({
  useSearchParams: () => new URLSearchParams(''),
}));

const getPosts = vi.fn().mockResolvedValue({
  data: [],
  meta: { total: 0, page: 1, per_page: 20, has_more: false },
});
vi.mock('@/lib/api', () => ({
  api: { getPosts: (...args: unknown[]) => getPosts(...args), search: vi.fn() },
  formatRelativeTime: () => '2h ago',
  truncateText: (t: string) => t,
}));

import { PostsPageClient } from './posts-page-client';

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
