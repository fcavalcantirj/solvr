/**
 * Tests for Admin Audit Log Page component
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from PRD line 526:
 * - Create /admin/audit page
 * - List audit log with filters
 */

import { render, screen, waitFor, fireEvent } from '@testing-library/react';
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
jest.mock('@/lib/api', () => ({
  api: {
    get: (...args: unknown[]) => mockApiGet(...args),
    patch: jest.fn(),
    post: jest.fn(),
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
import AdminAuditPage from '../app/admin/audit/page';

// Test data - Audit log entries
const mockAuditEntries = [
  {
    id: 'audit-1',
    admin_id: 'admin-123',
    admin_name: 'Admin User',
    action: 'ban_user',
    target_type: 'user',
    target_id: 'user-456',
    details: { reason: 'Spam' },
    ip_address: '192.168.1.1',
    created_at: '2026-01-31T19:00:00Z',
  },
  {
    id: 'audit-2',
    admin_id: 'admin-123',
    admin_name: 'Admin User',
    action: 'dismiss_flag',
    target_type: 'flag',
    target_id: 'flag-789',
    details: null,
    ip_address: '192.168.1.1',
    created_at: '2026-01-31T18:45:00Z',
  },
  {
    id: 'audit-3',
    admin_id: 'admin-456',
    admin_name: 'Super Admin',
    action: 'delete',
    target_type: 'post',
    target_id: 'post-123',
    details: { reason: 'Inappropriate content' },
    ip_address: '192.168.1.2',
    created_at: '2026-01-31T18:30:00Z',
  },
  {
    id: 'audit-4',
    admin_id: 'admin-123',
    admin_name: 'Admin User',
    action: 'warn_user',
    target_type: 'user',
    target_id: 'user-abc',
    details: { message: 'First warning' },
    ip_address: '192.168.1.1',
    created_at: '2026-01-31T18:00:00Z',
  },
];

describe('AdminAuditPage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockAuthUser = mockAdminUser;
    mockAuthLoading = false;

    // Default successful API responses
    mockApiGet.mockImplementation((url: string) => {
      if (url.includes('/admin/audit')) {
        return Promise.resolve({
          data: mockAuditEntries,
          total: mockAuditEntries.length,
          page: 1,
        });
      }
      return Promise.reject(new Error('Unknown endpoint'));
    });
  });

  // === Basic Structure Tests ===

  describe('Basic Structure', () => {
    it('renders audit log page with heading', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('heading', { name: /audit/i, level: 1 })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('renders main container', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('main')).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays back link to admin dashboard', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          const backLink = screen.getByRole('link', { name: /back to admin/i });
          expect(backLink).toBeInTheDocument();
          expect(backLink).toHaveAttribute('href', '/admin');
        },
        { timeout: 10000 }
      );
    });

    it('displays description text', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByText(/admin action history/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });
  });

  // === Authentication Tests ===

  describe('Authentication', () => {
    it('shows loading state while checking auth', async () => {
      mockAuthLoading = true;
      mockAuthUser = null;

      render(<AdminAuditPage />);

      await waitFor(() => {
        expect(screen.getByRole('status', { name: /loading/i })).toBeInTheDocument();
      });
    });

    it('redirects to login when not authenticated', async () => {
      mockAuthLoading = false;
      mockAuthUser = null;

      render(<AdminAuditPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith('/login');
      });
    });

    it('redirects to home when user is not admin', async () => {
      mockAuthUser = mockRegularUser;

      render(<AdminAuditPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith('/');
      });
    });

    it('renders content when user is admin', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('heading', { name: /audit/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('renders content when user is super_admin', async () => {
      mockAuthUser = { ...mockAdminUser, role: 'super_admin' };

      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('heading', { name: /audit/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });
  });

  // === Audit Log Display Tests ===

  describe('Audit Log Display', () => {
    it('fetches audit log from API', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(mockApiGet).toHaveBeenCalledWith(expect.stringContaining('/admin/audit'));
        },
        { timeout: 10000 }
      );
    });

    it('displays audit log entries', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByText(/ban_user/i)).toBeInTheDocument();
          expect(screen.getByText(/dismiss_flag/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays admin name for each entry', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getAllByText(/admin user/i).length).toBeGreaterThan(0);
          expect(screen.getByText(/super admin/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays target type for each entry', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          // Check for target type badges - use getAllByText for types that appear multiple times
          const userBadges = screen.getAllByText('user');
          expect(userBadges.length).toBeGreaterThan(0);
          expect(screen.getByText('post')).toBeInTheDocument();
          expect(screen.getByText('flag')).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('displays timestamp for each entry', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          // Check for formatted dates
          expect(screen.getAllByText(/jan.*31.*2026/i).length).toBeGreaterThan(0);
        },
        { timeout: 10000 }
      );
    });

    it('shows empty state when no entries', async () => {
      mockApiGet.mockResolvedValue({ data: [], total: 0, page: 1 });

      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByText(/no audit entries/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });
  });

  // === Filter Tests ===

  describe('Filters', () => {
    it('displays action filter dropdown', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('combobox', { name: /action/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('filters by action type', async () => {
      const user = userEvent.setup();
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('combobox', { name: /action/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );

      // Select an action filter
      await user.selectOptions(screen.getByRole('combobox', { name: /action/i }), 'ban_user');

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(expect.stringContaining('action=ban_user'));
      });
    });

    it('displays date range filter', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByLabelText(/from date/i)).toBeInTheDocument();
          expect(screen.getByLabelText(/to date/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('filters by date range', async () => {
      const user = userEvent.setup();
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByLabelText(/from date/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );

      // Set a from date
      await user.type(screen.getByLabelText(/from date/i), '2026-01-01');

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(expect.stringContaining('from_date=2026-01-01'));
      });
    });
  });

  // === Pagination Tests ===

  describe('Pagination', () => {
    it('displays pagination controls when multiple pages', async () => {
      mockApiGet.mockResolvedValue({
        data: mockAuditEntries,
        total: 50,
        page: 1,
      });

      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('navigation', { name: /pagination/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('shows page number', async () => {
      mockApiGet.mockResolvedValue({
        data: mockAuditEntries,
        total: 50,
        page: 1,
      });

      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByText(/page 1/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('navigates to next page', async () => {
      const user = userEvent.setup();
      mockApiGet.mockResolvedValue({
        data: mockAuditEntries,
        total: 50,
        page: 1,
      });

      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /next/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );

      await user.click(screen.getByRole('button', { name: /next/i }));

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(expect.stringContaining('page=2'));
      });
    });

    it('disables previous button on first page', async () => {
      mockApiGet.mockResolvedValue({
        data: mockAuditEntries,
        total: 50,
        page: 1,
      });

      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /previous/i })).toBeDisabled();
        },
        { timeout: 10000 }
      );
    });
  });

  // === Entry Details Tests ===

  describe('Entry Details', () => {
    it('can expand entry to show details', async () => {
      const user = userEvent.setup();
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByText(/ban_user/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );

      // Click to expand the first entry
      const expandButtons = screen.getAllByRole('button', { name: /expand/i });
      await user.click(expandButtons[0]);

      await waitFor(() => {
        expect(screen.getByText(/spam/i)).toBeInTheDocument();
      });
    });

    it('shows IP address in expanded view', async () => {
      const user = userEvent.setup();
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByText(/ban_user/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );

      // Click to expand the first entry
      const expandButtons = screen.getAllByRole('button', { name: /expand/i });
      await user.click(expandButtons[0]);

      await waitFor(() => {
        expect(screen.getByText(/192\.168\.1\.1/)).toBeInTheDocument();
      });
    });
  });

  // === Error Handling Tests ===

  describe('Error Handling', () => {
    it('displays error when fetch fails', async () => {
      mockApiGet.mockRejectedValue(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));

      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByText(/failed to load/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('shows retry button on error', async () => {
      mockApiGet.mockRejectedValue(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));

      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('retries fetch when retry button clicked', async () => {
      const user = userEvent.setup();
      mockApiGet.mockRejectedValueOnce(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));
      mockApiGet.mockResolvedValueOnce({ data: mockAuditEntries, total: mockAuditEntries.length, page: 1 });

      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
        },
        { timeout: 10000 }
      );

      await user.click(screen.getByRole('button', { name: /retry/i }));

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledTimes(2);
      });
    });
  });

  // === Loading States Tests ===

  describe('Loading States', () => {
    it('shows loading skeleton while fetching entries', async () => {
      // Make API hang to keep loading state
      mockApiGet.mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve({ data: mockAuditEntries, total: 4, page: 1 }), 5000))
      );

      render(<AdminAuditPage />);

      await waitFor(() => {
        expect(screen.getByRole('status', { name: /loading audit/i })).toBeInTheDocument();
      });
    });
  });

  // === Accessibility Tests ===

  describe('Accessibility', () => {
    it('has proper heading hierarchy', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          const h1 = screen.getByRole('heading', { level: 1 });
          expect(h1).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('has accessible filter inputs with labels', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByLabelText(/action/i)).toBeInTheDocument();
          expect(screen.getByLabelText(/from date/i)).toBeInTheDocument();
          expect(screen.getByLabelText(/to date/i)).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });

    it('has accessible table structure', async () => {
      render(<AdminAuditPage />);

      await waitFor(
        () => {
          expect(screen.getByRole('table')).toBeInTheDocument();
        },
        { timeout: 10000 }
      );
    });
  });
});
