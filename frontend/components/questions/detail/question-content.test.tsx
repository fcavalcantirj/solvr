import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QuestionContent } from './question-content';
import { QuestionData } from '@/hooks/use-question';

const mockUseVote = vi.fn().mockReturnValue({
  score: 15,
  userVote: null,
  isVoting: false,
  error: null,
  upvote: vi.fn(),
  downvote: vi.fn(),
});

// Mock useVote - should NOT be used in question-content after refactor
vi.mock('@/hooks/use-vote', () => ({
  useVote: (...args: unknown[]) => mockUseVote(...args),
}));

// Mock report modal
vi.mock('@/components/ui/report-modal', () => ({
  ReportModal: ({ isOpen }: { isOpen: boolean }) => (
    isOpen ? <div data-testid="report-modal">ReportModal</div> : null
  ),
}));

const mockQuestion: QuestionData = {
  id: 'question-abc123',
  title: 'How to handle async errors in Go?',
  description: 'I am struggling with error handling in goroutines...',
  status: 'open',
  voteScore: 15,
  upvotes: 18,
  downvotes: 3,
  author: {
    id: 'user-1',
    type: 'human',
    displayName: 'testuser',
  },
  tags: ['go', 'async', 'error-handling'],
  createdAt: '2026-02-10T10:00:00Z',
  updatedAt: '2026-02-10T10:00:00Z',
  time: '5d ago',
  answersCount: 3,
  views: 42,
};

describe('QuestionContent', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('does NOT render ThumbsUp/ThumbsDown vote buttons', () => {
    const { container } = render(<QuestionContent question={mockQuestion} />);

    // After refactoring, there should be no vote column with ThumbsUp/ThumbsDown
    // The vote buttons (ThumbsUp, score display, ThumbsDown) should be removed
    // Look for the vote score span that was previously inline
    const allButtons = container.querySelectorAll('button');
    const voteButtons = Array.from(allButtons).filter(btn => {
      const svg = btn.querySelector('svg');
      return svg && (svg.classList.contains('lucide-thumbs-up') || svg.classList.contains('lucide-thumbs-down'));
    });
    expect(voteButtons.length).toBe(0);
  });

  it('does not call useVote hook', () => {
    render(<QuestionContent question={mockQuestion} />);

    // useVote should NOT be called by QuestionContent since voting moved to header
    expect(mockUseVote).not.toHaveBeenCalled();
  });

  it('renders question description content', () => {
    render(<QuestionContent question={mockQuestion} />);

    expect(screen.getByText('I am struggling with error handling in goroutines...')).toBeDefined();
  });

  it('renders tags', () => {
    render(<QuestionContent question={mockQuestion} />);

    expect(screen.getByText('go')).toBeDefined();
    expect(screen.getByText('async')).toBeDefined();
    expect(screen.getByText('error-handling')).toBeDefined();
  });

  it('renders Flag button', () => {
    render(<QuestionContent question={mockQuestion} />);

    expect(screen.getByText('FLAG')).toBeDefined();
  });
});
