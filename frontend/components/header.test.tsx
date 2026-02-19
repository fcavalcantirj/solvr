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

  describe('desktop navigation links', () => {
    it('renders logo linking to home', () => {
      render(<Header />);

      const logo = screen.getByText('SOLVR_');
      expect(logo.closest('a')).toHaveAttribute('href', '/');
    });

    it('renders all 8 nav links in desktop nav', () => {
      render(<Header />);

      const expectedLinks = [
        { name: 'FEED', href: '/feed' },
        { name: 'PROBLEMS', href: '/problems' },
        { name: 'QUESTIONS', href: '/questions' },
        { name: 'IDEAS', href: '/ideas' },
        { name: 'AGENTS', href: '/agents' },
        { name: 'IPFS', href: '/ipfs' },
        { name: 'LEADERBOARD', href: '/leaderboard' },
        { name: 'SKILL', href: '/skill' },
      ];

      for (const { name, href } of expectedLinks) {
        const link = screen.getByRole('link', { name });
        expect(link).toHaveAttribute('href', href);
      }
    });

    it('positions IPFS between AGENTS and LEADERBOARD', () => {
      render(<Header />);

      const links = screen.getAllByRole('link');
      const agentsIndex = links.findIndex(link => link.textContent === 'AGENTS');
      const ipfsIndex = links.findIndex(link => link.textContent === 'IPFS');
      const leaderboardIndex = links.findIndex(link => link.textContent === 'LEADERBOARD');

      expect(ipfsIndex).toBeGreaterThan(agentsIndex);
      expect(ipfsIndex).toBeLessThan(leaderboardIndex);
    });
  });

  describe('mobile navigation', () => {
    it('displays all nav links in mobile menu', () => {
      render(<Header />);

      const menuButton = screen.getByRole('button');
      fireEvent.click(menuButton);

      // Should find IPFS in mobile menu (desktop + mobile = 2)
      const ipfsLinks = screen.getAllByRole('link', { name: 'IPFS' });
      expect(ipfsLinks.length).toBeGreaterThanOrEqual(2);
      expect(ipfsLinks[1]).toHaveAttribute('href', '/ipfs');
    });

    it('displays LEADERBOARD in mobile menu', () => {
      render(<Header />);

      const menuButton = screen.getByRole('button');
      fireEvent.click(menuButton);

      const leaderboardLinks = screen.getAllByRole('link', { name: 'LEADERBOARD' });
      expect(leaderboardLinks.length).toBeGreaterThanOrEqual(2);
      expect(leaderboardLinks[1]).toHaveAttribute('href', '/leaderboard');
    });
  });
});
