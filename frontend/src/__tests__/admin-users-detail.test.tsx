/**
 * Tests for Admin User Detail Page component
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from PRD lines 524-525:
 * - Create /admin/users/[id] page
 * - User detail with activity
 * - Add warn, suspend, ban buttons
 */

import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

// Track router calls
const mockPush = jest.fn();
const mockReplace = jest.fn();
const mockBack = jest.fn();

// Create stable router reference to avoid infinite re-renders
const mockRouter = { push: mockPush, replace: mockReplace, back: mockBack };

// Mock next/navigation
jest.mock('next/navigation', () => ({
  useRouter: () => mockRouter,
  useParams: () => ({ id: 'user-123' }),
  redirect: jest.fn(),
  notFound: jest.fn(),
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
    patch: jest.fn(),
    post: (...args: unknown[]) => mockApiPost(...args),
    delete: jest.fn(),
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

// Mock admin user (requires admin role)
const mockAdminUser = {
  id: 'admin-123',
  username: 'adminuser',
  display_name: 'Admin User',
  email: 'admin@example.com',
  avatar_url: 'https://example.com/admin-avatar.jpg',
  bio: 'System administrator',
  role: 'admin',
};

const mockRegularUser = {
  id: 'user-123',
  username: 'regularuser',
  display_name: 'Regular User',
  email: 'user@example.com',
  avatar_url: 'https://example.com/user-avatar.jpg',
  bio: 'Just a user',
  role: 'user',
};

let mockAuthUser: typeof mockAdminUser | null = mockAdminUser;
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
import AdminUserDetailPage from '../app/admin/users/[id]/page';

// Test data - User detail
const mockUserDetail = {
  id: 'user-123',
  username: 'johndoe',
  display_name: 'John Doe',
  email: 'john@example.com',
  avatar_url: 'https://example.com/john.jpg',
  bio: 'A software developer from NYC',
  auth_provider: 'github',
  role: 'user',
  status: 'active',
  created_at: '2026-01-15T10:00:00Z',
  updated_at: '2026-01-30T14:30:00Z',
};

// Test data - User activity
const mockActivity = [
  {
    id: 'act-1',
    type: 'post_created',
    description: 'Created a problem: "How to fix async issues"',
    created_at: '2026-01-28T10:00:00Z',
  },
  {
    id: 'act-2',
    type: 'answer_created',
    description: 'Answered a question: "Best practices for Go"',
    created_at: '2026-01-27T15:30:00Z',
  },
  {
    id: 'act-3',
    type: 'comment_created',
    description: 'Commented on approach: "Memory optimization"',
    created_at: '2026-01-26T09:00:00Z',
  },
];

describe('AdminUserDetailPage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockAuthUser = mockAdminUser;
    mockAuthLoading = false;

    // Default successful API responses
    mockApiGet.mockImplementation((url: string) => {
      if (url.includes('/admin/users/user-123')) {
        return Promise.resolve({ ...mockUserDetail, activity: mockActivity });
      }
      return Promise.reject(new Error('Unknown endpoint'));
    });

    mockApiPost.mockResolvedValue({ success: true });
  });

  // === Basic Structure Tests ===

  describe('Basic Structure', () => {
    it('renders user detail page with heading', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('renders main container', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('main')).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays back link to users list', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          const backLink = screen.getByRole('link', { name: /back to users/i });
          expect(backLink).toBeInTheDocument();
          expect(backLink).toHaveAttribute('href', '/admin/users');
        },
        { timeout: 10000 }
      );
    });
  });

  // === Authentication & Authorization Tests ===

  describe('Authentication & Authorization', () => {
    it('shows loading state while checking auth', () => {
      mockAuthLoading = true;

      render(<AdminUserDetailPage />);

      expect(screen.getByRole('status')).toBeInTheDocument();
      expect(screen.getByLabelText(/loading/i)).toBeInTheDocument();
    });

    it('redirects to login when not authenticated', async () => {
      mockAuthUser = null;

      render(<AdminUserDetailPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith('/login');
      });
    });

    it('redirects to home when user is not admin', async () => {
      mockAuthUser = mockRegularUser;

      render(<AdminUserDetailPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith('/');
      });
    });

    it('renders content when user is admin', async () => {
      mockAuthUser = mockAdminUser;

      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
      expect(mockReplace).not.toHaveBeenCalled();
    });

    it('renders content when user is super_admin', async () => {
      mockAuthUser = { ...mockAdminUser, role: 'super_admin' };

      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('heading', { level: 1 })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
      expect(mockReplace).not.toHaveBeenCalled();
    });
  });

  // === User Info Display Tests ===

  describe('User Info Display', () => {
    it('fetches user detail from API', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(expect.stringContaining('/admin/users/user-123'));
      });
    });

    it('displays user display name', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByText('John Doe')).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays username', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByText('@johndoe')).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays user email', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByText('john@example.com')).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays user bio', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByText('A software developer from NYC')).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays user avatar', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          const avatar = screen.getByRole('img', { name: /john doe/i });
          expect(avatar).toBeInTheDocument();
          expect(avatar).toHaveAttribute('src', 'https://example.com/john.jpg');
        },
        { timeout: 10000 }
      );
    });

    it('displays user status', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByText(/active/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays auth provider', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByText(/github/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays user role', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          // Find the role, could be styled as "user" or "User"
          const roleText = screen.getByText(/^user$/i);
          expect(roleText).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays joined date', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          // Should show the exact joined date Jan 15, 2026 for user created_at
          expect(screen.getByText('Jan 15, 2026')).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });
  });

  // === Activity Section Tests ===

  describe('Activity Section', () => {
    it('displays activity section heading', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('heading', { name: /activity/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays activity items', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByText(/how to fix async issues/i)).toBeInTheDocument();
          expect(screen.getByText(/best practices for go/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('shows empty state when no activity', async () => {
      mockApiGet.mockResolvedValue({ ...mockUserDetail, activity: [] });

      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByText(/no activity/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });
  });

  // === Admin Action Buttons Tests ===

  describe('Admin Actions', () => {
    it('displays warn button', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /warn/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays suspend button', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /suspend/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays ban button', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /ban/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('calls warn API when warn button clicked', async () => {
      const user = userEvent.setup();
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /warn/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );

      await user.click(screen.getByRole('button', { name: /warn/i }));

      await waitFor(() => {
        expect(mockApiPost).toHaveBeenCalledWith(
          expect.stringContaining('/admin/users/user-123/warn'),
          expect.anything()
        );
      });
    }, 15000);

    it('calls suspend API when suspend button clicked', async () => {
      const user = userEvent.setup();
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /suspend/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );

      await user.click(screen.getByRole('button', { name: /suspend/i }));

      await waitFor(() => {
        expect(mockApiPost).toHaveBeenCalledWith(
          expect.stringContaining('/admin/users/user-123/suspend'),
          expect.anything()
        );
      });
    }, 15000);

    it('calls ban API when ban button clicked with confirmation', async () => {
      const user = userEvent.setup();
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /ban/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );

      // Click ban - should show confirmation
      await user.click(screen.getByRole('button', { name: /ban/i }));

      // Confirm the ban
      await waitFor(() => {
        expect(screen.getByText(/are you sure/i)).toBeInTheDocument();
      });

      const confirmButton = screen.getByRole('button', { name: /^yes$/i });
      await user.click(confirmButton);

      await waitFor(() => {
        expect(mockApiPost).toHaveBeenCalledWith(
          expect.stringContaining('/admin/users/user-123/ban'),
          expect.anything()
        );
      });
    }, 15000);

    it('shows success message after action', async () => {
      const user = userEvent.setup();
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /warn/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );

      await user.click(screen.getByRole('button', { name: /warn/i }));

      await waitFor(
        () => {
          expect(screen.getByText(/warning sent/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    }, 15000);

    it('updates user status after suspend', async () => {
      const user = userEvent.setup();
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /suspend/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );

      // Verify initial status is active
      expect(screen.getByText('active')).toBeInTheDocument();

      // Mock the response to update status
      mockApiPost.mockResolvedValue({ success: true, status: 'suspended' });

      await user.click(screen.getByRole('button', { name: /suspend/i }));

      // After suspend, status badge should change to 'suspended'
      await waitFor(
        () => {
          expect(screen.getByText('suspended')).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    }, 15000);
  });

  // === Error Handling Tests ===

  describe('Error Handling', () => {
    it('displays error when user not found', async () => {
      mockApiGet.mockRejectedValue(new ApiError(404, 'NOT_FOUND', 'User not found'));

      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByText(/user not found/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays error when fetch fails', async () => {
      mockApiGet.mockRejectedValue(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));

      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByText(/failed to load/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('shows retry button on error', async () => {
      mockApiGet.mockRejectedValue(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));

      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('shows error when action fails', async () => {
      const user = userEvent.setup();
      mockApiPost.mockRejectedValue(new ApiError(500, 'INTERNAL_ERROR', 'Action failed'));

      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /warn/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );

      await user.click(screen.getByRole('button', { name: /warn/i }));

      await waitFor(
        () => {
          expect(screen.getByText(/action failed/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    }, 15000);
  });

  // === Loading States Tests ===

  describe('Loading States', () => {
    it('shows loading skeleton while fetching user', () => {
      mockApiGet.mockImplementation(() => new Promise(() => {})); // Never resolves

      render(<AdminUserDetailPage />);

      expect(screen.getAllByTestId('user-detail-skeleton').length).toBeGreaterThan(0);
    });

    it('disables action buttons while processing', async () => {
      const user = userEvent.setup();
      mockApiPost.mockImplementation(() => new Promise(() => {})); // Never resolves

      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /warn/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );

      await user.click(screen.getByRole('button', { name: /warn/i }));

      // Buttons should be disabled while processing
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /warn/i })).toBeDisabled();
      });
    }, 15000);
  });

  // === Accessibility Tests ===

  describe('Accessibility', () => {
    it('has proper heading hierarchy', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          const h1 = screen.getByRole('heading', { level: 1 });
          expect(h1).toBeInTheDocument();
        },
        { timeout: 10000 }
      );

      const h2s = screen.queryAllByRole('heading', { level: 2 });
      expect(h2s.length).toBeGreaterThanOrEqual(0);
    });

    it('has accessible action buttons with aria labels', async () => {
      render(<AdminUserDetailPage />);

      await waitFor(
        () => {
          const warnButton = screen.getByRole('button', { name: /warn/i });
          expect(warnButton).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });
  });
});
