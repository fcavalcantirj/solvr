import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { UserMenu } from './user-menu';
import { useAuth } from '@/hooks/use-auth';

vi.mock('@/hooks/use-auth', () => ({
  useAuth: vi.fn(() => ({
    user: { id: 'user-1', displayName: 'Felipe', email: 'felipe@test.com' },
    logout: vi.fn(),
  })),
}));

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const defaultUser = { id: 'user-1', displayName: 'Felipe', email: 'felipe@test.com' };

function mockAuth(overrides: { user?: unknown; logout?: () => void } = {}) {
  const auth = { user: defaultUser, logout: vi.fn(), ...overrides };
  vi.mocked(useAuth).mockReturnValue(auth as unknown as ReturnType<typeof useAuth>);
  return auth;
}

describe('UserMenu', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth();
  });

  it('renders user display name', () => {
    render(<UserMenu />);
    expect(screen.getByText('Felipe')).toBeInTheDocument();
  });

  it('shows dropdown menu when clicked', () => {
    render(<UserMenu />);
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    expect(screen.getByText('PROFILE')).toBeInTheDocument();
    expect(screen.getByText('SETTINGS')).toBeInTheDocument();
  });

  it('includes WRITE BLOG link pointing to /blog/create', () => {
    render(<UserMenu />);
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    const blogLink = screen.getByText('WRITE BLOG');
    expect(blogLink).toBeInTheDocument();
    expect(blogLink.closest('a')).toHaveAttribute('href', '/blog/create');
  });

  it('includes all expected menu items', () => {
    render(<UserMenu />);
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    expect(screen.getByText('PROFILE')).toBeInTheDocument();
    expect(screen.getByText('MY AGENTS')).toBeInTheDocument();
    expect(screen.getByText('MY PINS')).toBeInTheDocument();
    expect(screen.getByText('WRITE BLOG')).toBeInTheDocument();
    expect(screen.getByText('SETTINGS')).toBeInTheDocument();
    expect(screen.getByText('API KEYS')).toBeInTheDocument();
    expect(screen.getByText('LOG OUT')).toBeInTheDocument();
  });
  it('closes when clicking outside the menu', () => {
    render(<UserMenu />);
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    expect(screen.getByText('PROFILE')).toBeInTheDocument();

    fireEvent.mouseDown(document.body);
    expect(screen.queryByText('PROFILE')).toBeNull();
  });

  it('closes on Escape', () => {
    render(<UserMenu />);
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    expect(screen.getByText('PROFILE')).toBeInTheDocument();

    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByText('PROFILE')).toBeNull();
  });

  it('closes when a menu item is chosen', () => {
    render(<UserMenu />);
    fireEvent.click(screen.getByRole('button', { expanded: false }));

    fireEvent.click(screen.getByText('SETTINGS'));
    expect(screen.queryByText('SETTINGS')).toBeNull();
  });

  it('logs out and closes from the menu', () => {
    const { logout } = mockAuth();

    render(<UserMenu />);
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    fireEvent.click(screen.getByText('LOG OUT'));

    expect(logout).toHaveBeenCalledTimes(1);
    expect(screen.queryByText('LOG OUT')).toBeNull();
  });

  it('renders nothing without a signed-in user', () => {
    mockAuth({ user: null });

    const { container } = render(<UserMenu />);
    expect(container).toBeEmptyDOMElement();
  });

  it('carries DASHBOARD, SETTINGS and API KEYS — the header no longer exposes them', () => {
    render(<UserMenu />);
    fireEvent.click(screen.getByRole('button', { expanded: false }));

    expect(screen.getByText('DASHBOARD').closest('a')).toHaveAttribute('href', '/dashboard');
    expect(screen.getByText('SETTINGS').closest('a')).toHaveAttribute('href', '/settings');
    expect(screen.getByText('API KEYS').closest('a')).toHaveAttribute('href', '/settings/api-keys');
  });
});

// SPEC.md 27.7: the account menu is one of the three homes of Cookie settings (many pages
// have no footer), and its links are marked for the site's click listener.
describe('UserMenu: Cookie settings and click marks', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth();
  });

  it('offers Cookie settings above Log out, opens the consent bar and closes the menu', async () => {
    const { onConsentSettingsRequest } = await import('@/lib/consent');
    const asked = vi.fn();
    const stop = onConsentSettingsRequest(asked);

    render(<UserMenu />);
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    const buttons = screen.getAllByRole('button').map((b) => (b.textContent || '').trim());
    expect(buttons.slice(-2)).toEqual(['Cookie settings', 'LOG OUT']);

    fireEvent.click(screen.getByRole('button', { name: 'Cookie settings' }));
    expect(asked).toHaveBeenCalledTimes(1);
    expect(screen.queryByText('PROFILE')).toBeNull();
    stop();
  });

  it('marks every link as account-menu navigation with a stable item', () => {
    const { container } = render(<UserMenu />);
    fireEvent.click(screen.getByRole('button', { expanded: false }));
    const marks = Array.from(container.querySelectorAll('a')).map((a) => [
      a.getAttribute('href'),
      a.getAttribute('data-track'),
      a.getAttribute('data-track-item'),
      a.getAttribute('data-track-location'),
    ]);
    expect(marks).toEqual([
      ['/users/user-1', 'nav', 'profile', 'account_menu'],
      ['/dashboard', 'nav', 'dashboard', 'account_menu'],
      ['/settings/agents', 'nav', 'my_agents', 'account_menu'],
      ['/pins', 'nav', 'my_pins', 'account_menu'],
      ['/blog/create', 'nav', 'write_blog', 'account_menu'],
      ['/notifications', 'nav', 'notifications', 'account_menu'],
      ['/settings', 'nav', 'settings', 'account_menu'],
      ['/settings/api-keys', 'nav', 'api_keys', 'account_menu'],
    ]);
  });
});
