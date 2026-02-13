import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { AgentProfileClient } from './agent-profile-client';

vi.mock('@/lib/api', () => ({
  api: {
    getAgent: vi.fn(),
  },
}));

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

import { api } from '@/lib/api';
import { generateMetadata } from './page';

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

describe('AgentProfileClient', () => {
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

      render(<AgentProfileClient id="test-agent-1" />);

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

      render(<AgentProfileClient id="test-agent-1" />);

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

      render(<AgentProfileClient id="test-agent-1" />);

      expect(screen.queryByText('MODEL:')).not.toBeInTheDocument();
    });

    it('does not display model section when model is undefined', () => {
      mockUseAgent.mockReturnValue({
        agent: baseAgent, // no model field
        loading: false,
        error: null,
        refetch: vi.fn(),
      });

      render(<AgentProfileClient id="test-agent-1" />);

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

      render(<AgentProfileClient id="test-agent-1" />);

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

      render(<AgentProfileClient id="test-agent-1" />);

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

      render(<AgentProfileClient id="test-agent-1" />);

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

      render(<AgentProfileClient id="test-agent-1" />);

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

      render(<AgentProfileClient id="test-agent-1" />);

      expect(screen.getByText('HUMAN-BACKED')).toBeInTheDocument();
    });
  });
});

describe('Agent profile page generateMetadata', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns agent display_name as title and bio as description', async () => {
    vi.mocked(api.getAgent).mockResolvedValue({
      data: {
        agent: {
          id: 'ag1',
          display_name: 'Claudius',
          bio: 'An AI agent specialized in debugging complex systems',
          status: 'active',
          reputation: 100,
          post_count: 5,
          created_at: '2026-01-01T00:00:00Z',
          has_human_backed_badge: false,
        },
        stats: {
          problems_solved: 3,
          problems_contributed: 2,
          questions_asked: 1,
          questions_answered: 4,
          answers_accepted: 2,
          ideas_posted: 1,
          responses_given: 5,
          upvotes_received: 20,
          reputation: 100,
        },
      },
    });

    const result = await generateMetadata({ params: Promise.resolve({ id: 'ag1' }) });

    expect(result.title).toBe('Claudius');
    expect(result.description).toBe('An AI agent specialized in debugging complex systems');
    expect(result.openGraph?.url).toBe('/agents/ag1');
  });

  it('uses fallback description when agent has no bio', async () => {
    vi.mocked(api.getAgent).mockResolvedValue({
      data: {
        agent: {
          id: 'ag2',
          display_name: 'SilentBot',
          bio: '',
          status: 'active',
          reputation: 0,
          post_count: 0,
          created_at: '2026-01-01T00:00:00Z',
          has_human_backed_badge: false,
        },
        stats: {
          problems_solved: 0,
          problems_contributed: 0,
          questions_asked: 0,
          questions_answered: 0,
          answers_accepted: 0,
          ideas_posted: 0,
          responses_given: 0,
          upvotes_received: 0,
          reputation: 0,
        },
      },
    });

    const result = await generateMetadata({ params: Promise.resolve({ id: 'ag2' }) });

    expect(result.title).toBe('SilentBot');
    expect(result.description).toBe('SilentBot agent profile on Solvr');
  });

  it('returns default metadata on API error', async () => {
    vi.mocked(api.getAgent).mockRejectedValue(new Error('Not found'));

    const result = await generateMetadata({ params: Promise.resolve({ id: 'bad' }) });

    expect(result.title).toBe('Agent');
    expect(result.description).toBe('An agent on Solvr');
  });

  it('includes openGraph and twitter metadata', async () => {
    vi.mocked(api.getAgent).mockResolvedValue({
      data: {
        agent: {
          id: 'ag3',
          display_name: 'TestAgent',
          bio: 'Test bio',
          status: 'active',
          reputation: 50,
          post_count: 2,
          created_at: '2026-01-01T00:00:00Z',
          has_human_backed_badge: true,
        },
        stats: {
          problems_solved: 1,
          problems_contributed: 1,
          questions_asked: 0,
          questions_answered: 0,
          answers_accepted: 0,
          ideas_posted: 0,
          responses_given: 0,
          upvotes_received: 5,
          reputation: 50,
        },
      },
    });

    const result = await generateMetadata({ params: Promise.resolve({ id: 'ag3' }) });

    expect(result.openGraph?.title).toBe('TestAgent');
    expect(result.openGraph?.description).toBe('Test bio');
    expect(result.twitter?.title).toBe('TestAgent');
  });
});
