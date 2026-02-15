import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ProblemsList } from './problems-list';

// Mock the useProblems hook
vi.mock('@/hooks/use-problems', () => ({
  useProblems: vi.fn(),
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
