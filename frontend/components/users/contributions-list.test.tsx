import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ContributionsList } from './contributions-list';

// Mock next/link
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

// Mock the useContributions hook
vi.mock('@/hooks/use-contributions', () => ({
  useContributions: vi.fn(),
}));

import { useContributions } from '@/hooks/use-contributions';

const mockContributions = [
  {
    type: 'answer' as const,
    id: 'answer-1',
    parentId: 'question-1',
    parentTitle: 'How to use React hooks?',
    parentType: 'question',
    contentPreview: 'You can use useState and useEffect...',
    status: '',
    timestamp: '2d ago',
    createdAt: '2026-02-10T10:00:00Z',
  },
  {
    type: 'approach' as const,
    id: 'approach-1',
    parentId: 'problem-1',
    parentTitle: 'Fix async race condition',
    parentType: 'problem',
    contentPreview: 'Use mutex locks to prevent...',
    status: 'working',
    timestamp: '3d ago',
    createdAt: '2026-02-09T10:00:00Z',
  },
  {
    type: 'response' as const,
    id: 'response-1',
    parentId: 'idea-1',
    parentTitle: 'AI-powered code review',
    parentType: 'idea',
    contentPreview: 'This is a great idea because...',
    status: '',
    timestamp: '4d ago',
    createdAt: '2026-02-08T10:00:00Z',
  },
];

describe('ContributionsList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders answer cards with "Answered:" prefix and link to question', () => {
    vi.mocked(useContributions).mockReturnValue({
      contributions: mockContributions,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      loadMore: vi.fn(),
    });

    render(<ContributionsList userId="user-123" />);

    expect(screen.getByText(/Answered:/)).toBeInTheDocument();
    expect(screen.getByText('How to use React hooks?')).toBeInTheDocument();

    const answerLink = screen.getByText('How to use React hooks?').closest('a');
    expect(answerLink).toHaveAttribute('href', '/questions/question-1');
  });

  it('renders approach cards with "Approach for:" prefix and link to problem', () => {
    vi.mocked(useContributions).mockReturnValue({
      contributions: mockContributions,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      loadMore: vi.fn(),
    });

    render(<ContributionsList userId="user-123" />);

    expect(screen.getByText(/Approach for:/)).toBeInTheDocument();
    expect(screen.getByText('Fix async race condition')).toBeInTheDocument();

    const approachLink = screen.getByText('Fix async race condition').closest('a');
    expect(approachLink).toHaveAttribute('href', '/problems/problem-1');
  });

  it('renders response cards with "Response to:" prefix and link to idea', () => {
    vi.mocked(useContributions).mockReturnValue({
      contributions: mockContributions,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      loadMore: vi.fn(),
    });

    render(<ContributionsList userId="user-123" />);

    expect(screen.getByText(/Response to:/)).toBeInTheDocument();
    expect(screen.getByText('AI-powered code review')).toBeInTheDocument();

    const responseLink = screen.getByText('AI-powered code review').closest('a');
    expect(responseLink).toHaveAttribute('href', '/ideas/idea-1');
  });

  it('renders type filter pills', () => {
    vi.mocked(useContributions).mockReturnValue({
      contributions: mockContributions,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      loadMore: vi.fn(),
    });

    render(<ContributionsList userId="user-123" />);

    expect(screen.getByText('ALL')).toBeInTheDocument();
    expect(screen.getByText('ANSWERS')).toBeInTheDocument();
    expect(screen.getByText('APPROACHES')).toBeInTheDocument();
    expect(screen.getByText('RESPONSES')).toBeInTheDocument();
  });

  it('clicking filter pill updates the type filter', () => {
    const mockUseContributions = vi.mocked(useContributions);
    mockUseContributions.mockReturnValue({
      contributions: mockContributions,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      loadMore: vi.fn(),
    });

    const { rerender } = render(<ContributionsList userId="user-123" />);

    // Initially called with no type filter
    expect(mockUseContributions).toHaveBeenCalledWith('user-123', { type: undefined });

    // Click ANSWERS filter
    fireEvent.click(screen.getByText('ANSWERS'));

    // Re-render to check new state
    rerender(<ContributionsList userId="user-123" />);

    // Should now be called with answers filter
    expect(mockUseContributions).toHaveBeenCalledWith('user-123', { type: 'answers' });
  });

  it('shows loading state', () => {
    vi.mocked(useContributions).mockReturnValue({
      contributions: [],
      loading: true,
      error: null,
      total: 0,
      hasMore: false,
      loadMore: vi.fn(),
    });

    render(<ContributionsList userId="user-123" />);

    // Should show a loading indicator
    expect(screen.getByText('Loading contributions...')).toBeInTheDocument();
  });

  it('shows empty state when no contributions', () => {
    vi.mocked(useContributions).mockReturnValue({
      contributions: [],
      loading: false,
      error: null,
      total: 0,
      hasMore: false,
      loadMore: vi.fn(),
    });

    render(<ContributionsList userId="user-123" />);

    expect(screen.getByText('No contributions yet')).toBeInTheDocument();
  });

  it('shows LOAD MORE button when hasMore is true', () => {
    const mockLoadMore = vi.fn();
    vi.mocked(useContributions).mockReturnValue({
      contributions: mockContributions,
      loading: false,
      error: null,
      total: 10,
      hasMore: true,
      loadMore: mockLoadMore,
    });

    render(<ContributionsList userId="user-123" />);

    const loadMoreBtn = screen.getByText('LOAD MORE');
    expect(loadMoreBtn).toBeInTheDocument();

    fireEvent.click(loadMoreBtn);
    expect(mockLoadMore).toHaveBeenCalledTimes(1);
  });

  it('hides LOAD MORE button when hasMore is false', () => {
    vi.mocked(useContributions).mockReturnValue({
      contributions: mockContributions,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      loadMore: vi.fn(),
    });

    render(<ContributionsList userId="user-123" />);

    expect(screen.queryByText('LOAD MORE')).not.toBeInTheDocument();
  });

  it('renders content preview for each contribution', () => {
    vi.mocked(useContributions).mockReturnValue({
      contributions: mockContributions,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      loadMore: vi.fn(),
    });

    render(<ContributionsList userId="user-123" />);

    expect(screen.getByText('You can use useState and useEffect...')).toBeInTheDocument();
    expect(screen.getByText('Use mutex locks to prevent...')).toBeInTheDocument();
    expect(screen.getByText('This is a great idea because...')).toBeInTheDocument();
  });
});
