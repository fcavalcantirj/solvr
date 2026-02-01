/**
 * Tests for CommentThread component
 * Per PRD requirement: Create CommentThread with list comments,
 * add new comment form, and delete functionality for owner
 */

import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import CommentThread from '../components/CommentThread';

// Mock the API module
jest.mock('../lib/api', () => ({
  api: {
    get: jest.fn(),
    post: jest.fn(),
    delete: jest.fn(),
  },
  getAuthToken: jest.fn(),
}));

// Import after mocks
import { api, getAuthToken } from '../lib/api';

const mockApiGet = api.get as jest.Mock;
const mockApiPost = api.post as jest.Mock;
const mockApiDelete = api.delete as jest.Mock;
const mockGetAuthToken = getAuthToken as jest.Mock;

// Sample comment data matching backend models
const mockComments = [
  {
    id: 'comment-1',
    target_type: 'approach',
    target_id: 'approach-123',
    author_type: 'human',
    author_id: 'user-1',
    content: 'Great approach! This is a helpful comment.',
    created_at: '2026-01-15T10:00:00Z',
    author: {
      id: 'user-1',
      type: 'human',
      display_name: 'Alice Smith',
      avatar_url: 'https://example.com/alice.png',
    },
  },
  {
    id: 'comment-2',
    target_type: 'approach',
    target_id: 'approach-123',
    author_type: 'agent',
    author_id: 'agent-1',
    content: 'I agree with this analysis.',
    created_at: '2026-01-15T11:00:00Z',
    author: {
      id: 'agent-1',
      type: 'agent',
      display_name: 'Claude Assistant',
    },
  },
];

describe('CommentThread', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockGetAuthToken.mockReturnValue(null);
  });

  describe('rendering comments list', () => {
    it('renders loading state initially', () => {
      mockApiGet.mockReturnValue(new Promise(() => {})); // Never resolves
      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );
      expect(screen.getByText(/loading/i)).toBeInTheDocument();
    });

    it('renders comments when data is loaded', async () => {
      mockApiGet.mockResolvedValue(mockComments);

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByText('Great approach! This is a helpful comment.')).toBeInTheDocument();
      });
      expect(screen.getByText('I agree with this analysis.')).toBeInTheDocument();
    });

    it('renders author names', async () => {
      mockApiGet.mockResolvedValue(mockComments);

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByText('Alice Smith')).toBeInTheDocument();
      });
      expect(screen.getByText('Claude Assistant')).toBeInTheDocument();
    });

    it('renders empty state when no comments', async () => {
      mockApiGet.mockResolvedValue([]);

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByText(/no comments yet/i)).toBeInTheDocument();
      });
    });

    it('renders error state on API failure', async () => {
      mockApiGet.mockRejectedValue(new Error('Failed to load'));

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByText(/error/i)).toBeInTheDocument();
      });
    });

    it('calls API with correct path for approach target', async () => {
      mockApiGet.mockResolvedValue([]);

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(
          '/v1/approaches/approach-123/comments'
        );
      });
    });

    it('calls API with correct path for answer target', async () => {
      mockApiGet.mockResolvedValue([]);

      render(
        <CommentThread targetType="answer" targetId="answer-456" />
      );

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(
          '/v1/answers/answer-456/comments'
        );
      });
    });

    it('calls API with correct path for response target', async () => {
      mockApiGet.mockResolvedValue([]);

      render(
        <CommentThread targetType="response" targetId="response-789" />
      );

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(
          '/v1/responses/response-789/comments'
        );
      });
    });
  });

  describe('new comment form', () => {
    it('shows comment form when user is authenticated', async () => {
      mockApiGet.mockResolvedValue([]);
      mockGetAuthToken.mockReturnValue('mock-token');

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByPlaceholderText(/add a comment/i)).toBeInTheDocument();
      });
    });

    it('hides comment form when user is not authenticated', async () => {
      mockApiGet.mockResolvedValue([]);
      mockGetAuthToken.mockReturnValue(null);

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.queryByPlaceholderText(/add a comment/i)).not.toBeInTheDocument();
      });
    });

    it('shows login prompt when not authenticated', async () => {
      mockApiGet.mockResolvedValue([]);
      mockGetAuthToken.mockReturnValue(null);

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByText(/log in to comment/i)).toBeInTheDocument();
      });
    });

    it('submits new comment on form submit', async () => {
      const user = userEvent.setup();
      mockApiGet.mockResolvedValue([]);
      mockGetAuthToken.mockReturnValue('mock-token');
      mockApiPost.mockResolvedValue({
        id: 'new-comment-123',
        content: 'This is my new comment',
        author_type: 'human',
        author_id: 'user-1',
        target_type: 'approach',
        target_id: 'approach-123',
        created_at: new Date().toISOString(),
        author: {
          id: 'user-1',
          type: 'human',
          display_name: 'Current User',
        },
      });

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByPlaceholderText(/add a comment/i)).toBeInTheDocument();
      });

      const textarea = screen.getByPlaceholderText(/add a comment/i);
      await user.type(textarea, 'This is my new comment');

      const submitButton = screen.getByRole('button', { name: /post comment/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(mockApiPost).toHaveBeenCalledWith(
          '/v1/approaches/approach-123/comments',
          { content: 'This is my new comment' }
        );
      });
    });

    it('clears form after successful submission', async () => {
      const user = userEvent.setup();
      mockApiGet.mockResolvedValue([]);
      mockGetAuthToken.mockReturnValue('mock-token');
      mockApiPost.mockResolvedValue({
        id: 'new-comment-123',
        content: 'Test comment',
        author_type: 'human',
        author_id: 'user-1',
        target_type: 'approach',
        target_id: 'approach-123',
        created_at: new Date().toISOString(),
        author: {
          id: 'user-1',
          type: 'human',
          display_name: 'Current User',
        },
      });

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByPlaceholderText(/add a comment/i)).toBeInTheDocument();
      });

      const textarea = screen.getByPlaceholderText(/add a comment/i) as HTMLTextAreaElement;
      await user.type(textarea, 'Test comment');

      const submitButton = screen.getByRole('button', { name: /post comment/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(textarea.value).toBe('');
      });
    });

    it('shows new comment in list after submission', async () => {
      const user = userEvent.setup();
      mockApiGet.mockResolvedValue([]);
      mockGetAuthToken.mockReturnValue('mock-token');
      mockApiPost.mockResolvedValue({
        id: 'new-comment-123',
        content: 'My brand new comment',
        author_type: 'human',
        author_id: 'user-1',
        target_type: 'approach',
        target_id: 'approach-123',
        created_at: new Date().toISOString(),
        author: {
          id: 'user-1',
          type: 'human',
          display_name: 'Current User',
        },
      });

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByPlaceholderText(/add a comment/i)).toBeInTheDocument();
      });

      const textarea = screen.getByPlaceholderText(/add a comment/i);
      await user.type(textarea, 'My brand new comment');

      const submitButton = screen.getByRole('button', { name: /post comment/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText('My brand new comment')).toBeInTheDocument();
      });
    });

    it('disables submit button when textarea is empty', async () => {
      mockApiGet.mockResolvedValue([]);
      mockGetAuthToken.mockReturnValue('mock-token');

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByPlaceholderText(/add a comment/i)).toBeInTheDocument();
      });

      const submitButton = screen.getByRole('button', { name: /post comment/i });
      expect(submitButton).toBeDisabled();
    });

    it('disables submit button while submitting', async () => {
      const user = userEvent.setup();
      mockApiGet.mockResolvedValue([]);
      mockGetAuthToken.mockReturnValue('mock-token');
      mockApiPost.mockImplementation(() => new Promise(() => {})); // Never resolves

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByPlaceholderText(/add a comment/i)).toBeInTheDocument();
      });

      const textarea = screen.getByPlaceholderText(/add a comment/i);
      await user.type(textarea, 'Test comment');

      const submitButton = screen.getByRole('button', { name: /post comment/i });
      await user.click(submitButton);

      expect(submitButton).toBeDisabled();
    });

    it('shows error message on submission failure', async () => {
      const user = userEvent.setup();
      mockApiGet.mockResolvedValue([]);
      mockGetAuthToken.mockReturnValue('mock-token');
      mockApiPost.mockRejectedValue(new Error('Failed to post comment'));

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByPlaceholderText(/add a comment/i)).toBeInTheDocument();
      });

      const textarea = screen.getByPlaceholderText(/add a comment/i);
      await user.type(textarea, 'Test comment');

      const submitButton = screen.getByRole('button', { name: /post comment/i });
      await user.click(submitButton);

      await waitFor(() => {
        expect(screen.getByText(/failed/i)).toBeInTheDocument();
      });
    });
  });

  describe('delete functionality', () => {
    const currentUserId = 'user-1';

    it('shows delete button for comment owner', async () => {
      mockApiGet.mockResolvedValue(mockComments);
      mockGetAuthToken.mockReturnValue('mock-token');

      render(
        <CommentThread
          targetType="approach"
          targetId="approach-123"
          currentUserId={currentUserId}
        />
      );

      await waitFor(() => {
        expect(screen.getByText('Great approach! This is a helpful comment.')).toBeInTheDocument();
      });

      // Should show delete button for comment-1 (authored by user-1)
      const deleteButtons = screen.getAllByRole('button', { name: /delete/i });
      expect(deleteButtons.length).toBeGreaterThan(0);
    });

    it('does not show delete button for comments by others', async () => {
      mockApiGet.mockResolvedValue(mockComments);
      mockGetAuthToken.mockReturnValue('mock-token');

      render(
        <CommentThread
          targetType="approach"
          targetId="approach-123"
          currentUserId="different-user"
        />
      );

      await waitFor(() => {
        expect(screen.getByText('Great approach! This is a helpful comment.')).toBeInTheDocument();
      });

      // Should not show delete buttons for comments not owned by current user
      const deleteButtons = screen.queryAllByRole('button', { name: /delete/i });
      expect(deleteButtons.length).toBe(0);
    });

    it('calls delete API when delete button is clicked', async () => {
      const user = userEvent.setup();
      mockApiGet.mockResolvedValue(mockComments);
      mockGetAuthToken.mockReturnValue('mock-token');
      mockApiDelete.mockResolvedValue(undefined);

      render(
        <CommentThread
          targetType="approach"
          targetId="approach-123"
          currentUserId={currentUserId}
        />
      );

      await waitFor(() => {
        expect(screen.getByText('Great approach! This is a helpful comment.')).toBeInTheDocument();
      });

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      await waitFor(() => {
        expect(mockApiDelete).toHaveBeenCalledWith('/v1/comments/comment-1');
      });
    });

    it('removes comment from list after successful deletion', async () => {
      const user = userEvent.setup();
      mockApiGet.mockResolvedValue(mockComments);
      mockGetAuthToken.mockReturnValue('mock-token');
      mockApiDelete.mockResolvedValue(undefined);

      render(
        <CommentThread
          targetType="approach"
          targetId="approach-123"
          currentUserId={currentUserId}
        />
      );

      await waitFor(() => {
        expect(screen.getByText('Great approach! This is a helpful comment.')).toBeInTheDocument();
      });

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      await waitFor(() => {
        expect(screen.queryByText('Great approach! This is a helpful comment.')).not.toBeInTheDocument();
      });
    });

    it('shows error on delete failure', async () => {
      const user = userEvent.setup();
      mockApiGet.mockResolvedValue(mockComments);
      mockGetAuthToken.mockReturnValue('mock-token');
      mockApiDelete.mockRejectedValue(new Error('Delete failed'));

      render(
        <CommentThread
          targetType="approach"
          targetId="approach-123"
          currentUserId={currentUserId}
        />
      );

      await waitFor(() => {
        expect(screen.getByText('Great approach! This is a helpful comment.')).toBeInTheDocument();
      });

      const deleteButton = screen.getByRole('button', { name: /delete/i });
      await user.click(deleteButton);

      await waitFor(() => {
        expect(screen.getByText(/failed to delete/i)).toBeInTheDocument();
      });
    });

    it('shows delete button for admin user on any comment', async () => {
      mockApiGet.mockResolvedValue(mockComments);
      mockGetAuthToken.mockReturnValue('mock-token');

      render(
        <CommentThread
          targetType="approach"
          targetId="approach-123"
          currentUserId="admin-user"
          isAdmin={true}
        />
      );

      await waitFor(() => {
        expect(screen.getByText('Great approach! This is a helpful comment.')).toBeInTheDocument();
      });

      // Admin should see delete buttons for all comments
      const deleteButtons = screen.getAllByRole('button', { name: /delete/i });
      expect(deleteButtons.length).toBe(2);
    });
  });

  describe('accessibility', () => {
    it('has accessible form elements', async () => {
      mockApiGet.mockResolvedValue([]);
      mockGetAuthToken.mockReturnValue('mock-token');

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        const textarea = screen.getByPlaceholderText(/add a comment/i);
        expect(textarea).toHaveAttribute('aria-label');
      });
    });

    it('comment list has proper structure', async () => {
      mockApiGet.mockResolvedValue(mockComments);

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByRole('list')).toBeInTheDocument();
      });
    });
  });

  describe('comment count', () => {
    it('displays comment count in header', async () => {
      mockApiGet.mockResolvedValue(mockComments);

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByText(/2 comments/i)).toBeInTheDocument();
      });
    });

    it('displays singular when one comment', async () => {
      mockApiGet.mockResolvedValue([mockComments[0]]);

      render(
        <CommentThread targetType="approach" targetId="approach-123" />
      );

      await waitFor(() => {
        expect(screen.getByText(/1 comment$/i)).toBeInTheDocument();
      });
    });
  });
});
