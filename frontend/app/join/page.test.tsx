import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import JoinPage from './page';

// Mock Next.js router and searchParams
const mockPush = vi.fn();
const mockGetSearchParam = vi.fn();
vi.mock('next/navigation', () => ({
  useRouter: () => ({
    push: mockPush,
  }),
  useSearchParams: () => ({
    get: mockGetSearchParam,
  }),
}));

// Mock useAuth hook
const mockUseAuth = vi.fn();
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockUseAuth(),
}));

describe('JoinPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default: no ref param
    mockGetSearchParam.mockReturnValue(null);
  });

  describe('AI Agent Account button', () => {
    it('navigates to /login?next=/settings/agents when not authenticated', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        isLoading: false,
        user: null,
        loginWithGitHub: vi.fn(),
        loginWithGoogle: vi.fn(),
      });

      render(<JoinPage />);

      const agentButton = screen.getByText('AI Agent Account').closest('button');
      expect(agentButton).toBeInTheDocument();

      fireEvent.click(agentButton!);
      expect(mockPush).toHaveBeenCalledWith('/login?next=/settings/agents');
    });

    it('navigates to /settings/agents when authenticated', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: true,
        isLoading: false,
        user: { id: '1', type: 'human', displayName: 'Test User' },
        loginWithGitHub: vi.fn(),
        loginWithGoogle: vi.fn(),
      });

      render(<JoinPage />);

      const agentButton = screen.getByText('AI Agent Account').closest('button');
      expect(agentButton).toBeInTheDocument();

      fireEvent.click(agentButton!);
      expect(mockPush).toHaveBeenCalledWith('/settings/agents');
    });

    it('displays correct description text for claiming agents', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        isLoading: false,
        user: null,
        loginWithGitHub: vi.fn(),
        loginWithGoogle: vi.fn(),
      });

      render(<JoinPage />);

      expect(screen.getByText('Claim an AI agent you operate')).toBeInTheDocument();
    });
  });

  describe('Human Account selection', () => {
    it('renders Human Account option', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        isLoading: false,
        user: null,
        loginWithGitHub: vi.fn(),
        loginWithGoogle: vi.fn(),
      });

      render(<JoinPage />);

      expect(screen.getByText('Human Account')).toBeInTheDocument();
      expect(screen.getByText('For individuals contributing their knowledge and creativity')).toBeInTheDocument();
    });
  });

  describe('OAuth buttons', () => {
    it('renders GitHub and Google login buttons', () => {
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        isLoading: false,
        user: null,
        loginWithGitHub: vi.fn(),
        loginWithGoogle: vi.fn(),
      });

      render(<JoinPage />);

      expect(screen.getByText('CONTINUE WITH GITHUB')).toBeInTheDocument();
      expect(screen.getByText('CONTINUE WITH GOOGLE')).toBeInTheDocument();
    });

    it('calls loginWithGitHub when GitHub button is clicked', () => {
      const mockLoginWithGitHub = vi.fn();
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        isLoading: false,
        user: null,
        loginWithGitHub: mockLoginWithGitHub,
        loginWithGoogle: vi.fn(),
      });

      render(<JoinPage />);

      fireEvent.click(screen.getByText('CONTINUE WITH GITHUB'));
      expect(mockLoginWithGitHub).toHaveBeenCalled();
    });

    it('calls loginWithGoogle when Google button is clicked', () => {
      const mockLoginWithGoogle = vi.fn();
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        isLoading: false,
        user: null,
        loginWithGitHub: vi.fn(),
        loginWithGoogle: mockLoginWithGoogle,
      });

      render(<JoinPage />);

      fireEvent.click(screen.getByText('CONTINUE WITH GOOGLE'));
      expect(mockLoginWithGoogle).toHaveBeenCalled();
    });
  });

  describe('Referral code forwarding', () => {
    it('passes ref to register when URL has ?ref=ABC123', async () => {
      mockGetSearchParam.mockImplementation((key: string) => key === 'ref' ? 'ABC123' : null);

      const mockRegister = vi.fn().mockResolvedValue({ success: true });
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        isLoading: false,
        user: null,
        loginWithGitHub: vi.fn(),
        loginWithGoogle: vi.fn(),
        register: mockRegister,
      });

      render(<JoinPage />);

      // Navigate to step 2
      fireEvent.click(screen.getByText('CONTINUE WITH EMAIL'));

      // Fill in required fields
      fireEvent.change(screen.getByPlaceholderText('Jane'), { target: { value: 'Jane' } });
      fireEvent.change(screen.getByPlaceholderText('Doe'), { target: { value: 'Doe' } });
      fireEvent.change(screen.getByPlaceholderText('janedoe'), { target: { value: 'janedoe' } });
      fireEvent.change(screen.getByPlaceholderText('you@example.com'), { target: { value: 'jane@example.com' } });
      fireEvent.change(screen.getByPlaceholderText('Min. 8 characters'), { target: { value: 'password123' } });

      // Accept terms
      fireEvent.click(screen.getByRole('checkbox'));

      // Submit form
      fireEvent.click(screen.getByText('CREATE ACCOUNT'));

      await waitFor(() => {
        expect(mockRegister).toHaveBeenCalledWith(
          'jane@example.com',
          'password123',
          'janedoe',
          'Jane Doe',
          'ABC123'
        );
      });
    });

    it('does not pass ref when URL has no ?ref param', async () => {
      // Default: mockGetSearchParam returns null
      mockGetSearchParam.mockReturnValue(null);

      const mockRegister = vi.fn().mockResolvedValue({ success: true });
      mockUseAuth.mockReturnValue({
        isAuthenticated: false,
        isLoading: false,
        user: null,
        loginWithGitHub: vi.fn(),
        loginWithGoogle: vi.fn(),
        register: mockRegister,
      });

      render(<JoinPage />);

      // Navigate to step 2
      fireEvent.click(screen.getByText('CONTINUE WITH EMAIL'));

      // Fill in required fields
      fireEvent.change(screen.getByPlaceholderText('Jane'), { target: { value: 'Jane' } });
      fireEvent.change(screen.getByPlaceholderText('Doe'), { target: { value: 'Doe' } });
      fireEvent.change(screen.getByPlaceholderText('janedoe'), { target: { value: 'janedoe' } });
      fireEvent.change(screen.getByPlaceholderText('you@example.com'), { target: { value: 'jane@example.com' } });
      fireEvent.change(screen.getByPlaceholderText('Min. 8 characters'), { target: { value: 'password123' } });

      // Accept terms
      fireEvent.click(screen.getByRole('checkbox'));

      // Submit form
      fireEvent.click(screen.getByText('CREATE ACCOUNT'));

      await waitFor(() => {
        expect(mockRegister).toHaveBeenCalledWith(
          'jane@example.com',
          'password123',
          'janedoe',
          'Jane Doe',
          undefined
        );
      });
    });
  });
});
