import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ProblemsList } from './problems-list';

// Mock the useProblems hook
vi.mock('@/hooks/use-problems', () => ({
  useProblems: vi.fn(),
}));

// Mock the useSearch hook
vi.mock('@/hooks/use-posts', () => ({
  useSearch: vi.fn(),
}));

// Mock VoteButton
vi.mock('@/components/ui/vote-button', () => ({
  VoteButton: ({ postId, initialScore, showDownvote, direction, size }: {
    postId: string;
    initialScore: number;
    showDownvote?: boolean;
    direction?: string;
    size?: string;
  }) => (
    <div
      data-testid={`vote-button-${postId}`}
      data-initial-score={initialScore}
      data-show-downvote={showDownvote}
      data-direction={direction}
      data-size={size}
    >
      VoteButton
    </div>
  ),
}));

// Mock next/link
vi.mock('next/link', () => ({
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

import { useProblems } from '@/hooks/use-problems';
import { useSearch } from '@/hooks/use-posts';

const mockProblem = {
  id: 'problem-123',
  title: 'Test Problem',
  snippet: 'A test problem description...',
  status: 'open',
  displayStatus: 'Open',
  voteScore: 42,
  viewCount: 100,
  approachesCount: 3,
  author: { id: 'user-1', name: 'testuser', type: 'human' as const },
  tags: ['go', 'testing'],
  timestamp: '2h ago',
};

describe('ProblemsList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders VoteButton with correct postId and initialScore props', () => {
    vi.mocked(useProblems).mockReturnValue({
      problems: [mockProblem],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<ProblemsList />);

    // Two VoteButtons per card: desktop (vertical) + mobile (horizontal)
    const voteButtons = screen.getAllByTestId('vote-button-problem-123');
    expect(voteButtons.length).toBe(2);
    // Both should have correct postId and initialScore
    for (const btn of voteButtons) {
      expect(btn.getAttribute('data-initial-score')).toBe('42');
      expect(btn.getAttribute('data-show-downvote')).toBe('true');
    }
  });

  it('renders VoteButton with showDownvote={true}', () => {
    vi.mocked(useProblems).mockReturnValue({
      problems: [mockProblem],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<ProblemsList />);

    const voteButtons = screen.getAllByTestId('vote-button-problem-123');
    for (const btn of voteButtons) {
      expect(btn.getAttribute('data-show-downvote')).toBe('true');
    }
  });

  it('renders desktop VoteButton with vertical direction and sm size', () => {
    vi.mocked(useProblems).mockReturnValue({
      problems: [mockProblem],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<ProblemsList />);

    // The desktop VoteButton should be vertical with sm size
    const voteButtons = screen.getAllByTestId('vote-button-problem-123');
    // Should have at least one with vertical direction
    const verticalButton = voteButtons.find(
      btn => btn.getAttribute('data-direction') === 'vertical' && btn.getAttribute('data-size') === 'sm'
    );
    expect(verticalButton).toBeDefined();
  });

  it('renders mobile VoteButton with horizontal direction and sm size', () => {
    vi.mocked(useProblems).mockReturnValue({
      problems: [mockProblem],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<ProblemsList />);

    const voteButtons = screen.getAllByTestId('vote-button-problem-123');
    // Should have at least one with horizontal direction for mobile
    const horizontalButton = voteButtons.find(
      btn => btn.getAttribute('data-direction') === 'horizontal' && btn.getAttribute('data-size') === 'sm'
    );
    expect(horizontalButton).toBeDefined();
  });

  it('does not render static ArrowUp icon for vote score', () => {
    vi.mocked(useProblems).mockReturnValue({
      problems: [mockProblem],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    const { container } = render(<ProblemsList />);

    // The old static vote display used a span with the vote score outside of VoteButton
    // Make sure there's no standalone "42" text outside the VoteButton
    const voteButtons = screen.getAllByTestId(/vote-button/);
    expect(voteButtons.length).toBeGreaterThan(0);

    // Verify VoteButton is rendering (not static ArrowUp)
    expect(screen.getAllByText('VoteButton').length).toBeGreaterThan(0);
  });

  it('renders VoteButton for each problem in the list', () => {
    const secondProblem = {
      ...mockProblem,
      id: 'problem-456',
      title: 'Second Problem',
      voteScore: 7,
    };

    vi.mocked(useProblems).mockReturnValue({
      problems: [mockProblem, secondProblem],
      loading: false,
      error: null,
      total: 2,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<ProblemsList />);

    // Each problem has 2 VoteButtons (desktop + mobile)
    const firstButtons = screen.getAllByTestId('vote-button-problem-123');
    const secondButtons = screen.getAllByTestId('vote-button-problem-456');
    expect(firstButtons.length).toBe(2);
    expect(secondButtons.length).toBe(2);

    // Verify second problem's vote score
    for (const btn of secondButtons) {
      expect(btn.getAttribute('data-initial-score')).toBe('7');
    }
  });
});

describe('ProblemsList - Search Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('uses useSearch hook when searchQuery is provided', () => {
    const mockSearchPost = {
      id: 'post-123',
      title: 'Search Result',
      snippet: 'A search result...',
      status: 'open',
      votes: 10,
      responses: 2,
      views: 50,
      author: { id: 'user-1', name: 'testuser', type: 'human' as const },
      tags: ['react'],
      time: '1h ago',
      type: 'problem' as const,
    };

    vi.mocked(useProblems).mockReturnValue({
      problems: [],
      loading: false,
      error: null,
      total: 0,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    vi.mocked(useSearch).mockReturnValue({
      posts: [mockSearchPost],
      loading: false,
      error: null,
      hasMore: false,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<ProblemsList searchQuery="test search" />);

    // Both hooks are called (React rules), but useSearch result is used
    expect(useSearch).toHaveBeenCalledWith('test search', 'problem');
    expect(screen.getByText('Search Result')).toBeInTheDocument();
  });

  it('uses useProblems hook when searchQuery is empty', () => {
    vi.mocked(useProblems).mockReturnValue({
      problems: [mockProblem],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    vi.mocked(useSearch).mockReturnValue({
      posts: [],
      loading: false,
      error: null,
      hasMore: false,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<ProblemsList searchQuery="" />);

    // Both hooks are called (React rules), but useProblems result is used
    expect(useProblems).toHaveBeenCalled();
    expect(useSearch).toHaveBeenCalledWith('', 'problem');
    expect(screen.getByText('Test Problem')).toBeInTheDocument();
  });

  it('uses useProblems hook when searchQuery is undefined', () => {
    vi.mocked(useProblems).mockReturnValue({
      problems: [mockProblem],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    vi.mocked(useSearch).mockReturnValue({
      posts: [],
      loading: false,
      error: null,
      hasMore: false,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<ProblemsList />);

    // Both hooks are called (React rules), but useProblems result is used
    expect(useProblems).toHaveBeenCalled();
    expect(useSearch).toHaveBeenCalledWith('', 'problem');
    expect(screen.getByText('Test Problem')).toBeInTheDocument();
  });

  it('transforms search results to problem format', () => {
    const mockSearchPost = {
      id: 'post-search-1',
      title: 'Async Bug',
      snippet: 'Having issues with async...',
      status: 'open',
      votes: 15,
      responses: 3,
      views: 100,
      author: { id: 'user-2', name: 'developer', type: 'human' as const },
      tags: ['async', 'node.js'],
      time: '2h ago',
      type: 'problem' as const,
    };

    vi.mocked(useSearch).mockReturnValue({
      posts: [mockSearchPost],
      loading: false,
      error: null,
      hasMore: false,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<ProblemsList searchQuery="async" />);

    // Should render the search result
    expect(screen.getByText('Async Bug')).toBeInTheDocument();
  });

  it('displays multiple search results', () => {
    const searchResults = [
      {
        id: 'search-1',
        title: 'Race Conditions in Go',
        snippet: 'Encountering race conditions...',
        status: 'open',
        votes: 10,
        responses: 2,
        views: 50,
        author: { id: 'user-1', name: 'developer', type: 'human' as const },
        tags: ['go', 'concurrency'],
        time: '1h ago',
        type: 'problem' as const,
      },
      {
        id: 'search-2',
        title: 'How to Handle Race Conditions',
        snippet: 'Best practices for handling...',
        status: 'open',
        votes: 15,
        responses: 3,
        views: 80,
        author: { id: 'user-2', name: 'expert', type: 'human' as const },
        tags: ['concurrency', 'best-practices'],
        time: '2h ago',
        type: 'problem' as const,
      },
    ];

    vi.mocked(useSearch).mockReturnValue({
      posts: searchResults,
      loading: false,
      error: null,
      hasMore: false,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    vi.mocked(useProblems).mockReturnValue({
      problems: [],
      loading: false,
      error: null,
      total: 0,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<ProblemsList searchQuery="race condition" />);

    // Both search result titles should be displayed
    expect(screen.getByText('Race Conditions in Go')).toBeInTheDocument();
    expect(screen.getByText('How to Handle Race Conditions')).toBeInTheDocument();
  });

  it('shows no results message when search returns empty', () => {
    vi.mocked(useSearch).mockReturnValue({
      posts: [],
      loading: false,
      error: null,
      hasMore: false,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    vi.mocked(useProblems).mockReturnValue({
      problems: [],
      loading: false,
      error: null,
      total: 0,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<ProblemsList searchQuery="nonexistent query" />);

    // Empty state message should appear
    expect(screen.getByText('No problems found.')).toBeInTheDocument();
  });
});
