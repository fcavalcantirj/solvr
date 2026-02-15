import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QuestionHeader } from './question-header';
import { QuestionData } from '@/hooks/use-question';

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

describe('QuestionHeader', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders VoteButton with correct postId, initialScore, direction, and showDownvote', () => {
    render(<QuestionHeader question={mockQuestion} />);

    const voteButton = screen.getByTestId('vote-button-question-abc123');
    expect(voteButton).toBeDefined();
    expect(voteButton.getAttribute('data-initial-score')).toBe('15');
    expect(voteButton.getAttribute('data-direction')).toBe('horizontal');
    expect(voteButton.getAttribute('data-size')).toBe('md');
    expect(voteButton.getAttribute('data-show-downvote')).toBe('true');
  });

  it('renders question title and status', () => {
    render(<QuestionHeader question={mockQuestion} />);

    expect(screen.getByText('How to handle async errors in Go?')).toBeDefined();
    expect(screen.getByText('open')).toBeDefined();
    expect(screen.getByText('QUESTION')).toBeDefined();
  });

  it('renders author info', () => {
    render(<QuestionHeader question={mockQuestion} />);

    expect(screen.getByText('testuser')).toBeDefined();
  });

  it('renders Share and Save buttons alongside VoteButton', () => {
    render(<QuestionHeader question={mockQuestion} />);

    expect(screen.getByText('SHARE')).toBeDefined();
    expect(screen.getByText('SAVE')).toBeDefined();
    expect(screen.getByText('VoteButton')).toBeDefined();
  });
});
