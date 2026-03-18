import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import ZhPromotePage from './page';

// Mock Next.js navigation
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

// Mock Next.js Link
vi.mock('next/link', () => ({
  default: ({ href, children, ...props }: { href: string; children: React.ReactNode; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
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

describe('ZhPromotePage', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('renders Chinese content — hero section with Chinese text', () => {
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: false,
    });

    render(<ZhPromotePage />);

    // Key Chinese section headings
    expect(screen.getByText('分享 Solvr')).toBeInTheDocument();
    expect(screen.getByText('为什么选择 Solvr？')).toBeInTheDocument();
    expect(screen.getByText('推荐分享平台')).toBeInTheDocument();
    expect(screen.getByText('您的专属邀请链接')).toBeInTheDocument();
  });

  it('renders Chinese platform names — Juejin, CSDN, V2EX, Zhihu, Gitee', () => {
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: false,
    });

    render(<ZhPromotePage />);

    expect(screen.getByText('Juejin')).toBeInTheDocument();
    expect(screen.getByText('CSDN')).toBeInTheDocument();
    expect(screen.getByText('V2EX')).toBeInTheDocument();
    expect(screen.getByText('Zhihu')).toBeInTheDocument();
    expect(screen.getByText('Gitee')).toBeInTheDocument();
  });

  it('authenticated user sees personalized referral link with code', async () => {
    mockUseAuth.mockReturnValue({
      user: { id: '1', type: 'human', displayName: 'Test User' },
      isAuthenticated: true,
      isLoading: false,
    });
    mockGetMyReferral.mockResolvedValue({
      referral_code: 'XYZ99999',
      referral_count: 5,
    });

    render(<ZhPromotePage />);

    await waitFor(() => {
      const link = screen.getByTestId('personalized-link');
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute('href', 'https://solvr.dev/join?ref=XYZ99999');
    }, { timeout: 3000 });
  });

  it('unauthenticated user sees generic /join link', () => {
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: false,
    });

    render(<ZhPromotePage />);

    const genericLink = screen.getByTestId('generic-link');
    expect(genericLink).toBeInTheDocument();
    expect(genericLink).toHaveAttribute('href', 'https://solvr.dev/join');
  });

  it('renders feedback section with email reply request', () => {
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: false,
    });

    render(<ZhPromotePage />);

    const feedbackSection = screen.getByTestId('feedback-section');
    expect(feedbackSection).toBeInTheDocument();
    expect(screen.getByText('您的反馈对我们非常重要')).toBeInTheDocument();
    // Check for the email reply instruction (multiple matches OK)
    const emailReplyTexts = screen.getAllByText(/直接回复.*邮件/);
    expect(emailReplyTexts.length).toBeGreaterThan(0);
  });

  it('feedback section asks about what users love and improvements', () => {
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: false,
    });

    render(<ZhPromotePage />);

    expect(screen.getByText(/最喜欢 Solvr/)).toBeInTheDocument();
    expect(screen.getByText(/改进|不满意/)).toBeInTheDocument();
    expect(screen.getByText(/新功能/)).toBeInTheDocument();
  });

  it('page is accessible without authentication (no redirect)', () => {
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: false,
    });

    render(<ZhPromotePage />);

    // Should render the page content (not redirect)
    expect(screen.getByText('分享 Solvr')).toBeInTheDocument();
    expect(screen.getByTestId('header')).toBeInTheDocument();
  });

  it('shows loading state gracefully — no crash during isLoading', () => {
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      isLoading: true,
    });

    // Should not throw
    expect(() => render(<ZhPromotePage />)).not.toThrow();
    expect(screen.getByText('分享 Solvr')).toBeInTheDocument();
  });
});
