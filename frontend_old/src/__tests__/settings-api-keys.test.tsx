/**
 * Tests for API Keys Settings Tab
 * TDD approach: RED -> GREEN -> REFACTOR
 * Requirements from prd-v2.json API-KEYS category:
 * - List existing keys with usage stats
 * - Create new key with name input
 * - Show key ONCE on creation (copy button)
 * - Revoke button with confirmation
 */

import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { act } from 'react';

// Track router calls
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

import { ApiError } from '@/lib/api';

// Mock useAuth hook
const mockUser = {
  id: 'user-123',
  username: 'johndoe',
  display_name: 'John Doe',
  email: 'john@example.com',
  avatar_url: null,
  bio: '',
};
let mockAuthUser: typeof mockUser | null = mockUser;
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

// Test data - User's API keys
const mockApiKeys = [
  {
    id: 'key-1',
    name: 'Development Key',
    key_prefix: 'solvr_sk_dev123...',
    created_at: '2025-06-15T10:00:00Z',
    last_used_at: '2025-07-20T14:30:00Z',
    revoked_at: null,
  },
  {
    id: 'key-2',
    name: 'Production Key',
    key_prefix: 'solvr_sk_prod456...',
    created_at: '2025-07-01T08:00:00Z',
    last_used_at: null,
    revoked_at: null,
  },
];

// Import component (import after mocks are set up)
import ApiKeysTab from '../components/settings/ApiKeysTab';

describe('API Keys Tab', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockAuthUser = mockUser;
    mockAuthLoading = false;

    // Default API mock responses
    mockApiGet.mockImplementation((path: string) => {
      if (path === '/v1/users/me/api-keys') {
        return Promise.resolve(mockApiKeys);
      }
      return Promise.resolve({});
    });
    mockApiPost.mockResolvedValue({
      id: 'key-3',
      name: 'New Key',
      api_key: 'solvr_sk_new789xyz...',
      created_at: new Date().toISOString(),
    });
    mockApiDelete.mockResolvedValue({});
  });

  // --- List Keys Tests ---

  describe('List Keys', () => {
    it('renders the API keys section heading', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: /api keys/i })).toBeInTheDocument();
      });
    });

    it('fetches and displays API keys', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith('/v1/users/me/api-keys');
      });

      await waitFor(() => {
        expect(screen.getByText('Development Key')).toBeInTheDocument();
        expect(screen.getByText('Production Key')).toBeInTheDocument();
      });
    });

    it('shows key prefix (masked) for each key', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByText(/solvr_sk_dev123\.\.\./i)).toBeInTheDocument();
        expect(screen.getByText(/solvr_sk_prod456\.\.\./i)).toBeInTheDocument();
      });
    });

    it('shows creation date for each key', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        // Look for formatted dates - component shows "Created: Jun 15, 2025" etc.
        const createdElements = screen.getAllByText(/created/i);
        expect(createdElements.length).toBe(2); // One for each key
      });
    });

    it('shows last used date when available', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        // Should show "Last used: Jul 20, 2025" for key-1 and "Never used" for key-2
        const lastUsedElements = screen.getAllByText(/last used/i);
        expect(lastUsedElements.length).toBe(2);
      });
    });

    it('shows "Never used" when last_used_at is null', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByText(/never used/i)).toBeInTheDocument();
      });
    });

    it('shows loading state while fetching keys', async () => {
      mockApiGet.mockImplementation(() => new Promise(() => {})); // Never resolves

      render(<ApiKeysTab userId="user-123" />);

      expect(screen.getByTestId('api-keys-loading')).toBeInTheDocument();
    });

    it('shows error state and retry button on fetch failure', async () => {
      mockApiGet.mockRejectedValue(new Error('Network error'));

      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByText(/failed to load/i)).toBeInTheDocument();
        expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
      });
    });

    it('shows empty state when no keys exist', async () => {
      mockApiGet.mockResolvedValue([]);

      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByText(/no api keys/i)).toBeInTheDocument();
      });
    });
  });

  // --- Create Key Tests ---

  describe('Create Key', () => {
    it('shows "Create API Key" button', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /create.*api key/i })).toBeInTheDocument();
      });
    });

    it('shows create form when button is clicked', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /create.*api key/i })).toBeInTheDocument();
      });

      const createButton = screen.getByRole('button', { name: /create.*api key/i });
      await act(async () => {
        fireEvent.click(createButton);
      });

      expect(screen.getByLabelText(/key name/i)).toBeInTheDocument();
    });

    it('validates key name is required', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /create.*api key/i })).toBeInTheDocument();
      });

      // Open form
      const createButton = screen.getByRole('button', { name: /create.*api key/i });
      await act(async () => {
        fireEvent.click(createButton);
      });

      // Submit without entering name
      const submitButton = screen.getByRole('button', { name: /^create$/i });
      await act(async () => {
        fireEvent.click(submitButton);
      });

      expect(screen.getByText(/name is required/i)).toBeInTheDocument();
      expect(mockApiPost).not.toHaveBeenCalled();
    });

    it('creates key with valid name', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /create.*api key/i })).toBeInTheDocument();
      });

      // Open form
      const createButton = screen.getByRole('button', { name: /create.*api key/i });
      await act(async () => {
        fireEvent.click(createButton);
      });

      // Enter name
      const nameInput = screen.getByLabelText(/key name/i);
      await act(async () => {
        fireEvent.change(nameInput, { target: { value: 'My New Key' } });
      });

      // Submit
      const submitButton = screen.getByRole('button', { name: /^create$/i });
      await act(async () => {
        fireEvent.click(submitButton);
      });

      await waitFor(() => {
        expect(mockApiPost).toHaveBeenCalledWith('/v1/users/me/api-keys', {
          name: 'My New Key',
        });
      });
    });

    it('shows API key after creation (only once)', async () => {
      const newKeyResponse = {
        id: 'key-new',
        name: 'My New Key',
        api_key: 'solvr_sk_fullkeyneverseenagain123',
        created_at: new Date().toISOString(),
      };
      mockApiPost.mockResolvedValue(newKeyResponse);

      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /create.*api key/i })).toBeInTheDocument();
      });

      // Open form and submit
      const createButton = screen.getByRole('button', { name: /create.*api key/i });
      await act(async () => {
        fireEvent.click(createButton);
      });

      const nameInput = screen.getByLabelText(/key name/i);
      await act(async () => {
        fireEvent.change(nameInput, { target: { value: 'My New Key' } });
      });

      const submitButton = screen.getByRole('button', { name: /^create$/i });
      await act(async () => {
        fireEvent.click(submitButton);
      });

      await waitFor(() => {
        expect(screen.getByText(/solvr_sk_fullkeyneverseenagain123/)).toBeInTheDocument();
      });

      // Check for warning message
      expect(screen.getByText(/save this key now|won't be shown again/i)).toBeInTheDocument();
    });

    it('shows copy button for new API key', async () => {
      const newKeyResponse = {
        id: 'key-new',
        name: 'My New Key',
        api_key: 'solvr_sk_newkey',
        created_at: new Date().toISOString(),
      };
      mockApiPost.mockResolvedValue(newKeyResponse);

      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /create.*api key/i })).toBeInTheDocument();
      });

      // Create key
      const createButton = screen.getByRole('button', { name: /create.*api key/i });
      await act(async () => {
        fireEvent.click(createButton);
      });

      const nameInput = screen.getByLabelText(/key name/i);
      await act(async () => {
        fireEvent.change(nameInput, { target: { value: 'My New Key' } });
      });

      const submitButton = screen.getByRole('button', { name: /^create$/i });
      await act(async () => {
        fireEvent.click(submitButton);
      });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /copy/i })).toBeInTheDocument();
      });
    });

    it('allows canceling the create form', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /create.*api key/i })).toBeInTheDocument();
      });

      // Open form
      const createButton = screen.getByRole('button', { name: /create.*api key/i });
      await act(async () => {
        fireEvent.click(createButton);
      });

      expect(screen.getByLabelText(/key name/i)).toBeInTheDocument();

      // Cancel
      const cancelButton = screen.getByRole('button', { name: /cancel/i });
      await act(async () => {
        fireEvent.click(cancelButton);
      });

      expect(screen.queryByLabelText(/key name/i)).not.toBeInTheDocument();
    });

    it('shows error message on create failure', async () => {
      mockApiPost.mockRejectedValue(new ApiError(400, 'VALIDATION_ERROR', 'Invalid key name'));

      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /create.*api key/i })).toBeInTheDocument();
      });

      // Open form and submit
      const createButton = screen.getByRole('button', { name: /create.*api key/i });
      await act(async () => {
        fireEvent.click(createButton);
      });

      const nameInput = screen.getByLabelText(/key name/i);
      await act(async () => {
        fireEvent.change(nameInput, { target: { value: 'Bad Name' } });
      });

      const submitButton = screen.getByRole('button', { name: /^create$/i });
      await act(async () => {
        fireEvent.click(submitButton);
      });

      await waitFor(() => {
        expect(screen.getByText(/invalid key name/i)).toBeInTheDocument();
      });
    });
  });

  // --- Revoke Key Tests ---

  describe('Revoke Key', () => {
    it('shows revoke button for each key', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByText('Development Key')).toBeInTheDocument();
      });

      const revokeButtons = screen.getAllByRole('button', { name: /revoke/i });
      expect(revokeButtons.length).toBe(2); // One for each key
    });

    it('shows confirmation dialog when revoke is clicked', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByText('Development Key')).toBeInTheDocument();
      });

      const revokeButtons = screen.getAllByRole('button', { name: /revoke/i });
      await act(async () => {
        fireEvent.click(revokeButtons[0]);
      });

      expect(screen.getByRole('dialog')).toBeInTheDocument();
      expect(screen.getByText(/are you sure|confirm/i)).toBeInTheDocument();
    });

    it('revokes key when confirmed', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByText('Development Key')).toBeInTheDocument();
      });

      // Click revoke
      const revokeButtons = screen.getAllByRole('button', { name: /revoke/i });
      await act(async () => {
        fireEvent.click(revokeButtons[0]);
      });

      // Confirm
      const confirmButton = screen.getByRole('button', { name: /confirm|yes.*revoke/i });
      await act(async () => {
        fireEvent.click(confirmButton);
      });

      await waitFor(() => {
        expect(mockApiDelete).toHaveBeenCalledWith('/v1/users/me/api-keys/key-1');
      });
    });

    it('closes dialog when cancel is clicked', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByText('Development Key')).toBeInTheDocument();
      });

      // Click revoke
      const revokeButtons = screen.getAllByRole('button', { name: /revoke/i });
      await act(async () => {
        fireEvent.click(revokeButtons[0]);
      });

      expect(screen.getByRole('dialog')).toBeInTheDocument();

      // Cancel
      const cancelButton = screen.getByRole('button', { name: /cancel|no/i });
      await act(async () => {
        fireEvent.click(cancelButton);
      });

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    it('removes key from list after successful revocation', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByText('Development Key')).toBeInTheDocument();
      });

      // Click revoke
      const revokeButtons = screen.getAllByRole('button', { name: /revoke/i });
      await act(async () => {
        fireEvent.click(revokeButtons[0]);
      });

      // Confirm
      const confirmButton = screen.getByRole('button', { name: /confirm|yes.*revoke/i });
      await act(async () => {
        fireEvent.click(confirmButton);
      });

      await waitFor(() => {
        expect(screen.queryByText('Development Key')).not.toBeInTheDocument();
      });

      // Production Key should still be there
      expect(screen.getByText('Production Key')).toBeInTheDocument();
    });

    it('shows error message on revoke failure', async () => {
      mockApiDelete.mockRejectedValue(new ApiError(500, 'INTERNAL_ERROR', 'Failed to revoke'));

      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByText('Development Key')).toBeInTheDocument();
      });

      // Click revoke
      const revokeButtons = screen.getAllByRole('button', { name: /revoke/i });
      await act(async () => {
        fireEvent.click(revokeButtons[0]);
      });

      // Confirm
      const confirmButton = screen.getByRole('button', { name: /confirm|yes.*revoke/i });
      await act(async () => {
        fireEvent.click(confirmButton);
      });

      await waitFor(() => {
        expect(screen.getByText(/failed to revoke/i)).toBeInTheDocument();
      });
    });
  });

  // --- Accessibility Tests ---

  describe('Accessibility', () => {
    it('has proper heading for the section', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: /api keys/i })).toBeInTheDocument();
      });
    });

    it('has accessible labels on form inputs', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /create.*api key/i })).toBeInTheDocument();
      });

      // Open form
      const createButton = screen.getByRole('button', { name: /create.*api key/i });
      await act(async () => {
        fireEvent.click(createButton);
      });

      expect(screen.getByLabelText(/key name/i)).toBeInTheDocument();
    });

    it('has proper dialog role for confirmation', async () => {
      render(<ApiKeysTab userId="user-123" />);

      await waitFor(() => {
        expect(screen.getByText('Development Key')).toBeInTheDocument();
      });

      const revokeButtons = screen.getAllByRole('button', { name: /revoke/i });
      await act(async () => {
        fireEvent.click(revokeButtons[0]);
      });

      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    it('shows loading states for screen readers', async () => {
      mockApiGet.mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve(mockApiKeys), 100))
      );

      render(<ApiKeysTab userId="user-123" />);

      expect(screen.getByTestId('api-keys-loading')).toBeInTheDocument();
    });
  });
});
