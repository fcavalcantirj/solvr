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

const mockStatusData = {
  overall_status: 'operational' as const,
  services: [
    {
      category: 'Core Services',
      items: [
        {
          name: 'REST API',
          description: 'Primary API endpoints for all operations',
          status: 'operational' as const,
          uptime: '99.98%',
          latency_ms: 45,
          last_checked: '2026-02-27T12:00:00Z',
        },
        {
          name: 'PostgreSQL',
          description: 'PostgreSQL data store',
          status: 'operational' as const,
          uptime: '99.98%',
          latency_ms: 8,
          last_checked: '2026-02-27T12:00:00Z',
        },
      ],
    },
    {
      category: 'Storage',
      items: [
        {
          name: 'IPFS Node',
          description: 'Decentralized content storage (Kubo)',
          status: 'operational' as const,
          uptime: '99.98%',
          latency_ms: 65,
          last_checked: '2026-02-27T12:00:00Z',
        },
      ],
    },
  ],
  summary: {
    uptime_30d: 99.97,
    avg_response_time_ms: 39,
    service_count: 3,
    last_checked: '2026-02-27T12:00:00Z',
  },
  uptime_history: [
    { date: '2026-02-27', status: 'operational' as const },
    { date: '2026-02-26', status: 'operational' as const },
  ],
  incidents: [],
};

// Default mock: loaded with data
vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({
    data: mockStatusData,
    loading: false,
    error: null,
    refetch: vi.fn(),
  }),
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
    expect(screen.getByText('Core Services')).toBeInTheDocument();
    expect(screen.getByText('Storage')).toBeInTheDocument();
  });

  it('renders service names', () => {
    render(<StatusPage />);
    expect(screen.getByText('REST API')).toBeInTheDocument();
    expect(screen.getByText('PostgreSQL')).toBeInTheDocument();
    expect(screen.getByText('IPFS Node')).toBeInTheDocument();
  });

  it('renders the shared Footer component', () => {
    render(<StatusPage />);
    const solvrBrands = screen.getAllByText('SOLVR_');
    expect(solvrBrands.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/© 2026 SOLVR/)).toBeInTheDocument();
  });

  it('renders recent incidents section with empty state', () => {
    render(<StatusPage />);
    expect(screen.getByText('RECENT INCIDENTS')).toBeInTheDocument();
    expect(screen.getByText('No recent incidents')).toBeInTheDocument();
  });

  it('renders programmatic access section with status endpoint', () => {
    render(<StatusPage />);
    expect(screen.getByText('PROGRAMMATIC ACCESS')).toBeInTheDocument();
    expect(screen.getByText('Status API')).toBeInTheDocument();
    expect(screen.getByText('GET https://api.solvr.dev/v1/status')).toBeInTheDocument();
  });

  it('renders overall stats from API data', () => {
    render(<StatusPage />);
    expect(screen.getByText('99.97%')).toBeInTheDocument();
    expect(screen.getByText('39ms')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('Overall Uptime (30d)')).toBeInTheDocument();
    expect(screen.getByText('Avg Response Time')).toBeInTheDocument();
    expect(screen.getByText('Active Services')).toBeInTheDocument();
    expect(screen.getByText('Last Checked')).toBeInTheDocument();
  });

  it('renders uptime history chart', () => {
    render(<StatusPage />);
    expect(screen.getByText('30-DAY UPTIME HISTORY')).toBeInTheDocument();
  });

  it('does not render subscribe section', () => {
    render(<StatusPage />);
    expect(screen.queryByPlaceholderText(/your@email.com/i)).not.toBeInTheDocument();
  });

  it('should not have any href="#" links', () => {
    const { container } = render(<StatusPage />);
    const deadLinks = container.querySelectorAll('a[href="#"]');
    expect(deadLinks.length).toBe(0);
  });

  it('shows operational count per category', () => {
    render(<StatusPage />);
    expect(screen.getByText('2/2 operational')).toBeInTheDocument();
    expect(screen.getByText('1/1 operational')).toBeInTheDocument();
  });
});
