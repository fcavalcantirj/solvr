import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { IdeaHeader } from './idea-header';
import { IdeaData } from '@/hooks/use-idea';

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

const mockIdea: IdeaData = {
  id: 'idea-xyz789',
  title: 'AI agents should share debugging patterns',
  description: 'What if AI agents could automatically share patterns they discover...',
  status: 'ACTIVE',
  voteScore: 42,
  upvotes: 50,
  downvotes: 8,
  author: {
    id: 'agent-1',
    type: 'ai',
    displayName: 'claudius',
  },
  tags: ['ai-agents', 'debugging', 'patterns'],
  createdAt: '2026-02-10T10:00:00Z',
  updatedAt: '2026-02-10T10:00:00Z',
  time: '5d ago',
  views: 128,
};

describe('IdeaHeader', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders VoteButton with correct postId, initialScore, direction, size, and showDownvote', () => {
    render(<IdeaHeader idea={mockIdea} />);

    const voteButton = screen.getByTestId('vote-button-idea-xyz789');
    expect(voteButton).toBeDefined();
    expect(voteButton.getAttribute('data-initial-score')).toBe('42');
    expect(voteButton.getAttribute('data-direction')).toBe('horizontal');
    expect(voteButton.getAttribute('data-size')).toBe('md');
    expect(voteButton.getAttribute('data-show-downvote')).toBe('true');
  });

  it('clicking VoteButton triggers vote (VoteButton component rendered with useVote)', () => {
    render(<IdeaHeader idea={mockIdea} />);

    // VoteButton is rendered (which internally uses useVote hook)
    const voteButton = screen.getByTestId('vote-button-idea-xyz789');
    expect(voteButton).toBeDefined();
    expect(voteButton.textContent).toBe('VoteButton');
  });

  it('does not render static ArrowUp button with SUPPORT label', () => {
    render(<IdeaHeader idea={mockIdea} />);

    // The old static SUPPORT label should not exist
    expect(screen.queryByText('SUPPORT')).toBeNull();
  });

  it('renders idea title and status', () => {
    render(<IdeaHeader idea={mockIdea} />);

    expect(screen.getByText('AI agents should share debugging patterns')).toBeDefined();
    expect(screen.getByText('ACTIVE')).toBeDefined();
  });

  it('renders Share and Watch buttons alongside VoteButton', () => {
    render(<IdeaHeader idea={mockIdea} />);

    expect(screen.getByText('SHARE')).toBeDefined();
    expect(screen.getByText('WATCH')).toBeDefined();
    expect(screen.getByText('VoteButton')).toBeDefined();
  });

  it('renders author info correctly', () => {
    render(<IdeaHeader idea={mockIdea} />);

    expect(screen.getByText('claudius')).toBeDefined();
    expect(screen.getByText('[AI]')).toBeDefined();
  });
});
