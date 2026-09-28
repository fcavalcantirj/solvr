import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import ClaimPage from './page';

// idx 75 step 5: the claim link carries the claim token after the #, a fragment a browser
// never sends to a server. The page reads it, takes it out of the address bar, and never puts
// it in a URL again (not in the login redirect either).

// Mock Next.js navigation
const mockRouterPush = vi.fn();
vi.mock('next/navigation', () => ({
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

const liveInfo = () => ({
  token_valid: true,
  agent: mockAgent,
  expires_at: new Date(Date.now() + 3600000).toISOString(),
});

const PENDING_KEY = 'solvr_pending_claim_token';

function openClaimLink(url: string) {
  window.history.replaceState(null, '', url);
}

describe('ClaimPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.sessionStorage.clear();
    openClaimLink('/claim#token=test-token-123');
    mockAuthState = {
      isAuthenticated: false,
      isLoading: false,
      user: null,
      loginWithGitHub: mockLoginWithGitHub,
      loginWithGoogle: mockLoginWithGoogle,
      logout: vi.fn(),
    };
  });

  afterEach(() => {
    vi.restoreAllMocks();
    openClaimLink('/');
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
    mockGetClaimInfo.mockResolvedValue(liveInfo());
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

    mockGetClaimInfo.mockResolvedValue(liveInfo());
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

    mockGetClaimInfo.mockResolvedValue(liveInfo());

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

    mockGetClaimInfo.mockResolvedValue(liveInfo());

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

  it('calls getClaimInfo with the token from the link fragment', async () => {
    mockGetClaimInfo.mockResolvedValue(liveInfo());
    render(<ClaimPage />);

    await waitFor(() => {
      expect(mockGetClaimInfo).toHaveBeenCalledWith('test-token-123');
    });
  });

  it('takes the token out of the address bar before asking the API anything', async () => {
    const replaceState = vi.spyOn(window.history, 'replaceState');
    mockGetClaimInfo.mockResolvedValue(liveInfo());
    render(<ClaimPage />);

    await waitFor(() => {
      expect(mockGetClaimInfo).toHaveBeenCalled();
    });
    expect(replaceState).toHaveBeenCalled();
    for (const call of replaceState.mock.calls) {
      expect(String(call[2])).not.toContain('test-token-123');
    }
    expect(window.location.hash).toBe('');
    expect(window.location.href).not.toContain('test-token-123');
  });

  it('exchanges nothing when the link has no token, and says how to get one', async () => {
    openClaimLink('/claim');
    render(<ClaimPage />);

    await waitFor(() => {
      expect(screen.getByText(/no claim token/i)).toBeInTheDocument();
    });
    expect(mockGetClaimInfo).not.toHaveBeenCalled();
  });

  it('does not read a token from the query string or a path', async () => {
    openClaimLink('/claim?token=from-the-query');
    render(<ClaimPage />);

    await waitFor(() => {
      expect(screen.getByText(/no claim token/i)).toBeInTheDocument();
    });
    expect(mockGetClaimInfo).not.toHaveBeenCalled();
  });

  it('sends the human to log in without putting the token in any URL', async () => {
    mockGetClaimInfo.mockResolvedValue(liveInfo());
    render(<ClaimPage />);

    await waitFor(() => {
      expect(screen.getByText(/log in to claim/i)).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText(/log in to claim/i));

    expect(mockRouterPush).toHaveBeenCalledTimes(1);
    const target = String(mockRouterPush.mock.calls[0][0]);
    expect(target).toBe('/login?next=/claim');
    expect(target).not.toContain('test-token-123');
    // The token waits in this tab's session storage, which no request carries.
    expect(window.sessionStorage.getItem(PENDING_KEY)).toBe('test-token-123');
  });

  it('picks the claim up again after login, when the address has no fragment', async () => {
    window.sessionStorage.setItem(PENDING_KEY, 'test-token-123');
    openClaimLink('/claim');
    mockAuthState = {
      ...mockAuthState,
      isAuthenticated: true,
      user: { id: 'user-1', type: 'human', displayName: 'Test User' },
    };
    mockGetClaimInfo.mockResolvedValue(liveInfo());
    mockClaimAgent.mockResolvedValue({ success: true, agent: mockAgent, message: 'Successfully claimed!' });

    render(<ClaimPage />);

    await waitFor(() => {
      expect(mockGetClaimInfo).toHaveBeenCalledWith('test-token-123');
    });
    await waitFor(() => {
      expect(screen.getByText(/claim this agent/i)).toBeInTheDocument();
    });
    fireEvent.click(screen.getByText(/claim this agent/i));
    await waitFor(() => {
      expect(screen.getByText(/successfully claimed/i)).toBeInTheDocument();
    });
    // A claimed token is spent; nothing is left waiting in storage.
    expect(window.sessionStorage.getItem(PENDING_KEY)).toBeNull();
  });

  it('forgets a waiting token the API says is no longer valid', async () => {
    window.sessionStorage.setItem(PENDING_KEY, 'test-token-123');
    openClaimLink('/claim');
    mockGetClaimInfo.mockResolvedValue({ token_valid: false, error: 'token has already been used' });

    render(<ClaimPage />);

    await waitFor(() => {
      expect(screen.getByText(/token has already been used/i)).toBeInTheDocument();
    });
    expect(window.sessionStorage.getItem(PENDING_KEY)).toBeNull();
  });
});
