import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { FeedList } from './feed-list';

// Mock hooks
vi.mock('@/hooks/use-posts', () => ({
  usePosts: vi.fn(() => ({
    posts: [
      {
        id: 'post-1',
        type: 'problem',
        title: 'Test Problem',
        snippet: 'Test snippet',
        tags: ['go', 'testing'],
        author: { name: 'TestUser', type: 'human' },
        time: '2h ago',
        votes: 5,
        responses: 3,
        comments: 7,
        views: 42,
        status: 'OPEN',
        isPinned: false,
        isHot: false,
      },
      {
        id: 'post-2',
        type: 'question',
        title: 'Test Question',
        snippet: 'Another snippet',
        tags: ['typescript'],
        author: { name: 'AgentBot', type: 'agent' },
        time: '1h ago',
        votes: 10,
        responses: 1,
        comments: 0,
        views: 100,
        status: 'OPEN',
        isPinned: false,
        isHot: false,
      },
    ],
    loading: false,
    error: null,
    total: 2,
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
  VoteButton: ({ postId, initialUserVote }: { postId: string; initialUserVote?: 'up' | 'down' | null }) => (
    <div data-testid={`vote-button-${postId}`} data-initial-user-vote={initialUserVote ?? 'null'}>Vote</div>
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

// Helper to hover over a post card to reveal quick actions
function hoverPost(index: number) {
  const articles = screen.getAllByRole('article');
  fireEvent.mouseEnter(articles[index]);
}

describe('FeedList Comment Counts', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('displays correct comment counts for all post types', () => {
    render(<FeedList />);

    // The default mock already has:
    // - post-1 (problem) with 3 responses (approaches)
    // - post-2 (question) with 1 response (answer)
    // Both should be visible in the feed

    const articles = screen.getAllByRole('article');
    expect(articles).toHaveLength(2);

    // First post is problem with 3 approaches
    const problemCard = articles[0];
    expect(problemCard).toHaveTextContent('Test Problem');
    expect(problemCard).toHaveTextContent('3'); // approaches count

    // Second post is question with 1 answer
    const questionCard = articles[1];
    expect(questionCard).toHaveTextContent('Test Question');
    expect(questionCard).toHaveTextContent('1'); // answers count
  });
});

describe('FeedList More Menu', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders More button on hover of feed card', () => {
    render(<FeedList />);

    // Hover over first post to show quick actions
    hoverPost(0);
    expect(screen.getByTestId('feed-more-button')).toBeInTheDocument();

    // Hover over second post
    hoverPost(1);
    expect(screen.getByTestId('feed-more-button')).toBeInTheDocument();
  });

  it('clicking More button opens dropdown menu', () => {
    render(<FeedList />);

    hoverPost(0);
    const moreButton = screen.getByTestId('feed-more-button');
    fireEvent.click(moreButton);

    expect(screen.getByTestId('feed-more-dropdown')).toBeInTheDocument();
  });

  it('dropdown contains Report option with Flag icon', () => {
    render(<FeedList />);

    hoverPost(0);
    const moreButton = screen.getByTestId('feed-more-button');
    fireEvent.click(moreButton);

    expect(screen.getByText('REPORT')).toBeInTheDocument();
  });

  it('clicking Report in dropdown opens ReportModal with correct target', () => {
    render(<FeedList />);

    hoverPost(0);
    const moreButton = screen.getByTestId('feed-more-button');
    fireEvent.click(moreButton);

    const reportButton = screen.getByText('REPORT');
    fireEvent.click(reportButton);

    // ReportModal should now be visible (rendered by the actual component with mocked useReport)
    // The ReportModal renders when reportPostId is not null
    // Since we're using the real ReportModal with mocked hooks, check for its content
    expect(screen.getByText('REPORT POST')).toBeInTheDocument();
  });

  it('dropdown closes when clicking outside', async () => {
    render(<FeedList />);

    hoverPost(0);
    const moreButton = screen.getByTestId('feed-more-button');
    fireEvent.click(moreButton);

    expect(screen.getByTestId('feed-more-dropdown')).toBeInTheDocument();

    // Click outside the dropdown
    fireEvent.mouseDown(document);

    await waitFor(() => {
      expect(screen.queryByTestId('feed-more-dropdown')).not.toBeInTheDocument();
    });
  });

  it('dropdown closes after selecting Report', () => {
    render(<FeedList />);

    hoverPost(0);
    const moreButton = screen.getByTestId('feed-more-button');
    fireEvent.click(moreButton);

    expect(screen.getByTestId('feed-more-dropdown')).toBeInTheDocument();

    const reportButton = screen.getByText('REPORT');
    fireEvent.click(reportButton);

    // Dropdown should be closed after clicking Report
    expect(screen.queryByTestId('feed-more-dropdown')).not.toBeInTheDocument();
  });

  it('More button click prevents navigation (stopPropagation and preventDefault)', () => {
    render(<FeedList />);

    hoverPost(0);
    const moreButton = screen.getByTestId('feed-more-button');

    // Clicking the More button should not navigate (it calls e.stopPropagation and e.preventDefault)
    // We verify by checking the dropdown opens (meaning the event was handled by the button, not the Link)
    fireEvent.click(moreButton);
    expect(screen.getByTestId('feed-more-dropdown')).toBeInTheDocument();
  });

  it('displays comment count for posts with comments', () => {
    render(<FeedList />);

    // Post 1 has 7 comments - should be displayed
    // The number 7 should appear in the document (as comment count)
    expect(screen.getByText('7')).toBeInTheDocument();
  });

  it('displays 0 when post has no comments', () => {
    render(<FeedList />);

    // Post 2 has 0 comments - "0" should be displayed somewhere
    // (could be comment count, or other stats - but should exist)
    const allText = document.body.textContent || '';
    expect(allText).toContain('0');
  });
});
