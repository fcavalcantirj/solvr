import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import BlogPage from './page';

// Mock next/link
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

// Mock useAuth for Header
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => ({
    isAuthenticated: false,
    isLoading: false,
    user: null,
    loginWithGitHub: vi.fn(),
    loginWithGoogle: vi.fn(),
    logout: vi.fn(),
  }),
}));

// Mock UserMenu
vi.mock('@/components/ui/user-menu', () => ({
  UserMenu: () => <div data-testid="user-menu">User Menu</div>,
}));

describe('BlogPage', () => {
  it('renders the blog page with header', () => {
    render(<BlogPage />);
    expect(screen.getByText('SOLVR BLOG')).toBeInTheDocument();
    expect(screen.getByText(/Thoughts on/i)).toBeInTheDocument();
  });

  it('does not render newsletter subscription form', () => {
    render(<BlogPage />);
    expect(screen.queryByPlaceholderText(/your@email.com/i)).not.toBeInTheDocument();
    const subscribeButtons = screen.queryAllByRole('button', { name: /subscribe/i });
    expect(subscribeButtons.length).toBe(0);
  });

  it('does not show fake subscriber count', () => {
    render(<BlogPage />);
    expect(screen.queryByText(/4,200\+ subscribers/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/sent every tuesday/i)).not.toBeInTheDocument();
  });

  it('does not show "Stay Updated" newsletter section', () => {
    render(<BlogPage />);
    expect(screen.queryByText(/STAY UPDATED/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Subscribe to our newsletter/i)).not.toBeInTheDocument();
  });

  it('renders blog posts', () => {
    render(<BlogPage />);
    expect(screen.getByText(/The Efficiency Flywheel/i)).toBeInTheDocument();
  });

  it('renders category filters', () => {
    render(<BlogPage />);
    expect(screen.getByRole('button', { name: /ALL POSTS/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /ENGINEERING/i })).toBeInTheDocument();
  });

  it('should not render "LOAD MORE POSTS" button', () => {
    render(<BlogPage />);
    const loadMoreButton = screen.queryByText(/LOAD MORE POSTS/i);
    expect(loadMoreButton).not.toBeInTheDocument();
  });

  it('should render all blog posts from static array', () => {
    render(<BlogPage />);
    expect(screen.getByText(/Introducing MCP/i)).toBeInTheDocument();
    expect(screen.getByText(/The Efficiency Flywheel/i)).toBeInTheDocument();
  });
});
