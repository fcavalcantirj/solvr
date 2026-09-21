import { render, screen, fireEvent, within } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { Header } from './header';

// Mock Next.js Link
vi.mock('next/link', () => ({
  default: ({ children, href, className, ...rest }: { children: React.ReactNode; href: string; className?: string; [key: string]: unknown }) => (
    <a href={href} className={className} {...rest}>{children}</a>
  ),
}));

// Mock useAuth hook — mutable so tests can flip between logged out / logged in
const mockLogout = vi.fn();
const authState: {
  isAuthenticated: boolean;
  isLoading: boolean;
  user: { id: string; displayName: string; email?: string } | null;
} = {
  isAuthenticated: false,
  isLoading: false,
  user: null,
};

vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => ({
    isAuthenticated: authState.isAuthenticated,
    isLoading: authState.isLoading,
    user: authState.user,
    loginWithGitHub: vi.fn(),
    loginWithGoogle: vi.fn(),
    logout: mockLogout,
  }),
}));

// Mock UserMenu component
vi.mock('@/components/ui/user-menu', () => ({
  UserMenu: () => <div data-testid="user-menu">User Menu</div>,
}));

function logOut() {
  authState.isAuthenticated = false;
  authState.isLoading = false;
  authState.user = null;
}

function logIn() {
  authState.isAuthenticated = true;
  authState.isLoading = false;
  authState.user = { id: 'user-1', displayName: 'Felipe', email: 'felipe@example.com' };
}

function primaryNav() {
  return screen.getByRole('navigation', { name: /primary/i });
}

function mobileNav() {
  return screen.getByRole('navigation', { name: /mobile/i });
}

function openMobileMenu() {
  fireEvent.click(screen.getByRole('button', { name: /toggle menu/i }));
}

/** Top-level entries of a nav — excludes anything nested inside the Docs group. */
function topLevelLabels(nav: HTMLElement) {
  return Array.from(nav.querySelectorAll('[data-nav-level="primary"]')).map(
    (el) => (el.textContent || '').trim()
  );
}

describe('Header', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    logOut();
  });

  describe('logo', () => {
    it('links to home', () => {
      render(<Header />);

      const logo = screen.getByText('SOLVR_');
      expect(logo.closest('a')).toHaveAttribute('href', '/');
    });
  });

  describe('primary navigation is Rooms, Posts, Docs — in that order', () => {
    it('exposes exactly three top-level destinations on desktop', () => {
      render(<Header />);

      expect(topLevelLabels(primaryNav())).toEqual(['ROOMS', 'POSTS', 'DOCS']);
    });

    it('points Rooms at /rooms and Posts at /posts', () => {
      render(<Header />);
      const nav = primaryNav();

      expect(within(nav).getByRole('link', { name: 'ROOMS' })).toHaveAttribute('href', '/rooms');
      expect(within(nav).getByRole('link', { name: 'POSTS' })).toHaveAttribute('href', '/posts');
    });

    it.each([
      ['/feed'],
      ['/problems'],
      ['/ideas'],
      ['/questions'],
      ['/agents'],
      ['/data'],
      ['/ipfs'],
      ['/leaderboard'],
      ['/dashboard'],
    ])('does not expose %s anywhere in the header', (href) => {
      const { container } = render(<Header />);
      openMobileMenu();

      expect(container.querySelector(`a[href="${href}"]`)).toBeNull();
    });
  });

  describe('Docs groups Skill, API reference, MCP, and Guides', () => {
    it('keeps the docs links collapsed until the group is opened', () => {
      render(<Header />);
      const nav = primaryNav();

      expect(within(nav).queryByRole('link', { name: 'SKILL' })).toBeNull();
      expect(within(nav).queryByRole('link', { name: 'MCP' })).toBeNull();
    });

    it('reveals all four docs destinations when opened', () => {
      render(<Header />);
      const nav = primaryNav();

      fireEvent.click(within(nav).getByRole('button', { name: /docs/i }));

      expect(within(nav).getByRole('link', { name: 'SKILL' })).toHaveAttribute('href', '/skill');
      expect(within(nav).getByRole('link', { name: 'API REFERENCE' })).toHaveAttribute('href', '/api-docs');
      expect(within(nav).getByRole('link', { name: 'MCP' })).toHaveAttribute('href', '/mcp');
      expect(within(nav).getByRole('link', { name: 'GUIDES' })).toHaveAttribute('href', '/docs/guides');
    });

    it('marks the docs trigger as an expandable group for assistive technology', () => {
      render(<Header />);
      const trigger = within(primaryNav()).getByRole('button', { name: /docs/i });

      expect(trigger).toHaveAttribute('aria-haspopup', 'true');
      expect(trigger).toHaveAttribute('aria-expanded', 'false');

      fireEvent.click(trigger);
      expect(trigger).toHaveAttribute('aria-expanded', 'true');
    });

    it('opens on hover and closes when the pointer leaves', () => {
      render(<Header />);
      const nav = primaryNav();
      const group = within(nav).getByRole('button', { name: /docs/i }).parentElement!;

      fireEvent.mouseEnter(group);
      expect(within(nav).getByRole('link', { name: 'SKILL' })).toBeInTheDocument();

      fireEvent.mouseLeave(group);
      expect(within(nav).queryByRole('link', { name: 'SKILL' })).toBeNull();
    });

    it('collapses the group once a docs destination is chosen', () => {
      render(<Header />);
      const nav = primaryNav();
      const trigger = within(nav).getByRole('button', { name: /docs/i });

      fireEvent.click(trigger);
      fireEvent.click(within(nav).getByRole('link', { name: 'GUIDES' }));

      expect(trigger).toHaveAttribute('aria-expanded', 'false');
      expect(within(nav).queryByRole('link', { name: 'GUIDES' })).toBeNull();
    });

    it('does not expose the docs children as peers of the primary destinations', () => {
      render(<Header />);
      const nav = primaryNav();

      fireEvent.click(within(nav).getByRole('button', { name: /docs/i }));

      expect(topLevelLabels(nav)).toEqual(['ROOMS', 'POSTS', 'DOCS']);
    });
  });

  describe('connect agents is the prominent action', () => {
    it('renders a Connect agents button pointing at /connect when logged out', () => {
      render(<Header />);

      const connect = screen.getByRole('link', { name: 'CONNECT AGENTS' });
      expect(connect).toHaveAttribute('href', '/connect');
      // Prominent = filled, not a plain text link
      expect(connect.className).toContain('bg-foreground');
    });

    it('still renders Connect agents when signed in', () => {
      logIn();
      render(<Header />);

      expect(screen.getByRole('link', { name: 'CONNECT AGENTS' })).toHaveAttribute('href', '/connect');
    });

    it('keeps Log in a quiet secondary action, not a filled button', () => {
      render(<Header />);

      const login = screen.getAllByRole('link', { name: 'LOG IN' })[0];
      expect(login).toHaveAttribute('href', '/login');
      expect(login.className).not.toContain('bg-foreground');
    });

    it('stays visible while the auth state is still loading', () => {
      // Server render / first paint: auth is unresolved. The primary action
      // must not wait on it, otherwise the header is an empty bar on load.
      authState.isLoading = true;
      render(<Header />);

      expect(screen.getByRole('link', { name: 'CONNECT AGENTS' })).toHaveAttribute('href', '/connect');
    });

    it('does not compete with a second JOIN call to action', () => {
      const { container } = render(<Header />);

      expect(container.querySelector('a[href="/join"]')).toBeNull();
    });
  });

  describe('signed-in users get an account menu', () => {
    it('renders the account menu instead of Log in', () => {
      logIn();
      render(<Header />);

      expect(screen.getByTestId('user-menu')).toBeInTheDocument();
      expect(screen.queryByRole('link', { name: 'LOG IN' })).toBeNull();
    });

    it('keeps settings and API keys out of the primary navigation', () => {
      logIn();
      const { container } = render(<Header />);

      const nav = primaryNav();
      expect(within(nav).queryByRole('link', { name: /settings/i })).toBeNull();
      expect(within(nav).queryByRole('link', { name: /api keys/i })).toBeNull();
      // Desktop chrome delegates them to the account menu component
      expect(container.querySelector('a[href="/settings"]')).toBeNull();
      expect(container.querySelector('a[href="/settings/api-keys"]')).toBeNull();
    });
  });

  describe('mobile navigation mirrors the same hierarchy', () => {
    it('shows the same three top-level destinations in the same order', () => {
      render(<Header />);
      openMobileMenu();

      expect(topLevelLabels(mobileNav())).toEqual(['ROOMS', 'POSTS', 'DOCS']);
    });

    it('nests the docs destinations under Docs on mobile too', () => {
      render(<Header />);
      openMobileMenu();
      const nav = mobileNav();

      expect(within(nav).queryByRole('link', { name: 'SKILL' })).toBeNull();

      fireEvent.click(within(nav).getByRole('button', { name: /docs/i }));

      expect(within(nav).getByRole('link', { name: 'SKILL' })).toHaveAttribute('href', '/skill');
      expect(within(nav).getByRole('link', { name: 'API REFERENCE' })).toHaveAttribute('href', '/api-docs');
      expect(within(nav).getByRole('link', { name: 'MCP' })).toHaveAttribute('href', '/mcp');
      expect(within(nav).getByRole('link', { name: 'GUIDES' })).toHaveAttribute('href', '/docs/guides');
    });

    it('offers Connect agents and a quiet Log in when logged out', () => {
      render(<Header />);
      openMobileMenu();
      const nav = mobileNav();

      expect(within(nav).getByRole('link', { name: 'CONNECT AGENTS' })).toHaveAttribute('href', '/connect');
      expect(within(nav).getByRole('link', { name: 'LOG IN' })).toHaveAttribute('href', '/login');
    });

    it('offers the account actions when signed in', () => {
      logIn();
      render(<Header />);
      openMobileMenu();
      const nav = mobileNav();

      expect(within(nav).getByRole('link', { name: /settings/i })).toHaveAttribute('href', '/settings');
      expect(within(nav).getByRole('link', { name: /api keys/i })).toHaveAttribute('href', '/settings/api-keys');
      expect(within(nav).getByRole('button', { name: /log out/i })).toBeInTheDocument();
    });

    it('logs out from the mobile account section', () => {
      logIn();
      render(<Header />);
      openMobileMenu();

      fireEvent.click(within(mobileNav()).getByRole('button', { name: /log out/i }));

      expect(mockLogout).toHaveBeenCalledTimes(1);
    });
  });
});


describe('the connection action stays reachable on mobile', () => {
  // The index is long and statistics-rich. A visitor scrolling it on a phone
  // must be able to start a connection without opening the menu first, and the
  // action must sit in the fixed header rather than floating over the content
  // and the table controls below it.
  it('puts a compact connect action in the mobile bar, outside the menu', () => {
    render(<Header />);
    const action = screen.getByRole('link', { name: 'CONNECT' });
    expect(action).toHaveAttribute('href', '/connect');
    expect(action.className).toContain('md:hidden');
    expect(screen.queryByRole('navigation', { name: 'Mobile' })).not.toBeInTheDocument();
  });

  it('keeps the full Connect agents action for wider screens', () => {
    render(<Header />);
    expect(screen.getByRole('link', { name: 'CONNECT AGENTS' })).toHaveAttribute('href', '/connect');
  });
});
