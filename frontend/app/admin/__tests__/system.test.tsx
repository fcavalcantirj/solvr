import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import AdminSystemPage from '../system/page';

// Mock next/navigation
vi.mock('next/navigation', () => ({
  usePathname: () => '/admin/system',
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
  }),
}));

// Mock the Header component
vi.mock('@/components/header', () => ({
  Header: () => <div data-testid="header">Header</div>,
}));

// Mock the IPFSStatusIndicator component
vi.mock('@/components/admin/ipfs-status', () => ({
  IPFSStatusIndicator: () => <div data-testid="ipfs-status">IPFS Status</div>,
}));

describe('AdminSystemPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the system page with IPFS status', () => {
    render(<AdminSystemPage />);
    expect(screen.getByText('SYSTEM')).toBeInTheDocument();
    expect(screen.getByTestId('ipfs-status')).toBeInTheDocument();
  });

  it('renders the admin header', () => {
    render(<AdminSystemPage />);
    expect(screen.getByText('ADMIN')).toBeInTheDocument();
    expect(screen.getByText('SYSTEM')).toBeInTheDocument();
  });

  it('renders the infrastructure section', () => {
    render(<AdminSystemPage />);
    expect(screen.getByText('INFRASTRUCTURE')).toBeInTheDocument();
  });
});
