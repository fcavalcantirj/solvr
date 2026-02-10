import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { CommentsList } from './comments-list';
import type { CommentData } from '@/hooks/use-comments';

// Mock useComments hook
let mockComments: CommentData[] = [];
let mockLoading = false;
let mockError: string | null = null;
let mockTotal = 0;
let mockHasMore = false;
const mockRefetch = vi.fn();
const mockLoadMore = vi.fn();

vi.mock('@/hooks/use-comments', () => ({
  useComments: () => ({
    comments: mockComments,
    loading: mockLoading,
    error: mockError,
    total: mockTotal,
    hasMore: mockHasMore,
    page: 1,
    refetch: mockRefetch,
    loadMore: mockLoadMore,
  }),
}));

// Mock useCommentForm hook
const mockSubmit = vi.fn();
let mockFormContent = '';
const mockSetContent = vi.fn((val: string) => { mockFormContent = val; });

vi.mock('@/hooks/use-comment-form', () => ({
  useCommentForm: () => ({
    content: mockFormContent,
    setContent: mockSetContent,
    isSubmitting: false,
    error: null,
    submit: mockSubmit,
  }),
}));

// Mock useAuth hook
let mockUser: { id: string; type: string } | null = { id: 'user-1', type: 'human' };
let mockIsAuthenticated = true;

vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => ({
    user: mockUser,
    isAuthenticated: mockIsAuthenticated,
    isLoading: false,
  }),
}));

// Mock ReportModal
let capturedReportModalProps: Record<string, unknown> = {};
vi.mock('@/components/ui/report-modal', () => ({
  ReportModal: (props: Record<string, unknown>) => {
    capturedReportModalProps = props;
    return props.isOpen ? <div data-testid="report-modal">ReportModal</div> : null;
  },
}));

// Mock api methods
const mockDeleteComment = vi.fn().mockResolvedValue(undefined);
let mockClaimedAgents: { id: string }[] = [];
const mockGetUserAgents = vi.fn().mockImplementation(() =>
  Promise.resolve({ data: mockClaimedAgents, meta: { total: mockClaimedAgents.length, page: 1, per_page: 20 } })
);
vi.mock('@/lib/api', () => ({
  api: {
    deleteComment: (...args: unknown[]) => mockDeleteComment(...args),
    getUserAgents: (...args: unknown[]) => mockGetUserAgents(...args),
  },
}));

const agentComment: CommentData = {
  id: 'comment-1',
  targetType: 'post',
  targetId: 'post-123',
  content: 'Agent comment here',
  author: {
    id: 'agent-1',
    type: 'ai',
    displayName: 'agent_Phil',
    avatarUrl: null,
  },
  createdAt: '2026-02-09T12:50:10Z',
  time: '5m ago',
};

const humanComment: CommentData = {
  id: 'comment-2',
  targetType: 'post',
  targetId: 'post-123',
  content: 'Human comment here',
  author: {
    id: 'user-1',
    type: 'human',
    displayName: 'Felipe',
    avatarUrl: 'https://example.com/avatar.jpg',
  },
  createdAt: '2026-02-09T13:00:00Z',
  time: '2m ago',
};

describe('CommentsList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockComments = [];
    mockLoading = false;
    mockError = null;
    mockTotal = 0;
    mockHasMore = false;
    mockFormContent = '';
    mockUser = { id: 'user-1', type: 'human' };
    mockIsAuthenticated = true;
    capturedReportModalProps = {};
    mockClaimedAgents = [];
  });

  it('renders loading state', () => {
    mockLoading = true;

    render(<CommentsList targetType="post" targetId="post-123" />);

    // Should show some loading indicator
    expect(screen.getByText('COMMENTS')).toBeDefined();
  });

  it('renders empty state with no comments', () => {
    mockComments = [];
    mockTotal = 0;

    render(<CommentsList targetType="post" targetId="post-123" />);

    expect(screen.getByText(/no comments yet/i)).toBeDefined();
  });

  it('renders comments with author info', () => {
    mockComments = [agentComment, humanComment];
    mockTotal = 2;

    render(<CommentsList targetType="post" targetId="post-123" />);

    // Check agent comment renders
    expect(screen.getByText('Agent comment here')).toBeDefined();
    expect(screen.getByText('agent_Phil')).toBeDefined();
    expect(screen.getByText('[AI]')).toBeDefined();

    // Check human comment renders
    expect(screen.getByText('Human comment here')).toBeDefined();
    expect(screen.getByText('Felipe')).toBeDefined();
    expect(screen.getByText('[HUMAN]')).toBeDefined();
  });

  it('renders comment count in header', () => {
    mockComments = [agentComment, humanComment];
    mockTotal = 2;

    render(<CommentsList targetType="post" targetId="post-123" />);

    expect(screen.getByText('COMMENTS (2)')).toBeDefined();
  });

  it('renders time for comments', () => {
    mockComments = [agentComment];
    mockTotal = 1;

    render(<CommentsList targetType="post" targetId="post-123" />);

    expect(screen.getByText('5m ago')).toBeDefined();
  });

  it('renders comment input for authenticated users', () => {
    mockComments = [];
    mockTotal = 0;

    render(<CommentsList targetType="post" targetId="post-123" />);

    expect(screen.getByPlaceholderText('Add a comment...')).toBeDefined();
  });

  it('renders Load more button when hasMore is true', () => {
    mockComments = [agentComment];
    mockTotal = 5;
    mockHasMore = true;

    render(<CommentsList targetType="post" targetId="post-123" />);

    const loadMoreButton = screen.getByText(/load more/i);
    expect(loadMoreButton).toBeDefined();
  });

  it('calls loadMore when Load more button is clicked', () => {
    mockComments = [agentComment];
    mockTotal = 5;
    mockHasMore = true;

    render(<CommentsList targetType="post" targetId="post-123" />);

    fireEvent.click(screen.getByText(/load more/i));
    expect(mockLoadMore).toHaveBeenCalled();
  });

  it('renders error state', () => {
    mockError = 'Failed to load comments';

    render(<CommentsList targetType="post" targetId="post-123" />);

    expect(screen.getByText('Failed to load comments')).toBeDefined();
  });

  it('shows FLAG button on comments for authenticated users', () => {
    mockComments = [agentComment];
    mockTotal = 1;

    render(<CommentsList targetType="post" targetId="post-123" />);

    expect(screen.getByText('FLAG')).toBeDefined();
  });

  it('opens ReportModal when FLAG is clicked', () => {
    mockComments = [agentComment];
    mockTotal = 1;

    render(<CommentsList targetType="post" targetId="post-123" />);

    fireEvent.click(screen.getByText('FLAG'));

    expect(screen.getByTestId('report-modal')).toBeDefined();
    expect(capturedReportModalProps.targetType).toBe('comment');
    expect(capturedReportModalProps.targetId).toBe('comment-1');
  });

  it('shows DELETE button only on own comments', () => {
    // user-1 is the current user (set in mockUser)
    mockComments = [agentComment, humanComment];
    mockTotal = 2;

    render(<CommentsList targetType="post" targetId="post-123" />);

    // humanComment has author.id 'user-1' which matches mockUser.id
    const deleteButtons = screen.getAllByText('DELETE');
    expect(deleteButtons).toHaveLength(1);
  });

  it('does NOT show DELETE on other users comments', () => {
    mockUser = { id: 'other-user', type: 'human' };
    mockComments = [agentComment, humanComment];
    mockTotal = 2;

    render(<CommentsList targetType="post" targetId="post-123" />);

    expect(screen.queryByText('DELETE')).toBeNull();
  });

  it('calls deleteComment and refetch when DELETE is clicked', async () => {
    mockComments = [humanComment];
    mockTotal = 1;

    render(<CommentsList targetType="post" targetId="post-123" />);

    fireEvent.click(screen.getByText('DELETE'));

    await waitFor(() => {
      expect(mockDeleteComment).toHaveBeenCalledWith('comment-2');
      expect(mockRefetch).toHaveBeenCalled();
    });
  });

  it('shows DELETE on agent comment when user owns the agent', async () => {
    mockComments = [agentComment];
    mockTotal = 1;
    mockUser = { id: 'user-1', type: 'human' };
    mockClaimedAgents = [{ id: 'agent-1' }];

    render(<CommentsList targetType="post" targetId="post-123" />);

    await waitFor(() => {
      expect(screen.getByText('DELETE')).toBeDefined();
    });
  });

  it('does NOT show DELETE on agent comment when user does NOT own the agent', async () => {
    mockComments = [agentComment];
    mockTotal = 1;
    mockUser = { id: 'user-1', type: 'human' };
    mockClaimedAgents = [];

    render(<CommentsList targetType="post" targetId="post-123" />);

    // Wait for claimed agents fetch to complete
    await waitFor(() => {
      expect(mockGetUserAgents).toHaveBeenCalled();
    });

    expect(screen.queryByText('DELETE')).toBeNull();
  });

  it('shows FLAG but not DELETE for unauthenticated users', () => {
    mockIsAuthenticated = false;
    mockUser = null;
    mockComments = [agentComment];
    mockTotal = 1;

    render(<CommentsList targetType="post" targetId="post-123" />);

    // FLAG is always visible
    expect(screen.getByText('FLAG')).toBeDefined();
    // DELETE requires ownership (which requires auth)
    expect(screen.queryByText('DELETE')).toBeNull();
  });
});
