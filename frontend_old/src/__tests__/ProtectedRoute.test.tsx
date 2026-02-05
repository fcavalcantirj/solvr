/**
 * Tests for ProtectedRoute component
 * TDD approach: Tests written FIRST per CLAUDE.md Golden Rules
 * Per PRD requirements:
 *   - Create ProtectedRoute component
 *   - Redirect to /login if not authenticated
 *   - Show loading state while checking auth
 *   - Render children when authenticated
 */

import { render, screen, waitFor } from '@testing-library/react';
import '@testing-library/jest-dom';

// Mock useAuth hook
const mockUseAuth = jest.fn();
jest.mock('../hooks/useAuth', () => ({
  useAuth: () => mockUseAuth(),
}));

// Mock next/navigation
const mockPush = jest.fn();
jest.mock('next/navigation', () => ({
  useRouter: () => ({
    push: mockPush,
    replace: jest.fn(),
    prefetch: jest.fn(),
  }),
}));

// Import component after mocks
import { ProtectedRoute } from '../components/ProtectedRoute';

describe('ProtectedRoute Component', () => {
  beforeEach(() => {
    mockUseAuth.mockReset();
    mockPush.mockClear();
  });

  describe('Loading State', () => {
    it('shows loading indicator while auth is loading', () => {
      mockUseAuth.mockReturnValue({
        user: null,
        isLoading: true,
        login: jest.fn(),
        logout: jest.fn(),
      });

      render(
        <ProtectedRoute>
          <div>Protected Content</div>
        </ProtectedRoute>
      );

      expect(screen.getByRole('status')).toBeInTheDocument();
      expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
    });

    it('shows loading text', () => {
      mockUseAuth.mockReturnValue({
        user: null,
        isLoading: true,
        login: jest.fn(),
        logout: jest.fn(),
      });

      render(
        <ProtectedRoute>
          <div>Protected Content</div>
        </ProtectedRoute>
      );

      expect(screen.getByText(/loading/i)).toBeInTheDocument();
    });
  });

  describe('Authenticated State', () => {
    it('renders children when user is authenticated', () => {
      mockUseAuth.mockReturnValue({
        user: { id: 'user-123', username: 'testuser', display_name: 'Test User' },
        isLoading: false,
        login: jest.fn(),
        logout: jest.fn(),
      });

      render(
        <ProtectedRoute>
          <div>Protected Content</div>
        </ProtectedRoute>
      );

      expect(screen.getByText('Protected Content')).toBeInTheDocument();
    });

    it('does not redirect when user is authenticated', () => {
      mockUseAuth.mockReturnValue({
        user: { id: 'user-123', username: 'testuser', display_name: 'Test User' },
        isLoading: false,
        login: jest.fn(),
        logout: jest.fn(),
      });

      render(
        <ProtectedRoute>
          <div>Protected Content</div>
        </ProtectedRoute>
      );

      expect(mockPush).not.toHaveBeenCalled();
    });

    it('does not show loading indicator when authenticated', () => {
      mockUseAuth.mockReturnValue({
        user: { id: 'user-123', username: 'testuser', display_name: 'Test User' },
        isLoading: false,
        login: jest.fn(),
        logout: jest.fn(),
      });

      render(
        <ProtectedRoute>
          <div>Protected Content</div>
        </ProtectedRoute>
      );

      expect(screen.queryByRole('status')).not.toBeInTheDocument();
    });
  });

  describe('Unauthenticated State', () => {
    it('redirects to /login when not authenticated', async () => {
      mockUseAuth.mockReturnValue({
        user: null,
        isLoading: false,
        login: jest.fn(),
        logout: jest.fn(),
      });

      render(
        <ProtectedRoute>
          <div>Protected Content</div>
        </ProtectedRoute>
      );

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith('/login');
      });
    });

    it('does not render children when not authenticated', () => {
      mockUseAuth.mockReturnValue({
        user: null,
        isLoading: false,
        login: jest.fn(),
        logout: jest.fn(),
      });

      render(
        <ProtectedRoute>
          <div>Protected Content</div>
        </ProtectedRoute>
      );

      expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
    });
  });

  describe('Custom Redirect', () => {
    it('redirects to custom path when specified', async () => {
      mockUseAuth.mockReturnValue({
        user: null,
        isLoading: false,
        login: jest.fn(),
        logout: jest.fn(),
      });

      render(
        <ProtectedRoute redirectTo="/custom-login">
          <div>Protected Content</div>
        </ProtectedRoute>
      );

      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith('/custom-login');
      });
    });
  });

  describe('Fallback UI', () => {
    it('renders custom fallback while loading', () => {
      mockUseAuth.mockReturnValue({
        user: null,
        isLoading: true,
        login: jest.fn(),
        logout: jest.fn(),
      });

      render(
        <ProtectedRoute fallback={<div>Custom Loading...</div>}>
          <div>Protected Content</div>
        </ProtectedRoute>
      );

      expect(screen.getByText('Custom Loading...')).toBeInTheDocument();
      expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
    });
  });

  describe('Multiple Children', () => {
    it('renders multiple children when authenticated', () => {
      mockUseAuth.mockReturnValue({
        user: { id: 'user-123', username: 'testuser', display_name: 'Test User' },
        isLoading: false,
        login: jest.fn(),
        logout: jest.fn(),
      });

      render(
        <ProtectedRoute>
          <div>First Child</div>
          <div>Second Child</div>
        </ProtectedRoute>
      );

      expect(screen.getByText('First Child')).toBeInTheDocument();
      expect(screen.getByText('Second Child')).toBeInTheDocument();
    });
  });
});
