import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import ApiDocsPage from './page';

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

describe('ApiDocsPage', () => {
  beforeEach(() => {
    // Mock fetch for api-mcp health check
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }));
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders without hardcoded ALL SYSTEMS OPERATIONAL text', async () => {
    render(<ApiDocsPage />);
    expect(screen.queryByText('ALL SYSTEMS OPERATIONAL')).not.toBeInTheDocument();
    // Wait for health check to settle
    await waitFor(() => {
      expect(screen.getByText('ONLINE')).toBeInTheDocument();
    });
  });

  it('uses shared Footer with SOLVR_ brand and copyright', async () => {
    render(<ApiDocsPage />);
    // Shared Footer contains SOLVR_ brand (Header also has it, so multiple)
    const solvrBrands = screen.getAllByText('SOLVR_');
    expect(solvrBrands.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/© 2026 SOLVR/)).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText('ONLINE')).toBeInTheDocument();
    });
  });

  it('shared Footer has /status link with status indicator', async () => {
    render(<ApiDocsPage />);
    const statusLink = screen.getByRole('link', { name: /Status/ });
    expect(statusLink).toHaveAttribute('href', '/status');
    await waitFor(() => {
      expect(screen.getByText('ONLINE')).toBeInTheDocument();
    });
  });

  it('does not contain hardcoded status.solvr.dev references', async () => {
    const { container } = render(<ApiDocsPage />);
    expect(container.innerHTML).not.toContain('status.solvr.dev');
    await waitFor(() => {
      expect(screen.getByText('ONLINE')).toBeInTheDocument();
    });
  });
});
