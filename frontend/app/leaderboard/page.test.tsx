import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { LeaderboardPageClient } from '@/components/leaderboard/leaderboard-page-client';

// Mock functions must be declared before vi.mock calls
const mockLoadMore = vi.fn();
const mockRefetch = vi.fn();

// Create mock data factory
const createMockLeaderboardData = (hasMore = true) => ({
  entries: [
    {
      rank: 1,
      id: 'agent-123',
      type: 'agent' as const,
      displayName: 'SolverBot',
      avatarUrl: 'https://example.com/avatar1.jpg',
      reputation: 1250,
      profileLink: '/agents/agent-123',
      keyStats: {
        problemsSolved: 15,
        answersAccepted: 28,
        upvotesReceived: 150,
        totalContributions: 193,
      },
    },
    {
      rank: 2,
      id: 'user-456',
      type: 'user' as const,
      displayName: 'Alice Dev',
      avatarUrl: undefined,
      reputation: 980,
      profileLink: '/users/user-456',
      keyStats: {
        problemsSolved: 8,
        answersAccepted: 42,
        upvotesReceived: 120,
        totalContributions: 170,
      },
    },
    {
      rank: 3,
      id: 'agent-789',
      type: 'agent' as const,
      displayName: 'CodeHelper',
      avatarUrl: 'https://example.com/avatar2.jpg',
      reputation: 850,
      profileLink: '/agents/agent-789',
      keyStats: {
        problemsSolved: 12,
        answersAccepted: 20,
        upvotesReceived: 90,
        totalContributions: 122,
      },
    },
  ],
  loading: false,
  error: null,
  total: hasMore ? 125 : 3,
  hasMore,
  loadMore: mockLoadMore,
  refetch: mockRefetch,
});

// Mock the useLeaderboard hook
vi.mock('@/hooks/use-leaderboard', () => ({
  useLeaderboard: vi.fn(() => createMockLeaderboardData()),
  transformLeaderboardEntry: vi.fn((entry: Record<string, unknown>) => entry),
}));

// Import after mocks
import { useLeaderboard } from '@/hooks/use-leaderboard';

describe('LeaderboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders timeframe tabs (all time, monthly, weekly)', () => {
    render(<LeaderboardPageClient initialEntries={[]} />);

    expect(screen.getByText('ALL TIME')).toBeInTheDocument();
    expect(screen.getByText('THIS MONTH')).toBeInTheDocument();
    expect(screen.getByText('THIS WEEK')).toBeInTheDocument();
  });

  it('clicking timeframe tab updates active state and refetches', async () => {
    render(<LeaderboardPageClient initialEntries={[]} />);

    const allTimeTab = screen.getByText('ALL TIME');
    const monthlyTab = screen.getByText('THIS MONTH');

    // ALL TIME should be active initially
    expect(allTimeTab.closest('button')).toHaveClass('bg-foreground');

    // Clear previous calls
    vi.clearAllMocks();

    // Click monthly tab
    fireEvent.click(monthlyTab);

    await waitFor(() => {
      // Hook should be called with monthly timeframe
      expect(useLeaderboard).toHaveBeenCalledWith(
        expect.objectContaining({ timeframe: 'monthly' })
      );
    });

    // Monthly tab should now be active
    expect(monthlyTab.closest('button')).toHaveClass('bg-foreground');
  });

  it('renders type filter pills (all, humans, agents)', () => {
    render(<LeaderboardPageClient initialEntries={[]} />);

    expect(screen.getByText('ALL')).toBeInTheDocument();
    expect(screen.getByText('HUMANS')).toBeInTheDocument();
    expect(screen.getByText('AGENTS')).toBeInTheDocument();
  });

  it('clicking type pill updates filter and refetches', async () => {
    render(<LeaderboardPageClient initialEntries={[]} />);

    const allPill = screen.getByText('ALL');
    const agentsPill = screen.getByText('AGENTS');

    // ALL should be active initially
    expect(allPill.closest('button')).toHaveClass('bg-foreground');

    // Clear previous calls
    vi.clearAllMocks();

    // Click agents pill
    fireEvent.click(agentsPill);

    await waitFor(() => {
      // Hook should be called with agents type
      expect(useLeaderboard).toHaveBeenCalledWith(
        expect.objectContaining({ type: 'agents' })
      );
    });

    // Agents pill should now be active
    expect(agentsPill.closest('button')).toHaveClass('bg-foreground');
  });

  it('renders leaderboard entries with correct rank badges', () => {
    render(<LeaderboardPageClient initialEntries={[]} />);

    expect(screen.getByText('#1')).toBeInTheDocument();
    expect(screen.getByText('#2')).toBeInTheDocument();
    expect(screen.getByText('#3')).toBeInTheDocument();

    expect(screen.getByText('SolverBot')).toBeInTheDocument();
    expect(screen.getByText('Alice Dev')).toBeInTheDocument();
    expect(screen.getByText('CodeHelper')).toBeInTheDocument();

    expect(screen.getByText('1,250')).toBeInTheDocument();
    expect(screen.getByText('980')).toBeInTheDocument();
    expect(screen.getByText('850')).toBeInTheDocument();
  });

  it('rank #1-#3 have special styling (gold/silver/bronze)', () => {
    render(<LeaderboardPageClient initialEntries={[]} />);

    const rank1Badge = screen.getByText('#1').closest('div');
    const rank2Badge = screen.getByText('#2').closest('div');
    const rank3Badge = screen.getByText('#3').closest('div');

    expect(rank1Badge?.className).toMatch(/bg-yellow/);
    expect(rank2Badge?.className).toMatch(/bg-(gray|slate)/);
    expect(rank3Badge?.className).toMatch(/bg-(orange|amber)/);
  });

  it('entries link to correct profile pages based on type', () => {
    render(<LeaderboardPageClient initialEntries={[]} />);

    const agentLink = screen.getByText('SolverBot').closest('a');
    expect(agentLink).toHaveAttribute('href', '/agents/agent-123');

    const userLink = screen.getByText('Alice Dev').closest('a');
    expect(userLink).toHaveAttribute('href', '/users/user-456');
  });

  it('LOAD MORE button calls loadMore() and is hidden when !hasMore', () => {
    const { rerender } = render(<LeaderboardPageClient initialEntries={[]} />);

    const loadMoreButton = screen.getByText('LOAD MORE');
    expect(loadMoreButton).toBeInTheDocument();

    fireEvent.click(loadMoreButton);
    expect(mockLoadMore).toHaveBeenCalledTimes(1);

    vi.mocked(useLeaderboard).mockReturnValue(createMockLeaderboardData(false));

    rerender(<LeaderboardPageClient initialEntries={[]} />);

    expect(screen.queryByText('LOAD MORE')).not.toBeInTheDocument();
  });
});
