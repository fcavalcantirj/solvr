import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ProblemHeader } from './problem-header';
import type { ProblemData } from '@/hooks/use-problem';

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

// Mock VoteButton to avoid its dependencies
vi.mock('@/components/ui/vote-button', () => ({
  VoteButton: ({ postId, initialScore }: { postId: string; initialScore: number }) => (
    <div data-testid="vote-button" data-post-id={postId} data-score={initialScore}>
      VoteButton
    </div>
  ),
}));

// Mock CopyResearchButton
vi.mock('./copy-research-button', () => ({
  CopyResearchButton: () => <div data-testid="copy-research-button">CopyResearchButton</div>,
}));

// Mock next/link
vi.mock('next/link', () => ({
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const mockProblem: ProblemData = {
  id: 'abc12345-6789-0000-1111-222233334444',
  title: 'Test Problem Title',
  description: 'Test description',
  status: 'OPEN',
  voteScore: 42,
  upvotes: 50,
  downvotes: 8,
  author: {
    id: 'user-1',
    type: 'human',
    displayName: 'TestUser',
  },
  tags: ['go', 'postgres'],
  createdAt: '2026-01-15T10:00:00Z',
  updatedAt: '2026-01-16T14:30:00Z',
  time: '2 days ago',
  approachesCount: 3,
  views: 100,
};

describe('ProblemHeader', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockShared = false;
    mockBookmarkedPosts = new Set<string>();
    // Mock window.location.origin
    Object.defineProperty(window, 'location', {
      value: { origin: 'https://solvr.dev', href: 'https://solvr.dev/problems/abc12345' },
      writable: true,
    });
  });

  it('clicking Share copies URL to clipboard', async () => {
    render(<ProblemHeader problem={mockProblem} />);

    const shareButton = screen.getByTestId('share-button');
    fireEvent.click(shareButton);

    expect(mockShare).toHaveBeenCalledWith(
      'Test Problem Title',
      `https://solvr.dev/problems/${mockProblem.id}`
    );
  });

  it('shows Check icon when share succeeds', () => {
    mockShared = true;
    render(<ProblemHeader problem={mockProblem} />);

    const shareButton = screen.getByTestId('share-button');
    // When shared is true, should show Check icon instead of Share2
    expect(shareButton.querySelector('svg')).toBeTruthy();
    // The Check icon from lucide-react - we verify the button has the shared visual state
    expect(shareButton.classList.toString()).toContain('text-emerald');
  });

  it('clicking Bookmark toggles bookmark state', async () => {
    render(<ProblemHeader problem={mockProblem} />);

    const bookmarkButton = screen.getByTestId('bookmark-button');
    fireEvent.click(bookmarkButton);

    expect(mockToggleBookmark).toHaveBeenCalledWith(mockProblem.id);
  });

  it('shows filled Bookmark icon when bookmarked', () => {
    mockBookmarkedPosts = new Set([mockProblem.id]);
    render(<ProblemHeader problem={mockProblem} />);

    const bookmarkButton = screen.getByTestId('bookmark-button');
    // The filled bookmark should have fill="currentColor"
    const svg = bookmarkButton.querySelector('svg');
    expect(svg).toBeTruthy();
    expect(bookmarkButton.classList.toString()).toContain('text-foreground');
  });

  it('renders problem title and status', () => {
    render(<ProblemHeader problem={mockProblem} />);
    expect(screen.getByText('Test Problem Title')).toBeTruthy();
    expect(screen.getByText('OPEN')).toBeTruthy();
  });

  it('renders VoteButton with correct props', () => {
    render(<ProblemHeader problem={mockProblem} />);
    const voteButton = screen.getByTestId('vote-button');
    expect(voteButton.getAttribute('data-post-id')).toBe(mockProblem.id);
    expect(voteButton.getAttribute('data-score')).toBe('42');
  });
});
