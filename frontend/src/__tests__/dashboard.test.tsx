/**
 * Tests for Dashboard Page component
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from PRD lines 495-497:
 * - Create /dashboard page
 * - Dashboard: your posts (list user's own posts)
 * - Dashboard: activity feed (show activity on your content)
 *
 * Requirements from SPEC.md Part 4.10:
 * - My AI Agents (list, stats, API keys)
 * - My Impact (problems solved, efficiency metrics)
 * - My Posts
 * - In Progress (active work)
 * - Notifications
 */

import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { act } from 'react';

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

// Test data - User's agents
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
  {
    id: 'agent_helper',
    display_name: 'Helper Bot',
    bio: 'Helps with tasks',
    specialties: ['tasks'],
    avatar_url: null,
    created_at: '2025-07-20T10:00:00Z',
    human_id: 'user-123',
    moltbook_verified: false,
    stats: {
      problems_solved: 2,
      questions_answered: 8,
      reputation: 200,
    },
  },
];

// Test data - User's posts
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
  {
    id: 'post-2',
    type: 'question',
    title: 'Best practices for PostgreSQL indexing',
    description: 'What are the recommended indexing strategies?',
    status: 'answered',
    tags: ['postgresql', 'database'],
    upvotes: 25,
    downvotes: 0,
    created_at: '2025-12-15T10:00:00Z',
  },
  {
    id: 'post-3',
    type: 'idea',
    title: 'AI-assisted code review tool',
    description: 'What if we built a tool that...',
    status: 'active',
    tags: ['ai', 'tools'],
    upvotes: 8,
    downvotes: 1,
    created_at: '2026-01-01T10:00:00Z',
  },
];

// Test data - Activity feed
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
  {
    id: 'activity-2',
    type: 'comment_created',
    post_id: 'post-1',
    post_title: 'How to handle async errors in Go',
    actor: {
      id: 'user-456',
      type: 'human',
      display_name: 'Jane Smith',
    },
    created_at: '2026-01-27T15:30:00Z',
  },
  {
    id: 'activity-3',
    type: 'upvote',
    post_id: 'post-3',
    post_title: 'AI-assisted code review tool',
    actor: {
      id: 'user-789',
      type: 'human',
      display_name: 'Alex Developer',
    },
    created_at: '2026-01-27T10:00:00Z',
  },
];

// Test data - User stats/impact
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

// Test data - In-progress work
const mockInProgress = [
  {
    id: 'approach-1',
    problem_id: 'problem-123',
    problem_title: 'Fix memory leak in production',
    status: 'working',
    updated_at: '2026-01-30T10:00:00Z',
  },
];

describe('Dashboard Page', () => {
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

  // --- Basic Structure Tests ---

  describe('Basic Structure', () => {
    it('renders the dashboard page with main container', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByRole('main')).toBeInTheDocument();
      });
    });

    it('renders the page heading with user greeting', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(
          screen.getByRole('heading', { name: /dashboard/i })
        ).toBeInTheDocument();
      });
    });

    it('renders section headers for all dashboard sections', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        // Per SPEC.md Part 4.10
        expect(screen.getByRole('heading', { name: /my ai agents/i })).toBeInTheDocument();
        expect(screen.getByRole('heading', { name: /my impact/i })).toBeInTheDocument();
        expect(screen.getByRole('heading', { name: /my posts/i })).toBeInTheDocument();
        expect(screen.getByRole('heading', { name: /in progress/i })).toBeInTheDocument();
        expect(screen.getByRole('heading', { name: /activity/i })).toBeInTheDocument();
      });
    });
  });

  // --- Authentication Required Tests ---

  describe('Authentication Required', () => {
    it('redirects to login when not authenticated', async () => {
      mockAuthUser = null;

      render(<DashboardPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith('/login');
      });
    });

    it('shows loading state while auth is loading', async () => {
      mockAuthLoading = true;

      render(<DashboardPage />);

      expect(screen.getByTestId('dashboard-skeleton')).toBeInTheDocument();
    });

    it('shows dashboard content when authenticated', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.queryByTestId('dashboard-skeleton')).not.toBeInTheDocument();
        expect(screen.getByRole('main')).toBeInTheDocument();
      });
    });
  });

  // --- My AI Agents Section Tests ---

  describe('My AI Agents Section', () => {
    it('fetches and displays user agents', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith('/v1/users/user-123/agents');
      });

      await waitFor(() => {
        expect(screen.getByText('Claude Assistant')).toBeInTheDocument();
        expect(screen.getByText('Helper Bot')).toBeInTheDocument();
      });
    });

    it('displays agent stats', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        // Claude's stats
        expect(screen.getByText(/5 problems solved/i)).toBeInTheDocument();
        expect(screen.getByText(/12 questions answered/i)).toBeInTheDocument();
      });
    });

    it('shows Moltbook verified badge for verified agents', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('moltbook-badge-agent_claude')).toBeInTheDocument();
        expect(screen.queryByTestId('moltbook-badge-agent_helper')).not.toBeInTheDocument();
      });
    });

    it('shows empty state when no agents registered', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/users/user-123/agents') {
          return Promise.resolve([]);
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
        expect(screen.getByText(/no agents registered/i)).toBeInTheDocument();
      });
    });

    it('provides link to register new agent', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        const registerLink = screen.getByRole('link', { name: /register.*agent/i });
        expect(registerLink).toHaveAttribute('href', '/settings?tab=agents');
      });
    });

    it('links agent name to agent profile page', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        const agentLink = screen.getByRole('link', { name: /claude assistant/i });
        expect(agentLink).toHaveAttribute('href', '/agents/agent_claude');
      });
    });
  });

  // --- My Impact Section Tests ---

  describe('My Impact Section', () => {
    it('fetches and displays user stats', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith('/v1/users/user-123/stats');
      });

      await waitFor(() => {
        expect(screen.getByText('850')).toBeInTheDocument(); // reputation
      });
    });

    it('displays all impact metrics', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        // Per SPEC.md Part 2.7 computed stats - look for labels instead
        expect(screen.getByText('Problems Solved')).toBeInTheDocument();
        expect(screen.getByText('Problems Contributed')).toBeInTheDocument();
        expect(screen.getByText('Questions Asked')).toBeInTheDocument();
        expect(screen.getByText('Questions Answered')).toBeInTheDocument();
      });
    });

    it('shows reputation prominently', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        const reputationElement = screen.getByTestId('reputation-score');
        expect(reputationElement).toHaveTextContent('850');
      });
    });
  });

  // --- My Posts Section Tests ---

  describe('My Posts Section', () => {
    it('fetches and displays user posts', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith('/v1/users/user-123/posts');
      });

      await waitFor(() => {
        // Post titles should be visible - may appear multiple times (posts + activity)
        expect(screen.getAllByRole('link', { name: 'How to handle async errors in Go' }).length).toBeGreaterThan(0);
        expect(screen.getAllByRole('link', { name: 'Best practices for PostgreSQL indexing' }).length).toBeGreaterThan(0);
        expect(screen.getAllByRole('link', { name: 'AI-assisted code review tool' }).length).toBeGreaterThan(0);
      });
    });

    it('displays post type badges', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('type-badge-post-1')).toHaveTextContent(/problem/i);
        expect(screen.getByTestId('type-badge-post-2')).toHaveTextContent(/question/i);
        expect(screen.getByTestId('type-badge-post-3')).toHaveTextContent(/idea/i);
      });
    });

    it('displays post status badges', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('status-badge-post-1')).toHaveTextContent(/open/i);
        expect(screen.getByTestId('status-badge-post-2')).toHaveTextContent(/answered/i);
        expect(screen.getByTestId('status-badge-post-3')).toHaveTextContent(/active/i);
      });
    });

    it('displays vote counts', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        // Post 1: 10 upvotes, 2 downvotes = 8 net
        expect(screen.getByTestId('votes-post-1')).toHaveTextContent('8');
        // Post 2: 25 upvotes, 0 downvotes = 25 net
        expect(screen.getByTestId('votes-post-2')).toHaveTextContent('25');
      });
    });

    it('links post titles to post detail pages', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        // May have multiple links to same post (in posts and activity), get the first one
        const postLinks = screen.getAllByRole('link', { name: 'How to handle async errors in Go' });
        expect(postLinks[0]).toHaveAttribute('href', '/posts/post-1');
      });
    });

    it('shows empty state when no posts', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/users/user-123/posts') {
          return Promise.resolve([]);
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
        expect(screen.getByText(/no posts yet/i)).toBeInTheDocument();
      });
    });

    it('provides link to create new post', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        const newPostLink = screen.getByRole('link', { name: /create post/i });
        expect(newPostLink).toHaveAttribute('href', '/new');
      });
    });
  });

  // --- In Progress Section Tests ---

  describe('In Progress Section', () => {
    it('fetches and displays in-progress work', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith('/v1/users/user-123/in-progress');
      });

      await waitFor(() => {
        expect(screen.getByText(/fix memory leak in production/i)).toBeInTheDocument();
      });
    });

    it('shows status indicator for in-progress items', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByTestId('progress-status-approach-1')).toHaveTextContent(/working/i);
      });
    });

    it('links to problem detail page', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        const problemLink = screen.getByRole('link', { name: /fix memory leak in production/i });
        expect(problemLink).toHaveAttribute('href', '/posts/problem-123');
      });
    });

    it('shows empty state when no in-progress work', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/users/user-123/in-progress') {
          return Promise.resolve([]);
        }
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
        return Promise.resolve({});
      });

      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByText(/no active work/i)).toBeInTheDocument();
      });
    });
  });

  // --- Activity Feed Section Tests ---

  describe('Activity Feed Section', () => {
    it('fetches and displays activity feed', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith('/v1/users/user-123/activity');
      });

      await waitFor(() => {
        // Activity items should show actor names
        expect(screen.getByText('DB Expert Agent')).toBeInTheDocument();
      });
    });

    it('displays activity type descriptions', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        // Activity types are shown as text - may appear multiple times
        expect(screen.getAllByText('answered').length).toBeGreaterThan(0);
        expect(screen.getAllByText('commented on').length).toBeGreaterThan(0);
        expect(screen.getAllByText('upvoted').length).toBeGreaterThan(0);
      });
    });

    it('displays actor names with type indicator', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        expect(screen.getByText('DB Expert Agent')).toBeInTheDocument();
        expect(screen.getByText('Jane Smith')).toBeInTheDocument();
        expect(screen.getByText('Alex Developer')).toBeInTheDocument();
      });
    });

    it('shows relative timestamps', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        // These should show relative times like "2 days ago"
        const timestamps = screen.getAllByTestId(/activity-time-/);
        expect(timestamps.length).toBeGreaterThan(0);
      });
    });

    it('links activity items to relevant posts', async () => {
      render(<DashboardPage />);

      await waitFor(() => {
        // Activity links should be rendered as anchor tags with post URLs
        // First activity item (activity-1) links to post-2
        const activitySection = screen.getByRole('heading', { name: /activity/i }).parentElement;
        expect(activitySection).toBeInTheDocument();

        // Find activity links within activity section using role
        const links = screen.getAllByRole('link');
        const activityLink = links.find(link =>
          link.getAttribute('href') === '/posts/post-2' &&
          link.textContent?.includes('Best practices for PostgreSQL')
        );
        expect(activityLink).toBeInTheDocument();
      });
    });

    it('shows empty state when no activity', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/users/user-123/activity') {
          return Promise.resolve([]);
        }
        if (path === '/v1/users/user-123/agents') {
          return Promise.resolve(mockAgents);
        }
        if (path === '/v1/users/user-123/posts') {
          return Promise.resolve(mockPosts);
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
        expect(screen.getByText(/no recent activity/i)).toBeInTheDocument();
      });
    });
  });

  // Note: Error Handling, Accessibility, and Layout tests are in dashboard-errors.test.tsx
});
