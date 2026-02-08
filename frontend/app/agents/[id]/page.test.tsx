import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import AgentProfilePage from './page';

// Mock Next.js navigation
vi.mock('next/navigation', () => ({
  useParams: () => ({ id: 'test-agent-1' }),
  useRouter: () => ({
    push: vi.fn(),
  }),
  usePathname: () => '/agents/test-agent-1',
}));

// Mock useAuth hook for Header component
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

// Mock the useAgent hook
const mockUseAgent = vi.fn();
vi.mock('@/hooks/use-agent', () => ({
  useAgent: () => mockUseAgent(),
}));

// Default agent data for tests
const baseAgent = {
  id: 'test-agent-1',
  displayName: 'Test Agent',
  bio: 'A test agent for testing',
  status: 'active',
  reputation: 100,
  createdAt: '2026-01-15T10:00:00Z',
  hasHumanBackedBadge: false,
  time: '3 weeks ago',
  stats: {
    reputation: 100,
    problemsSolved: 5,
    problemsContributed: 10,
    ideasPosted: 3,
    responsesGiven: 15,
  },
};

describe('AgentProfilePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Model field display', () => {
    it('displays model when agent has model set', () => {
      mockUseAgent.mockReturnValue({
        agent: {
          ...baseAgent,
          model: 'claude-opus-4',
        },
        loading: false,
        error: null,
        refetch: vi.fn(),
      });

      render(<AgentProfilePage />);

      expect(screen.getByText('MODEL:')).toBeInTheDocument();
      expect(screen.getByText('claude-opus-4')).toBeInTheDocument();
    });

    it('does not display model section when model is null', () => {
      mockUseAgent.mockReturnValue({
        agent: {
          ...baseAgent,
          model: null,
        },
        loading: false,
        error: null,
        refetch: vi.fn(),
      });

      render(<AgentProfilePage />);

      expect(screen.queryByText('MODEL:')).not.toBeInTheDocument();
    });

    it('does not display model section when model is empty string', () => {
      mockUseAgent.mockReturnValue({
        agent: {
          ...baseAgent,
          model: '',
        },
        loading: false,
        error: null,
        refetch: vi.fn(),
      });

      render(<AgentProfilePage />);

      expect(screen.queryByText('MODEL:')).not.toBeInTheDocument();
    });

    it('does not display model section when model is undefined', () => {
      mockUseAgent.mockReturnValue({
        agent: baseAgent, // no model field
        loading: false,
        error: null,
        refetch: vi.fn(),
      });

      render(<AgentProfilePage />);

      expect(screen.queryByText('MODEL:')).not.toBeInTheDocument();
    });
  });

  describe('Loading state', () => {
    it('shows loading spinner when loading', () => {
      mockUseAgent.mockReturnValue({
        agent: null,
        loading: true,
        error: null,
        refetch: vi.fn(),
      });

      render(<AgentProfilePage />);

      expect(screen.getByText('Loading agent profile...')).toBeInTheDocument();
    });
  });

  describe('Error state', () => {
    it('shows error message when error occurs', () => {
      mockUseAgent.mockReturnValue({
        agent: null,
        loading: false,
        error: 'Failed to fetch agent',
        refetch: vi.fn(),
      });

      render(<AgentProfilePage />);

      expect(screen.getByText('Failed to load agent profile')).toBeInTheDocument();
      expect(screen.getByText('Failed to fetch agent')).toBeInTheDocument();
    });
  });

  describe('Not found state', () => {
    it('shows not found message when agent is null', () => {
      mockUseAgent.mockReturnValue({
        agent: null,
        loading: false,
        error: null,
        refetch: vi.fn(),
      });

      render(<AgentProfilePage />);

      expect(screen.getByText('Agent not found')).toBeInTheDocument();
    });
  });

  describe('Agent display', () => {
    it('displays agent name and bio', () => {
      mockUseAgent.mockReturnValue({
        agent: baseAgent,
        loading: false,
        error: null,
        refetch: vi.fn(),
      });

      render(<AgentProfilePage />);

      expect(screen.getByText('Test Agent')).toBeInTheDocument();
      expect(screen.getByText('A test agent for testing')).toBeInTheDocument();
    });

    it('displays Human-Backed badge when hasHumanBackedBadge is true', () => {
      mockUseAgent.mockReturnValue({
        agent: {
          ...baseAgent,
          hasHumanBackedBadge: true,
        },
        loading: false,
        error: null,
        refetch: vi.fn(),
      });

      render(<AgentProfilePage />);

      expect(screen.getByText('HUMAN-BACKED')).toBeInTheDocument();
    });
  });
});
