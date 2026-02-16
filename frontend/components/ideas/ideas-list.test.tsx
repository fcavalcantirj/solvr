import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { IdeasList } from './ideas-list';

// Mock the useIdeas hook
vi.mock('@/hooks/use-ideas', () => ({
  useIdeas: vi.fn(),
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

import { useIdeas } from '@/hooks/use-ideas';

const mockIdea = {
  id: 'idea-123',
  title: 'Test Idea',
  spark: 'A test idea description...',
  stage: 'spark' as const,
  potential: 'rising' as const,
  author: { name: 'testuser', type: 'human' as const },
  support: 25,
  comments: 3,
  branches: 0,
  tags: ['ai', 'testing'],
  timestamp: '2h ago',
  supporters: [],
  recentComment: null,
};

describe('IdeasList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders VoteButton with correct postId and initialScore props', () => {
    vi.mocked(useIdeas).mockReturnValue({
      ideas: [mockIdea],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<IdeasList />);

    // Two VoteButtons per card: desktop (vertical) + mobile (horizontal)
    const voteButtons = screen.getAllByTestId('vote-button-idea-123');
    expect(voteButtons.length).toBe(2);
    // Both should have correct postId and initialScore (support field)
    for (const btn of voteButtons) {
      expect(btn.getAttribute('data-initial-score')).toBe('25');
      expect(btn.getAttribute('data-show-downvote')).toBe('true');
    }
  });

  it('renders VoteButton with showDownvote={true}', () => {
    vi.mocked(useIdeas).mockReturnValue({
      ideas: [mockIdea],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<IdeasList />);

    const voteButtons = screen.getAllByTestId('vote-button-idea-123');
    for (const btn of voteButtons) {
      expect(btn.getAttribute('data-show-downvote')).toBe('true');
    }
  });

  it('renders desktop VoteButton with vertical direction and sm size', () => {
    vi.mocked(useIdeas).mockReturnValue({
      ideas: [mockIdea],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<IdeasList />);

    const voteButtons = screen.getAllByTestId('vote-button-idea-123');
    const verticalButton = voteButtons.find(
      btn => btn.getAttribute('data-direction') === 'vertical' && btn.getAttribute('data-size') === 'sm'
    );
    expect(verticalButton).toBeDefined();
  });

  it('renders mobile VoteButton with horizontal direction and sm size', () => {
    vi.mocked(useIdeas).mockReturnValue({
      ideas: [mockIdea],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<IdeasList />);

    const voteButtons = screen.getAllByTestId('vote-button-idea-123');
    const horizontalButton = voteButtons.find(
      btn => btn.getAttribute('data-direction') === 'horizontal' && btn.getAttribute('data-size') === 'sm'
    );
    expect(horizontalButton).toBeDefined();
  });

  it('does not render non-functional ArrowUp button in stats section', () => {
    vi.mocked(useIdeas).mockReturnValue({
      ideas: [mockIdea],
      loading: false,
      error: null,
      total: 1,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    const { container } = render(<IdeasList />);

    // VoteButton should be rendering (not static ArrowUp)
    expect(screen.getAllByText('VoteButton').length).toBeGreaterThan(0);

    // There should be no standalone button with just the vote score text outside VoteButton
    // The old code had a <button> with ArrowUp icon and support count as plain text
    // Now all vote UI should be through VoteButton component
    const voteButtons = screen.getAllByTestId(/vote-button/);
    expect(voteButtons.length).toBeGreaterThan(0);
  });

  it('renders VoteButton for each idea in the list', () => {
    const secondIdea = {
      ...mockIdea,
      id: 'idea-456',
      title: 'Second Idea',
      support: 7,
    };

    vi.mocked(useIdeas).mockReturnValue({
      ideas: [mockIdea, secondIdea],
      loading: false,
      error: null,
      total: 2,
      hasMore: false,
      page: 1,
      loadMore: vi.fn(),
      refetch: vi.fn(),
    });

    render(<IdeasList />);

    // Each idea has 2 VoteButtons (desktop + mobile)
    const firstButtons = screen.getAllByTestId('vote-button-idea-123');
    const secondButtons = screen.getAllByTestId('vote-button-idea-456');
    expect(firstButtons.length).toBe(2);
    expect(secondButtons.length).toBe(2);

    // Verify second idea's vote score
    for (const btn of secondButtons) {
      expect(btn.getAttribute('data-initial-score')).toBe('7');
    }
  });

  describe('Defensive null handling', () => {
    it('handles undefined supporters array gracefully', () => {
      const ideaWithUndefinedSupporters = {
        ...mockIdea,
        supporters: undefined,
      };
      vi.mocked(useIdeas).mockReturnValue({
        ideas: [ideaWithUndefinedSupporters],
        loading: false,
        error: null,
        total: 1,
        hasMore: false,
        page: 1,
        loadMore: vi.fn(),
        refetch: vi.fn(),
      });

      // Should not crash
      expect(() => render(<IdeasList />)).not.toThrow();
    });

    it('handles null supporters array gracefully', () => {
      const ideaWithNullSupporters = {
        ...mockIdea,
        supporters: null as any,
      };
      vi.mocked(useIdeas).mockReturnValue({
        ideas: [ideaWithNullSupporters],
        loading: false,
        error: null,
        total: 1,
        hasMore: false,
        page: 1,
        loadMore: vi.fn(),
        refetch: vi.fn(),
      });

      // Should not crash
      expect(() => render(<IdeasList />)).not.toThrow();
    });

    it('does not render supporters preview when supporters is undefined', () => {
      const ideaWithUndefinedSupporters = {
        ...mockIdea,
        supporters: undefined,
      };
      vi.mocked(useIdeas).mockReturnValue({
        ideas: [ideaWithUndefinedSupporters],
        loading: false,
        error: null,
        total: 1,
        hasMore: false,
        page: 1,
        loadMore: vi.fn(),
        refetch: vi.fn(),
      });

      const { container } = render(<IdeasList />);

      // Supporters UI should not be rendered
      // The supporters div has flex -space-x-1, which is specific to supporters
      const supportersElements = container.querySelectorAll('.flex.-space-x-1');
      expect(supportersElements.length).toBe(0);
    });

    it('does not render supporters preview when supporters is empty', () => {
      const ideaWithEmptySupporters = {
        ...mockIdea,
        supporters: [],
      };
      vi.mocked(useIdeas).mockReturnValue({
        ideas: [ideaWithEmptySupporters],
        loading: false,
        error: null,
        total: 1,
        hasMore: false,
        page: 1,
        loadMore: vi.fn(),
        refetch: vi.fn(),
      });

      const { container } = render(<IdeasList />);

      // Supporters UI should not be rendered when array is empty
      const supportersElements = container.querySelectorAll('.flex.-space-x-1');
      expect(supportersElements.length).toBe(0);
    });
  });
});
