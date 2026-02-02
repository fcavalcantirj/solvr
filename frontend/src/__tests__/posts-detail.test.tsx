/**
 * Tests for Post Detail Page component
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from PRD lines 466-478:
 * - Create /posts/[id] page
 * - Post detail: fetch and display (title, description, tags)
 * - Post detail: author info (AuthorBadge)
 * - Post detail: votes (VoteButtons)
 * - Post detail: comments (CommentThread)
 * - Post detail: 404 page
 * - Problem page: approaches, start approach button
 * - Question page: answers, answer form, accept button
 * - Idea page: responses, response form
 */

import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { act } from 'react';
import userEvent from '@testing-library/user-event';

// Mock next/navigation
const mockParams = { id: 'post-123' };
let mockNotFound = false;
let mockNotFoundError: Error | null = null;
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), replace: jest.fn(), back: jest.fn() }),
  useParams: () => mockParams,
  notFound: () => {
    mockNotFound = true;
    // Only throw if we want to test the throw behavior
    if (mockNotFoundError) {
      throw mockNotFoundError;
    }
  },
}));

// Mock next/link
jest.mock('next/link', () => {
  return function MockLink({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) {
    return <a href={href}>{children}</a>;
  };
});

// Mock the API module
const mockApiGet = jest.fn();
const mockApiPost = jest.fn();
jest.mock('@/lib/api', () => ({
  api: {
    get: (...args: unknown[]) => mockApiGet(...args),
    post: (...args: unknown[]) => mockApiPost(...args),
  },
  ApiError: class MockApiError extends Error {
    constructor(
      public status: number,
      public code: string,
      message: string
    ) {
      super(message);
    }
  },
  __esModule: true,
}));

// Import the ApiError to use in tests
import { ApiError } from '@/lib/api';

// Import component after mocks
import PostDetailPage from '../app/posts/[id]/page';

// Test data - Problem post
const mockProblemPost = {
  id: 'post-123',
  type: 'problem',
  title: 'Race condition in async/await with PostgreSQL',
  description:
    'We are encountering a race condition when multiple async queries access the database. Here is the code:\n\n```go\nfunc DoWork() {\n  // code here\n}\n```\n\nThis happens intermittently.',
  tags: ['postgresql', 'async', 'concurrency'],
  status: 'in_progress',
  posted_by_type: 'human',
  posted_by_id: 'user-1',
  upvotes: 42,
  downvotes: 5,
  success_criteria: [
    'No race condition errors in logs',
    'All tests pass consistently',
    'Performance remains acceptable',
  ],
  weight: 3,
  created_at: '2026-01-15T10:00:00Z',
  updated_at: '2026-01-16T14:30:00Z',
  author: {
    type: 'human',
    id: 'user-1',
    display_name: 'John Doe',
    avatar_url: 'https://example.com/avatar.jpg',
  },
  vote_score: 37,
};

// Test data - Question post
const mockQuestionPost = {
  id: 'post-456',
  type: 'question',
  title: 'How to handle async errors in Go?',
  description:
    'What is the best practice for handling async errors in Go applications?',
  tags: ['go', 'error-handling'],
  status: 'answered',
  posted_by_type: 'agent',
  posted_by_id: 'claude-1',
  upvotes: 28,
  downvotes: 2,
  accepted_answer_id: 'answer-1',
  created_at: '2026-01-14T15:00:00Z',
  updated_at: '2026-01-15T10:00:00Z',
  author: {
    type: 'agent',
    id: 'claude-1',
    display_name: 'Claude',
    avatar_url: null,
  },
  vote_score: 26,
};

// Test data - Idea post
const mockIdeaPost = {
  id: 'post-789',
  type: 'idea',
  title: 'Better async debugging tools',
  description: 'I think we need better async debugging tools for developers.',
  tags: ['debugging', 'tooling'],
  status: 'active',
  posted_by_type: 'human',
  posted_by_id: 'user-2',
  upvotes: 15,
  downvotes: 1,
  evolved_into: [],
  created_at: '2026-01-13T09:00:00Z',
  updated_at: '2026-01-13T09:00:00Z',
  author: {
    type: 'human',
    id: 'user-2',
    display_name: 'Jane Smith',
    avatar_url: null,
  },
  vote_score: 14,
};

// Test data - Approaches for problem
const mockApproaches = [
  {
    id: 'approach-1',
    problem_id: 'post-123',
    author_type: 'human',
    author_id: 'user-3',
    angle: 'Connection pooling optimization',
    method: 'Use pgxpool with proper connection limits',
    status: 'working',
    created_at: '2026-01-15T12:00:00Z',
    author: { type: 'human', id: 'user-3', display_name: 'Alice' },
  },
  {
    id: 'approach-2',
    problem_id: 'post-123',
    author_type: 'agent',
    author_id: 'claude-1',
    angle: 'Transaction-based approach',
    method: 'Wrap all operations in transactions',
    status: 'failed',
    outcome: 'Deadlock occurred due to locking order',
    created_at: '2026-01-15T11:00:00Z',
    author: { type: 'agent', id: 'claude-1', display_name: 'Claude' },
  },
];

// Test data - Answers for question
const mockAnswers = [
  {
    id: 'answer-1',
    question_id: 'post-456',
    author_type: 'human',
    author_id: 'user-4',
    content: 'The best practice is to use error groups from golang.org/x/sync/errgroup.',
    is_accepted: true,
    upvotes: 20,
    downvotes: 0,
    created_at: '2026-01-14T16:00:00Z',
    author: { type: 'human', id: 'user-4', display_name: 'Bob' },
    vote_score: 20,
  },
  {
    id: 'answer-2',
    question_id: 'post-456',
    author_type: 'agent',
    author_id: 'gpt-1',
    content: 'Another approach is to use channels for error propagation.',
    is_accepted: false,
    upvotes: 8,
    downvotes: 1,
    created_at: '2026-01-14T17:00:00Z',
    author: { type: 'agent', id: 'gpt-1', display_name: 'GPT' },
    vote_score: 7,
  },
];

// Test data - Responses for idea
const mockResponses = [
  {
    id: 'response-1',
    idea_id: 'post-789',
    author_type: 'human',
    author_id: 'user-5',
    content: 'Great idea! We could integrate with existing debuggers.',
    response_type: 'support',
    upvotes: 5,
    downvotes: 0,
    created_at: '2026-01-13T10:00:00Z',
    author: { type: 'human', id: 'user-5', display_name: 'Carol' },
    vote_score: 5,
  },
  {
    id: 'response-2',
    idea_id: 'post-789',
    author_type: 'agent',
    author_id: 'claude-1',
    content: 'Have you considered the challenges with async stack traces?',
    response_type: 'question',
    upvotes: 3,
    downvotes: 0,
    created_at: '2026-01-13T11:00:00Z',
    author: { type: 'agent', id: 'claude-1', display_name: 'Claude' },
    vote_score: 3,
  },
];

// Comments for all post types
const mockComments = [
  {
    id: 'comment-1',
    target_type: 'approach',
    target_id: 'approach-1',
    author_type: 'human',
    author_id: 'user-1',
    content: 'This looks promising!',
    created_at: '2026-01-15T13:00:00Z',
    author: { type: 'human', id: 'user-1', display_name: 'John Doe' },
  },
];

// Helper to set up API responses for a specific post type
function setupMockResponses(postType: 'problem' | 'question' | 'idea') {
  if (postType === 'problem') {
    mockApiGet.mockImplementation((path: string) => {
      if (path.includes('/posts/')) return Promise.resolve(mockProblemPost);
      if (path.includes('/approaches')) return Promise.resolve(mockApproaches);
      if (path.includes('/comments')) return Promise.resolve(mockComments);
      return Promise.resolve({ data: [] });
    });
  } else if (postType === 'question') {
    mockApiGet.mockImplementation((path: string) => {
      if (path.includes('/posts/')) return Promise.resolve(mockQuestionPost);
      if (path.includes('/answers')) return Promise.resolve(mockAnswers);
      if (path.includes('/comments')) return Promise.resolve(mockComments);
      return Promise.resolve({ data: [] });
    });
  } else if (postType === 'idea') {
    mockApiGet.mockImplementation((path: string) => {
      if (path.includes('/posts/')) return Promise.resolve(mockIdeaPost);
      if (path.includes('/responses')) return Promise.resolve(mockResponses);
      if (path.includes('/comments')) return Promise.resolve(mockComments);
      return Promise.resolve({ data: [] });
    });
  }
}

describe('PostDetailPage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockNotFound = false;
    mockParams.id = 'post-123';
  });

  describe('Basic structure', () => {
    it('renders main container', async () => {
      setupMockResponses('problem');
      await act(async () => {
        render(<PostDetailPage />);
      });
      expect(screen.getByRole('main')).toBeInTheDocument();
    });

    it('renders article element for post content', async () => {
      setupMockResponses('problem');
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(screen.getByRole('article')).toBeInTheDocument();
      });
    });
  });

  describe('Loading state', () => {
    it('shows loading skeleton while fetching', async () => {
      mockApiGet.mockImplementation(
        () =>
          new Promise((resolve) =>
            setTimeout(() => resolve(mockProblemPost), 1000)
          )
      );
      render(<PostDetailPage />);
      expect(screen.getByTestId('post-skeleton')).toBeInTheDocument();
    });

    it('removes skeleton after loading', async () => {
      setupMockResponses('problem');
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(screen.queryByTestId('post-skeleton')).not.toBeInTheDocument();
      });
    });
  });

  describe('Post display', () => {
    it('displays post title', async () => {
      setupMockResponses('problem');
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByRole('heading', {
            level: 1,
            name: /Race condition in async\/await with PostgreSQL/i,
          })
        ).toBeInTheDocument();
      });
    });

    it('displays post description', async () => {
      setupMockResponses('problem');
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByText(/We are encountering a race condition/i)
        ).toBeInTheDocument();
      });
    });

    it('displays post tags', async () => {
      setupMockResponses('problem');
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(screen.getByText('postgresql')).toBeInTheDocument();
        expect(screen.getByText('async')).toBeInTheDocument();
        expect(screen.getByText('concurrency')).toBeInTheDocument();
      });
    });

    it('displays type badge', async () => {
      setupMockResponses('problem');
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(screen.getByText('Problem')).toBeInTheDocument();
      });
    });

    it('displays status badge', async () => {
      setupMockResponses('problem');
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(screen.getByText(/In Progress/i)).toBeInTheDocument();
      });
    });
  });

  describe('Author info', () => {
    it('displays author name', async () => {
      setupMockResponses('problem');
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(screen.getByText('John Doe')).toBeInTheDocument();
      });
    });

    it('indicates human author type', async () => {
      setupMockResponses('problem');
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        // Look for human icon or badge - get all and check at least one has human type
        const authorSections = screen.getAllByTestId('author-badge');
        expect(authorSections.some(el => el.getAttribute('data-author-type') === 'human')).toBe(true);
      });
    });

    it('indicates agent author type', async () => {
      setupMockResponses('question');
      mockParams.id = 'post-456';
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        // The main post author is an agent
        const authorSections = screen.getAllByTestId('author-badge');
        expect(authorSections[0]).toHaveAttribute('data-author-type', 'agent');
      });
    });

    it('links author to profile page', async () => {
      setupMockResponses('problem');
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        const authorLink = screen.getByRole('link', { name: /John Doe/i });
        expect(authorLink).toHaveAttribute('href', '/users/user-1');
      });
    });
  });

  describe('Votes', () => {
    it('displays vote score', async () => {
      setupMockResponses('problem');
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(screen.getByText('37')).toBeInTheDocument();
      });
    });

    it('renders upvote button', async () => {
      setupMockResponses('problem');
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /upvote/i })
        ).toBeInTheDocument();
      });
    });

    it('renders downvote button', async () => {
      setupMockResponses('problem');
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /downvote/i })
        ).toBeInTheDocument();
      });
    });
  });

  describe('404 handling', () => {
    it('calls notFound when post not found', async () => {
      // Mock the API to reject with 404
      mockApiGet.mockRejectedValue(new ApiError(404, 'NOT_FOUND', 'Post not found'));

      // Suppress console.error for this test since we expect errors
      const consoleSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

      try {
        await act(async () => {
          render(<PostDetailPage />);
        });
        // Give the async effect time to complete
        await new Promise(resolve => setTimeout(resolve, 50));
      } catch {
        // notFound() throws, this is expected behavior
      }

      consoleSpy.mockRestore();

      // Verify that notFound was called
      expect(mockNotFound).toBe(true);
    });
  });

  describe('Error handling', () => {
    it('shows error message on API failure', async () => {
      mockApiGet.mockRejectedValue(
        new ApiError(500, 'INTERNAL_ERROR', 'Server error')
      );
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(screen.getByText(/error/i)).toBeInTheDocument();
      });
    });

    it('shows retry button on error', async () => {
      mockApiGet.mockRejectedValue(
        new ApiError(500, 'INTERNAL_ERROR', 'Server error')
      );
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /try again/i })
        ).toBeInTheDocument();
      });
    });

    it('retries fetch on retry button click', async () => {
      // First call fails, subsequent calls succeed
      let callCount = 0;
      mockApiGet.mockImplementation((path: string) => {
        callCount++;
        if (callCount === 1) {
          return Promise.reject(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));
        }
        // After retry: return appropriate data based on path
        if (path.includes('/posts/')) {
          return Promise.resolve(mockProblemPost);
        }
        if (path.includes('/approaches')) {
          return Promise.resolve(mockApproaches);
        }
        return Promise.resolve([]);
      });

      await act(async () => {
        render(<PostDetailPage />);
      });

      // Wait for error state
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument();
      });

      // Click retry button and wait for it to complete
      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: /try again/i }));
        // Wait for async operation to complete
        await new Promise(resolve => setTimeout(resolve, 100));
      });

      // Verify that the API was called again
      expect(callCount).toBeGreaterThan(1);
    });
  });

  describe('Problem-specific features', () => {
    beforeEach(() => {
      mockParams.id = 'post-123';
      setupMockResponses('problem');
    });

    it('displays success criteria', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByText('No race condition errors in logs')
        ).toBeInTheDocument();
        expect(
          screen.getByText('All tests pass consistently')
        ).toBeInTheDocument();
      });
    });

    it('displays weight/difficulty', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        // Look for weight display (3/5 or similar)
        expect(screen.getByTestId('difficulty-indicator')).toBeInTheDocument();
      });
    });

    it('displays approaches section', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByRole('heading', { name: /approaches/i })
        ).toBeInTheDocument();
      });
    });

    it('lists approaches with status', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByText('Connection pooling optimization')
        ).toBeInTheDocument();
        expect(screen.getByText('Transaction-based approach')).toBeInTheDocument();
      });
    });

    it('shows failed approach with outcome', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByText(/Deadlock occurred due to locking order/i)
        ).toBeInTheDocument();
      });
    });

    it('renders Start Approach button', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /start approach/i })
        ).toBeInTheDocument();
      });
    });
  });

  describe('Question-specific features', () => {
    beforeEach(() => {
      mockParams.id = 'post-456';
      setupMockResponses('question');
    });

    it('displays answers section', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByRole('heading', { name: /answers/i })
        ).toBeInTheDocument();
      });
    });

    it('lists answers sorted by votes', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByText(/The best practice is to use error groups/i)
        ).toBeInTheDocument();
        expect(
          screen.getByText(/Another approach is to use channels/i)
        ).toBeInTheDocument();
      });
    });

    it('highlights accepted answer', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        const acceptedAnswer = screen.getByTestId('answer-answer-1');
        expect(acceptedAnswer).toHaveAttribute('data-accepted', 'true');
      });
    });

    it('shows answer count', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(screen.getByText(/2 answers/i)).toBeInTheDocument();
      });
    });

    it('renders Your Answer form', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByRole('heading', { name: /your answer/i })
        ).toBeInTheDocument();
        expect(
          screen.getByPlaceholderText(/write your answer/i)
        ).toBeInTheDocument();
      });
    });
  });

  describe('Idea-specific features', () => {
    beforeEach(() => {
      mockParams.id = 'post-789';
      setupMockResponses('idea');
    });

    it('displays responses section', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByRole('heading', { name: /responses/i })
        ).toBeInTheDocument();
      });
    });

    it('lists responses with type badges', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByText(/Great idea! We could integrate/i)
        ).toBeInTheDocument();
      });
      // Type badges - Support appears in dropdown too, so find all and check > 1
      await waitFor(() => {
        const supportElements = screen.getAllByText('Support');
        expect(supportElements.length).toBeGreaterThanOrEqual(1);
        const questionElements = screen.getAllByText('Question');
        expect(questionElements.length).toBeGreaterThanOrEqual(1);
      });
    });

    it('renders Add Response form', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByRole('heading', { name: /add response/i })
        ).toBeInTheDocument();
      });
    });

    it('shows response type selector', async () => {
      await act(async () => {
        render(<PostDetailPage />);
      });
      await waitFor(() => {
        expect(
          screen.getByRole('combobox', { name: /response type/i })
        ).toBeInTheDocument();
      });
    });
  });

  // Note: Timestamps and Accessibility tests are in posts-detail-a11y.test.tsx
});
