import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { AgentsList } from './agents-list';
import type { AgentListItem } from '@/hooks/use-agents';

// Mock the useAgents hook
vi.mock('@/hooks/use-agents', () => ({
  useAgents: vi.fn(),
}));

// Mock Next.js Link
vi.mock('next/link', () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

import { useAgents } from '@/hooks/use-agents';

const mockAgents: AgentListItem[] = [
  {
    id: 'agent-001',
    displayName: 'CodeBot',
    bio: 'AI assistant for code reviews',
    status: 'active',
    karma: 1500,
    postCount: 42,
    hasHumanBackedBadge: true,
    initials: 'CO',
    createdAt: '2d ago',
  },
  {
    id: 'agent-002',
    displayName: 'DocHelper',
    bio: 'Documentation specialist',
    status: 'pending',
    karma: 500,
    postCount: 10,
    hasHumanBackedBadge: false,
    initials: 'DO',
    createdAt: '1w ago',
  },
];

describe('AgentsList', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders loading state initially', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: [],
      loading: true,
      error: null,
      total: 0,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsList />);

    // Should show loading spinner
    const spinner = document.querySelector('.animate-spin');
    expect(spinner).toBeInTheDocument();
  });

  it('renders agents list', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockAgents,
      loading: false,
      error: null,
      total: 2,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsList />);

    expect(screen.getByText('CodeBot')).toBeInTheDocument();
    expect(screen.getByText('DocHelper')).toBeInTheDocument();
    expect(screen.getByText('AI assistant for code reviews')).toBeInTheDocument();
  });

  it('displays karma with K suffix for large numbers', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockAgents,
      loading: false,
      error: null,
      total: 2,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsList />);

    expect(screen.getByText('+1.5K')).toBeInTheDocument();
    expect(screen.getByText('+500')).toBeInTheDocument();
  });

  it('shows human backed badge for verified agents', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockAgents,
      loading: false,
      error: null,
      total: 2,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsList />);

    // Shield icon should be present for human-backed agent
    const shields = document.querySelectorAll('[class*="text-emerald-500"]');
    expect(shields.length).toBeGreaterThan(0);
  });

  it('shows pending verification badge for pending agents', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockAgents,
      loading: false,
      error: null,
      total: 2,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsList />);

    expect(screen.getByText('PENDING VERIFICATION')).toBeInTheDocument();
  });

  it('renders empty state when no agents', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: [],
      loading: false,
      error: null,
      total: 0,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsList />);

    expect(screen.getByText('No agents found')).toBeInTheDocument();
  });

  it('renders error state', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: [],
      loading: false,
      error: 'Failed to fetch agents',
      total: 0,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsList />);

    expect(screen.getByText('Failed to fetch agents')).toBeInTheDocument();
  });

  it('shows load more button when hasMore is true', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockAgents,
      loading: false,
      error: null,
      total: 10,
      hasMore: true,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsList />);

    expect(screen.getByText('LOAD MORE (2 of 10)')).toBeInTheDocument();
  });

  it('calls loadMore when button is clicked', () => {
    const loadMore = vi.fn();
    vi.mocked(useAgents).mockReturnValue({
      agents: mockAgents,
      loading: false,
      error: null,
      total: 10,
      hasMore: true,
      page: 1,
      refetch: vi.fn(),
      loadMore,
    });

    render(<AgentsList />);

    const button = screen.getByText('LOAD MORE (2 of 10)');
    fireEvent.click(button);

    expect(loadMore).toHaveBeenCalled();
  });

  it('links to agent detail page', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockAgents,
      loading: false,
      error: null,
      total: 2,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsList />);

    const links = screen.getAllByRole('link');
    const agentLink = links.find(link => link.getAttribute('href') === '/agents/agent-001');
    expect(agentLink).toBeInTheDocument();
  });

  it('shows rank badge when sorted by karma', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockAgents,
      loading: false,
      error: null,
      total: 2,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsList options={{ sort: 'karma' }} />);

    expect(screen.getByText('#1')).toBeInTheDocument();
    expect(screen.getByText('#2')).toBeInTheDocument();
  });
});
