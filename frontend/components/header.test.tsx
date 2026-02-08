import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { Header } from './header';

// Mock Next.js Link
vi.mock('next/link', () => ({
  default: ({ children, href, className }: { children: React.ReactNode; href: string; className?: string }) => (
    <a href={href} className={className}>{children}</a>
  ),
}));

// Mock useAuth hook
const mockLogout = vi.fn();
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => ({
    isAuthenticated: false,
    isLoading: false,
    user: null,
    loginWithGitHub: vi.fn(),
    loginWithGoogle: vi.fn(),
    logout: mockLogout,
  }),
}));

// Mock UserMenu component
vi.mock('@/components/ui/user-menu', () => ({
  UserMenu: () => <div data-testid="user-menu">User Menu</div>,
}));

describe('Header', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('USERS link in navigation', () => {
    it('shows USERS link in desktop navigation', () => {
      render(<Header />);

      const usersLink = screen.getByRole('link', { name: 'USERS' });
      expect(usersLink).toBeInTheDocument();
      expect(usersLink).toHaveAttribute('href', '/users');
    });

    it('positions USERS link between AGENTS and API in desktop nav', () => {
      render(<Header />);

      const links = screen.getAllByRole('link');
      const agentsIndex = links.findIndex(link => link.textContent === 'AGENTS');
      const usersIndex = links.findIndex(link => link.textContent === 'USERS');
      const apiIndex = links.findIndex(link => link.textContent === 'API');

      expect(usersIndex).toBeGreaterThan(agentsIndex);
      expect(usersIndex).toBeLessThan(apiIndex);
    });

    it('shows USERS link in mobile navigation', () => {
      render(<Header />);

      // Open mobile menu
      const menuButton = screen.getByRole('button');
      fireEvent.click(menuButton);

      // Should find USERS in mobile menu (there are now 2 USERS links - desktop and mobile)
      const usersLinks = screen.getAllByRole('link', { name: 'USERS' });
      expect(usersLinks.length).toBeGreaterThanOrEqual(2);

      // Mobile link should also point to /users
      const mobileUsersLink = usersLinks[1];
      expect(mobileUsersLink).toHaveAttribute('href', '/users');
    });

    it('applies correct styling to USERS link', () => {
      render(<Header />);

      const usersLink = screen.getByRole('link', { name: 'USERS' });
      expect(usersLink).toHaveClass('font-mono', 'text-xs', 'tracking-wider', 'text-muted-foreground');
    });
  });

  describe('main navigation links', () => {
    it('renders logo linking to home', () => {
      render(<Header />);

      const logo = screen.getByText('SOLVR_');
      expect(logo.closest('a')).toHaveAttribute('href', '/');
    });

    it('renders AGENTS link', () => {
      render(<Header />);

      const agentsLink = screen.getByRole('link', { name: 'AGENTS' });
      expect(agentsLink).toHaveAttribute('href', '/agents');
    });

    it('renders API link', () => {
      render(<Header />);

      const apiLink = screen.getByRole('link', { name: 'API' });
      expect(apiLink).toHaveAttribute('href', '/api-docs');
    });
  });
});
