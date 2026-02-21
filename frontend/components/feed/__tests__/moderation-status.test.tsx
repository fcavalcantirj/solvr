import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { FeedList } from '../feed-list';

// Mock hooks
vi.mock('@/hooks/use-posts', () => ({
  usePosts: vi.fn(() => ({
    posts: [],
    loading: false,
    error: null,
    total: 0,
    hasMore: false,
    page: 1,
    refetch: vi.fn(),
    loadMore: vi.fn(),
  })),
  useSearch: vi.fn(() => ({
    posts: [],
    loading: false,
    error: null,
    searchMethod: undefined,
  })),
  PostType: {},
  FeedPost: {},
}));

vi.mock('@/hooks/use-share', () => ({
  useShare: () => ({ share: vi.fn() }),
}));

vi.mock('@/hooks/use-polling', () => ({
  usePolling: vi.fn(),
}));

vi.mock('@/hooks/use-bookmarks', () => ({
  useBookmarks: () => ({
    bookmarkedPosts: new Set(),
    toggleBookmark: vi.fn(),
  }),
}));

vi.mock('@/lib/api', () => ({
  api: {
    getPosts: vi.fn(),
  },
}));

vi.mock('@/lib/filter-utils', () => ({
  mapStatusFilter: vi.fn((v: string) => v),
  mapSortFilter: vi.fn((v: string) => v),
  mapTimeframeFilter: vi.fn((v: string) => v),
}));

vi.mock('@/components/search/search-method-badge', () => ({
  SearchMethodBadge: ({ method }: { method?: string }) =>
    method === 'hybrid' ? <div data-testid="search-method-badge">Semantic search enabled</div> : null,
}));

vi.mock('@/components/ui/vote-button', () => ({
  VoteButton: ({ postId }: { postId: string }) => (
    <div data-testid={`vote-button-${postId}`}>Vote</div>
  ),
}));

vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => ({
    isAuthenticated: true,
    loginWithGitHub: vi.fn(),
  }),
}));

vi.mock('@/hooks/use-report', () => ({
  useReport: () => ({
    isSubmitting: false,
    error: null,
    submitReport: vi.fn(),
    clearError: vi.fn(),
  }),
  REPORT_REASONS: [
    { value: 'spam', label: 'Spam', description: 'Spam content' },
  ],
}));

const { usePosts } = await import('@/hooks/use-posts');

describe('FeedList Moderation Status Display', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders "UNDER REVIEW" badge for pending_review status', () => {
    vi.mocked(usePosts).mockReturnValue({
      posts: [
        {
          id: 'post-review',
          type: 'problem',
          title: 'Post Under Review',
          snippet: 'This post is under review',
          tags: ['test'],
          author: { name: 'TestUser', type: 'human' },
          time: '1h ago',
          votes: 0,
          responses: 0,
          comments: 0,
          views: 0,
          status: 'UNDER REVIEW',
          isPinned: false,
          isHot: false,
        },
      ],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<FeedList />);

    expect(screen.getByText('UNDER REVIEW')).toBeInTheDocument();
  });

  it('renders "REJECTED" badge for rejected status', () => {
    vi.mocked(usePosts).mockReturnValue({
      posts: [
        {
          id: 'post-rejected',
          type: 'question',
          title: 'Rejected Post',
          snippet: 'This post was rejected',
          tags: ['test'],
          author: { name: 'TestUser', type: 'human' },
          time: '2h ago',
          votes: 0,
          responses: 0,
          comments: 0,
          views: 0,
          status: 'REJECTED',
          isPinned: false,
          isHot: false,
        },
      ],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<FeedList />);

    expect(screen.getByText('REJECTED')).toBeInTheDocument();
  });

  it('renders yellow dot for UNDER REVIEW status', () => {
    vi.mocked(usePosts).mockReturnValue({
      posts: [
        {
          id: 'post-review-2',
          type: 'idea',
          title: 'Under Review Idea',
          snippet: 'Review idea',
          tags: [],
          author: { name: 'Bot', type: 'ai' },
          time: '5m ago',
          votes: 0,
          responses: 0,
          comments: 0,
          views: 0,
          status: 'UNDER REVIEW',
          isPinned: false,
          isHot: false,
        },
      ],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<FeedList />);

    // The status dot should have the yellow color class
    const statusDot = document.querySelector('.bg-yellow-500');
    expect(statusDot).toBeInTheDocument();
  });

  it('renders red dot for REJECTED status', () => {
    vi.mocked(usePosts).mockReturnValue({
      posts: [
        {
          id: 'post-rejected-2',
          type: 'problem',
          title: 'Rejected Problem',
          snippet: 'Rejected',
          tags: [],
          author: { name: 'User', type: 'human' },
          time: '3h ago',
          votes: 0,
          responses: 0,
          comments: 0,
          views: 0,
          status: 'REJECTED',
          isPinned: false,
          isHot: false,
        },
      ],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<FeedList />);

    const statusDot = document.querySelector('.bg-red-500');
    expect(statusDot).toBeInTheDocument();
  });
});
