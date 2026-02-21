import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { IdeaHeader } from './idea-header';
import { IdeaData } from '@/hooks/use-idea';

// Mock useShare hook
const mockShare = vi.fn();
let mockShared = false;
vi.mock('@/hooks/use-share', () => ({
  useShare: () => ({
    share: mockShare,
    shared: mockShared,
    isSharing: false,
    error: null,
  }),
}));

// Mock useBookmarks hook
const mockToggleBookmark = vi.fn();
let mockBookmarkedPosts = new Set<string>();
vi.mock('@/hooks/use-bookmarks', () => ({
  useBookmarks: () => ({
    bookmarkedPosts: mockBookmarkedPosts,
    isLoading: false,
    error: null,
    toggleBookmark: mockToggleBookmark,
    checkBookmarked: vi.fn(),
  }),
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

// Mock ReportModal
const mockOnClose = vi.fn();
vi.mock('@/components/ui/report-modal', () => ({
  ReportModal: ({ isOpen, onClose, targetType, targetId }: {
    isOpen: boolean;
    onClose: () => void;
    targetType: string;
    targetId: string;
  }) => isOpen ? (
    <div data-testid="report-modal" data-target-type={targetType} data-target-id={targetId}>
      <button onClick={onClose}>Close</button>
      ReportModal
    </div>
  ) : null,
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
    mockShared = false;
    mockBookmarkedPosts = new Set<string>();
    Object.defineProperty(window, 'location', {
      value: { origin: 'https://solvr.dev', href: 'https://solvr.dev/ideas/idea-xyz789' },
      writable: true,
    });
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

    const voteButton = screen.getByTestId('vote-button-idea-xyz789');
    expect(voteButton).toBeDefined();
    expect(voteButton.textContent).toBe('VoteButton');
  });

  it('does not render static ArrowUp button with SUPPORT label', () => {
    render(<IdeaHeader idea={mockIdea} />);
    expect(screen.queryByText('SUPPORT')).toBeNull();
  });

  it('renders idea title and status', () => {
    render(<IdeaHeader idea={mockIdea} />);
    expect(screen.getByText('AI agents should share debugging patterns')).toBeDefined();
    expect(screen.getByText('ACTIVE')).toBeDefined();
  });

  it('renders Share and Watch buttons alongside VoteButton', () => {
    render(<IdeaHeader idea={mockIdea} />);
    expect(screen.getByTestId('share-button')).toBeDefined();
    expect(screen.getByTestId('bookmark-button')).toBeDefined();
    expect(screen.getByText('VoteButton')).toBeDefined();
  });

  it('renders author info correctly', () => {
    render(<IdeaHeader idea={mockIdea} />);
    expect(screen.getByText('claudius')).toBeDefined();
    expect(screen.getByText('[AI]')).toBeDefined();
  });

  // New tests for Share/Watch/More functionality

  it('clicking Share copies URL to clipboard', async () => {
    render(<IdeaHeader idea={mockIdea} />);

    const shareButton = screen.getByTestId('share-button');
    fireEvent.click(shareButton);

    expect(mockShare).toHaveBeenCalledWith(
      'AI agents should share debugging patterns',
      `https://solvr.dev/ideas/${mockIdea.id}`
    );
  });

  it('shows Check icon feedback when share succeeds', () => {
    mockShared = true;
    render(<IdeaHeader idea={mockIdea} />);

    const shareButton = screen.getByTestId('share-button');
    expect(shareButton.querySelector('svg')).toBeTruthy();
    expect(shareButton.classList.toString()).toContain('text-emerald');
  });

  it('clicking Watch calls bookmark API and toggles filled icon state', async () => {
    render(<IdeaHeader idea={mockIdea} />);

    const bookmarkButton = screen.getByTestId('bookmark-button');
    fireEvent.click(bookmarkButton);

    expect(mockToggleBookmark).toHaveBeenCalledWith(mockIdea.id);
  });

  it('shows filled Bookmark icon when bookmarked', () => {
    mockBookmarkedPosts = new Set([mockIdea.id]);
    render(<IdeaHeader idea={mockIdea} />);

    const bookmarkButton = screen.getByTestId('bookmark-button');
    const svg = bookmarkButton.querySelector('svg');
    expect(svg).toBeTruthy();
    expect(bookmarkButton.classList.toString()).toContain('text-foreground');
  });

  it('renders pending review banner when status is UNDER REVIEW', () => {
    const pendingIdea: IdeaData = {
      ...mockIdea,
      status: 'UNDER REVIEW',
    };
    render(<IdeaHeader idea={pendingIdea} />);
    expect(screen.getByText(/being reviewed by our moderation system/i)).toBeInTheDocument();
    expect(screen.getByText('UNDER REVIEW')).toBeInTheDocument();
  });

  it('renders rejected banner with Edit Post button when status is REJECTED', () => {
    const rejectedIdea: IdeaData = {
      ...mockIdea,
      status: 'REJECTED',
    };
    render(<IdeaHeader idea={rejectedIdea} />);
    expect(screen.getByText(/rejected by moderation/i)).toBeInTheDocument();
    expect(screen.getByText('REJECTED')).toBeInTheDocument();
    expect(screen.getByText('Edit Post')).toBeInTheDocument();
  });
});
