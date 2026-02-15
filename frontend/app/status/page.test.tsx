import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import StatusPage from './page';

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

describe('StatusPage', () => {
  it('renders the status page with system status header', () => {
    render(<StatusPage />);
    expect(screen.getByText('SYSTEM STATUS')).toBeInTheDocument();
    expect(screen.getByText('Solvr Status')).toBeInTheDocument();
  });

  it('shows All Systems Operational when all services are operational', () => {
    render(<StatusPage />);
    expect(screen.getByText('All Systems Operational')).toBeInTheDocument();
  });

  it('renders service categories', () => {
    render(<StatusPage />);
    expect(screen.getByText('Core API')).toBeInTheDocument();
    expect(screen.getByText('Database')).toBeInTheDocument();
    // MCP Server appears in both nav and service list
    expect(screen.getAllByText('MCP Server').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('Infrastructure')).toBeInTheDocument();
  });

  it('renders the shared Footer component (not a custom footer)', () => {
    render(<StatusPage />);
    // Shared Footer contains SOLVR_ brand (Header also has it, so multiple)
    const solvrBrands = screen.getAllByText('SOLVR_');
    expect(solvrBrands.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/© 2026 SOLVR/)).toBeInTheDocument();
  });

  it('renders recent incidents section', () => {
    render(<StatusPage />);
    expect(screen.getByText('RECENT INCIDENTS')).toBeInTheDocument();
  });

  it('renders programmatic access section with health endpoint', () => {
    render(<StatusPage />);
    expect(screen.getByText('PROGRAMMATIC ACCESS')).toBeInTheDocument();
    expect(screen.getByText('Status API')).toBeInTheDocument();
  });
});
