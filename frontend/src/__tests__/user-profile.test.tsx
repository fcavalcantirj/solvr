/**
 * Tests for User Profile Page component
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from PRD lines 485-488:
 * - Create /users/[username] page
 * - User profile: display info (name, bio, avatar, stats)
 * - User profile: activity
 * - User profile: edit button (if own profile)
 */

import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { act } from 'react';

// Track notFound calls
let mockNotFoundCalled = false;

// Mock next/navigation
const mockParams = { username: 'johndoe' };
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
  id: 'current-user-id',
  username: 'currentuser',
  display_name: 'Current User',
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
import UserProfilePage from '../app/users/[username]/page';

// Test data - User profile
const mockUserProfile = {
  id: 'user-123',
  username: 'johndoe',
  display_name: 'John Doe',
  email: 'john@example.com',
  avatar_url: 'https://example.com/avatar.jpg',
  bio: 'Software engineer passionate about open source and AI.',
  created_at: '2025-06-15T10:00:00Z',
};

// Test data - User stats
const mockUserStats = {
  posts_created: 42,
  answers_given: 87,
  answers_accepted: 23,
  upvotes_received: 156,
  reputation: 1250,
};

// Test data - User with stats combined
const mockUserWithStats = {
  ...mockUserProfile,
  stats: mockUserStats,
};

// Test data - User activity
const mockUserActivity = [
  {
    id: 'activity-1',
    type: 'post',
    action: 'created',
    title: 'How to handle async errors in Go?',
    post_type: 'question',
    created_at: '2026-01-30T14:00:00Z',
    target_id: 'post-1',
  },
  {
    id: 'activity-2',
    type: 'answer',
    action: 'created',
    title: 'Answered: Best practices for PostgreSQL connections',
    created_at: '2026-01-28T10:00:00Z',
    target_id: 'answer-1',
    target_title: 'Best practices for PostgreSQL connections',
  },
  {
    id: 'activity-3',
    type: 'approach',
    action: 'succeeded',
    title: 'Approach succeeded: Race condition fix',
    status: 'succeeded',
    created_at: '2026-01-25T08:00:00Z',
    target_id: 'approach-1',
  },
];

describe('UserProfilePage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockNotFoundCalled = false;
    mockAuthUser = null;
    mockAuthLoading = false;
    mockParams.username = 'johndoe';

    // Default mock implementations
    mockApiGet.mockImplementation((path: string) => {
      if (path === '/v1/users/johndoe') {
        return Promise.resolve(mockUserWithStats);
      }
      if (path.startsWith('/v1/users/johndoe/activity')) {
        return Promise.resolve(mockUserActivity);
      }
      return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
    });
  });

  describe('Basic Structure', () => {
    it('renders main container with profile page', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByRole('main')).toBeInTheDocument();
      });
    });

    it('renders profile heading with username', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
          'John Doe'
        );
      });
    });
  });

  describe('Loading State', () => {
    it('shows loading skeleton initially', async () => {
      // Delay the API response
      mockApiGet.mockImplementation(() => new Promise(() => {}));

      await act(async () => {
        render(<UserProfilePage />);
      });

      expect(screen.getByTestId('profile-skeleton')).toBeInTheDocument();
    });

    it('removes skeleton after loading', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.queryByTestId('profile-skeleton')).not.toBeInTheDocument();
      });
    });
  });

  describe('Profile Display', () => {
    it('displays user avatar', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        const avatar = screen.getByRole('img', { name: /john doe/i });
        expect(avatar).toHaveAttribute('src', mockUserProfile.avatar_url);
      });
    });

    it('displays user bio', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(mockUserProfile.bio)).toBeInTheDocument();
      });
    });

    it('displays joined date', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/joined/i)).toBeInTheDocument();
      });
    });

    it('displays username with @ prefix', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('@johndoe')).toBeInTheDocument();
      });
    });

    it('shows default avatar when avatar_url is missing', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/users/johndoe') {
          return Promise.resolve({
            ...mockUserWithStats,
            avatar_url: null,
          });
        }
        if (path.startsWith('/v1/users/johndoe/activity')) {
          return Promise.resolve(mockUserActivity);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        // Should show initials or default avatar
        const avatar = screen.getByTestId('user-avatar');
        expect(avatar).toBeInTheDocument();
      });
    });
  });

  describe('Stats Display', () => {
    it('displays reputation score', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('1,250')).toBeInTheDocument();
        expect(screen.getByText(/reputation/i)).toBeInTheDocument();
      });
    });

    it('displays posts count', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('42')).toBeInTheDocument();
        expect(screen.getByText(/posts/i)).toBeInTheDocument();
      });
    });

    it('displays answers count', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('87')).toBeInTheDocument();
        expect(screen.getByText(/answers/i)).toBeInTheDocument();
      });
    });

    it('displays accepted answers count', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('23')).toBeInTheDocument();
        expect(screen.getByText(/accepted/i)).toBeInTheDocument();
      });
    });
  });

  describe('Activity Section', () => {
    it('displays activity section heading', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(
          screen.getByRole('heading', { name: /recent activity/i })
        ).toBeInTheDocument();
      });
    });

    it('fetches and displays activity items', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(
          expect.stringContaining('/v1/users/johndoe/activity')
        );
      });

      await waitFor(() => {
        expect(
          screen.getByText('How to handle async errors in Go?')
        ).toBeInTheDocument();
      });
    });

    it('displays activity timestamps', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        // Check for relative or formatted time
        const activityItems = screen.getAllByTestId('activity-item');
        expect(activityItems.length).toBeGreaterThan(0);
      });
    });

    it('shows activity type badges', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/question/i)).toBeInTheDocument();
      });
    });

    it('shows empty state when no activity', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/users/johndoe') {
          return Promise.resolve(mockUserWithStats);
        }
        if (path.startsWith('/v1/users/johndoe/activity')) {
          return Promise.resolve([]);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText(/no recent activity/i)).toBeInTheDocument();
      });
    });
  });

  describe('Edit Button', () => {
    it('shows edit button when viewing own profile', async () => {
      mockAuthUser = {
        id: 'user-123',
        username: 'johndoe',
        display_name: 'John Doe',
      };

      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        const editButton = screen.getByRole('link', { name: /edit profile/i });
        expect(editButton).toBeInTheDocument();
        expect(editButton).toHaveAttribute('href', '/settings');
      });
    });

    it('hides edit button when viewing other profile', async () => {
      mockAuthUser = {
        id: 'different-user',
        username: 'differentuser',
        display_name: 'Different User',
      };

      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('John Doe')).toBeInTheDocument();
      });

      expect(
        screen.queryByRole('link', { name: /edit profile/i })
      ).not.toBeInTheDocument();
    });

    it('hides edit button when not logged in', async () => {
      mockAuthUser = null;

      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByText('John Doe')).toBeInTheDocument();
      });

      expect(
        screen.queryByRole('link', { name: /edit profile/i })
      ).not.toBeInTheDocument();
    });
  });

  describe('404 Handling', () => {
    it('calls notFound when user does not exist', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/users/johndoe') {
          return Promise.reject(new ApiError(404, 'NOT_FOUND', 'User not found'));
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<UserProfilePage />);
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
        render(<UserProfilePage />);
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
        render(<UserProfilePage />);
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
        if (path === '/v1/users/johndoe') {
          return Promise.resolve(mockUserWithStats);
        }
        if (path.startsWith('/v1/users/johndoe/activity')) {
          return Promise.resolve(mockUserActivity);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument();
      });

      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: /try again/i }));
      });

      await waitFor(() => {
        expect(screen.getByText('John Doe')).toBeInTheDocument();
      });
    });
  });

  describe('Accessibility', () => {
    it('has proper heading hierarchy', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument();
        expect(screen.getByRole('heading', { level: 2, name: /recent activity/i })).toBeInTheDocument();
      });
    });

    it('has accessible labels for stats', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        // Stats should have accessible labels
        const statsSection = screen.getByTestId('user-stats');
        expect(statsSection).toBeInTheDocument();
      });
    });

    it('uses semantic article elements for activity items', async () => {
      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        const activityItems = screen.getAllByTestId('activity-item');
        activityItems.forEach((item) => {
          expect(item.tagName.toLowerCase()).toBe('article');
        });
      });
    });
  });

  describe('User Agents Link', () => {
    it('links to user agents when user has agents', async () => {
      mockApiGet.mockImplementation((path: string) => {
        if (path === '/v1/users/johndoe') {
          return Promise.resolve({
            ...mockUserWithStats,
            agents_count: 3,
          });
        }
        if (path.startsWith('/v1/users/johndoe/activity')) {
          return Promise.resolve(mockUserActivity);
        }
        return Promise.reject(new ApiError(404, 'NOT_FOUND', 'Not found'));
      });

      await act(async () => {
        render(<UserProfilePage />);
      });

      await waitFor(() => {
        const agentsLink = screen.getByRole('link', { name: /3 agents/i });
        expect(agentsLink).toBeInTheDocument();
      });
    });
  });
});
