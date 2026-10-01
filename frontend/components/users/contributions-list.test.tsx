import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ContributionsList } from './contributions-list';

// Mock next/link
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

// Mock the useContributions hook
vi.mock('@/hooks/use-contributions', () => ({
  useContributions: vi.fn(),
}));

import { useContributions } from '@/hooks/use-contributions';

// idx 73 step 3: the profile's contributions are the user's replies (GET /v1/replies), each
// linking to the reply on its post.
const mockContributions = [
  {
    id: 'reply-3',
    postId: 'post-1',
    postTitle: 'How to drain a Go server?',
    legacyType: null,
    body: 'Use http.Server.Shutdown with a context deadline.',
    timestamp: '1d ago',
    createdAt: '2026-02-11T10:00:00Z',
  },
  {
    id: 'reply-2',
    postId: 'post-2',
    postTitle: 'How to use React hooks?',
    legacyType: 'answer',
    body: 'You can use useState and useEffect...',
    timestamp: '2d ago',
    createdAt: '2026-02-10T10:00:00Z',
  },
  {
    id: 'reply-1',
    postId: 'post-3',
    postTitle: 'Fix async race condition',
    legacyType: 'approach',
    body: 'Use mutex locks to prevent...',
    timestamp: '3d ago',
    createdAt: '2026-02-09T10:00:00Z',
  },
];

function hookResult(overrides: Partial<ReturnType<typeof useContributions>> = {}) {
  return {
    contributions: mockContributions,
    loading: false,
    error: null,
    total: 3,
    hasMore: false,
    loadMore: vi.fn(),
    ...overrides,
  };
}

describe('ContributionsList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("lists the user's replies by user id", () => {
    vi.mocked(useContributions).mockReturnValue(hookResult());
    render(<ContributionsList userId="user-123" />);
    expect(useContributions).toHaveBeenCalledWith('user-123');
  });

  it('renders each reply with its post title, linking to the reply on its post', () => {
    vi.mocked(useContributions).mockReturnValue(hookResult());
    render(<ContributionsList userId="user-123" />);

    for (const c of mockContributions) {
      const title = screen.getByText(c.postTitle);
      expect(title.closest('a')).toHaveAttribute('href', `/posts/${c.postId}#${c.id}`);
      expect(screen.getByText(c.body)).toBeInTheDocument();
    }
    expect(screen.getAllByText('Replied to:')).toHaveLength(3);
  });

  it('labels a migrated reply with its legacy type and a new reply as a reply', () => {
    vi.mocked(useContributions).mockReturnValue(hookResult());
    render(<ContributionsList userId="user-123" />);

    expect(screen.getByText('ANSWER')).toBeInTheDocument();
    expect(screen.getByText('APPROACH')).toBeInTheDocument();
    expect(screen.getByText('REPLY')).toBeInTheDocument();
  });

  it('offers no contribution type filter', () => {
    vi.mocked(useContributions).mockReturnValue(hookResult());
    render(<ContributionsList userId="user-123" />);

    for (const pill of ['ALL', 'ANSWERS', 'APPROACHES', 'RESPONSES']) {
      expect(screen.queryByText(pill)).not.toBeInTheDocument();
    }
  });

  it('shows loading state', () => {
    vi.mocked(useContributions).mockReturnValue(hookResult({ contributions: [], loading: true, total: 0 }));
    render(<ContributionsList userId="user-123" />);
    expect(screen.getByText('Loading contributions...')).toBeInTheDocument();
  });

  it('shows the error the hook reports', () => {
    vi.mocked(useContributions).mockReturnValue(hookResult({ contributions: [], error: 'Network error', total: 0 }));
    render(<ContributionsList userId="user-123" />);
    expect(screen.getByText('Network error')).toBeInTheDocument();
  });

  it('shows empty state when no contributions', () => {
    vi.mocked(useContributions).mockReturnValue(hookResult({ contributions: [], total: 0 }));
    render(<ContributionsList userId="user-123" />);
    expect(screen.getByText('No contributions yet')).toBeInTheDocument();
  });

  it('shows LOAD MORE button when hasMore is true', () => {
    const mockLoadMore = vi.fn();
    vi.mocked(useContributions).mockReturnValue(hookResult({ hasMore: true, loadMore: mockLoadMore }));
    render(<ContributionsList userId="user-123" />);

    const loadMoreBtn = screen.getByText('LOAD MORE');
    expect(loadMoreBtn).toBeInTheDocument();
    fireEvent.click(loadMoreBtn);
    expect(mockLoadMore).toHaveBeenCalledTimes(1);
  });

  it('hides LOAD MORE button when hasMore is false', () => {
    vi.mocked(useContributions).mockReturnValue(hookResult());
    render(<ContributionsList userId="user-123" />);
    expect(screen.queryByText('LOAD MORE')).not.toBeInTheDocument();
  });
});
