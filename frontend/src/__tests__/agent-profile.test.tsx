/**
 * Tests for Agent Profile Page component
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from PRD lines 489-490:
 * - Create /agents/[id] page
 * - Agent profile: display (name, bio, specialties, stats)
 */

import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { act } from 'react';

// Track notFound calls
let mockNotFoundCalled = false;

// Mock next/navigation
const mockParams = { id: 'test-agent' };
const mockPush = jest.fn();
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush, replace: jest.fn(), back: jest.fn() }),
  useParams: () => mockParams,
  notFound: () => {
    mockNotFoundCalled = true;
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

// Import the ApiError to use in tests
import { ApiError } from '@/lib/api';

// Mock useAuth hook
const mockUser = {
  id: 'owner-user-id',
  username: 'owner_user',
  display_name: 'Owner User',
};
let mockAuthUser: typeof mockUser | null = null;
let mockAuthLoading = false;

jest.mock('@/hooks/useAuth', () => ({
  useAuth: () => ({
    user: mockAuthUser,
    isLoading: mockAuthLoading,
    login: jest.fn(),
    logout: jest.fn(),
  }),
  __esModule: true,
}));

// Import component after mocks
import AgentProfilePage from '../app/agents/[id]/page';

// Test data - Agent profile per SPEC.md Part 2.7
const mockAgentProfile = {
  id: 'test-agent',
  display_name: 'Test Agent',
  human_id: 'owner-user-id',
  bio: 'An AI assistant specialized in debugging and code review.',
  specialties: ['debugging', 'code-review', 'golang', 'typescript'],
  avatar_url: 'https://example.com/agent-avatar.jpg',
  moltbook_verified: true,
  created_at: '2025-08-15T10:00:00Z',
  owner: {
    id: 'owner-user-id',
    username: 'owner_user',
    display_name: 'Owner User',
  },
};

// Test data - Agent stats per SPEC.md Part 2.7 and Part 10.3
const mockAgentStats = {
  problems_solved: 15,
  problems_contributed: 42,
  questions_asked: 8,
  questions_answered: 67,
  answers_accepted: 23,
  ideas_posted: 12,
  responses_given: 35,
  upvotes_received: 234,
  reputation: 3150,
};

// Test data - Agent with stats combined
const mockAgentWithStats = {
  ...mockAgentProfile,
  stats: mockAgentStats,
};

// Test data - Agent activity
const mockAgentActivity = [
  {
    id: 'activity-1',
    type: 'answer',
    action: 'created',
    title: 'Answered: How to handle async errors in Go?',
    created_at: '2026-01-30T14:00:00Z',
    target_id: 'answer-1',
    target_title: 'How to handle async errors in Go?',
  },
  {
    id: 'activity-2',
    type: 'approach',
    action: 'succeeded',
    title: 'Approach succeeded: Race condition fix',
    status: 'succeeded',
    created_at: '2026-01-28T10:00:00Z',
    target_id: 'approach-1',
  },
  {
    id: 'activity-3',
    type: 'post',
    action: 'created',
    title: 'Observation: Patterns in async error handling',
    post_type: 'idea',
    created_at: '2026-01-25T08:00:00Z',
    target_id: 'post-1',
  },
];

describe('AgentProfilePage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockNotFoundCalled = false;
    mockAuthUser = null;
    mockAuthLoading = false;
    mockParams.id = 'test-agent';

    // Default mock implementations
    mockApiGet.mockImplementation((path: string) => {
      if (path === '/v1/agents/test-agent') {
        return Promise.resolve(mockAgentWithStats);
      }
      if (path.startsWith('/v1/agents/test-agent/activity')) {
        return Promise.resolve(mockAgentActivity);
      }
      return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
    });
  });

  describe('Basic Structure', () => {
    it('renders main container with profile page', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByRole('main')).toBeInTheDocument();
      });
    });

    it('renders profile heading with agent name', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
          'Test Agent'
        );
      });
    });
  });

  describe('Loading State', () => {
    it('shows loading skeleton initially', async () => {
      // Delay the API response
      mockApiGet.mockImplementation(() => new Promise(() => {}));

      await act(async () => {
        render(<AgentProfilePage />);
      });

      expect(screen.getByTestId('agent-profile-skeleton')).toBeInTheDocument();
    });

    it('removes skeleton after loading', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(
          screen.queryByTestId('agent-profile-skeleton')
        ).not.toBeInTheDocument();
      });
    });
  });

  describe('Profile Display', () => {
    it('displays agent avatar', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        const avatar = screen.getByRole('img', { name: /test agent/i });
        expect(avatar).toHaveAttribute('src', mockAgentProfile.avatar_url);
      });
    });

    it('displays agent bio', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(mockAgentProfile.bio)).toBeInTheDocument();
      });
    });

    it('displays agent ID', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('@test-agent')).toBeInTheDocument();
      });
    });

    it('displays AI Agent type badge', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/ai agent/i)).toBeInTheDocument();
      });
    });

    it('displays Moltbook verified badge when verified', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/moltbook verified/i)).toBeInTheDocument();
      });
    });

    it('hides Moltbook badge when not verified', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/test-agent') {
          return Promise.resolve({
            ...mockAgentWithStats,
            moltbook_verified: false,
          });
        }
        if (path.startsWith('/v1/agents/test-agent/activity')) {
          return Promise.resolve(mockAgentActivity);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('Test Agent')).toBeInTheDocument();
      });

      expect(screen.queryByText(/moltbook verified/i)).not.toBeInTheDocument();
    });

    it('shows default avatar when avatar_url is missing', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/test-agent') {
          return Promise.resolve({
            ...mockAgentWithStats,
            avatar_url: null,
          });
        }
        if (path.startsWith('/v1/agents/test-agent/activity')) {
          return Promise.resolve(mockAgentActivity);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        const avatar = screen.getByTestId('agent-avatar');
        expect(avatar).toBeInTheDocument();
      });
    });

    it('displays creation date', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/created/i)).toBeInTheDocument();
      });
    });
  });

  describe('Specialties Display', () => {
    it('displays all specialty tags', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        mockAgentProfile.specialties.forEach((specialty) => {
          expect(screen.getByText(specialty)).toBeInTheDocument();
        });
      });
    });

    it('displays specialties section heading', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/specialties/i)).toBeInTheDocument();
      });
    });

    it('handles agent with no specialties', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/test-agent') {
          return Promise.resolve({
            ...mockAgentWithStats,
            specialties: [],
          });
        }
        if (path.startsWith('/v1/agents/test-agent/activity')) {
          return Promise.resolve(mockAgentActivity);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('Test Agent')).toBeInTheDocument();
      });

      // Specialties section should not be shown or show empty state
      expect(
        screen.queryByText('debugging')
      ).not.toBeInTheDocument();
    });
  });

  describe('Owner Link', () => {
    it('displays link to owner profile', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        const ownerLink = screen.getByRole('link', { name: /owner_user/i });
        expect(ownerLink).toBeInTheDocument();
        expect(ownerLink).toHaveAttribute('href', '/users/owner_user');
      });
    });

    it('displays owner label text', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/owned by/i)).toBeInTheDocument();
      });
    });
  });

  describe('Stats Display', () => {
    it('displays reputation score', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('3,150')).toBeInTheDocument();
        expect(screen.getByText(/reputation/i)).toBeInTheDocument();
      });
    });

    it('displays problems solved count', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('15')).toBeInTheDocument();
        expect(screen.getByText(/problems solved/i)).toBeInTheDocument();
      });
    });

    it('displays questions answered count', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('67')).toBeInTheDocument();
        expect(screen.getByText(/questions answered/i)).toBeInTheDocument();
      });
    });

    it('displays accepted answers count', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('23')).toBeInTheDocument();
        expect(screen.getByText(/accepted/i)).toBeInTheDocument();
      });
    });

    it('displays ideas posted count', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('12')).toBeInTheDocument();
        expect(screen.getByText(/ideas/i)).toBeInTheDocument();
      });
    });

    it('displays upvotes received count', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('234')).toBeInTheDocument();
        expect(screen.getByText(/upvotes/i)).toBeInTheDocument();
      });
    });
  });

  describe('Activity Section', () => {
    it('displays activity section heading', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(
          screen.getByRole('heading', { name: /recent activity/i })
        ).toBeInTheDocument();
      });
    });

    it('fetches and displays activity items', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(
          expect.stringContaining('/v1/agents/test-agent/activity')
        );
      });

      await waitFor(() => {
        expect(
          screen.getByText('Answered: How to handle async errors in Go?')
        ).toBeInTheDocument();
      });
    });

    it('displays activity timestamps', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        const activityItems = screen.getAllByTestId('agent-activity-item');
        expect(activityItems.length).toBeGreaterThan(0);
      });
    });

    it('shows activity type badges', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        // Check for activity type badges - activity items have type badges
        const activityItems = screen.getAllByTestId('agent-activity-item');
        expect(activityItems.length).toBeGreaterThan(0);
        // The first activity has type 'answer', so it should show 'answer' badge
        // Activity badges are rendered based on post_type || type
        expect(screen.getByText('answer')).toBeInTheDocument();
      });
    });

    it('shows empty state when no activity', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/test-agent') {
          return Promise.resolve(mockAgentWithStats);
        }
        if (path.startsWith('/v1/agents/test-agent/activity')) {
          return Promise.resolve([]);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/no recent activity/i)).toBeInTheDocument();
      });
    });
  });

  describe('Edit/Manage Button (Owner Only)', () => {
    it('shows manage button when viewing agent owned by current user', async () => {
      mockAuthUser = {
        id: 'owner-user-id',
        username: 'owner_user',
        display_name: 'Owner User',
      };

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        const manageButton = screen.getByRole('link', { name: /manage agent/i });
        expect(manageButton).toBeInTheDocument();
        expect(manageButton).toHaveAttribute('href', '/settings/agents/test-agent');
      });
    });

    it('hides manage button when viewing agent owned by different user', async () => {
      mockAuthUser = {
        id: 'different-user-id',
        username: 'different_user',
        display_name: 'Different User',
      };

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('Test Agent')).toBeInTheDocument();
      });

      expect(
        screen.queryByRole('link', { name: /manage agent/i })
      ).not.toBeInTheDocument();
    });

    it('hides manage button when not logged in', async () => {
      mockAuthUser = null;

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('Test Agent')).toBeInTheDocument();
      });

      expect(
        screen.queryByRole('link', { name: /manage agent/i })
      ).not.toBeInTheDocument();
    });
  });

  describe('404 Handling', () => {
    it('calls notFound when agent does not exist', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/test-agent') {
          return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Agent not found'));
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(mockNotFoundCalled).toBe(true);
      });
    });
  });

  describe('Error Handling', () => {
    it('shows error message on API failure', async () => {
      mockApiGet.mockImplementation(() =>
        Promise.reject(new Error('Network error'))
      );

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByRole('alert')).toBeInTheDocument();
        expect(screen.getByText(/failed to load/i)).toBeInTheDocument();
      });
    });

    it('shows retry button on error', async () => {
      mockApiGet.mockImplementation(() =>
        Promise.reject(new Error('Network error'))
      );

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(
          screen.getByRole('button', { name: /try again/i })
        ).toBeInTheDocument();
      });
    });

    it('retries fetching when retry button clicked', async () => {
      let callCount = 0;
      mockApiGet.mockImplementation((path: string) => {
        callCount++;
        if (callCount === 1) {
          return Promise.reject(new Error('Network error'));
        }
        if (path === '/v1/agents/test-agent') {
          return Promise.resolve(mockAgentWithStats);
        }
        if (path.startsWith('/v1/agents/test-agent/activity')) {
          return Promise.resolve(mockAgentActivity);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument();
      });

      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: /try again/i }));
      });

      await waitFor(() => {
        expect(screen.getByText('Test Agent')).toBeInTheDocument();
      });
    });
  });

  describe('Accessibility', () => {
    it('has proper heading hierarchy', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument();
        expect(
          screen.getByRole('heading', { level: 2, name: /recent activity/i })
        ).toBeInTheDocument();
      });
    });

    it('has accessible labels for stats', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        const statsSection = screen.getByTestId('agent-stats');
        expect(statsSection).toBeInTheDocument();
      });
    });

    it('uses semantic article elements for activity items', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        const activityItems = screen.getAllByTestId('agent-activity-item');
        activityItems.forEach((item) => {
          expect(item.tagName.toLowerCase()).toBe('article');
        });
      });
    });

    it('has descriptive alt text for avatar', async () => {
      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        const avatar = screen.getByRole('img', { name: /test agent/i });
        expect(avatar).toBeInTheDocument();
      });
    });
  });

  describe('Different Agent ID Formats', () => {
    it('handles agent ID with underscores', async () => {
      mockParams.id = 'claude_assistant_v2';
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/claude_assistant_v2') {
          return Promise.resolve({
            ...mockAgentWithStats,
            id: 'claude_assistant_v2',
          });
        }
        if (path.startsWith('/v1/agents/claude_assistant_v2/activity')) {
          return Promise.resolve(mockAgentActivity);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('@claude_assistant_v2')).toBeInTheDocument();
      });
    });

    it('handles agent ID with numbers', async () => {
      mockParams.id = 'agent123';
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/agents/agent123') {
          return Promise.resolve({
            ...mockAgentWithStats,
            id: 'agent123',
          });
        }
        if (path.startsWith('/v1/agents/agent123/activity')) {
          return Promise.resolve(mockAgentActivity);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<AgentProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('@agent123')).toBeInTheDocument();
      });
    });
  });
});
