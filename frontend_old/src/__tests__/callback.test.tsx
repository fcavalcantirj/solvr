/**
 * Tests for Auth Callback page
 * TDD approach: Tests written FIRST per CLAUDE.md Golden Rules
 * Per PRD requirements:
 *   - Create callback page
 *   - Callback: handle tokens
 */

import { render, screen, waitFor } from '@testing-library/react';

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
const mockSearchParamsGet = jest.fn();
jest.mock('next/navigation', () => ({
  useRouter: () => ({
    push: mockPush,
    replace: mockReplace,
    prefetch: jest.fn(),
  }),
  useSearchParams: () => ({
    get: mockSearchParamsGet,
  }),
}));

// Import component after mocks
import CallbackPage from '../app/auth/callback/page';

describe('Auth Callback Page', () => {
  beforeEach(() => {
    mockPush.mockClear();
    mockReplace.mockClear();
    mockSearchParamsGet.mockReset();
    mockLocalStorage.clear();
    mockLocalStorage.setItem.mockClear();
  });

  it('renders a status message', () => {
    mockSearchParamsGet.mockReturnValue(null);

    render(<CallbackPage />);

    // Should show some status message (completing, error, or redirecting)
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  describe('Success Flow', () => {
    it('stores token in localStorage when present', async () => {
      mockSearchParamsGet.mockImplementation((key: string) => {
        if (key === 'token') return 'test-jwt-token';
        return null;
      });

      render(<CallbackPage />);

      await waitFor(() => {
        expect(mockLocalStorage.setItem).toHaveBeenCalledWith(
          'solvr_auth_token',
          'test-jwt-token'
        );
      });
    });

    it('redirects to dashboard after storing token', async () => {
      mockSearchParamsGet.mockImplementation((key: string) => {
        if (key === 'token') return 'test-jwt-token';
        return null;
      });

      render(<CallbackPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith('/dashboard');
      });
    });

    it('uses redirect_to param if present', async () => {
      mockSearchParamsGet.mockImplementation((key: string) => {
        if (key === 'token') return 'test-jwt-token';
        if (key === 'redirect_to') return '/problems/123';
        return null;
      });

      render(<CallbackPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith('/problems/123');
      });
    });
  });

  describe('Error Flow', () => {
    it('redirects to login with error when error param present', async () => {
      mockSearchParamsGet.mockImplementation((key: string) => {
        if (key === 'error') return 'access_denied';
        return null;
      });

      render(<CallbackPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith(
          expect.stringContaining('/login?error=')
        );
      });
    });

    it('redirects to login when no token present', async () => {
      mockSearchParamsGet.mockReturnValue(null);

      render(<CallbackPage />);

      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalledWith(
          expect.stringContaining('/login')
        );
      });
    });

    it('shows error message briefly before redirect', async () => {
      mockSearchParamsGet.mockImplementation((key: string) => {
        if (key === 'error') return 'server_error';
        return null;
      });

      render(<CallbackPage />);

      // May show error message or just redirect
      await waitFor(() => {
        expect(mockReplace).toHaveBeenCalled();
      });
    });
  });

  describe('Security', () => {
    it('sanitizes redirect_to to prevent open redirect', async () => {
      mockSearchParamsGet.mockImplementation((key: string) => {
        if (key === 'token') return 'test-jwt-token';
        if (key === 'redirect_to') return 'https://evil.com/phishing';
        return null;
      });

      render(<CallbackPage />);

      await waitFor(() => {
        // Should not redirect to external URL
        expect(mockReplace).toHaveBeenCalledWith('/dashboard');
      });
    });

    it('only allows internal paths for redirect_to', async () => {
      mockSearchParamsGet.mockImplementation((key: string) => {
        if (key === 'token') return 'test-jwt-token';
        if (key === 'redirect_to') return '//evil.com/phishing';
        return null;
      });

      render(<CallbackPage />);

      await waitFor(() => {
        // Should fallback to dashboard, not use malicious redirect
        expect(mockReplace).toHaveBeenCalledWith('/dashboard');
      });
    });
  });

  describe('Accessibility', () => {
    it('has main landmark', () => {
      mockSearchParamsGet.mockReturnValue(null);

      render(<CallbackPage />);

      expect(screen.getByRole('main')).toBeInTheDocument();
    });

    it('has status message for screen readers', () => {
      mockSearchParamsGet.mockReturnValue(null);

      render(<CallbackPage />);

      // Should have aria-live or status role for loading indication
      const statusElement = screen.getByRole('status');
      expect(statusElement).toBeInTheDocument();
    });
  });
});
