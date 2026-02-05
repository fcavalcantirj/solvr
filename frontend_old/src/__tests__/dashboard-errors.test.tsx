/**
 * Tests for Dashboard Page - Error Handling and Accessibility
 * Split from dashboard.test.tsx to keep file sizes under 800 lines
 */

import { render, screen, waitFor, fireEvent } from '@testing-library/react';

// Track router push calls
const mockPush = jest.fn();
const mockReplace = jest.fn();

// Mock next/navigation
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace, back: jest.fn() }),
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
const mockApiPatch = jest.fn();
const mockApiPost = jest.fn();
const mockApiDelete = jest.fn();
jest.mock('@/lib/api', () => ({
  api: {
    get: (...args: unknown[]) => mockApiGet(...args),
    patch: (...args: unknown[]) => mockApiPatch(...args),
    post: (...args: unknown[]) => mockApiPost(...args),
    delete: (...args: unknown[]) => mockApiDelete(...args),
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

// Mock useAuth hook
const mockUser = {
  id: 'user-123',
  username: 'johndoe',
  display_name: 'John Doe',
  email: 'john@example.com',
  avatar_url: 'https://example.com/avatar.jpg',
  bio: 'Software engineer',
};
let mockAuthUser: typeof mockUser | null = mockUser;
let mockAuthLoading = false;
const mockLogout = jest.fn();

jest.mock('@/hooks/useAuth', () => ({
  useAuth: () => ({
    user: mockAuthUser,
    isLoading: mockAuthLoading,
    login: jest.fn(),
    logout: mockLogout,
  }),
  __esModule: true,
}));

// Import component after mocks
import DashboardPage from '../app/dashboard/page';

// Test data
const mockAgents = [
  {
    id: 'agent_claude',
    display_name: 'Claude Assistant',
    bio: 'A helpful AI assistant',
    specialties: ['coding', 'writing'],
    avatar_url: 'https://example.com/claude.png',
    created_at: '2025-06-15T10:00:00Z',
    human_id: 'user-123',
    moltbook_verified: true,
    stats: {
      problems_solved: 5,
      questions_answered: 12,
      reputation: 450,
    },
  },
];

const mockPosts = [
  {
    id: 'post-1',
    type: 'problem',
    title: 'How to handle async errors in Go',
    description: 'I have an issue with error handling...',
    status: 'open',
    tags: ['go', 'async'],
    upvotes: 10,
    downvotes: 2,
    created_at: '2025-12-01T10:00:00Z',
  },
];

const mockActivity = [
  {
    id: 'activity-1',
    type: 'answer_created',
    post_id: 'post-2',
    post_title: 'Best practices for PostgreSQL indexing',
    actor: {
      id: 'agent_expert',
      type: 'agent',
      display_name: 'DB Expert Agent',
    },
    created_at: '2026-01-28T10:00:00Z',
  },
];

const mockStats = {
  problems_solved: 3,
  problems_contributed: 5,
  questions_asked: 8,
  questions_answered: 12,
  answers_accepted: 4,
  ideas_posted: 2,
  upvotes_received: 45,
  reputation: 850,
};

const mockInProgress = [
  {
    id: 'approach-1',
    problem_id: 'problem-123',
    problem_title: 'Fix memory leak in production',
    status: 'working',
    updated_at: '2026-01-30T10:00:00Z',
  },
];

describe('Dashboard Page - Error Handling', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockAuthUser = mockUser;
    mockAuthLoading = false;
  });

  it('shows error message when agents fetch fails', async () => {
    mockApiGet.mockImplementation((path: string) => {
      if (path === '/v1/users/user-123/agents') {
        return Promise.reject(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));
      }
      if (path === '/v1/users/user-123/posts') {
        return Promise.resolve(mockPosts);
      }
      if (path === '/v1/users/user-123/activity') {
        return Promise.resolve(mockActivity);
      }
      if (path === '/v1/users/user-123/stats') {
        return Promise.resolve(mockStats);
      }
      if (path === '/v1/users/user-123/in-progress') {
        return Promise.resolve(mockInProgress);
      }
      return Promise.resolve({});
    });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByTestId('agents-error')).toBeInTheDocument();
    });
  });

  it('shows error message when posts fetch fails', async () => {
    mockApiGet.mockImplementation((path: string) => {
      if (path === '/v1/users/user-123/posts') {
        return Promise.reject(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));
      }
      if (path === '/v1/users/user-123/agents') {
        return Promise.resolve(mockAgents);
      }
      if (path === '/v1/users/user-123/activity') {
        return Promise.resolve(mockActivity);
      }
      if (path === '/v1/users/user-123/stats') {
        return Promise.resolve(mockStats);
      }
      if (path === '/v1/users/user-123/in-progress') {
        return Promise.resolve(mockInProgress);
      }
      return Promise.resolve({});
    });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByTestId('posts-error')).toBeInTheDocument();
    });
  });

  it('allows retry after fetch error', async () => {
    let callCount = 0;
    mockApiGet.mockImplementation((path: string) => {
      if (path === '/v1/users/user-123/agents') {
        callCount++;
        if (callCount === 1) {
          return Promise.reject(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));
        }
        return Promise.resolve(mockAgents);
      }
      if (path === '/v1/users/user-123/posts') {
        return Promise.resolve(mockPosts);
      }
      if (path === '/v1/users/user-123/activity') {
        return Promise.resolve(mockActivity);
      }
      if (path === '/v1/users/user-123/stats') {
        return Promise.resolve(mockStats);
      }
      if (path === '/v1/users/user-123/in-progress') {
        return Promise.resolve(mockInProgress);
      }
      return Promise.resolve({});
    });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByTestId('agents-error')).toBeInTheDocument();
    });

    const retryButton = screen.getByRole('button', { name: /retry/i });
    fireEvent.click(retryButton);

    await waitFor(() => {
      expect(screen.getByText('Claude Assistant')).toBeInTheDocument();
    });
  });
});

describe('Dashboard Page - Accessibility', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockAuthUser = mockUser;
    mockAuthLoading = false;

    // Default API mock responses
    mockApiGet.mockImplementation((path: string) => {
      if (path === '/v1/users/user-123/agents') {
        return Promise.resolve(mockAgents);
      }
      if (path === '/v1/users/user-123/posts') {
        return Promise.resolve(mockPosts);
      }
      if (path === '/v1/users/user-123/activity') {
        return Promise.resolve(mockActivity);
      }
      if (path === '/v1/users/user-123/stats') {
        return Promise.resolve(mockStats);
      }
      if (path === '/v1/users/user-123/in-progress') {
        return Promise.resolve(mockInProgress);
      }
      return Promise.resolve({});
    });
  });

  it('has proper heading hierarchy', async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      const h1 = screen.getByRole('heading', { level: 1 });
      expect(h1).toHaveTextContent(/dashboard/i);

      const h2s = screen.getAllByRole('heading', { level: 2 });
      expect(h2s.length).toBeGreaterThanOrEqual(5); // All section headings
    });
  });

  it('has proper ARIA labels for interactive elements', async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      const links = screen.getAllByRole('link');
      links.forEach((link) => {
        expect(link).toHaveAccessibleName();
      });
    });
  });

  it('has proper loading state announcements', async () => {
    mockAuthLoading = true;

    render(<DashboardPage />);

    const skeleton = screen.getByTestId('dashboard-skeleton');
    expect(skeleton).toHaveAttribute('aria-busy', 'true');
  });
});

describe('Dashboard Page - Layout', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockAuthUser = mockUser;
    mockAuthLoading = false;

    // Default API mock responses
    mockApiGet.mockImplementation((path: string) => {
      if (path === '/v1/users/user-123/agents') {
        return Promise.resolve(mockAgents);
      }
      if (path === '/v1/users/user-123/posts') {
        return Promise.resolve(mockPosts);
      }
      if (path === '/v1/users/user-123/activity') {
        return Promise.resolve(mockActivity);
      }
      if (path === '/v1/users/user-123/stats') {
        return Promise.resolve(mockStats);
      }
      if (path === '/v1/users/user-123/in-progress') {
        return Promise.resolve(mockInProgress);
      }
      return Promise.resolve({});
    });
  });

  it('renders with responsive grid classes', async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      const main = screen.getByRole('main');
      // Should have grid layout for dashboard cards
      expect(main.querySelector('[class*="grid"]')).toBeInTheDocument();
    });
  });
});
