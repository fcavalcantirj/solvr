/**
 * Tests for Login page
 * TDD approach: Tests written FIRST per CLAUDE.md Golden Rules
 * Per PRD requirements:
 *   - Create /login page
 *   - Login: GitHub button
 *   - Login: Google button
 *   - Login: redirect to OAuth
 */

import { render, screen } from '@testing-library/react';

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

// Mock next/navigation
const mockPush = jest.fn();
const mockSearchParamsGet = jest.fn();
jest.mock('next/navigation', () => ({
  useRouter: () => ({
    push: mockPush,
    replace: jest.fn(),
    prefetch: jest.fn(),
  }),
  useSearchParams: () => ({
    get: mockSearchParamsGet,
  }),
}));

// Import component after mocks
import LoginPage from '../app/login/page';

describe('Login Page', () => {
  beforeEach(() => {
    mockPush.mockClear();
    mockSearchParamsGet.mockReturnValue(null);
  });

  it('renders the login page', () => {
    render(<LoginPage />);

    // Page should render
    expect(screen.getByRole('main')).toBeInTheDocument();
  });

  it('displays the Solvr branding', () => {
    render(<LoginPage />);

    // Should show Solvr branding/logo (use getAllByText since there may be multiple)
    const solvrElements = screen.getAllByText(/solvr/i);
    expect(solvrElements.length).toBeGreaterThan(0);
  });

  it('displays a welcome message', () => {
    render(<LoginPage />);

    // Should have a welcome message
    expect(screen.getByText(/welcome/i)).toBeInTheDocument();
  });

  describe('GitHub OAuth', () => {
    it('renders GitHub sign in button', () => {
      render(<LoginPage />);

      const githubButton = screen.getByRole('button', {
        name: /github/i,
      });
      expect(githubButton).toBeInTheDocument();
    });

    it('GitHub button has correct styling', () => {
      render(<LoginPage />);

      const githubButton = screen.getByRole('button', {
        name: /github/i,
      });
      // GitHub's brand color styling class
      expect(githubButton).toHaveClass('bg-github');
    });

    it('GitHub button is clickable', () => {
      render(<LoginPage />);

      const githubButton = screen.getByRole('button', {
        name: /github/i,
      });

      // The button should exist and be clickable
      expect(githubButton).toBeInTheDocument();
      expect(githubButton).not.toBeDisabled();
    });
  });

  describe('Google OAuth', () => {
    it('renders Google sign in button', () => {
      render(<LoginPage />);

      const googleButton = screen.getByRole('button', {
        name: /google/i,
      });
      expect(googleButton).toBeInTheDocument();
    });

    it('Google button has correct styling', () => {
      render(<LoginPage />);

      const googleButton = screen.getByRole('button', {
        name: /google/i,
      });
      // Google's button styling class
      expect(googleButton).toHaveClass('bg-google');
    });

    it('Google button is clickable', () => {
      render(<LoginPage />);

      const googleButton = screen.getByRole('button', {
        name: /google/i,
      });

      // The button should exist and be clickable
      expect(googleButton).toBeInTheDocument();
      expect(googleButton).not.toBeDisabled();
    });
  });

  describe('UI Elements', () => {
    it('displays divider between OAuth buttons', () => {
      render(<LoginPage />);

      // Should have an "or" divider
      expect(screen.getByText(/or/i)).toBeInTheDocument();
    });

    it('has link to terms of service', () => {
      render(<LoginPage />);

      const termsLink = screen.getByRole('link', { name: /terms/i });
      expect(termsLink).toBeInTheDocument();
      expect(termsLink).toHaveAttribute('href', '/terms');
    });

    it('has link to privacy policy', () => {
      render(<LoginPage />);

      const privacyLink = screen.getByRole('link', { name: /privacy/i });
      expect(privacyLink).toBeInTheDocument();
      expect(privacyLink).toHaveAttribute('href', '/privacy');
    });

    it('has link back to home page', () => {
      render(<LoginPage />);

      // The Solvr logo/brand links to home
      const homeLink = screen.getByRole('link', { name: /solvr/i });
      expect(homeLink).toBeInTheDocument();
      expect(homeLink).toHaveAttribute('href', '/');
    });
  });

  describe('Accessibility', () => {
    it('has proper heading hierarchy', () => {
      render(<LoginPage />);

      // Should have an h1 for the page
      const heading = screen.getByRole('heading', { level: 1 });
      expect(heading).toBeInTheDocument();
    });

    it('buttons have accessible names', () => {
      render(<LoginPage />);

      const githubButton = screen.getByRole('button', { name: /github/i });
      const googleButton = screen.getByRole('button', { name: /google/i });

      expect(githubButton).toHaveAccessibleName();
      expect(googleButton).toHaveAccessibleName();
    });

    it('main region is properly labeled', () => {
      render(<LoginPage />);

      expect(screen.getByRole('main')).toBeInTheDocument();
    });
  });

  describe('Responsive Design', () => {
    it('centers content on the page', () => {
      render(<LoginPage />);

      const container = screen.getByTestId('login-container');
      expect(container).toHaveClass('flex');
      expect(container).toHaveClass('items-center');
      expect(container).toHaveClass('justify-center');
    });

    it('has max width on login card', () => {
      render(<LoginPage />);

      const card = screen.getByTestId('login-card');
      expect(card).toHaveClass('max-w-md');
    });
  });
});

describe('Login Page - Error Handling', () => {
  beforeEach(() => {
    mockPush.mockClear();
  });

  it('displays error message when error query param is present', () => {
    // Mock useSearchParams to return an error
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === 'error' ? 'access_denied' : null
    );

    render(<LoginPage />);

    expect(screen.getByText(/denied|error/i)).toBeInTheDocument();
  });

  it('displays generic error for unknown error types', () => {
    mockSearchParamsGet.mockImplementation((key: string) =>
      key === 'error' ? 'unknown_error' : null
    );

    render(<LoginPage />);

    expect(screen.getByText(/error/i)).toBeInTheDocument();
  });

  it('does not display error when no error param', () => {
    mockSearchParamsGet.mockReturnValue(null);

    render(<LoginPage />);

    // Should not have an alert role element
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
