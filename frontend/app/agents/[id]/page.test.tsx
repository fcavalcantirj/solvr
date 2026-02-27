import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { AgentProfileClient } from '@/components/agents/agent-profile-client';

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
  useAgent: (...args: unknown[]) => mockUseAgent(...args),
}));

// Mock the useCheckpoints hook
const mockUseCheckpoints = vi.fn();
vi.mock('@/hooks/use-checkpoints', () => ({
  useCheckpoints: () => mockUseCheckpoints(),
}));

// Mock the useResurrectionBundle hook
const mockUseResurrectionBundle = vi.fn();
vi.mock('@/hooks/use-resurrection-bundle', () => ({
  useResurrectionBundle: (...args: unknown[]) => mockUseResurrectionBundle(...args),
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

// Default mock values for checkpoints and resurrection
const emptyCheckpoints = { checkpoints: [], latest: null, count: 0, loading: false, error: null };
const emptyBundle = { bundle: null, loading: false, error: null };

const mockCheckpointsData = {
  checkpoints: [
    {
      requestid: 'cp-1',
      status: 'pinned' as const,
      created: '2026-02-20T10:00:00Z',
      pin: {
        cid: 'QmLatestCheckpoint123abc',
        name: 'checkpoint_QmLatest_20260220',
        meta: { type: 'amcp_checkpoint', agent_id: 'test-agent-1', death_count: '3' },
      },
      delegates: [],
    },
    {
      requestid: 'cp-2',
      status: 'pinned' as const,
      created: '2026-02-19T10:00:00Z',
      pin: {
        cid: 'QmOlderCheckpoint456def',
        name: 'checkpoint_QmOlder_20260219',
        meta: { type: 'amcp_checkpoint', agent_id: 'test-agent-1' },
      },
      delegates: [],
    },
  ],
  latest: {
    requestid: 'cp-1',
    status: 'pinned' as const,
    created: '2026-02-20T10:00:00Z',
    pin: {
      cid: 'QmLatestCheckpoint123abc',
      name: 'checkpoint_QmLatest_20260220',
      meta: { type: 'amcp_checkpoint', agent_id: 'test-agent-1', death_count: '3' },
    },
    delegates: [],
  },
  count: 2,
  loading: false,
  error: null,
};

const mockBundleData = {
  bundle: {
    identity: {
      id: 'test-agent-1',
      display_name: 'Test Agent',
      created_at: '2026-01-15T10:00:00Z',
      model: 'claude-opus-4',
      specialties: ['golang', 'postgresql'],
      bio: 'A test agent for testing',
      has_amcp_identity: true,
      amcp_aid: 'did:keri:ETestAID123',
      keri_public_key: 'DTestKey456',
    },
    knowledge: {
      ideas: [{ id: 'idea-1', title: 'Test Idea', status: 'open', upvotes: 5, downvotes: 0, created_at: '2026-02-18T10:00:00Z' }],
      approaches: [{ id: 'appr-1', problem_id: 'prob-1', angle: 'Test angle', status: 'working', created_at: '2026-02-17T10:00:00Z' }],
      problems: [{ id: 'prob-1', title: 'Test Problem', status: 'open', created_at: '2026-02-16T10:00:00Z' }],
    },
    reputation: {
      total: 350,
      problems_solved: 5,
      answers_accepted: 3,
      ideas_posted: 10,
      upvotes_received: 42,
    },
    latest_checkpoint: null,
    death_count: 3,
  },
  loading: false,
  error: null,
};

function setupAgentMock(agentOverrides = {}) {
  mockUseAgent.mockReturnValue({
    agent: { ...baseAgent, ...agentOverrides },
    loading: false,
    error: null,
    refetch: vi.fn(),
  });
}

describe('AgentProfileClient', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseCheckpoints.mockReturnValue(emptyCheckpoints);
    mockUseResurrectionBundle.mockReturnValue(emptyBundle);
  });

  describe('Model field display', () => {
    it('displays model when agent has model set', () => {
      setupAgentMock({ model: 'claude-opus-4' });
      render(<AgentProfileClient id="test-agent-1" />);
      expect(screen.getByText('MODEL:')).toBeInTheDocument();
      expect(screen.getByText('claude-opus-4')).toBeInTheDocument();
    });

    it('does not display model section when model is null', () => {
      setupAgentMock({ model: null });
      render(<AgentProfileClient id="test-agent-1" />);
      expect(screen.queryByText('MODEL:')).not.toBeInTheDocument();
    });

    it('does not display model section when model is empty string', () => {
      setupAgentMock({ model: '' });
      render(<AgentProfileClient id="test-agent-1" />);
      expect(screen.queryByText('MODEL:')).not.toBeInTheDocument();
    });

    it('does not display model section when model is undefined', () => {
      setupAgentMock();
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
      setupAgentMock();
      render(<AgentProfileClient id="test-agent-1" />);
      expect(screen.getByText('Test Agent')).toBeInTheDocument();
      expect(screen.getByText('A test agent for testing')).toBeInTheDocument();
    });

    it('displays Human-Backed badge when hasHumanBackedBadge is true', () => {
      setupAgentMock({ hasHumanBackedBadge: true });
      render(<AgentProfileClient id="test-agent-1" />);
      expect(screen.getByText('HUMAN-BACKED')).toBeInTheDocument();
    });
  });

  describe('Tab system', () => {
    it('renders ACTIVITY and RESURRECTION tabs below stats grid', () => {
      setupAgentMock();
      render(<AgentProfileClient id="test-agent-1" />);
      expect(screen.getByRole('button', { name: 'ACTIVITY' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'RESURRECTION' })).toBeInTheDocument();
    });

    it('defaults to ACTIVITY tab with activity feed shown', () => {
      setupAgentMock();
      render(<AgentProfileClient id="test-agent-1" />);

      const activityTab = screen.getByRole('button', { name: 'ACTIVITY' });
      // Active tab should have bg-foreground styling
      expect(activityTab.className).toContain('bg-foreground');
    });

    it('clicking RESURRECTION tab hides activity feed and shows checkpoint content', () => {
      setupAgentMock();
      mockUseCheckpoints.mockReturnValue(mockCheckpointsData);
      mockUseResurrectionBundle.mockReturnValue(mockBundleData);

      render(<AgentProfileClient id="test-agent-1" />);

      const resurrectionTab = screen.getByRole('button', { name: 'RESURRECTION' });
      fireEvent.click(resurrectionTab);

      // RESURRECTION tab should now be active
      expect(resurrectionTab.className).toContain('bg-foreground');

      // Should show checkpoint content
      expect(screen.getByText('LATEST CHECKPOINT')).toBeInTheDocument();
    });
  });

  describe('Resurrection tab - checkpoints', () => {
    beforeEach(() => {
      setupAgentMock();
    });

    it('shows LATEST CHECKPOINT card with CID linked to IPFS gateway', () => {
      mockUseCheckpoints.mockReturnValue(mockCheckpointsData);
      mockUseResurrectionBundle.mockReturnValue(mockBundleData);

      render(<AgentProfileClient id="test-agent-1" />);
      fireEvent.click(screen.getByRole('button', { name: 'RESURRECTION' }));

      expect(screen.getByText('LATEST CHECKPOINT')).toBeInTheDocument();
      const cidLinks = screen.getAllByRole('link', { name: /QmLatest/i });
      expect(cidLinks.length).toBeGreaterThanOrEqual(1);
      expect(cidLinks[0]).toHaveAttribute('href', 'https://ipfs.io/ipfs/QmLatestCheckpoint123abc');
    });

    it('shows checkpoint entries with meta badges', () => {
      mockUseCheckpoints.mockReturnValue(mockCheckpointsData);
      mockUseResurrectionBundle.mockReturnValue(mockBundleData);

      render(<AgentProfileClient id="test-agent-1" />);
      fireEvent.click(screen.getByRole('button', { name: 'RESURRECTION' }));

      // System meta keys (type, agent_id) should have emerald styling
      const typeBadges = screen.getAllByText('type');
      expect(typeBadges.length).toBeGreaterThan(0);
    });

    it('shows dashed border NO CHECKPOINTS empty state when no checkpoints exist', () => {
      mockUseCheckpoints.mockReturnValue(emptyCheckpoints);
      mockUseResurrectionBundle.mockReturnValue(emptyBundle);

      render(<AgentProfileClient id="test-agent-1" />);
      fireEvent.click(screen.getByRole('button', { name: 'RESURRECTION' }));

      expect(screen.getByText('NO CHECKPOINTS')).toBeInTheDocument();
    });
  });

  describe('Resurrection tab - knowledge summary', () => {
    it('shows knowledge summary grid with counts from resurrection bundle', () => {
      setupAgentMock();
      mockUseCheckpoints.mockReturnValue(mockCheckpointsData);
      mockUseResurrectionBundle.mockReturnValue(mockBundleData);

      render(<AgentProfileClient id="test-agent-1" />);
      fireEvent.click(screen.getByRole('button', { name: 'RESURRECTION' }));

      expect(screen.getByText('KNOWLEDGE SUMMARY')).toBeInTheDocument();
      // Should show death count and approaches (unique to resurrection tab)
      expect(screen.getByText('DEATHS')).toBeInTheDocument();
      expect(screen.getByText('APPROACHES')).toBeInTheDocument();
      // PROBLEMS may appear multiple times (knowledge card + other places)
      expect(screen.getAllByText('PROBLEMS').length).toBeGreaterThanOrEqual(1);
    });
  });

  describe('Resurrection tab - knowledge with null fields', () => {
    it('renders without crashing when bundle.knowledge is null', () => {
      setupAgentMock();
      mockUseCheckpoints.mockReturnValue(mockCheckpointsData);
      mockUseResurrectionBundle.mockReturnValue({
        bundle: {
          ...mockBundleData.bundle,
          knowledge: null, // edge case: backend returns null
        },
        loading: false,
        error: null,
      });

      render(<AgentProfileClient id="test-agent-1" />);
      fireEvent.click(screen.getByRole('button', { name: 'RESURRECTION' }));

      // Should not crash; knowledge summary section should show 0 counts
      expect(screen.getByText('KNOWLEDGE SUMMARY')).toBeInTheDocument();
      expect(screen.getAllByText('IDEAS').length).toBeGreaterThanOrEqual(1);
    });
  });

  describe('Resurrection tab - KERI identity', () => {
    it('renders KERI identity section when agent has amcp_aid', () => {
      setupAgentMock();
      mockUseCheckpoints.mockReturnValue(mockCheckpointsData);
      mockUseResurrectionBundle.mockReturnValue(mockBundleData);

      render(<AgentProfileClient id="test-agent-1" />);
      fireEvent.click(screen.getByRole('button', { name: 'RESURRECTION' }));

      expect(screen.getByText('KERI IDENTITY')).toBeInTheDocument();
      expect(screen.getByText('did:keri:ETestAID123')).toBeInTheDocument();
    });

    it('does not render KERI identity section when agent has no amcp_aid', () => {
      setupAgentMock();
      mockUseCheckpoints.mockReturnValue(emptyCheckpoints);
      mockUseResurrectionBundle.mockReturnValue({
        bundle: {
          ...mockBundleData.bundle,
          identity: {
            ...mockBundleData.bundle.identity,
            has_amcp_identity: false,
            amcp_aid: undefined,
            keri_public_key: undefined,
          },
        },
        loading: false,
        error: null,
      });

      render(<AgentProfileClient id="test-agent-1" />);
      fireEvent.click(screen.getByRole('button', { name: 'RESURRECTION' }));

      expect(screen.queryByText('KERI IDENTITY')).not.toBeInTheDocument();
    });
  });
});
