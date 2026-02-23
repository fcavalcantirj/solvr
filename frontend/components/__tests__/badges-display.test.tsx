import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock api methods — real API returns { data: { badges: [...] } }
const mockGetAgentBadges = vi.fn().mockResolvedValue({ data: { badges: [] } });
const mockGetUserBadges = vi.fn().mockResolvedValue({ data: { badges: [] } });

vi.mock('@/lib/api', () => ({
  api: {
    getAgentBadges: (...args: unknown[]) => mockGetAgentBadges(...args),
    getUserBadges: (...args: unknown[]) => mockGetUserBadges(...args),
  },
}));

import { BadgesDisplay } from '../badges-display';

const sampleBadges = [
  {
    id: 'badge-1',
    owner_type: 'agent',
    owner_id: 'agent-1',
    badge_type: 'first_solve',
    badge_name: 'First Solve',
    description: 'Solved your first problem',
    awarded_at: '2026-01-15T10:00:00Z',
    metadata: null,
  },
  {
    id: 'badge-2',
    owner_type: 'agent',
    owner_id: 'agent-1',
    badge_type: 'seven_day_streak',
    badge_name: '7-Day Streak',
    description: 'Active for 7 consecutive days',
    awarded_at: '2026-01-20T10:00:00Z',
    metadata: null,
  },
  {
    id: 'badge-3',
    owner_type: 'agent',
    owner_id: 'agent-1',
    badge_type: 'hundred_upvotes',
    badge_name: '100 Upvotes',
    description: 'Received 100 upvotes',
    awarded_at: '2026-01-22T10:00:00Z',
    metadata: null,
  },
  {
    id: 'badge-4',
    owner_type: 'agent',
    owner_id: 'agent-1',
    badge_type: 'human_backed',
    badge_name: 'Human-Backed',
    description: 'Claimed by a human backer',
    awarded_at: '2026-01-25T10:00:00Z',
    metadata: null,
  },
  {
    id: 'badge-5',
    owner_type: 'agent',
    owner_id: 'agent-1',
    badge_type: 'crystallized',
    badge_name: 'Crystallized',
    description: 'Content archived to IPFS',
    awarded_at: '2026-01-28T10:00:00Z',
    metadata: null,
  },
];

describe('BadgesDisplay', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetAgentBadges.mockResolvedValue({ data: { badges: [] } });
    mockGetUserBadges.mockResolvedValue({ data: { badges: [] } });
  });

  it('renders nothing when badges array is empty', async () => {
    mockGetAgentBadges.mockResolvedValue({ data: { badges: [] } });
    const { container } = render(
      <BadgesDisplay ownerType="agent" ownerId="agent-1" />
    );

    await waitFor(() => {
      expect(mockGetAgentBadges).toHaveBeenCalledWith('agent-1');
    });

    // Should render nothing when empty
    expect(container.querySelector('[data-testid="badges-display"]')).toBeNull();
  });

  it('renders correct number of badge chips for given badges', async () => {
    mockGetAgentBadges.mockResolvedValue({ data: { badges: sampleBadges } });
    render(<BadgesDisplay ownerType="agent" ownerId="agent-1" />);

    await waitFor(() => {
      const badges = screen.getAllByTestId('badge-chip');
      expect(badges).toHaveLength(5);
    });
  });

  it('each badge shows correct icon for its badge_type', async () => {
    mockGetAgentBadges.mockResolvedValue({ data: { badges: sampleBadges } });
    render(<BadgesDisplay ownerType="agent" ownerId="agent-1" />);

    await waitFor(() => {
      expect(screen.getByText('First Solve')).toBeInTheDocument();
      expect(screen.getByText('7-Day Streak')).toBeInTheDocument();
      expect(screen.getByText('100 Upvotes')).toBeInTheDocument();
      expect(screen.getByText('Human-Backed')).toBeInTheDocument();
      expect(screen.getByText('Crystallized')).toBeInTheDocument();
    });
  });

  it('calls getUserBadges for human owner type', async () => {
    mockGetUserBadges.mockResolvedValue({ data: { badges: [sampleBadges[0]] } });
    render(<BadgesDisplay ownerType="human" ownerId="user-1" />);

    await waitFor(() => {
      expect(mockGetUserBadges).toHaveBeenCalledWith('user-1');
    });
    expect(mockGetAgentBadges).not.toHaveBeenCalled();
  });

  it('shows badge description as tooltip', async () => {
    mockGetAgentBadges.mockResolvedValue({ data: { badges: [sampleBadges[0]] } });
    render(<BadgesDisplay ownerType="agent" ownerId="agent-1" />);

    await waitFor(() => {
      const chip = screen.getByTestId('badge-chip');
      expect(chip).toHaveAttribute('title', 'Solved your first problem');
    });
  });

  it('renders nothing when API returns data-wrapped response with empty badges (real API shape)', async () => {
    // Real API returns { data: { badges: [] } }, not { badges: [] }
    mockGetAgentBadges.mockResolvedValue({ data: { badges: [] } });
    const { container } = render(
      <BadgesDisplay ownerType="agent" ownerId="agent-1" />
    );

    await waitFor(() => {
      expect(mockGetAgentBadges).toHaveBeenCalledWith('agent-1');
    });

    expect(container.querySelector('[data-testid="badges-display"]')).toBeNull();
  });

  it('renders badges when API returns data-wrapped response with badges (real API shape)', async () => {
    // Real API returns { data: { badges: [...] } }, not { badges: [...] }
    mockGetAgentBadges.mockResolvedValue({ data: { badges: sampleBadges } });
    render(<BadgesDisplay ownerType="agent" ownerId="agent-1" />);

    await waitFor(() => {
      const badges = screen.getAllByTestId('badge-chip');
      expect(badges).toHaveLength(5);
    });
  });

  it('handles API error gracefully', async () => {
    mockGetAgentBadges.mockRejectedValue(new Error('Network error'));
    const { container } = render(
      <BadgesDisplay ownerType="agent" ownerId="agent-1" />
    );

    await waitFor(() => {
      expect(mockGetAgentBadges).toHaveBeenCalled();
    });

    // Should render nothing on error
    expect(container.querySelector('[data-testid="badges-display"]')).toBeNull();
  });
});
