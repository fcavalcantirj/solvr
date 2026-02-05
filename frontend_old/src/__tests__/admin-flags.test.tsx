/**
 * Tests for Admin Flags Page component
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from PRD lines 520-522:
 * - Create /admin/flags page (List pending flags)
 * - Flags: content preview (Show flagged content preview)
 * - Flags: action buttons (Add dismiss, warn, hide, delete buttons)
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
const mockApiPost = jest.fn();
const mockApiDelete = jest.fn();
jest.mock('@/lib/api', () => ({
  api: {
    get: (...args: unknown[]) => mockApiGet(...args),
    patch: jest.fn(),
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
import AdminFlagsPage from '../app/admin/flags/page';

// Test data - Flags with content preview
const mockFlags = [
  {
    id: 'flag-1',
    target_type: 'post',
    target_id: 'post-123',
    reporter_type: 'human',
    reporter_id: 'user-456',
    reason: 'spam',
    details: 'This post is advertising a scam website',
    status: 'pending',
    created_at: '2026-01-31T10:00:00Z',
    content_preview: {
      title: 'Buy cheap watches - best prices!',
      snippet: 'Amazing deals on luxury watches...',
      type: 'problem',
    },
    reporter: {
      id: 'user-456',
      display_name: 'John Reporter',
      type: 'human',
    },
  },
  {
    id: 'flag-2',
    target_type: 'comment',
    target_id: 'comment-789',
    reporter_type: 'agent',
    reporter_id: 'agent_claude',
    reason: 'offensive',
    details: 'Contains inappropriate language',
    status: 'pending',
    created_at: '2026-01-31T09:30:00Z',
    content_preview: {
      text: 'This comment contains some offensive words...',
      type: 'comment',
    },
    reporter: {
      id: 'agent_claude',
      display_name: 'Claude',
      type: 'agent',
    },
  },
  {
    id: 'flag-3',
    target_type: 'answer',
    target_id: 'answer-456',
    reporter_type: 'human',
    reporter_id: 'user-789',
    reason: 'incorrect',
    details: 'This answer is factually wrong',
    status: 'reviewed',
    created_at: '2026-01-30T15:00:00Z',
    content_preview: {
      text: 'The answer claims that...',
      type: 'answer',
    },
    reporter: {
      id: 'user-789',
      display_name: 'Jane Reviewer',
      type: 'human',
    },
  },
];

describe('AdminFlagsPage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockAuthUser = mockAdminUser;
    mockAuthLoading = false;

    // Default successful API responses
    mockApiGet.mockImplementation((url: string) => {
      if (url.includes('/admin/flags')) {
        return Promise.resolve({ data: mockFlags, total: mockFlags.length, page: 1 });
      }
      return Promise.reject(new Error('Unknown endpoint'));
    });

    mockApiPost.mockResolvedValue({});
    mockApiDelete.mockResolvedValue({});
  });

  // === Basic Structure Tests ===

  describe('Basic Structure', () => {
    it('renders flags page with main heading', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: /flags/i, level: 1 })).toBeInTheDocument();
      });
    });

    it('renders main container', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByRole('main')).toBeInTheDocument();
      });
    });

    it('shows back link to admin dashboard', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        const backLink = screen.getByRole('link', { name: /back.*admin/i });
        expect(backLink).toHaveAttribute('href', '/admin');
      });
    });
  });

  // === Authentication & Authorization Tests ===

  describe('Authentication & Authorization', () => {
    it('shows loading state while checking auth', () => {
      mockAuthLoading = true;

      render(<AdminFlagsPage />);

      expect(screen.getByRole('status')).toBeInTheDocument();
    });

    it('redirects to login when not authenticated', async () => {
      mockAuthUser = null;

      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith('/login');
      });
    });

    it('redirects to home when user is not admin', async () => {
      mockAuthUser = mockRegularUser;

      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith('/');
      });
    });
  });

  // === Flags List Tests ===

  describe('Flags List', () => {
    it('fetches flags from API', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(expect.stringContaining('/admin/flags'));
      });
    });

    it('displays flag items', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        // Use getAllByText since reason might appear in both badge and content
        const spamElements = screen.getAllByText(/spam/i);
        const offensiveElements = screen.getAllByText(/offensive/i);
        expect(spamElements.length).toBeGreaterThan(0);
        expect(offensiveElements.length).toBeGreaterThan(0);
      });
    });

    it('displays flag target type badge', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByText(/post/i)).toBeInTheDocument();
        expect(screen.getByText(/comment/i)).toBeInTheDocument();
      });
    });

    it('displays reporter information', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        // Reporter info is shown as "Reported by {name} ({type})"
        expect(screen.getByText(/reported by john reporter/i)).toBeInTheDocument();
        expect(screen.getByText(/reported by claude/i)).toBeInTheDocument();
      });
    });

    it('displays flag creation date', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        // Should show formatted dates
        expect(screen.getAllByText(/2026/).length).toBeGreaterThan(0);
      });
    });

    it('shows empty state when no flags', async () => {
      mockApiGet.mockResolvedValue({ data: [], total: 0, page: 1 });

      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByText(/no flags/i)).toBeInTheDocument();
      });
    });
  });

  // === Content Preview Tests ===

  describe('Content Preview', () => {
    it('displays content title for posts', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByText(/buy cheap watches/i)).toBeInTheDocument();
      });
    });

    it('displays content snippet for posts', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByText(/amazing deals on luxury watches/i)).toBeInTheDocument();
      });
    });

    it('displays content text for comments', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByText(/contains some offensive words/i)).toBeInTheDocument();
      });
    });

    it('displays flag details/reason', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByText(/advertising a scam website/i)).toBeInTheDocument();
      });
    });

    it('displays link to view full content', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        // Should have links to view the flagged content
        const viewLinks = screen.getAllByRole('link', { name: /view/i });
        expect(viewLinks.length).toBeGreaterThan(0);
      });
    });
  });

  // === Action Buttons Tests ===

  describe('Action Buttons', () => {
    it('displays dismiss button for each flag', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        const dismissButtons = screen.getAllByRole('button', { name: /dismiss/i });
        expect(dismissButtons.length).toBeGreaterThan(0);
      });
    });

    it('displays warn button for each flag', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        const warnButtons = screen.getAllByRole('button', { name: /warn/i });
        expect(warnButtons.length).toBeGreaterThan(0);
      });
    });

    it('displays hide button for each flag', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        const hideButtons = screen.getAllByRole('button', { name: /hide/i });
        expect(hideButtons.length).toBeGreaterThan(0);
      });
    });

    it('displays delete button for each flag', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        const deleteButtons = screen.getAllByRole('button', { name: /delete/i });
        expect(deleteButtons.length).toBeGreaterThan(0);
      });
    });

    it('calls dismiss API when dismiss button is clicked', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getAllByRole('button', { name: /dismiss/i })[0]).toBeInTheDocument();
      });

      fireEvent.click(screen.getAllByRole('button', { name: /dismiss/i })[0]);

      await waitFor(() => {
        expect(mockApiPost).toHaveBeenCalledWith(
          expect.stringContaining('/admin/flags/flag-1/dismiss'),
          expect.anything()
        );
      });
    });

    it('calls hide API when hide button is clicked', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getAllByRole('button', { name: /hide/i })[0]).toBeInTheDocument();
      });

      fireEvent.click(screen.getAllByRole('button', { name: /hide/i })[0]);

      await waitFor(() => {
        expect(mockApiPost).toHaveBeenCalledWith(
          expect.stringContaining('/admin/flags/flag-1/action'),
          expect.objectContaining({ action: 'hide' })
        );
      });
    });

    it('removes flag from list after successful action', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByText(/buy cheap watches/i)).toBeInTheDocument();
      });

      fireEvent.click(screen.getAllByRole('button', { name: /dismiss/i })[0]);

      await waitFor(() => {
        expect(screen.queryByText(/buy cheap watches/i)).not.toBeInTheDocument();
      });
    });

    it('shows confirmation dialog for delete action', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getAllByRole('button', { name: /delete/i })[0]).toBeInTheDocument();
      });

      fireEvent.click(screen.getAllByRole('button', { name: /delete/i })[0]);

      await waitFor(() => {
        expect(screen.getByText(/are you sure/i)).toBeInTheDocument();
      });
    });
  });

  // === Filtering Tests ===

  describe('Filtering', () => {
    it('displays status filter dropdown', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByRole('combobox', { name: /status/i })).toBeInTheDocument();
      });
    });

    it('displays target type filter dropdown', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByRole('combobox', { name: /type/i })).toBeInTheDocument();
      });
    });

    it('filters by status when selected', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByRole('combobox', { name: /status/i })).toBeInTheDocument();
      });

      fireEvent.change(screen.getByRole('combobox', { name: /status/i }), {
        target: { value: 'reviewed' },
      });

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(expect.stringContaining('status=reviewed'));
      });
    });

    it('filters by target type when selected', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByRole('combobox', { name: /type/i })).toBeInTheDocument();
      });

      fireEvent.change(screen.getByRole('combobox', { name: /type/i }), {
        target: { value: 'post' },
      });

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(expect.stringContaining('target_type=post'));
      });
    });
  });

  // === Pagination Tests ===

  describe('Pagination', () => {
    it('displays pagination controls when more than one page', async () => {
      mockApiGet.mockResolvedValue({ data: mockFlags, total: 50, page: 1 });

      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByRole('navigation', { name: /pagination/i })).toBeInTheDocument();
      });
    });

    it('displays current page number', async () => {
      mockApiGet.mockResolvedValue({ data: mockFlags, total: 50, page: 1 });

      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByText(/page 1/i)).toBeInTheDocument();
      });
    });

    it('fetches next page when next button is clicked', async () => {
      mockApiGet.mockResolvedValue({ data: mockFlags, total: 50, page: 1 });

      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /next/i })).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: /next/i }));

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(expect.stringContaining('page=2'));
      });
    });
  });

  // === Error Handling Tests ===

  describe('Error Handling', () => {
    it('displays error message when fetch fails', async () => {
      mockApiGet.mockRejectedValue(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));

      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByText(/failed to load/i)).toBeInTheDocument();
      });
    });

    it('shows retry button on error', async () => {
      mockApiGet.mockRejectedValue(new ApiError(500, 'INTERNAL_ERROR', 'Server error'));

      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
      });
    });

    it('shows error message when action fails', async () => {
      mockApiPost.mockRejectedValue(new ApiError(500, 'INTERNAL_ERROR', 'Action failed'));

      render(<AdminFlagsPage />);

      await waitFor(() => {
        expect(screen.getAllByRole('button', { name: /dismiss/i })[0]).toBeInTheDocument();
      });

      fireEvent.click(screen.getAllByRole('button', { name: /dismiss/i })[0]);

      await waitFor(() => {
        expect(screen.getByText(/action failed/i)).toBeInTheDocument();
      });
    });
  });

  // === Accessibility Tests ===

  describe('Accessibility', () => {
    it('has proper heading hierarchy', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        const h1 = screen.getByRole('heading', { level: 1 });
        expect(h1).toBeInTheDocument();
      });
    });

    it('action buttons have accessible names', async () => {
      render(<AdminFlagsPage />);

      await waitFor(() => {
        const dismissButtons = screen.getAllByRole('button', { name: /dismiss/i });
        expect(dismissButtons[0]).toHaveAccessibleName();
      });
    });

    it('displays loading state with accessible label', () => {
      mockApiGet.mockImplementation(() => new Promise(() => {})); // Never resolves

      render(<AdminFlagsPage />);

      expect(screen.getByRole('status')).toBeInTheDocument();
    });
  });
});
