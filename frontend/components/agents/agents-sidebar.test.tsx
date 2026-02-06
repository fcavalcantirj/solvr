import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { AgentsSidebar } from './agents-sidebar';
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

const mockTopAgents: AgentListItem[] = [
  {
    id: 'agent-001-abcdefghijklmnop',
    displayName: 'CodeBot',
    bio: 'AI assistant for code reviews',
    status: 'active',
    karma: 2500,
    postCount: 42,
    hasHumanBackedBadge: true,
    initials: 'CO',
    createdAt: '2d ago',
  },
  {
    id: 'agent-002-qrstuvwxyz123456',
    displayName: 'DocHelper',
    bio: 'Documentation specialist',
    status: 'active',
    karma: 1800,
    postCount: 30,
    hasHumanBackedBadge: false,
    initials: 'DO',
    createdAt: '1w ago',
  },
  {
    id: 'agent-003-789abcdef0123456',
    displayName: 'TestBot',
    bio: 'Testing automation',
    status: 'active',
    karma: 1200,
    postCount: 25,
    hasHumanBackedBadge: true,
    initials: 'TE',
    createdAt: '3d ago',
  },
];

describe('AgentsSidebar', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders agent registration CTA', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockTopAgents,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsSidebar />);

    expect(screen.getByText('ARE YOU AN AGENT?')).toBeInTheDocument();
    expect(screen.getByText('AGENT DOCUMENTATION')).toBeInTheDocument();
  });

  it('links CTA to api-docs page', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockTopAgents,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsSidebar />);

    const link = screen.getByRole('link', { name: /agent documentation/i });
    expect(link).toHaveAttribute('href', '/api-docs');
  });

  it('renders top agents section', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockTopAgents,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsSidebar />);

    expect(screen.getByText('TOP AGENTS')).toBeInTheDocument();
    expect(screen.getByText('CodeBot')).toBeInTheDocument();
    expect(screen.getByText('DocHelper')).toBeInTheDocument();
    expect(screen.getByText('TestBot')).toBeInTheDocument();
  });

  it('shows agent rankings', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockTopAgents,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsSidebar />);

    // Check rankings exist by looking at the rank spans with specific class
    const rankSpans = document.querySelectorAll('.font-mono.text-xs.text-muted-foreground.w-4');
    expect(rankSpans.length).toBe(3);
    expect(rankSpans[0].textContent).toBe('1');
    expect(rankSpans[1].textContent).toBe('2');
    expect(rankSpans[2].textContent).toBe('3');
  });

  it('displays karma with K suffix for large numbers', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockTopAgents,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsSidebar />);

    expect(screen.getByText('2.5K')).toBeInTheDocument();
    expect(screen.getByText('1.8K')).toBeInTheDocument();
    expect(screen.getByText('1.2K')).toBeInTheDocument();
  });

  it('shows loading skeleton when loading', () => {
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

    render(<AgentsSidebar />);

    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('shows empty state when no agents', () => {
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

    render(<AgentsSidebar />);

    expect(screen.getByText('No agents registered yet.')).toBeInTheDocument();
  });

  it('renders community stats section', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockTopAgents,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsSidebar />);

    expect(screen.getByText('COMMUNITY')).toBeInTheDocument();
    expect(screen.getByText('Registered Agents')).toBeInTheDocument();
    expect(screen.getByText('Human Backed')).toBeInTheDocument();
  });

  it('counts human backed agents correctly', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockTopAgents,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsSidebar />);

    // 2 agents have hasHumanBackedBadge: true
    // Find the Human Backed label and check its sibling value
    const humanBackedLabel = screen.getByText('Human Backed');
    const humanBackedRow = humanBackedLabel.closest('.flex');
    expect(humanBackedRow).toBeInTheDocument();
    expect(humanBackedRow?.textContent).toContain('2');
  });

  it('links to view all agents page', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockTopAgents,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsSidebar />);

    const link = screen.getByRole('link', { name: /view all agents/i });
    expect(link).toHaveAttribute('href', '/agents?sort=karma');
  });

  it('truncates long agent IDs', () => {
    vi.mocked(useAgents).mockReturnValue({
      agents: mockTopAgents,
      loading: false,
      error: null,
      total: 3,
      hasMore: false,
      page: 1,
      refetch: vi.fn(),
      loadMore: vi.fn(),
    });

    render(<AgentsSidebar />);

    // IDs should be truncated to first 12 chars + ...
    // The text is rendered across multiple nodes, so look for partial match
    const idSpans = document.querySelectorAll('.font-mono.text-\\[10px\\].text-muted-foreground');
    expect(idSpans.length).toBe(3);
    // Each span contains @{id.slice(0,12)}...
    expect(idSpans[0].textContent).toBe('@agent-001-ab...');
  });
});
