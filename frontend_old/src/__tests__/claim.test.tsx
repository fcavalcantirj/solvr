/**
 * Tests for Claim Page (/claim/[token])
 * TDD approach: Tests written FIRST per CLAUDE.md Golden Rules
 * Per PRD AGENT-LINKING requirements:
 *   - Claim page: /claim/:token (human confirms)
 *   - Human clicks claim URL from agent
 *   - If not logged in, redirect to login first
 *   - Show agent name/description for confirmation
 *   - Human clicks 'Confirm' to link
 *   - Grant badge + 50 karma to agent
 *   - Redirect to agent's profile
 */

import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';

// Mock localStorage
const mockLocalStorage = (() => {
  let store: { [key: string]: string } = {};
  return {
    getItem: jest.fn((key: string) => store[key] || null),
    setItem: jest.fn((key: string, value: string) => {
      store[key] = value;
    }),
    removeItem: jest.fn((key: string) => {
      delete store[key];
    }),
    clear: jest.fn(() => {
      store = {};
    }),
    get length() {
      return Object.keys(store).length;
    },
    key: jest.fn((i: number) => Object.keys(store)[i] || null),
  };
})();

Object.defineProperty(window, 'localStorage', { value: mockLocalStorage });

// Mock next/navigation
const mockPush = jest.fn();
const mockReplace = jest.fn();
jest.mock('next/navigation', () => ({
  useRouter: () => ({
    push: mockPush,
    replace: mockReplace,
    prefetch: jest.fn(),
  }),
  useParams: () => ({
    token: 'test-claim-token-abc123',
  }),
}));

// Mock useAuth hook - use a mutable reference that tests can modify
const mockAuthState = {
  user: null as { id: string; username: string; email: string } | null,
  isLoading: false,
};

jest.mock('@/hooks/useAuth', () => ({
  useAuth: () => ({
    user: mockAuthState.user,
    isLoading: mockAuthState.isLoading,
    login: jest.fn(),
    logout: jest.fn(),
  }),
}));

// Mock fetch for API calls
const mockFetch = jest.fn();
global.fetch = mockFetch;

// Import component after mocks
import ClaimPage from '../app/claim/[token]/page';

// Helper to flush all promises
const flushPromises = () => new Promise(resolve => setTimeout(resolve, 0));

describe('Claim Page', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockFetch.mockReset();
    mockLocalStorage.clear();
    mockAuthState.user = null;
    mockAuthState.isLoading = false;
    jest.useRealTimers();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  describe('Loading State', () => {
    it('shows loading indicator while fetching claim info', async () => {
      // Set up authenticated user
      mockAuthState.user = { id: 'user-123', username: 'testuser', email: 'test@example.com' };

      // Set up fetch to return valid claim info but delay
      mockFetch.mockImplementation(() => new Promise(() => {})); // Never resolves

      render(<ClaimPage />);

      expect(screen.getByRole('status')).toBeInTheDocument();
    });

    it('shows loading while auth is being checked', () => {
      mockAuthState.isLoading = true;
      mockAuthState.user = null;

      render(<ClaimPage />);

      expect(screen.getByRole('status')).toBeInTheDocument();
    });
  });

  describe('Unauthenticated User Flow', () => {
    it('redirects to login when user is not authenticated', async () => {
      mockAuthState.user = null;
      mockAuthState.isLoading = false;

      render(<ClaimPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith(
          expect.stringContaining('/login')
        );
      });
    });

    it('includes return URL in login redirect', async () => {
      mockAuthState.user = null;
      mockAuthState.isLoading = false;

      render(<ClaimPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith(
          expect.stringContaining('redirect_to=')
        );
      });
    });
  });

  describe('Valid Token Flow', () => {
    beforeEach(() => {
      mockAuthState.user = { id: 'user-123', username: 'testuser', email: 'test@example.com' };
      mockLocalStorage.setItem('solvr_auth_token', 'valid-jwt-token');
    });

    it('fetches claim info on mount', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          token_valid: true,
          agent: {
            id: 'agent-123',
            display_name: 'TestAgent',
            bio: 'A helpful AI agent',
          },
          expires_at: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
        }),
      });

      render(<ClaimPage />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          expect.stringContaining('/v1/claim/test-claim-token-abc123'),
          expect.objectContaining({ method: 'GET' })
        );
      });
    });

    it('displays agent information for confirmation', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          token_valid: true,
          agent: {
            id: 'agent-123',
            display_name: 'TestAgent',
            bio: 'A helpful AI agent',
          },
          expires_at: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
        }),
      });

      render(<ClaimPage />);

      await waitFor(() => {
        expect(screen.getByText('TestAgent')).toBeInTheDocument();
      });

      expect(screen.getByText('A helpful AI agent')).toBeInTheDocument();
    });

    it('shows confirm button', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          token_valid: true,
          agent: {
            id: 'agent-123',
            display_name: 'TestAgent',
          },
        }),
      });

      render(<ClaimPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /confirm/i })).toBeInTheDocument();
      });
    });

    it('calls confirm endpoint when user clicks Confirm', async () => {
      // GET claim info
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          token_valid: true,
          agent: {
            id: 'agent-123',
            display_name: 'TestAgent',
          },
        }),
      });

      render(<ClaimPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /confirm/i })).toBeInTheDocument();
      });

      // POST confirm
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          success: true,
          agent: {
            id: 'agent-123',
            display_name: 'TestAgent',
            human_id: 'user-123',
          },
          redirect_url: '/agents/agent-123',
          message: 'Successfully linked!',
        }),
      });

      await act(async () => {
        fireEvent.click(screen.getByRole('button', { name: /confirm/i }));
        await flushPromises();
      });

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          expect.stringContaining('/v1/claim/test-claim-token-abc123'),
          expect.objectContaining({ method: 'POST' })
        );
      });
    });

    it('redirects to agent profile after successful claim', async () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            token_valid: true,
            agent: {
              id: 'agent-123',
              display_name: 'TestAgent',
            },
          }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            success: true,
            agent: {
              id: 'agent-123',
              display_name: 'TestAgent',
            },
            redirect_url: '/agents/agent-123',
            message: 'Successfully linked!',
          }),
        });

      render(<ClaimPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /confirm/i })).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: /confirm/i }));

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: /successfully linked/i })).toBeInTheDocument();
      });

      // Wait for the redirect setTimeout (2000ms) to complete
      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith('/agents/agent-123');
      }, { timeout: 3000 });
    });

    it('shows success message after claim', async () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            token_valid: true,
            agent: { id: 'agent-123', display_name: 'TestAgent' },
          }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            success: true,
            agent: { id: 'agent-123', display_name: 'TestAgent' },
            redirect_url: '/agents/agent-123',
            message: 'Successfully linked! You are now the verified human behind TestAgent',
          }),
        });

      render(<ClaimPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /confirm/i })).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: /confirm/i }));

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: /successfully linked/i })).toBeInTheDocument();
      });
    });
  });

  describe('Invalid Token Flow', () => {
    beforeEach(() => {
      mockAuthState.user = { id: 'user-123', username: 'testuser', email: 'test@example.com' };
      mockLocalStorage.setItem('solvr_auth_token', 'valid-jwt-token');
    });

    it('shows error for invalid/expired token', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          token_valid: false,
          error: 'claim token has expired',
        }),
      });

      render(<ClaimPage />);

      await waitFor(() => {
        expect(screen.getByText(/expired/i)).toBeInTheDocument();
      });
    });

    it('shows error for already used token', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          token_valid: false,
          error: 'claim token has already been used',
        }),
      });

      render(<ClaimPage />);

      await waitFor(() => {
        expect(screen.getByText(/already.*used/i)).toBeInTheDocument();
      });
    });

    it('shows error for not found token', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          token_valid: false,
          error: 'claim token not found or invalid',
        }),
      });

      render(<ClaimPage />);

      await waitFor(() => {
        expect(screen.getByText(/not found|invalid/i)).toBeInTheDocument();
      });
    });

    it('shows error when agent is already claimed', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          token_valid: false,
          error: 'agent is already linked to a human',
        }),
      });

      render(<ClaimPage />);

      await waitFor(() => {
        expect(screen.getByText(/already.*linked/i)).toBeInTheDocument();
      });
    });

    it('does not show confirm button when token is invalid', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          token_valid: false,
          error: 'claim token has expired',
        }),
      });

      render(<ClaimPage />);

      await waitFor(() => {
        expect(screen.getByText(/expired/i)).toBeInTheDocument();
      });

      // Confirm button should not be present when token is invalid
      expect(screen.queryByRole('button', { name: /confirm/i })).not.toBeInTheDocument();
    });
  });

  describe('Error Handling', () => {
    beforeEach(() => {
      mockAuthState.user = { id: 'user-123', username: 'testuser', email: 'test@example.com' };
      mockLocalStorage.setItem('solvr_auth_token', 'valid-jwt-token');
    });

    it('shows error on network failure', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      render(<ClaimPage />);

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: /claim failed/i })).toBeInTheDocument();
      });
    });

    it('shows error when confirm fails', async () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            token_valid: true,
            agent: { id: 'agent-123', display_name: 'TestAgent' },
          }),
        })
        .mockResolvedValueOnce({
          ok: false,
          status: 409,
          json: async () => ({
            error: {
              code: 'AGENT_ALREADY_CLAIMED',
              message: 'Agent is already linked to a human',
            },
          }),
        });

      render(<ClaimPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /confirm/i })).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: /confirm/i }));

      await waitFor(() => {
        expect(screen.getByText(/already.*linked/i)).toBeInTheDocument();
      });
    });

    it('shows loading state while confirming', async () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            token_valid: true,
            agent: { id: 'agent-123', display_name: 'TestAgent' },
          }),
        })
        .mockImplementationOnce(() => new Promise(() => {})); // Never resolves

      render(<ClaimPage />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /confirm/i })).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole('button', { name: /confirm/i }));

      // Button should show loading text
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /linking/i })).toBeInTheDocument();
      });
    });
  });

  describe('Accessibility', () => {
    beforeEach(() => {
      mockAuthState.user = { id: 'user-123', username: 'testuser', email: 'test@example.com' };
    });

    it('has main landmark', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          token_valid: true,
          agent: { id: 'agent-123', display_name: 'TestAgent' },
        }),
      });

      render(<ClaimPage />);

      expect(screen.getByRole('main')).toBeInTheDocument();
    });

    it('has descriptive heading when loaded', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          token_valid: true,
          agent: { id: 'agent-123', display_name: 'TestAgent' },
        }),
      });

      render(<ClaimPage />);

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: /claim agent/i })).toBeInTheDocument();
      });
    });

    it('has status announcements for screen readers', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          token_valid: true,
          agent: { id: 'agent-123', display_name: 'TestAgent' },
        }),
      });

      render(<ClaimPage />);

      // Should have aria-live or status role during loading
      expect(screen.getByRole('status')).toBeInTheDocument();
    });
  });
});
