/**
 * Tests for Post Detail Page - Accessibility and Timestamps
 * Split from posts-detail.test.tsx to keep file sizes under 800 lines
 */

import { render, screen, waitFor } from '@testing-library/react';
import { act } from 'react';

// Mock next/navigation
const mockParams = { id: 'post-123' };
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), replace: jest.fn(), back: jest.fn() }),
  useParams: () => mockParams,
  notFound: () => {},
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

// Import component after mocks
import PostDetailPage from '../app/posts/[id]/page';

// Test data - Problem post
const mockProblemPost = {
  id: 'post-123',
  type: 'problem',
  title: 'Race condition in async/await with PostgreSQL',
  description:
    'We are encountering a race condition when multiple async queries access the database.',
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
];

const mockComments = [
  {
    id: 'comment-1',
    target_type: 'post',
    target_id: 'post-123',
    author_type: 'human',
    author_id: 'user-5',
    content: 'Have you tried profiling the database connections?',
    created_at: '2026-01-15T13:00:00Z',
    author: { type: 'human', id: 'user-5', display_name: 'Charlie' },
  },
];

// Helper to setup mock responses
function setupMockResponses() {
  mockApiGet.mockImplementation((path: string) => {
    if (path === '/v1/posts/post-123') {
      return Promise.resolve(mockProblemPost);
    }
    if (path === '/v1/problems/post-123/approaches') {
      return Promise.resolve(mockApproaches);
    }
    if (path === '/v1/posts/post-123/comments') {
      return Promise.resolve(mockComments);
    }
    return Promise.resolve({});
  });
}

describe('Post Detail Page - Timestamps', () => {
  beforeEach(() => {
    mockParams.id = 'post-123';
    jest.clearAllMocks();
    setupMockResponses();
  });

  it('displays created date', async () => {
    await act(async () => {
      render(<PostDetailPage />);
    });
    await waitFor(() => {
      // Should show relative or formatted date
      expect(screen.getByText(/Jan 15, 2026/i)).toBeInTheDocument();
    });
  });
});

describe('Post Detail Page - Accessibility', () => {
  beforeEach(() => {
    mockParams.id = 'post-123';
    jest.clearAllMocks();
    setupMockResponses();
  });

  it('has proper heading hierarchy', async () => {
    await act(async () => {
      render(<PostDetailPage />);
    });
    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument();
      expect(
        screen.getAllByRole('heading', { level: 2 }).length
      ).toBeGreaterThan(0);
    });
  });

  it('has accessible vote buttons', async () => {
    await act(async () => {
      render(<PostDetailPage />);
    });
    await waitFor(() => {
      const upvoteBtn = screen.getByRole('button', { name: /upvote/i });
      const downvoteBtn = screen.getByRole('button', { name: /downvote/i });
      expect(upvoteBtn).toHaveAttribute('aria-label');
      expect(downvoteBtn).toHaveAttribute('aria-label');
    });
  });

  it('uses semantic article element', async () => {
    await act(async () => {
      render(<PostDetailPage />);
    });
    await waitFor(() => {
      expect(screen.getByRole('article')).toBeInTheDocument();
    });
  });
});
