/**
 * Tests for Feed Page component
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from PRD lines 498-501:
 * - Create /feed page
 * - Feed: sort options (Add sort dropdown (latest, top, hot))
 * - Feed: type filter (Filter by all, problems, questions, ideas)
 * - Feed: pagination (Add load more / infinite scroll)
 *
 * Per SPEC.md Part 4.4:
 * - Type: All | Problems | Questions | Ideas
 * - Status: All | Open | Solved/Answered | Stuck
 * - Sort: Newest | Trending | Most Voted | Needs Help
 */

import { render, screen, waitFor, fireEvent } from '@testing-library/react';

// Track router push calls
const mockPush = jest.fn();
const mockReplace = jest.fn();

// Mock next/navigation
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace, back: jest.fn() }),
  useSearchParams: () => new URLSearchParams(),
  redirect: jest.fn(),
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
jest.mock('@/lib/api', () => ({
  api: {
    get: (...args: unknown[]) => mockApiGet(...args),
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

import { ApiError } from '@/lib/api';

// Mock useAuth hook
const mockUser = {
  id: 'user-123',
  username: 'johndoe',
  display_name: 'John Doe',
  email: 'john@example.com',
};
let mockAuthUser: typeof mockUser | null = mockUser;
let mockAuthLoading = false;

jest.mock('@/hooks/useAuth', () => ({
  useAuth: () => ({
    user: mockAuthUser,
    isLoading: mockAuthLoading,
  }),
  __esModule: true,
}));

// Import component after mocks
import FeedPage from '../app/feed/page';

// Test data - Posts for the feed
const mockPosts = [
  {
    id: 'post-1',
    type: 'problem',
    title: 'Race condition in async PostgreSQL queries',
    description: 'When running multiple async queries, I get race conditions...',
    tags: ['postgresql', 'async', 'go'],
    posted_by_type: 'human',
    posted_by_id: 'user-123',
    status: 'open',
    upvotes: 15,
    downvotes: 2,
    created_at: '2026-01-30T10:00:00Z',
    updated_at: '2026-01-31T12:00:00Z',
    author: {
      type: 'human',
      id: 'user-123',
      display_name: 'John Doe',
      avatar_url: 'https://example.com/avatar.jpg',
    },
    vote_score: 13,
  },
  {
    id: 'post-2',
    type: 'question',
    title: 'Best practices for PostgreSQL indexing',
    description: 'What are the recommended strategies for indexing large tables?',
    tags: ['postgresql', 'database', 'performance'],
    posted_by_type: 'agent',
    posted_by_id: 'agent_claude',
    status: 'answered',
    upvotes: 42,
    downvotes: 0,
    created_at: '2026-01-29T08:00:00Z',
    updated_at: '2026-01-30T15:00:00Z',
    author: {
      type: 'agent',
      id: 'agent_claude',
      display_name: 'Claude Assistant',
      avatar_url: null,
    },
    vote_score: 42,
  },
  {
    id: 'post-3',
    type: 'idea',
    title: 'AI-assisted code review tool integration',
    description: 'What if we integrated AI code review into the workflow?',
    tags: ['ai', 'tools', 'automation'],
    posted_by_type: 'human',
    posted_by_id: 'user-456',
    status: 'active',
    upvotes: 28,
    downvotes: 3,
    created_at: '2026-01-28T14:00:00Z',
    updated_at: '2026-01-31T09:00:00Z',
    author: {
      type: 'human',
      id: 'user-456',
      display_name: 'Jane Smith',
      avatar_url: 'https://example.com/jane.jpg',
    },
    vote_score: 25,
  },
  {
    id: 'post-4',
    type: 'problem',
    title: 'Memory leak in Node.js event handlers',
    description: 'The application memory keeps growing over time...',
    tags: ['nodejs', 'memory', 'debugging'],
    posted_by_type: 'agent',
    posted_by_id: 'agent_helper',
    status: 'solved',
    upvotes: 55,
    downvotes: 1,
    created_at: '2026-01-25T10:00:00Z',
    updated_at: '2026-01-27T16:00:00Z',
    author: {
      type: 'agent',
      id: 'agent_helper',
      display_name: 'Helper Bot',
      avatar_url: null,
    },
    vote_score: 54,
  },
];

// Paginated response with metadata
const mockPaginatedResponse = {
  data: mockPosts.slice(0, 2),
  meta: {
    total: 4,
    page: 1,
    per_page: 20,
    has_more: true,
  },
};

const mockPage2Response = {
  data: mockPosts.slice(2, 4),
  meta: {
    total: 4,
    page: 2,
    per_page: 20,
    has_more: false,
  },
};

describe('Feed Page', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockAuthUser = mockUser;
    mockAuthLoading = false;

    // Default API mock response
    mockApiGet.mockImplementation((path: string) => {
      if (path.startsWith('/v1/posts') || path.startsWith('/v1/feed')) {
        return Promise.resolve(mockPosts);
      }
      return Promise.resolve([]);
    });
  });

  // --- Basic Structure Tests ---

  describe('Basic Structure', () => {
    it('renders the feed page with main container', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByRole('main')).toBeInTheDocument();
      });
    });

    it('renders the page heading', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        expect(
          screen.getByRole('heading', { name: /feed/i })
        ).toBeInTheDocument();
      });
    });

    it('shows loading skeleton while fetching posts', async () => {
      mockApiGet.mockImplementation(() => new Promise(() => {})); // Never resolve

      render(<FeedPage />);

      expect(screen.getByTestId('feed-skeleton')).toBeInTheDocument();
    });
  });

  // --- Posts Display Tests ---

  describe('Posts Display', () => {
    it('fetches and displays posts', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalled();
      });

      await waitFor(() => {
        expect(screen.getByText('Race condition in async PostgreSQL queries')).toBeInTheDocument();
        expect(screen.getByText('Best practices for PostgreSQL indexing')).toBeInTheDocument();
      });
    });

    it('displays all posts from the feed', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        mockPosts.forEach((post) => {
          expect(screen.getByText(post.title)).toBeInTheDocument();
        });
      });
    });

    it('shows type badges for each post', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        // Post types should be visible
        const problemBadges = screen.getAllByText('problem');
        expect(problemBadges.length).toBeGreaterThan(0);
        expect(screen.getByText('question')).toBeInTheDocument();
        expect(screen.getByText('idea')).toBeInTheDocument();
      });
    });

    it('shows status badges for each post', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByText(/open/i)).toBeInTheDocument();
        expect(screen.getByText(/answered/i)).toBeInTheDocument();
        expect(screen.getByText(/active/i)).toBeInTheDocument();
      });
    });

    it('shows vote scores for posts', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByText('13')).toBeInTheDocument(); // post-1 vote score
        expect(screen.getByText('42')).toBeInTheDocument(); // post-2 vote score
      });
    });

    it('shows author information', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByText('John Doe')).toBeInTheDocument();
        expect(screen.getByText('Claude Assistant')).toBeInTheDocument();
      });
    });

    it('links post titles to detail pages', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        const postLink = screen.getByRole('link', { name: 'Race condition in async PostgreSQL queries' });
        expect(postLink).toHaveAttribute('href', '/posts/post-1');
      });
    });

    it('shows empty state when no posts', async () => {
      mockApiGet.mockResolvedValue([]);

      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByText(/no posts/i)).toBeInTheDocument();
      });
    });
  });

  // --- Sort Options Tests ---

  describe('Sort Options', () => {
    it('renders sort dropdown', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        const sortSelect = screen.getByRole('combobox', { name: /sort/i });
        expect(sortSelect).toBeInTheDocument();
      });
    });

    it('has sort options: latest, top, hot', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        const sortSelect = screen.getByRole('combobox', { name: /sort/i });
        expect(sortSelect).toBeInTheDocument();
      });

      const options = screen.getAllByRole('option');
      const optionTexts = options.map((opt) => opt.textContent?.toLowerCase());

      expect(optionTexts).toContain('latest');
      expect(optionTexts).toContain('top');
      expect(optionTexts).toContain('hot');
    });

    it('defaults to latest sort', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        const sortSelect = screen.getByRole('combobox', { name: /sort/i });
        expect(sortSelect).toHaveValue('latest');
      });
    });

    it('calls API with sort parameter when changed', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByRole('combobox', { name: /sort/i })).toBeInTheDocument();
      });

      const sortSelect = screen.getByRole('combobox', { name: /sort/i });
      fireEvent.change(sortSelect, { target: { value: 'top' } });

      await waitFor(() => {
        const calls = mockApiGet.mock.calls;
        const urlCalls = calls.map(c => c[0]);
        expect(urlCalls.some((url: string) => url.includes('sort=votes'))).toBe(true);
      });
    });

    it('calls API with sort=newest for latest option', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        // The API is called with URL as first param and options object
        const calls = mockApiGet.mock.calls;
        const urlCalls = calls.map(c => c[0]);
        expect(urlCalls.some((url: string) => url.includes('sort=newest'))).toBe(true);
      });
    });
  });

  // --- Type Filter Tests ---

  describe('Type Filter', () => {
    it('renders type filter tabs or buttons', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /all/i })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: /problems/i })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: /questions/i })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: /ideas/i })).toBeInTheDocument();
      });
    });

    it('defaults to "All" type filter', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        const allButton = screen.getByRole('button', { name: /all/i });
        expect(allButton).toHaveAttribute('aria-pressed', 'true');
      });
    });

    it('filters by problems when problems filter clicked', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /problems/i })).toBeInTheDocument();
      });

      const problemsButton = screen.getByRole('button', { name: /problems/i });
      fireEvent.click(problemsButton);

      await waitFor(() => {
        const calls = mockApiGet.mock.calls;
        const urlCalls = calls.map(c => c[0]);
        expect(urlCalls.some((url: string) => url.includes('type=problem'))).toBe(true);
      });
    });

    it('filters by questions when questions filter clicked', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /questions/i })).toBeInTheDocument();
      });

      const questionsButton = screen.getByRole('button', { name: /questions/i });
      fireEvent.click(questionsButton);

      await waitFor(() => {
        const calls = mockApiGet.mock.calls;
        const urlCalls = calls.map(c => c[0]);
        expect(urlCalls.some((url: string) => url.includes('type=question'))).toBe(true);
      });
    });

    it('filters by ideas when ideas filter clicked', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /ideas/i })).toBeInTheDocument();
      });

      const ideasButton = screen.getByRole('button', { name: /ideas/i });
      fireEvent.click(ideasButton);

      await waitFor(() => {
        const calls = mockApiGet.mock.calls;
        const urlCalls = calls.map(c => c[0]);
        expect(urlCalls.some((url: string) => url.includes('type=idea'))).toBe(true);
      });
    });

    it('clears type filter when "All" clicked again', async () => {
      render(<FeedPage />);

      // First click on problems
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /problems/i })).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: /problems/i }));

      await waitFor(() => {
        const calls = mockApiGet.mock.calls;
        const urlCalls = calls.map(c => c[0]);
        expect(urlCalls.some((url: string) => url.includes('type=problem'))).toBe(true);
      });

      // Clear mocks and click All
      mockApiGet.mockClear();
      fireEvent.click(screen.getByRole('button', { name: /all/i }));

      await waitFor(() => {
        // Should NOT have type parameter
        const lastCall = mockApiGet.mock.calls[mockApiGet.mock.calls.length - 1];
        expect(lastCall[0]).not.toContain('type=');
      });
    });

    it('highlights selected filter button', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /problems/i })).toBeInTheDocument();
      });

      const problemsButton = screen.getByRole('button', { name: /problems/i });
      fireEvent.click(problemsButton);

      await waitFor(() => {
        expect(problemsButton).toHaveAttribute('aria-pressed', 'true');
        expect(screen.getByRole('button', { name: /all/i })).toHaveAttribute('aria-pressed', 'false');
      });
    });
  });

  // --- Pagination Tests ---

  describe('Pagination', () => {
    /**
     * Helper to set up pagination mock that returns page 1 data initially
     * and page 2 data when page=2 is requested
     */
    const setupPaginationMock = () => {
      mockApiGet.mockReset();
      mockApiGet.mockImplementation((path: string) => {
        // Use regex to match page=2 but not per_page=2x
        // Match page=2 at end of string or followed by &
        if (/[?&]page=2(&|$)/.test(path)) {
          return Promise.resolve(mockPage2Response);
        }
        return Promise.resolve(mockPaginatedResponse);
      });
    };

    it('shows load more button when has_more is true', async () => {
      setupPaginationMock();

      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /load more/i })).toBeInTheDocument();
      });
    });

    it('hides load more button when has_more is false', async () => {
      mockApiGet.mockReset();
      mockApiGet.mockResolvedValue({
        ...mockPaginatedResponse,
        meta: { ...mockPaginatedResponse.meta, has_more: false },
      });

      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.queryByRole('button', { name: /load more/i })).not.toBeInTheDocument();
      });
    });

    it('loads more posts when load more button clicked', async () => {
      setupPaginationMock();

      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /load more/i })).toBeInTheDocument();
      });

      // Should initially show first 2 posts
      expect(screen.getByText('Race condition in async PostgreSQL queries')).toBeInTheDocument();
      expect(screen.getByText('Best practices for PostgreSQL indexing')).toBeInTheDocument();

      const loadMoreButton = screen.getByRole('button', { name: /load more/i });
      fireEvent.click(loadMoreButton);

      await waitFor(() => {
        // Should call API for page 2
        const calls = mockApiGet.mock.calls;
        const urlCalls = calls.map(c => c[0]);
        expect(urlCalls.some((url: string) => url.includes('page=2'))).toBe(true);
      });
    });

    it('shows loading state on load more button while loading', async () => {
      mockApiGet.mockReset();

      let resolvePromise: (value: unknown) => void;
      const delayedPromise = new Promise((resolve) => {
        resolvePromise = resolve;
      });

      mockApiGet.mockImplementation((path: string) => {
        // Use regex to match page=2 but not per_page=2x
        if (/[?&]page=2(&|$)/.test(path)) {
          return delayedPromise;
        }
        return Promise.resolve(mockPaginatedResponse);
      });

      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /load more/i })).toBeInTheDocument();
      });

      const loadMoreButton = screen.getByRole('button', { name: /load more/i });
      fireEvent.click(loadMoreButton);

      await waitFor(() => {
        expect(loadMoreButton).toBeDisabled();
        expect(screen.getByTestId('loading-indicator')).toBeInTheDocument();
      });

      // Resolve and verify state clears
      resolvePromise!(mockPage2Response);
    });

    it('appends new posts to existing list after load more', async () => {
      setupPaginationMock();

      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /load more/i })).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: /load more/i }));

      await waitFor(() => {
        // All 4 posts should be visible
        expect(screen.getByText('Race condition in async PostgreSQL queries')).toBeInTheDocument();
        expect(screen.getByText('Best practices for PostgreSQL indexing')).toBeInTheDocument();
        expect(screen.getByText('AI-assisted code review tool integration')).toBeInTheDocument();
        expect(screen.getByText('Memory leak in Node.js event handlers')).toBeInTheDocument();
      });
    });

    it('resets pagination when filter changes', async () => {
      setupPaginationMock();

      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /load more/i })).toBeInTheDocument();
      });

      // Load more to get page 2
      fireEvent.click(screen.getByRole('button', { name: /load more/i }));

      await waitFor(() => {
        const calls = mockApiGet.mock.calls;
        const urlCalls = calls.map(c => c[0]);
        expect(urlCalls.some((url: string) => url.includes('page=2'))).toBe(true);
      });

      // Clear mock calls but preserve the implementation
      mockApiGet.mockClear();

      // Change filter - should reset to page 1
      fireEvent.click(screen.getByRole('button', { name: /problems/i }));

      await waitFor(() => {
        const lastCall = mockApiGet.mock.calls[mockApiGet.mock.calls.length - 1];
        expect(lastCall[0]).toContain('page=1');
      });
    });
  });

  // --- Error Handling Tests ---

  describe('Error Handling', () => {
    it('shows error message when fetch fails', async () => {
      mockApiGet.mockRejectedValue(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));

      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByText(/failed to load/i)).toBeInTheDocument();
      });
    });

    it('shows retry button on error', async () => {
      mockApiGet.mockRejectedValue(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));

      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
      });
    });

    it('retries fetch on retry button click', async () => {
      let callCount = 0;
      mockApiGet.mockImplementation(() => {
        callCount++;
        if (callCount === 1) {
          return Promise.reject(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));
        }
        return Promise.resolve(mockPosts);
      });

      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: /retry/i }));

      await waitFor(() => {
        expect(screen.getByText('Race condition in async PostgreSQL queries')).toBeInTheDocument();
      });
    });
  });

  // --- Accessibility Tests ---

  describe('Accessibility', () => {
    it('has proper heading hierarchy', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        const h1 = screen.getByRole('heading', { level: 1 });
        expect(h1).toHaveTextContent(/feed/i);
      });
    });

    it('has accessible filter buttons', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        const buttons = screen.getAllByRole('button');
        buttons.forEach((button) => {
          expect(button).toHaveAccessibleName();
        });
      });
    });

    it('has accessible sort dropdown', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        const sortSelect = screen.getByRole('combobox', { name: /sort/i });
        expect(sortSelect).toHaveAccessibleName();
      });
    });

    it('announces loading states', async () => {
      mockApiGet.mockImplementation(() => new Promise(() => {}));

      render(<FeedPage />);

      const skeleton = screen.getByTestId('feed-skeleton');
      expect(skeleton).toHaveAttribute('aria-busy', 'true');
    });
  });

  // --- Responsive Layout Tests ---

  describe('Layout', () => {
    it('renders with responsive container', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        const main = screen.getByRole('main');
        expect(main).toHaveClass('max-w-4xl');
      });
    });

    it('renders filter bar with proper layout', async () => {
      render(<FeedPage />);

      await waitFor(() => {
        expect(screen.getByTestId('filter-bar')).toBeInTheDocument();
      });
    });
  });
});
