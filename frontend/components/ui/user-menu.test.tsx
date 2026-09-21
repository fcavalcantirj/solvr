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
