/**
 * Tests for Admin Page component
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from PRD lines 517-519:
 * - Create /admin page
 * - Admin: overview stats (Display key metrics from /v1/admin/stats)
 * - Admin: quick links (Links to flags, users, audit sub-pages)
 */

import { render, screen, waitFor, fireEvent } from '@testing-library/react';

// Track router push calls
const mockPush = jest.fn();
const mockReplace = jest.fn();

// Mock next/navigation
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace, back: jest.fn() }),
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
import AdminPage from '../app/admin/page';

// Test data - Admin stats per SPEC.md Part 16.3
const mockStats = {
  users_count: 1250,
  agents_count: 89,
  posts_count: 4567,
  flags_count: 12,
  rate_limit_hits: 34,
  active_users_24h: 156,
};

// Test data - Recent flags
const mockRecentFlags = [
  {
    id: 'flag-1',
    target_type: 'post',
    target_id: 'post-123',
    reporter_type: 'human',
    reporter_id: 'user-456',
    reason: 'spam',
    status: 'pending',
    created_at: '2026-01-31T10:00:00Z',
  },
  {
    id: 'flag-2',
    target_type: 'comment',
    target_id: 'comment-789',
    reporter_type: 'agent',
    reporter_id: 'agent_claude',
    reason: 'offensive',
    status: 'pending',
    created_at: '2026-01-31T09:30:00Z',
  },
];

describe('AdminPage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockAuthUser = mockAdminUser;
    mockAuthLoading = false;

    // Default successful API responses
    mockApiGet.mockImplementation((url: string) => {
      if (url.includes('/admin/stats')) {
        return Promise.resolve(mockStats);
      }
      if (url.includes('/admin/flags')) {
        return Promise.resolve({ data: mockRecentFlags, total: mockRecentFlags.length });
      }
      return Promise.reject(new Error('Unknown endpoint'));
    });
  });

  // === Basic Structure Tests ===

  describe('Basic Structure', () => {
    it('renders admin page with main heading', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: /admin/i, level: 1 })).toBeInTheDocument();
      });
    });

    it('renders admin dashboard container', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        expect(screen.getByRole('main')).toBeInTheDocument();
      });
    });
  });

  // === Authentication & Authorization Tests ===

  describe('Authentication & Authorization', () => {
    it('shows loading state while checking auth', () => {
      mockAuthLoading = true;

      render(<AdminPage />);

      expect(screen.getByRole('status')).toBeInTheDocument();
      expect(screen.getByLabelText(/loading/i)).toBeInTheDocument();
    });

    it('redirects to login when not authenticated', async () => {
      mockAuthUser = null;

      render(<AdminPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith('/login');
      });
    });

    it('redirects to home when user is not admin', async () => {
      mockAuthUser = mockRegularUser;

      render(<AdminPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith('/');
      });
    });

    it('renders content when user is admin', async () => {
      mockAuthUser = mockAdminUser;

      render(<AdminPage />);

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: /admin/i, level: 1 })).toBeInTheDocument();
      });
      expect(mockReplace).not.toHaveBeenCalled();
    });

    it('renders content when user is super_admin', async () => {
      mockAuthUser = { ...mockAdminUser, role: 'super_admin' };

      render(<AdminPage />);

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: /admin/i, level: 1 })).toBeInTheDocument();
      });
      expect(mockReplace).not.toHaveBeenCalled();
    });
  });

  // === Stats Display Tests ===

  describe('Overview Stats', () => {
    it('fetches stats from /v1/admin/stats', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith('/v1/admin/stats');
      });
    });

    it('displays users count stat', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        // Find the "1,250" value - this is uniquely the users count
        expect(screen.getByText('1,250')).toBeInTheDocument();
        // Check that there's at least one "Users" label in the page
        const usersTexts = screen.getAllByText(/users/i);
        expect(usersTexts.length).toBeGreaterThan(0);
      });
    });

    it('displays agents count stat', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        // Find the "89" value - this is uniquely the agents count
        expect(screen.getByText('89')).toBeInTheDocument();
        // Check that there's at least one "Agents" label in the page
        const agentsTexts = screen.getAllByText(/agents/i);
        expect(agentsTexts.length).toBeGreaterThan(0);
      });
    });

    it('displays posts count stat', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        expect(screen.getByText('4,567')).toBeInTheDocument();
        expect(screen.getByText(/posts/i)).toBeInTheDocument();
      });
    });

    it('displays pending flags count stat', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        expect(screen.getByText('12')).toBeInTheDocument();
        expect(screen.getByText(/pending flags/i)).toBeInTheDocument();
      });
    });

    it('displays active users stat', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        expect(screen.getByText('156')).toBeInTheDocument();
        expect(screen.getByText(/active.*24h/i)).toBeInTheDocument();
      });
    });

    it('shows loading skeletons while fetching stats', () => {
      mockApiGet.mockImplementation(() => new Promise(() => {})); // Never resolves

      render(<AdminPage />);

      expect(screen.getAllByTestId('stat-skeleton').length).toBeGreaterThan(0);
    });
  });

  // === Quick Links Tests ===

  describe('Quick Links', () => {
    it('displays link to flags page', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        const flagsLinks = screen.getAllByRole('link', { name: /flags/i });
        expect(flagsLinks.length).toBeGreaterThan(0);
        expect(flagsLinks.some(link => link.getAttribute('href') === '/admin/flags')).toBe(true);
      });
    });

    it('displays link to users page', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        const usersLinks = screen.getAllByRole('link', { name: /users/i });
        expect(usersLinks.length).toBeGreaterThan(0);
        expect(usersLinks.some(link => link.getAttribute('href') === '/admin/users')).toBe(true);
      });
    });

    it('displays link to audit log page', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        const auditLink = screen.getByRole('link', { name: /audit/i });
        expect(auditLink).toBeInTheDocument();
        expect(auditLink).toHaveAttribute('href', '/admin/audit');
      });
    });

    it('displays link to agents management', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        const agentsLink = screen.getByRole('link', { name: /manage agents/i });
        expect(agentsLink).toBeInTheDocument();
        expect(agentsLink).toHaveAttribute('href', '/admin/agents');
      });
    });
  });

  // === Recent Flags Section Tests ===

  describe('Recent Flags Section', () => {
    it('displays recent flags section heading', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: /recent flags/i })).toBeInTheDocument();
      });
    });

    it('fetches recent flags from API', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(expect.stringContaining('/admin/flags'));
      });
    });

    it('displays flag items with reason', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        expect(screen.getByText(/spam/i)).toBeInTheDocument();
        expect(screen.getByText(/offensive/i)).toBeInTheDocument();
      });
    });

    it('displays flag target type', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        // Check for "on post" and "on comment" text
        expect(screen.getByText(/on post/i)).toBeInTheDocument();
        expect(screen.getByText(/on comment/i)).toBeInTheDocument();
      });
    });

    it('shows "View all flags" link', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        const viewAllLink = screen.getByRole('link', { name: /view all flags/i });
        expect(viewAllLink).toBeInTheDocument();
        expect(viewAllLink).toHaveAttribute('href', '/admin/flags');
      });
    });

    it('shows empty state when no flags', async () => {
      mockApiGet.mockImplementation((url: string) => {
        if (url.includes('/admin/stats')) {
          return Promise.resolve(mockStats);
        }
        if (url.includes('/admin/flags')) {
          return Promise.resolve({ data: [], total: 0 });
        }
        return Promise.reject(new Error('Unknown endpoint'));
      });

      render(<AdminPage />);

      await waitFor(() => {
        expect(screen.getByText(/no pending flags/i)).toBeInTheDocument();
      });
    });
  });

  // === Error Handling Tests ===

  describe('Error Handling', () => {
    it('displays error message when stats fetch fails', async () => {
      mockApiGet.mockRejectedValue(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));

      render(<AdminPage />);

      await waitFor(() => {
        expect(screen.getByText(/failed to load/i)).toBeInTheDocument();
      });
    });

    it('shows retry button on error', async () => {
      mockApiGet.mockRejectedValue(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));

      render(<AdminPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
      });
    });

    it('retries fetching data when retry button is clicked', async () => {
      mockApiGet.mockRejectedValueOnce(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));

      render(<AdminPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
      });

      // Reset mock to succeed on retry
      mockApiGet.mockImplementation((url: string) => {
        if (url.includes('/admin/stats')) {
          return Promise.resolve(mockStats);
        }
        if (url.includes('/admin/flags')) {
          return Promise.resolve({ data: mockRecentFlags, total: mockRecentFlags.length });
        }
        return Promise.reject(new Error('Unknown endpoint'));
      });

      fireEvent.click(screen.getByRole('button', { name: /retry/i }));

      await waitFor(() => {
        expect(screen.getByText('1,250')).toBeInTheDocument();
      });
    });
  });

  // === Accessibility Tests ===

  describe('Accessibility', () => {
    it('has proper heading hierarchy', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        const h1 = screen.getByRole('heading', { level: 1 });
        expect(h1).toBeInTheDocument();
      });

      // Section headings should be h2
      const h2s = screen.getAllByRole('heading', { level: 2 });
      expect(h2s.length).toBeGreaterThan(0);
    });

    it('has accessible stat cards with labels', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        // Stats should be in a region or section
        const statsSection = screen.getByRole('region', { name: /statistics/i });
        expect(statsSection).toBeInTheDocument();
      });
    });

    it('has accessible navigation links', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        const nav = screen.getByRole('navigation', { name: /admin/i });
        expect(nav).toBeInTheDocument();
      });
    });
  });

  // === Layout Tests ===

  describe('Layout', () => {
    it('renders stats in a grid layout', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        const statsGrid = screen.getByTestId('stats-grid');
        expect(statsGrid).toHaveClass('grid');
      });
    });

    it('renders quick links in a grid', async () => {
      render(<AdminPage />);

      await waitFor(() => {
        const linksGrid = screen.getByTestId('quick-links');
        expect(linksGrid).toHaveClass('grid');
      });
    });
  });
});
