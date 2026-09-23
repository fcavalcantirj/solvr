import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { RoomListClient } from './room-list';
import { api } from '@/lib/api';
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
  beforeEach(() => {
    vi.mocked(api.fetchRooms).mockReset();
  });

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

  it('renders a search field, Recent/Active now sort controls, and a Connect agents action', () => {
    render(<RoomListClient initialRooms={partialRooms} />);
    expect(screen.getByLabelText('Search rooms')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Recent' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Active now' })).toBeInTheDocument();
    const connect = screen.getByRole('link', { name: /CONNECT AGENTS/i });
    expect(connect).toHaveAttribute('href', '/connect');
  });

  it('re-queries the API with sort=active when Active now is selected', async () => {
    vi.mocked(api.fetchRooms).mockResolvedValue({ data: [createMockRoom('99')] });
    render(<RoomListClient initialRooms={partialRooms} />);

    fireEvent.click(screen.getByRole('button', { name: 'Active now' }));

    await waitFor(() => {
      expect(api.fetchRooms).toHaveBeenCalledWith(
        expect.objectContaining({ sort: 'active', offset: 0 }),
      );
    });
    // The list is replaced with the server response.
    await waitFor(() => {
      expect(screen.getByText('Room 99')).toBeInTheDocument();
    });
    expect(screen.queryByText('Room 1')).not.toBeInTheDocument();
  });

  it('re-queries the API with the search text on submit', async () => {
    vi.mocked(api.fetchRooms).mockResolvedValue({ data: [createMockRoom('42')] });
    render(<RoomListClient initialRooms={partialRooms} />);

    fireEvent.change(screen.getByLabelText('Search rooms'), { target: { value: 'parser' } });
    fireEvent.submit(screen.getByRole('search'));

    await waitFor(() => {
      expect(api.fetchRooms).toHaveBeenCalledWith(
        expect.objectContaining({ q: 'parser', offset: 0 }),
      );
    });
  });

  it('deduplicates rooms already shown when loading more', async () => {
    // Server returns a page that overlaps with rooms already on screen (room 20
    // moved due to new activity) plus a genuinely new room.
    vi.mocked(api.fetchRooms).mockResolvedValue({
      data: [createMockRoom('20'), createMockRoom('21')],
    });
    render(<RoomListClient initialRooms={fullPageRooms} />);

    fireEvent.click(screen.getByRole('button', { name: /LOAD MORE ROOMS/i }));

    await waitFor(() => {
      expect(screen.getByText('Room 21')).toBeInTheDocument();
    });
    // Room 20 already existed; it must appear exactly once, not duplicated.
    expect(screen.getAllByText('Room 20')).toHaveLength(1);
  });

  it('shows an error state with Retry when a re-query fails', async () => {
    vi.mocked(api.fetchRooms).mockRejectedValue(new Error('network'));
    render(<RoomListClient initialRooms={partialRooms} />);

    fireEvent.click(screen.getByRole('button', { name: 'Active now' }));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'RETRY' })).toBeInTheDocument();
    });
  });

  it('shows a plain "no agents online" note while still listing recent rooms when none has live presence', () => {
    // Rooms exist (recent collaborations) but no server-confirmed presence:
    // live_agent_count is 0 for every one even though they have historical
    // participants. Offline participants must never read as live activity.
    const offlineRooms: APIRoomWithStats[] = Array.from({ length: 3 }, (_, i) => ({
      ...createMockRoom(String(i + 1)),
      live_agent_count: 0,
      unique_participant_count: 3,
    }));
    render(<RoomListClient initialRooms={offlineRooms} />);

    // The page says plainly that no agents are online right now...
    expect(screen.getByText(/no agents are online right now/i)).toBeInTheDocument();
    // ...while STILL showing the recent public collaborations (not an empty state).
    expect(screen.getAllByTestId('room-card')).toHaveLength(3);
    expect(screen.queryByText('No rooms yet')).not.toBeInTheDocument();
  });

  it('does NOT show the no-agents-online note when at least one room has a live agent', () => {
    // partialRooms all have live_agent_count: 1 (see createMockRoom).
    render(<RoomListClient initialRooms={partialRooms} />);
    expect(screen.queryByText(/no agents are online right now/i)).not.toBeInTheDocument();
  });

  it('empty search state preserves the search text and offers Connect agents', async () => {
    vi.mocked(api.fetchRooms).mockResolvedValue({ data: [] });
    render(<RoomListClient initialRooms={partialRooms} />);

    fireEvent.change(screen.getByLabelText('Search rooms'), {
      target: { value: 'nonexistent-xyz' },
    });
    fireEvent.submit(screen.getByRole('search'));

    await waitFor(() => {
      expect(screen.getByText(/No rooms match "nonexistent-xyz"/)).toBeInTheDocument();
    });
    // The typed query stays in the search field so the user can refine it.
    expect(screen.getByLabelText('Search rooms')).toHaveValue('nonexistent-xyz');
    // And a Connect agents action is offered from the empty state.
    const connect = screen.getAllByRole('link', { name: /CONNECT AGENTS/i });
    expect(connect.length).toBeGreaterThan(0);
  });
});
