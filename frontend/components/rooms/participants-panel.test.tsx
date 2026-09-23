import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ParticipantsPanel } from './participants-panel';
import type { APIRoomMember, APIRoom } from '@/lib/api-types';

// Mock the api module
vi.mock('@/lib/api', () => ({
  api: {
    getMembers: vi.fn(),
  },
}));

const mockRoom: APIRoom = {
  id: 'room-123',
  slug: 'test-room',
  display_name: 'Test Room',
  tags: [],
  is_private: false,
  message_count: 5,
  created_at: '2026-09-20T00:00:00Z',
  updated_at: '2026-09-20T00:00:00Z',
  last_active_at: '2026-09-20T00:00:00Z',
  capacity_max: 10, // Step 6: expose capacity
};

const mockMembers: APIRoomMember[] = [
  {
    room_id: 'room-123',
    agent_id: 'agent-1',
    role: 'owner',
    added_by: 'system',
    created_at: '2026-09-20T00:00:00Z',
  },
  {
    room_id: 'room-123',
    agent_id: 'agent-2',
    role: 'member',
    added_by: 'agent-1',
    created_at: '2026-09-20T00:01:00Z',
  },
  {
    room_id: 'room-123',
    agent_id: 'agent-3',
    role: 'member',
    added_by: 'agent-1',
    created_at: '2026-09-20T00:02:00Z',
  },
];

describe('ParticipantsPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders participants heading', () => {
    render(<ParticipantsPanel room={mockRoom} members={mockMembers} />);
    expect(screen.getByText(/PARTICIPANTS/i)).toBeDefined();
  });

  it('displays participant count with capacity', () => {
    render(<ParticipantsPanel room={mockRoom} members={mockMembers} />);
    // With capacity_max, should show "current / max" format
    expect(screen.getByText(/3 \/ 10/)).toBeDefined();
  });

  it('shows capacity information', () => {
    render(<ParticipantsPanel room={mockRoom} members={mockMembers} />);
    // Step 6: display current/max
    expect(screen.getByText(/3 \/ 10/i)).toBeDefined(); // current / max
  });

  it('displays each member with role', () => {
    render(<ParticipantsPanel room={mockRoom} members={mockMembers} />);

    // Step 5: display full member list
    expect(screen.getByText(/agent-1/)).toBeDefined();
    expect(screen.getByText(/agent-2/)).toBeDefined();
    expect(screen.getByText(/agent-3/)).toBeDefined();

    // Step 4: show role for each member
    expect(screen.getAllByText(/owner|member/i).length).toBeGreaterThan(0);
  });

  it('handles unlimited capacity (no max)', () => {
    const roomNoCapacity = { ...mockRoom, capacity_max: undefined };
    render(
      <ParticipantsPanel
        room={roomNoCapacity}
        members={mockMembers}
      />
    );
    // Should show members without capacity limit
    expect(screen.getByText(/3 members/i)).toBeDefined();
    expect(screen.queryByText(/unlimited/i)).toBeNull(); // Don't advertise "unlimited"
  });

  it('shows add agent action', () => {
    render(<ParticipantsPanel room={mockRoom} members={mockMembers} />);
    // Step 5: "Connect an agent action clear on desktop and mobile"
    const addButton = screen.getByRole('link', {
      name: /add another agent|connect an agent/i,
    });
    expect(addButton).toBeDefined();
  });

  it('does not advertise infinite capacity', () => {
    const roomUnlimited = { ...mockRoom, capacity_max: undefined };
    render(
      <ParticipantsPanel
        room={roomUnlimited}
        members={mockMembers}
      />
    );
    // Step 6: do not advertise "infinite" or "unlimited"
    expect(screen.queryByText(/infinite|unlimited|no limit/i)).toBeNull();
  });
});
