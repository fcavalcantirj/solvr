import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import ReferralsPage from './page';

// Mock Next.js router
const mockPush = vi.fn();
const mockRouter = { push: mockPush };
vi.mock('next/navigation', () => ({
  useRouter: () => mockRouter,
}));

// Mock Header component
vi.mock('@/components/header', () => ({
  Header: () => <div data-testid="header" />,
}));

// Mock useAuth hook
const mockUseAuth = vi.fn();
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockUseAuth(),
}));

// Mock API
const mockGetMyReferral = vi.fn();
vi.mock('@/lib/api', () => ({
  api: {
    getMyReferral: (...args: unknown[]) => mockGetMyReferral(...args),
  },
}));

// Mock clipboard
const mockWriteText = vi.fn().mockResolvedValue(undefined);
Object.defineProperty(navigator, 'clipboard', {
  value: { writeText: mockWriteText },
  writable: true,
  configurable: true,
});

const mockReferralData = {
  referral_code: 'ABC12345',
  referral_count: 3,
};

describe('ReferralsPage', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    mockWriteText.mockResolvedValue(undefined);
  });

  it('redirects unauthenticated users to /login?next=/referrals', async () => {
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: false,
    });

    render(<ReferralsPage />);

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith('/login?next=/referrals');
    });
  });

  it('shows skeleton during loading', () => {
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: true,
    });

    render(<ReferralsPage />);

    // Skeleton: animate-pulse divs should be present
    const skeletonElements = document.querySelectorAll('.animate-pulse');
    expect(skeletonElements.length).toBeGreaterThan(0);
  });

  it('shows skeleton while API fetch is in progress', () => {
    mockUseAuth.mockReturnValue({
      user: { id: '1', type: 'human', displayName: 'Test User' },
      isAuthenticated: true,
      isLoading: false,
    });
    // Return a promise that never resolves to simulate loading
    mockGetMyReferral.mockReturnValue(new Promise(() => {}));

    render(<ReferralsPage />);

    const skeletonElements = document.querySelectorAll('.animate-pulse');
    expect(skeletonElements.length).toBeGreaterThan(0);
  });

  it('displays referral code after API returns data', async () => {
    mockUseAuth.mockReturnValue({
      user: { id: '1', type: 'human', displayName: 'Test User' },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetMyReferral.mockResolvedValue(mockReferralData);

    render(<ReferralsPage />);

    await waitFor(() => {
      expect(screen.getByTestId('referral-code')).toHaveTextContent('ABC12345');
    }, { timeout: 3000 });
  });

  it('copy code button calls navigator.clipboard.writeText with the code', async () => {
    mockUseAuth.mockReturnValue({
      user: { id: '1', type: 'human', displayName: 'Test User' },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetMyReferral.mockResolvedValue(mockReferralData);

    render(<ReferralsPage />);

    await waitFor(() => {
      expect(screen.getByTestId('referral-code')).toBeInTheDocument();
    }, { timeout: 3000 });

    const copyCodeBtn = screen.getByLabelText('Copy referral code');
    fireEvent.click(copyCodeBtn);

    expect(mockWriteText).toHaveBeenCalledWith('ABC12345');

    await waitFor(() => {
      expect(copyCodeBtn).toHaveTextContent('Copied!');
    }, { timeout: 3000 });
  });

  it('shows referral count from API', async () => {
    mockUseAuth.mockReturnValue({
      user: { id: '1', type: 'human', displayName: 'Test User' },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetMyReferral.mockResolvedValue(mockReferralData);

    render(<ReferralsPage />);

    await waitFor(() => {
      expect(screen.getByTestId('referral-count')).toHaveTextContent('3');
    }, { timeout: 3000 });

    expect(screen.getByText(/successful referral/i)).toBeInTheDocument();
  });

  it('tweet link href contains encodeURIComponent of tweet text and referral URL', async () => {
    mockUseAuth.mockReturnValue({
      user: { id: '1', type: 'human', displayName: 'Test User' },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetMyReferral.mockResolvedValue(mockReferralData);

    render(<ReferralsPage />);

    await waitFor(() => {
      expect(screen.getByTestId('tweet-link')).toBeInTheDocument();
    }, { timeout: 3000 });

    const tweetLink = screen.getByTestId('tweet-link');
    const href = tweetLink.getAttribute('href') || '';

    const expectedText = encodeURIComponent(
      "I'm using @SolvrDev to solve programming problems faster. Join me:"
    );
    const expectedUrl = encodeURIComponent('https://solvr.dev/join?ref=ABC12345');

    expect(href).toContain(expectedText);
    expect(href).toContain(expectedUrl);
    expect(href).toContain('twitter.com/intent/tweet');
  });

  it('copy referral link button calls clipboard.writeText with the referral URL', async () => {
    mockUseAuth.mockReturnValue({
      user: { id: '1', type: 'human', displayName: 'Test User' },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetMyReferral.mockResolvedValue(mockReferralData);

    render(<ReferralsPage />);

    await waitFor(() => {
      expect(screen.getByTestId('referral-code')).toBeInTheDocument();
    }, { timeout: 3000 });

    const copyLinkBtn = screen.getByLabelText('Copy referral link');
    fireEvent.click(copyLinkBtn);

    expect(mockWriteText).toHaveBeenCalledWith(
      'https://solvr.dev/join?ref=ABC12345'
    );

    await waitFor(() => {
      expect(copyLinkBtn).toHaveTextContent('Copied!');
    }, { timeout: 3000 });
  });

  it('shows error state when API fetch fails', async () => {
    mockUseAuth.mockReturnValue({
      user: { id: '1', type: 'human', displayName: 'Test User' },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetMyReferral.mockRejectedValue(new Error('Network error'));

    render(<ReferralsPage />);

    await waitFor(() => {
      expect(screen.getByText('Failed to load referral data')).toBeInTheDocument();
    }, { timeout: 3000 });
  });
});
