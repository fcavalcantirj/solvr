import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { RoomListClient } from './room-list';
import type { APIRoomWithStats } from '@/lib/api-types';

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

// Mock the room-card component to simplify list tests
vi.mock('./room-card', () => ({
  RoomCard: ({ room }: { room: APIRoomWithStats }) => (
    <div data-testid="room-card">{room.display_name}</div>
  ),
}));

// Mock api module to avoid real fetch calls
vi.mock('@/lib/api', () => ({
  api: {
    fetchRooms: vi.fn(),
  },
}));

const createMockRoom = (id: string): APIRoomWithStats => ({
  id,
  slug: `room-${id}`,
  display_name: `Room ${id}`,
  description: `Description for room ${id}`,
  category: 'general',
  tags: [],
  is_private: false,
  owner_id: `owner-${id}`,
  message_count: 10,
  created_at: '2026-04-01T10:00:00Z',
  updated_at: '2026-04-04T10:00:00Z',
  last_active_at: '2026-04-04T10:00:00Z',
  live_agent_count: 1,
  unique_participant_count: 3,
  owner_display_name: `Owner ${id}`,
});

// Create a full page of rooms (20)
const fullPageRooms: APIRoomWithStats[] = Array.from({ length: 20 }, (_, i) =>
  createMockRoom(String(i + 1))
);

// Create a partial list (fewer than 20)
const partialRooms: APIRoomWithStats[] = Array.from({ length: 5 }, (_, i) =>
  createMockRoom(String(i + 1))
);

describe('RoomListClient', () => {
  it('renders room cards for each room in initialRooms', () => {
    render(<RoomListClient initialRooms={partialRooms} />);
    const cards = screen.getAllByTestId('room-card');
    expect(cards).toHaveLength(5);
  });

  it('renders empty state with "No rooms yet" when initialRooms is empty', () => {
    render(<RoomListClient initialRooms={[]} />);
    expect(screen.getByText('No rooms yet')).toBeInTheDocument();
  });

  it('renders "LOAD MORE ROOMS" button when rooms fill a full page (>= 20)', () => {
    render(<RoomListClient initialRooms={fullPageRooms} />);
    expect(screen.getByRole('button', { name: /LOAD MORE ROOMS/i })).toBeInTheDocument();
  });

  it('does NOT render load more button when fewer than 20 rooms', () => {
    render(<RoomListClient initialRooms={partialRooms} />);
    expect(screen.queryByRole('button', { name: /LOAD MORE ROOMS/i })).not.toBeInTheDocument();
  });
});
