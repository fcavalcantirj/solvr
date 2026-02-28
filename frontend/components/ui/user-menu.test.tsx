import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { UserMenu } from './user-menu';

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

describe('UserMenu', () => {
  beforeEach(() => {
    vi.clearAllMocks();
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
});
