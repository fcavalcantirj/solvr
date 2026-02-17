import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { HeroSection } from './hero-section';

// Mock the hooks
vi.mock('@/hooks/use-stats', () => ({
  useStats: () => ({ stats: null, loading: false }),
}));

// Mock useAuth hook
const mockUseAuth = vi.fn();
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockUseAuth(),
}));

describe('HeroSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows JOIN AS HUMAN when not authenticated', () => {
    mockUseAuth.mockReturnValue({
      isAuthenticated: false,
      isLoading: false,
      user: null,
    });

    render(<HeroSection />);

    // Should show join CTA for non-authenticated users
    expect(screen.getByText('JOIN AS HUMAN')).toBeInTheDocument();
    expect(screen.getByText('CONNECT AI AGENT')).toBeInTheDocument();
  });

  it('shows ASK A QUESTION when authenticated', () => {
    mockUseAuth.mockReturnValue({
      isAuthenticated: true,
      isLoading: false,
      user: { id: '1', type: 'human', displayName: 'Test User' },
    });

    render(<HeroSection />);

    // Should show contextual CTA for authenticated users
    expect(screen.getByText('ASK A QUESTION')).toBeInTheDocument();
    // JOIN AS HUMAN should not be shown when logged in
    expect(screen.queryByText('JOIN AS HUMAN')).not.toBeInTheDocument();
  });

  it('shows loading state during auth check', () => {
    mockUseAuth.mockReturnValue({
      isAuthenticated: false,
      isLoading: true,
      user: null,
    });

    render(<HeroSection />);

    // During loading, default CTAs should be shown
    expect(screen.getByText('JOIN AS HUMAN')).toBeInTheDocument();
  });
});
