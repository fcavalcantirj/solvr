import { render, screen, fireEvent, within } from '@testing-library/react';
import { renderToStaticMarkup } from 'react-dom/server';
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

/** The Docs trigger and its dropdown — Skill is also a top-level link, so docs queries scope here. */
function docsGroup(nav: HTMLElement) {
  return within(nav).getByRole('button', { name: /docs/i }).parentElement!;
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

  describe('top-level navigation is Rooms, Posts, Data, Skill, Docs — in that order', () => {
    it('exposes Rooms, Posts, Data, Skill and Docs as the top-level destinations on desktop, in that order', () => {
      render(<Header />);

      expect(topLevelLabels(primaryNav())).toEqual(['ROOMS', 'POSTS', 'DATA', 'SKILL', 'DOCS']);
    });

    it('points Rooms at /rooms and Posts at /posts', () => {
      render(<Header />);
      const nav = primaryNav();

      expect(within(nav).getByRole('link', { name: 'ROOMS' })).toHaveAttribute('href', '/rooms');
      expect(within(nav).getByRole('link', { name: 'POSTS' })).toHaveAttribute('href', '/posts');
    });

    it('points Data at /data and Skill at /skill on desktop, styled like Rooms', () => {
      render(<Header />);
      const nav = primaryNav();
      const rooms = within(nav).getByRole('link', { name: 'ROOMS' });
      const data = within(nav).getByRole('link', { name: 'DATA' });
      const skill = within(nav).getByRole('link', { name: 'SKILL' });

      expect(data).toHaveAttribute('href', '/data');
      expect(skill).toHaveAttribute('href', '/skill');
      expect(data.className).toBe(rooms.className);
      expect(skill.className).toBe(rooms.className);
    });

    it.each([
      ['/feed'],
      ['/problems'],
      ['/ideas'],
      ['/questions'],
      ['/agents'],
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

      expect(within(docsGroup(nav)).queryByRole('link', { name: 'SKILL' })).toBeNull();
      expect(within(nav).queryByRole('link', { name: 'MCP' })).toBeNull();
    });

    it('reveals all four docs destinations when opened', () => {
      render(<Header />);
      const nav = primaryNav();

      fireEvent.click(within(nav).getByRole('button', { name: /docs/i }));
      const group = within(docsGroup(nav));

      expect(group.getByRole('link', { name: 'OVERVIEW' })).toHaveAttribute('href', '/docs');
      expect(group.getByRole('link', { name: 'SKILL' })).toHaveAttribute('href', '/skill');
      expect(group.getByRole('link', { name: 'API REFERENCE' })).toHaveAttribute('href', '/api-docs');
      expect(group.getByRole('link', { name: 'MCP' })).toHaveAttribute('href', '/mcp');
      expect(group.getByRole('link', { name: 'GUIDES' })).toHaveAttribute('href', '/docs/guides');
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
      expect(within(group).getByRole('link', { name: 'SKILL' })).toBeInTheDocument();

      fireEvent.mouseLeave(group);
      expect(within(group).queryByRole('link', { name: 'SKILL' })).toBeNull();
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

    it('keeps the docs children out of the top level when the group is open', () => {
      render(<Header />);
      const nav = primaryNav();

      fireEvent.click(within(nav).getByRole('button', { name: /docs/i }));

      expect(topLevelLabels(nav)).toEqual(['ROOMS', 'POSTS', 'DATA', 'SKILL', 'DOCS']);
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
    it('shows Rooms, Posts, Data, Skill and Docs as the top-level destinations on mobile, in the same order', () => {
      render(<Header />);
      openMobileMenu();

      expect(topLevelLabels(mobileNav())).toEqual(['ROOMS', 'POSTS', 'DATA', 'SKILL', 'DOCS']);
    });

    it('points Data at /data and Skill at /skill on mobile and closes the menu when chosen', () => {
      render(<Header />);
      openMobileMenu();
      const nav = mobileNav();

      expect(within(nav).getByRole('link', { name: 'DATA' })).toHaveAttribute('href', '/data');
      expect(within(nav).getByRole('link', { name: 'SKILL' })).toHaveAttribute('href', '/skill');

      fireEvent.click(within(nav).getByRole('link', { name: 'DATA' }));

      expect(screen.queryByRole('navigation', { name: /mobile/i })).toBeNull();
    });

    it('nests the docs destinations under Docs on mobile too', () => {
      render(<Header />);
      openMobileMenu();
      const nav = mobileNav();

      expect(within(docsGroup(nav)).queryByRole('link', { name: 'SKILL' })).toBeNull();

      fireEvent.click(within(nav).getByRole('button', { name: /docs/i }));
      const group = within(docsGroup(nav));

      expect(group.getByRole('link', { name: 'OVERVIEW' })).toHaveAttribute('href', '/docs');
      expect(group.getByRole('link', { name: 'SKILL' })).toHaveAttribute('href', '/skill');
      expect(group.getByRole('link', { name: 'API REFERENCE' })).toHaveAttribute('href', '/api-docs');
      expect(group.getByRole('link', { name: 'MCP' })).toHaveAttribute('href', '/mcp');
      expect(group.getByRole('link', { name: 'GUIDES' })).toHaveAttribute('href', '/docs/guides');
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

// SPEC.md 27.7: one click listener reads these marks. item says WHAT was pressed (a stable
// id, never the label), location says WHERE it sits.
describe('every header link is marked for the click listener', () => {
  const marks = (root: Element) =>
    Array.from(root.querySelectorAll('a')).map((a) => [
      a.getAttribute('href'),
      a.getAttribute('data-track'),
      a.getAttribute('data-track-item'),
      a.getAttribute('data-track-location'),
    ]);

  beforeEach(() => {
    vi.clearAllMocks();
    logOut();
  });

  // The docs menu's links are in the document while it is closed (hidden, not absent), so
  // they are among the bar's links, each marked as the docs menu's.
  it('marks the bar: logo, the four links, the closed docs menu, log in, connect and the compact connect', () => {
    const { container } = render(<Header />);
    expect(marks(container.querySelector('header')!)).toEqual([
      ['/', 'nav', 'logo', 'header'],
      ['/rooms', 'nav', 'rooms', 'header'],
      ['/posts', 'nav', 'posts', 'header'],
      ['/data', 'nav', 'data', 'header'],
      ['/skill', 'nav', 'skill', 'header'],
      ['/docs', 'nav', 'docs_overview', 'docs_menu'],
      ['/skill', 'nav', 'skill', 'docs_menu'],
      ['/api-docs', 'nav', 'api_docs', 'docs_menu'],
      ['/mcp', 'nav', 'mcp', 'docs_menu'],
      ['/docs/guides', 'nav', 'guides', 'docs_menu'],
      ['/docs/protocol', 'nav', 'protocol', 'docs_menu'],
      ['/login', 'nav', 'log_in', 'header'],
      ['/connect', 'nav', 'connect_agents', 'header'],
      ['/connect', 'nav', 'connect_agents', 'header'],
    ]);
  });

  it('marks the docs menu as its own place', () => {
    render(<Header />);
    const nav = primaryNav();
    fireEvent.click(within(nav).getByRole('button', { name: /docs/i }));
    expect(marks(docsGroup(nav))).toEqual([
      ['/docs', 'nav', 'docs_overview', 'docs_menu'],
      ['/skill', 'nav', 'skill', 'docs_menu'],
      ['/api-docs', 'nav', 'api_docs', 'docs_menu'],
      ['/mcp', 'nav', 'mcp', 'docs_menu'],
      ['/docs/guides', 'nav', 'guides', 'docs_menu'],
      ['/docs/protocol', 'nav', 'protocol', 'docs_menu'],
    ]);
  });

  it('marks the mobile menu for a signed-out visitor, docs links included', () => {
    render(<Header />);
    openMobileMenu();
    const nav = mobileNav();
    fireEvent.click(within(nav).getByRole('button', { name: /docs/i }));
    expect(marks(nav)).toEqual([
      ['/rooms', 'nav', 'rooms', 'mobile_menu'],
      ['/posts', 'nav', 'posts', 'mobile_menu'],
      ['/data', 'nav', 'data', 'mobile_menu'],
      ['/skill', 'nav', 'skill', 'mobile_menu'],
      ['/docs', 'nav', 'docs_overview', 'mobile_menu'],
      ['/skill', 'nav', 'skill', 'mobile_menu'],
      ['/api-docs', 'nav', 'api_docs', 'mobile_menu'],
      ['/mcp', 'nav', 'mcp', 'mobile_menu'],
      ['/docs/guides', 'nav', 'guides', 'mobile_menu'],
      ['/docs/protocol', 'nav', 'protocol', 'mobile_menu'],
      ['/connect', 'nav', 'connect_agents', 'mobile_menu'],
      ['/login', 'nav', 'log_in', 'mobile_menu'],
    ]);
  });

  it('marks the account links of the mobile menu for a signed-in visitor', () => {
    logIn();
    render(<Header />);
    openMobileMenu();
    expect(marks(mobileNav()).slice(-4)).toEqual([
      ['/users/user-1', 'nav', 'profile', 'mobile_menu'],
      ['/settings/agents', 'nav', 'my_agents', 'mobile_menu'],
      ['/settings', 'nav', 'settings', 'mobile_menu'],
      ['/settings/api-keys', 'nav', 'api_keys', 'mobile_menu'],
    ]);
  });

  it('leaves no link unmarked, whatever is open', () => {
    logIn();
    const { container } = render(<Header />);
    fireEvent.click(within(primaryNav()).getByRole('button', { name: /docs/i }));
    openMobileMenu();
    fireEvent.click(within(mobileNav()).getByRole('button', { name: /docs/i }));

    const links = Array.from(container.querySelectorAll('header a'));
    expect(links.length).toBeGreaterThan(20);
    for (const link of links) {
      expect(link.getAttribute('data-track'), link.outerHTML).toBe('nav');
      expect(link.getAttribute('data-track-item'), link.outerHTML).toMatch(/^[a-z][a-z0-9_]*$/);
      expect(['header', 'docs_menu', 'mobile_menu'], link.outerHTML).toContain(link.getAttribute('data-track-location'));
    }
  });
});

// The footer is missing on many pages, so the mobile menu carries Cookie settings too.
describe('Cookie settings in the mobile menu', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    logOut();
  });

  it('offers it to a signed-out visitor, opens the consent bar and closes the menu', async () => {
    const { onConsentSettingsRequest } = await import('@/lib/consent');
    const asked = vi.fn();
    const stop = onConsentSettingsRequest(asked);

    render(<Header />);
    openMobileMenu();
    fireEvent.click(within(mobileNav()).getByRole('button', { name: 'Cookie settings' }));

    expect(asked).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('navigation', { name: /mobile/i })).toBeNull();
    stop();
  });

  it('offers it to a signed-in visitor as well, after Log out', () => {
    logIn();
    render(<Header />);
    openMobileMenu();
    const buttons = within(mobileNav()).getAllByRole('button').map((b) => (b.textContent || '').trim());
    expect(buttons.slice(-2)).toEqual(['LOG OUT', 'Cookie settings']);
  });

  it('keeps it out of the bar itself', () => {
    render(<Header />);
    expect(screen.queryByRole('button', { name: 'Cookie settings' })).toBeNull();
  });
});

// The docs menu was rendered only once it was opened, so no server HTML carried its links:
// a crawler never saw them, and /docs/protocol had no inbound link from any page (recon
// 2026-10-04). The menu is now always in the document and hidden until it opens.
describe('the docs menu is in the server HTML', () => {
  const DOCS = ['/docs', '/skill', '/api-docs', '/mcp', '/docs/guides', '/docs/protocol'];

  beforeEach(() => {
    vi.clearAllMocks();
    logOut();
  });

  it('server-renders every docs destination as a plain link, the protocol among them', () => {
    const html = renderToStaticMarkup(<Header />);
    const hrefs = [...html.matchAll(/<a\b[^>]*\shref="([^"]+)"/g)].map((m) => m[1]);
    for (const href of DOCS) expect(hrefs, href).toContain(href);
    expect(hrefs).toContain('/docs/protocol');
  });

  it('keeps the closed menu out of sight and out of the accessibility tree', () => {
    const { container } = render(<Header />);
    const menu = container.querySelector('[data-docs-menu]') as HTMLElement;
    expect(menu).not.toBeNull();
    expect(menu.hidden).toBe(true);
    expect(Array.from(menu.querySelectorAll('a')).map((a) => a.getAttribute('href'))).toEqual(DOCS);
    // Hidden content is not reachable by role: a screen reader and the Tab key skip it.
    expect(within(primaryNav()).queryByRole('link', { name: 'PROTOCOL' })).toBeNull();
  });

  it('shows the same links when the menu opens, and hides them again when it closes', () => {
    const { container } = render(<Header />);
    const nav = primaryNav();
    const trigger = within(nav).getByRole('button', { name: /docs/i });
    const menu = container.querySelector('[data-docs-menu]') as HTMLElement;

    fireEvent.click(trigger);
    expect(menu.hidden).toBe(false);
    expect(within(nav).getByRole('link', { name: 'PROTOCOL' })).toHaveAttribute('href', '/docs/protocol');

    fireEvent.click(trigger);
    expect(menu.hidden).toBe(true);
    expect(container.querySelector('[data-docs-menu]')).toBe(menu);
  });

  it('writes each docs link once: the mobile menu adds its own only when it is open', () => {
    const html = renderToStaticMarkup(<Header />);
    expect(html.match(/href="\/docs\/protocol"/g)).toHaveLength(1);
  });
});
