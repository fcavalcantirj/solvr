import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { RoomCard } from './room-card';
import type { APIRoomWithStats } from '@/lib/api-types';

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

// Mock date-fns to avoid time-dependent test failures
vi.mock('date-fns', () => ({
  formatDistanceToNow: () => '3 minutes ago',
}));

const mockRoom: APIRoomWithStats = {
  id: 'room-abc123',
  slug: 'solvr-usage-analysis',
  display_name: 'Solvr Usage Analysis',
  description: 'A deep analysis of how Solvr is being used by agents and humans in production.',
  category: 'analytics',
  tags: ['solvr', 'analytics', 'agents'],
  is_private: false,
  owner_id: 'user-owner-1',
  message_count: 42,
  created_at: '2026-04-01T10:00:00Z',
  updated_at: '2026-04-04T15:30:00Z',
  last_active_at: '2026-04-04T15:30:00Z',
  live_agent_count: 3,
  unique_participant_count: 7,
  owner_display_name: 'fcavalcanti',
};

describe('RoomCard', () => {
  it('renders room display_name text', () => {
    render(<RoomCard room={mockRoom} />);
    expect(screen.getByText('Solvr Usage Analysis')).toBeInTheDocument();
  });

  it('renders description snippet when present', () => {
    render(<RoomCard room={mockRoom} />);
    expect(screen.getByText('A deep analysis of how Solvr is being used by agents and humans in production.')).toBeInTheDocument();
  });

  it('renders category badge when category is present', () => {
    render(<RoomCard room={mockRoom} />);
    expect(screen.getByText('analytics')).toBeInTheDocument();
  });

  it('does NOT render category badge when category is absent', () => {
    const roomWithoutCategory: APIRoomWithStats = { ...mockRoom, category: undefined };
    render(<RoomCard room={roomWithoutCategory} />);
    expect(screen.queryByText('analytics')).not.toBeInTheDocument();
  });

  it('renders slug-based link href as /rooms/{slug}', () => {
    render(<RoomCard room={mockRoom} />);
    const links = screen.getAllByRole('link');
    const roomLink = links.find(link => link.getAttribute('href') === '/rooms/solvr-usage-analysis');
    expect(roomLink).toBeDefined();
  });

  it('renders live_agent_count with green pulsing dot when count > 0', () => {
    render(<RoomCard room={mockRoom} />);
    // Check that the animate-pulse element exists (green dot)
    const pulseDot = document.querySelector('.animate-pulse.bg-green-500');
    expect(pulseDot).toBeInTheDocument();
  });

  it('does NOT render green pulsing dot when live_agent_count is 0', () => {
    const roomNoLive: APIRoomWithStats = { ...mockRoom, live_agent_count: 0 };
    render(<RoomCard room={roomNoLive} />);
    const pulseDot = document.querySelector('.animate-pulse.bg-green-500');
    expect(pulseDot).not.toBeInTheDocument();
  });

  it('renders unique_participant_count as "{N} participants"', () => {
    render(<RoomCard room={mockRoom} />);
    expect(screen.getByText('7 participants')).toBeInTheDocument();
  });

  it('renders message_count as "{N} messages"', () => {
    render(<RoomCard room={mockRoom} />);
    expect(screen.getByText('42 messages')).toBeInTheDocument();
  });

  it('renders owner_display_name as clickable link when present', () => {
    render(<RoomCard room={mockRoom} />);
    // The owner link points to /users/{owner_id}
    const ownerLink = screen.getByRole('link', { name: 'fcavalcanti' });
    expect(ownerLink).toBeInTheDocument();
    expect(ownerLink).toHaveAttribute('href', '/users/user-owner-1');
  });

  it('does NOT render owner section when owner_display_name is absent', () => {
    const roomNoOwner: APIRoomWithStats = { ...mockRoom, owner_display_name: undefined };
    render(<RoomCard room={roomNoOwner} />);
    // No owner link should exist pointing to user profile
    const links = screen.getAllByRole('link');
    const ownerLink = links.find(link => link.getAttribute('href')?.startsWith('/users/'));
    expect(ownerLink).toBeUndefined();
  });

  it('renders relative time for last_active_at', () => {
    render(<RoomCard room={mockRoom} />);
    expect(screen.getByText('3 minutes ago')).toBeInTheDocument();
  });
});
