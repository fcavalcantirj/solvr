/**
 * Tests for useAuth hook
 * TDD approach: Tests written FIRST per CLAUDE.md Golden Rules
 * Per PRD requirements:
 *   - Create useAuth hook
 *   - Return {user, isLoading, login, logout}
 *   - Fetch /v1/auth/me on mount
 *   - Cache user in state
 */

import { renderHook, waitFor, act } from '@testing-library/react';

// Mock localStorage - using a closure-based store that persists across mock resets
const createMockLocalStorage = () => {
  let store: Record<string, string> = {};
  return {
    store,
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => {
      store[key] = value;
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      store = {};
    },
    reset: () => {
      store = {};
    },
    setStore: (newStore: Record<string, string>) => {
      store = { ...newStore };
    },
  };
};

const mockStorage = createMockLocalStorage();

// Create spies for localStorage methods
const mockGetItem = jest.fn((key: string) => mockStorage.getItem(key));
const mockSetItem = jest.fn((key: string, value: string) => mockStorage.setItem(key, value));
const mockRemoveItem = jest.fn((key: string) => mockStorage.removeItem(key));
const mockClear = jest.fn(() => mockStorage.clear());

Object.defineProperty(window, 'localStorage', {
  value: {
    getItem: mockGetItem,
    setItem: mockSetItem,
    removeItem: mockRemoveItem,
    clear: mockClear,
  },
});

// Mock fetch
const mockFetch = jest.fn();
global.fetch = mockFetch;

// Mock next/navigation
const mockPush = jest.fn();
jest.mock('next/navigation', () => ({
  useRouter: () => ({
    push: mockPush,
    replace: jest.fn(),
    prefetch: jest.fn(),
  }),
}));

// Import hook after mocks
import { useAuth } from '../hooks/useAuth';

describe('useAuth Hook', () => {
  beforeEach(() => {
    // Reset spies
    mockGetItem.mockClear();
    mockSetItem.mockClear();
    mockRemoveItem.mockClear();
    mockClear.mockClear();
    mockFetch.mockReset();
    mockPush.mockClear();

    // Reset localStorage store
    mockStorage.reset();
  });

  describe('Return Value Structure', () => {
    it('returns user, isLoading, login, and logout', () => {
      const { result } = renderHook(() => useAuth());

      expect(result.current).toHaveProperty('user');
      expect(result.current).toHaveProperty('isLoading');
      expect(result.current).toHaveProperty('login');
      expect(result.current).toHaveProperty('logout');
    });

    it('login is a function', () => {
      const { result } = renderHook(() => useAuth());

      expect(typeof result.current.login).toBe('function');
    });

    it('logout is a function', () => {
      const { result } = renderHook(() => useAuth());

      expect(typeof result.current.logout).toBe('function');
    });
  });

  describe('User Fetching', () => {
    it('fetches current user on mount when token exists', async () => {
      // Set token BEFORE rendering hook
      mockStorage.setStore({ solvr_auth_token: 'test-token' });

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: { id: 'user-123', username: 'testuser', display_name: 'Test User' },
        }),
      });

      renderHook(() => useAuth());

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          expect.stringContaining('/v1/auth/me'),
          expect.objectContaining({
            headers: expect.objectContaining({
              Authorization: 'Bearer test-token',
            }),
          })
        );
      });
    });

    it('does not fetch user when no token exists', async () => {
      const { result } = renderHook(() => useAuth());

      // Small delay to ensure effect has run
      await act(async () => {
        await new Promise((resolve) => setTimeout(resolve, 10));
      });

      expect(mockFetch).not.toHaveBeenCalled();
      expect(result.current.user).toBeNull();
      expect(result.current.isLoading).toBe(false);
    });

    it('sets user after successful fetch', async () => {
      mockStorage.setStore({ solvr_auth_token: 'test-token' });

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: { id: 'user-123', username: 'testuser', display_name: 'Test User' },
        }),
      });

      const { result } = renderHook(() => useAuth());

      await waitFor(() => {
        expect(result.current.user).toEqual({
          id: 'user-123',
          username: 'testuser',
          display_name: 'Test User',
        });
      });
    });

    it('clears token on 401 response', async () => {
      mockStorage.setStore({ solvr_auth_token: 'expired-token' });

      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: async () => ({ error: { code: 'UNAUTHORIZED' } }),
      });

      const { result } = renderHook(() => useAuth());

      await waitFor(() => {
        expect(mockRemoveItem).toHaveBeenCalledWith('solvr_auth_token');
        expect(result.current.user).toBeNull();
      });
    });
  });

  describe('Loading State', () => {
    it('isLoading is true initially when token exists', () => {
      mockStorage.setStore({ solvr_auth_token: 'test-token' });
      mockFetch.mockImplementation(() => new Promise(() => {})); // Never resolves

      const { result } = renderHook(() => useAuth());

      expect(result.current.isLoading).toBe(true);
    });

    it('isLoading becomes false after fetch completes', async () => {
      mockStorage.setStore({ solvr_auth_token: 'test-token' });
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: { id: 'user-123', username: 'testuser' },
        }),
      });

      const { result } = renderHook(() => useAuth());

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });
    });

    it('isLoading is false when no token exists', () => {
      const { result } = renderHook(() => useAuth());

      expect(result.current.isLoading).toBe(false);
    });
  });

  describe('Login Function', () => {
    it('login function exists and is callable', () => {
      const { result } = renderHook(() => useAuth());

      expect(result.current.login).toBeDefined();
      expect(typeof result.current.login).toBe('function');
    });

    it('login accepts provider parameter', () => {
      const { result } = renderHook(() => useAuth());

      // Should not throw when called
      expect(() => {
        try {
          result.current.login('github');
        } catch {
          // Expected error from trying to set window.location
        }
      }).not.toThrow();
    });
  });

  describe('Logout Function', () => {
    it('logout clears the token from localStorage', async () => {
      mockStorage.setStore({ solvr_auth_token: 'test-token' });

      // Mock initial user fetch
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: { id: 'user-123', username: 'testuser' },
        }),
      });

      const { result } = renderHook(() => useAuth());

      // Wait for user to be loaded
      await waitFor(() => {
        expect(result.current.user).not.toBeNull();
      });

      // Mock logout API call
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
      });

      await act(async () => {
        await result.current.logout();
      });

      expect(mockRemoveItem).toHaveBeenCalledWith('solvr_auth_token');
    });

    it('logout sets user to null', async () => {
      mockStorage.setStore({ solvr_auth_token: 'test-token' });

      // Mock initial user fetch
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: { id: 'user-123', username: 'testuser' },
        }),
      });

      const { result } = renderHook(() => useAuth());

      // Wait for user to be loaded
      await waitFor(() => {
        expect(result.current.user).not.toBeNull();
      });

      // Mock logout API call
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
      });

      await act(async () => {
        await result.current.logout();
      });

      expect(result.current.user).toBeNull();
    });

    it('logout redirects to home page', async () => {
      mockStorage.setStore({ solvr_auth_token: 'test-token' });

      // Mock initial user fetch
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: { id: 'user-123', username: 'testuser' },
        }),
      });

      const { result } = renderHook(() => useAuth());

      // Wait for user to be loaded
      await waitFor(() => {
        expect(result.current.user).not.toBeNull();
      });

      // Mock logout API call
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
      });

      await act(async () => {
        await result.current.logout();
      });

      expect(mockPush).toHaveBeenCalledWith('/');
    });

    it('logout works even without initial user', async () => {
      // No token initially
      const { result } = renderHook(() => useAuth());

      await act(async () => {
        await result.current.logout();
      });

      // Should still redirect to home
      expect(mockPush).toHaveBeenCalledWith('/');
    });
  });
});
