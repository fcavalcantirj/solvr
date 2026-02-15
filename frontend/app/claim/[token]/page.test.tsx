import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import ClaimPage from './page';

// Mock Next.js navigation
const mockRouterPush = vi.fn();
vi.mock('next/navigation', () => ({
  useParams: () => ({ token: 'test-token-123' }),
  useRouter: () => ({
    push: mockRouterPush,
  }),
}));

// Mock useAuth hook
const mockLoginWithGitHub = vi.fn();
const mockLoginWithGoogle = vi.fn();
let mockAuthState = {
  isAuthenticated: false,
  isLoading: false,
  user: null as { id: string; type: string; displayName: string } | null,
  loginWithGitHub: mockLoginWithGitHub,
  loginWithGoogle: mockLoginWithGoogle,
  logout: vi.fn(),
};

vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockAuthState,
}));

// Mock API
const mockGetClaimInfo = vi.fn();
const mockClaimAgent = vi.fn();

vi.mock('@/lib/api', () => ({
  api: {
    getClaimInfo: (token: string) => mockGetClaimInfo(token),
    claimAgent: (token: string) => mockClaimAgent(token),
  },
}));

// Mock next/link
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const mockAgent = {
  id: 'agent-1',
  display_name: 'Claude Helper',
  bio: 'A helpful AI agent',
  reputation: 42,
  status: 'active',
  created_at: '2026-01-15T10:00:00Z',
};

describe('ClaimPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuthState = {
      isAuthenticated: false,
      isLoading: false,
      user: null,
      loginWithGitHub: mockLoginWithGitHub,
      loginWithGoogle: mockLoginWithGoogle,
      logout: vi.fn(),
    };
  });

  it('shows loading state while fetching claim info', () => {
    mockGetClaimInfo.mockReturnValue(new Promise(() => {})); // Never resolves
    render(<ClaimPage />);
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('shows error state for invalid token', async () => {
    mockGetClaimInfo.mockResolvedValue({
      token_valid: false,
      error: 'invalid or unknown token',
    });
    render(<ClaimPage />);

    await waitFor(() => {
      expect(screen.getByText(/invalid or unknown token/i)).toBeInTheDocument();
    });
  });

  it('shows error state for expired token', async () => {
    mockGetClaimInfo.mockResolvedValue({
      token_valid: false,
      error: 'token has expired',
    });
    render(<ClaimPage />);

    await waitFor(() => {
      expect(screen.getByText(/token has expired/i)).toBeInTheDocument();
    });
  });

  it('shows agent info and login button when not authenticated', async () => {
    mockGetClaimInfo.mockResolvedValue({
      token_valid: true,
      agent: mockAgent,
      expires_at: new Date(Date.now() + 3600000).toISOString(),
    });
    render(<ClaimPage />);

    await waitFor(() => {
      expect(screen.getByText('Claude Helper')).toBeInTheDocument();
    });

    expect(screen.getByText('A helpful AI agent')).toBeInTheDocument();
    // Should show login prompt, not claim button
    expect(screen.getByText(/log in to claim/i)).toBeInTheDocument();
  });

  it('shows agent info and claim button when authenticated', async () => {
    mockAuthState = {
      ...mockAuthState,
      isAuthenticated: true,
      user: { id: 'user-1', type: 'human', displayName: 'Test User' },
    };

    mockGetClaimInfo.mockResolvedValue({
      token_valid: true,
      agent: mockAgent,
      expires_at: new Date(Date.now() + 3600000).toISOString(),
    });
    render(<ClaimPage />);

    await waitFor(() => {
      expect(screen.getByText('Claude Helper')).toBeInTheDocument();
    });

    expect(screen.getByText(/claim this agent/i)).toBeInTheDocument();
  });

  it('shows success state after successful claim', async () => {
    mockAuthState = {
      ...mockAuthState,
      isAuthenticated: true,
      user: { id: 'user-1', type: 'human', displayName: 'Test User' },
    };

    mockGetClaimInfo.mockResolvedValue({
      token_valid: true,
      agent: mockAgent,
      expires_at: new Date(Date.now() + 3600000).toISOString(),
    });

    mockClaimAgent.mockResolvedValue({
      success: true,
      agent: { ...mockAgent, has_human_backed_badge: true },
      message: 'Successfully claimed!',
    });

    render(<ClaimPage />);

    await waitFor(() => {
      expect(screen.getByText('Claude Helper')).toBeInTheDocument();
    });

    const claimButton = screen.getByText(/claim this agent/i);
    fireEvent.click(claimButton);

    await waitFor(() => {
      expect(screen.getByText(/successfully claimed/i)).toBeInTheDocument();
    });
  });

  it('shows error when claim fails', async () => {
    mockAuthState = {
      ...mockAuthState,
      isAuthenticated: true,
      user: { id: 'user-1', type: 'human', displayName: 'Test User' },
    };

    mockGetClaimInfo.mockResolvedValue({
      token_valid: true,
      agent: mockAgent,
      expires_at: new Date(Date.now() + 3600000).toISOString(),
    });

    mockClaimAgent.mockRejectedValue(new Error('agent is already claimed'));

    render(<ClaimPage />);

    await waitFor(() => {
      expect(screen.getByText('Claude Helper')).toBeInTheDocument();
    });

    const claimButton = screen.getByText(/claim this agent/i);
    fireEvent.click(claimButton);

    await waitFor(() => {
      expect(screen.getByText(/agent is already claimed/i)).toBeInTheDocument();
    });
  });

  it('calls getClaimInfo with correct token', async () => {
    mockGetClaimInfo.mockResolvedValue({
      token_valid: true,
      agent: mockAgent,
      expires_at: new Date(Date.now() + 3600000).toISOString(),
    });
    render(<ClaimPage />);

    await waitFor(() => {
      expect(mockGetClaimInfo).toHaveBeenCalledWith('test-token-123');
    });
  });
});
