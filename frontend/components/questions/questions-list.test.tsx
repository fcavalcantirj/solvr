import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QuestionsList } from './questions-list';

// Mock the useQuestions hook
vi.mock('@/hooks/use-questions', () => ({
  useQuestions: vi.fn(),
}));

// Mock VoteButton
vi.mock('@/components/ui/vote-button', () => ({
  VoteButton: ({ postId, initialScore, initialUserVote, showDownvote, direction, size }: {
    postId: string;
    initialScore: number;
    initialUserVote?: 'up' | 'down' | null;
    showDownvote?: boolean;
    direction?: string;
    size?: string;
  }) => (
    <div
      data-testid={`vote-button-${postId}`}
      data-initial-score={initialScore}
      data-initial-user-vote={initialUserVote ?? 'null'}
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

import { useQuestions } from '@/hooks/use-questions';

const mockQuestion = {
  id: 'question-123',
  title: 'Test Question',
  snippet: 'A test question description...',
  status: 'open',
  displayStatus: 'Unanswered',
  voteScore: 15,
  answersCount: 3,
  author: { id: 'user-1', name: 'testuser', type: 'human' as const },
  tags: ['typescript', 'react'],
  timestamp: '3h ago',
};

describe('QuestionsList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders VoteButton with correct postId and initialScore props', () => {
    vi.mocked(useQuestions).mockReturnValue({
      questions: [mockQuestion],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<QuestionsList />);

    // Two VoteButtons per card: desktop (vertical) + mobile (horizontal)
    const voteButtons = screen.getAllByTestId('vote-button-question-123');
    expect(voteButtons.length).toBe(2);
    // Both should have correct postId and initialScore
    for (const btn of voteButtons) {
      expect(btn.getAttribute('data-initial-score')).toBe('15');
    }
  });

  it('renders VoteButton with showDownvote={true}', () => {
    vi.mocked(useQuestions).mockReturnValue({
      questions: [mockQuestion],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<QuestionsList />);

    const voteButtons = screen.getAllByTestId('vote-button-question-123');
    for (const btn of voteButtons) {
      expect(btn.getAttribute('data-show-downvote')).toBe('true');
    }
  });

  it('renders desktop VoteButton with vertical direction and sm size', () => {
    vi.mocked(useQuestions).mockReturnValue({
      questions: [mockQuestion],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<QuestionsList />);

    const voteButtons = screen.getAllByTestId('vote-button-question-123');
    const verticalButton = voteButtons.find(
      btn => btn.getAttribute('data-direction') === 'vertical' && btn.getAttribute('data-size') === 'sm'
    );
    expect(verticalButton).toBeDefined();
  });

  it('renders mobile VoteButton with horizontal direction and sm size', () => {
    vi.mocked(useQuestions).mockReturnValue({
      questions: [mockQuestion],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<QuestionsList />);

    const voteButtons = screen.getAllByTestId('vote-button-question-123');
    const horizontalButton = voteButtons.find(
      btn => btn.getAttribute('data-direction') === 'horizontal' && btn.getAttribute('data-size') === 'sm'
    );
    expect(horizontalButton).toBeDefined();
  });

  it('does not render static ArrowUp icon for vote score', () => {
    vi.mocked(useQuestions).mockReturnValue({
      questions: [mockQuestion],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<QuestionsList />);

    // Verify VoteButton is rendering (not static ArrowUp)
    const voteButtons = screen.getAllByTestId(/vote-button/);
    expect(voteButtons.length).toBeGreaterThan(0);
    expect(screen.getAllByText('VoteButton').length).toBeGreaterThan(0);
  });

  it('passes userVote to VoteButton from question data', () => {
    const questionWithVote = {
      ...mockQuestion,
      userVote: 'down' as const,
    };

    vi.mocked(useQuestions).mockReturnValue({
      questions: [questionWithVote],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<QuestionsList />);

    // Both VoteButtons (desktop + mobile) should receive userVote='down'
    const voteButtons = screen.getAllByTestId('vote-button-question-123');
    expect(voteButtons.length).toBe(2);
    for (const btn of voteButtons) {
      expect(btn.getAttribute('data-initial-user-vote')).toBe('down');
    }
  });

  it('renders VoteButton for each question in the list', () => {
    const secondQuestion = {
      ...mockQuestion,
      id: 'question-456',
      title: 'Second Question',
      voteScore: 7,
    };

    vi.mocked(useQuestions).mockReturnValue({
      questions: [mockQuestion, secondQuestion],
      loading: false,
      error: null,
      total: 2,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<QuestionsList />);

    // Each question has 2 VoteButtons (desktop + mobile)
    const firstButtons = screen.getAllByTestId('vote-button-question-123');
    const secondButtons = screen.getAllByTestId('vote-button-question-456');
    expect(firstButtons.length).toBe(2);
    expect(secondButtons.length).toBe(2);

    // Verify second question's vote score
    for (const btn of secondButtons) {
      expect(btn.getAttribute('data-initial-score')).toBe('7');
    }
  });
});
