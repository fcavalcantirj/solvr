import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';

// What a visitor does with the Rooms list, as Google Analytics events (SPEC.md 27.7):
// search (on submit), sort_change and load_more, each sent once, after the list that was
// asked for arrived. The Public rooms / My rooms switch is filter_change.

const track = vi.hoisted(() => vi.fn());
vi.mock('@/lib/analytics', () => ({ track, trackOnNextPage: vi.fn() }));

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));
vi.mock('./room-card', () => ({
  RoomCard: ({ room }: { room: { display_name: string } }) => <div data-testid="room-card">{room.display_name}</div>,
}));
vi.mock('./my-rooms-list', () => ({ MyRoomsList: () => <div data-testid="my-rooms" /> }));
vi.mock('@/lib/api', () => ({ api: { fetchRooms: vi.fn() } }));

const auth = vi.hoisted(() => ({ isAuthenticated: true }));
vi.mock('@/hooks/use-auth', () => ({ useAuth: () => auth }));

import { api } from '@/lib/api';
import type { APIRoomWithStats } from '@/lib/api-types';
import { RoomListClient } from './room-list';
import { RoomsBrowser } from './rooms-browser';

const room = (id: string) => ({ id, slug: `room-${id}`, display_name: `Room ${id}`, live_agent_count: 0 }) as unknown as APIRoomWithStats;
const rooms = (count: number, prefix = 'r') => Array.from({ length: count }, (_, i) => room(`${prefix}${i + 1}`));

const eventsNamed = (name: string) => track.mock.calls.filter(([event]) => event === name);
const searchBox = () => screen.getByLabelText(/search rooms/i);
const submitSearch = (term: string) => {
  fireEvent.change(searchBox(), { target: { value: term } });
  fireEvent.submit(screen.getByRole('search'));
};

beforeEach(() => {
  track.mockReset();
  vi.mocked(api.fetchRooms).mockReset();
  vi.mocked(api.fetchRooms).mockResolvedValue({ data: rooms(3) });
  auth.isAuthenticated = true;
});

describe('search on /rooms', () => {
  it('is sent on submit once the answer arrived, with the term and the number of rooms it found', async () => {
    render(<RoomListClient initialRooms={rooms(5)} />);
    fireEvent.change(searchBox(), { target: { value: 'parser' } });
    // Typing alone searches nothing here: the box has a button.
    expect(api.fetchRooms).not.toHaveBeenCalled();
    expect(track).not.toHaveBeenCalled();

    fireEvent.submit(screen.getByRole('search'));
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(api.fetchRooms).toHaveBeenCalledWith(expect.objectContaining({ q: 'parser', offset: 0 }));
    expect(track).toHaveBeenCalledWith('search', { search_term: 'parser', results: 3, list: 'rooms' });
  });

  it('says zero results when nothing matched', async () => {
    vi.mocked(api.fetchRooms).mockResolvedValue({ data: [] });
    render(<RoomListClient initialRooms={rooms(5)} />);
    submitSearch('  no such room  ');

    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(track).toHaveBeenCalledWith('search', { search_term: 'no such room', results: 0, list: 'rooms' });
  });

  it('is not sent for an empty submit: that clears the search', async () => {
    render(<RoomListClient initialRooms={rooms(5)} />);
    submitSearch('   ');
    await waitFor(() => expect(api.fetchRooms).toHaveBeenCalledTimes(1));
    await act(async () => {});

    expect(track).not.toHaveBeenCalled();
  });

  it('is not sent when the search failed, and is sent once when Retry then works', async () => {
    vi.mocked(api.fetchRooms).mockRejectedValueOnce(new Error('down'));
    render(<RoomListClient initialRooms={rooms(5)} />);
    submitSearch('parser');

    const retry = await screen.findByRole('button', { name: /retry/i });
    expect(track).not.toHaveBeenCalled();

    fireEvent.click(retry);
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(track).toHaveBeenCalledWith('search', { search_term: 'parser', results: 3, list: 'rooms' });
  });
});

describe('sort_change on /rooms', () => {
  it('is sent once the list in the new order arrived', async () => {
    let answer: (value: { data: APIRoomWithStats[] }) => void = () => {};
    vi.mocked(api.fetchRooms).mockReturnValueOnce(new Promise((resolve) => (answer = resolve)));
    render(<RoomListClient initialRooms={rooms(5)} />);

    fireEvent.click(screen.getByRole('button', { name: /active now/i }));
    expect(api.fetchRooms).toHaveBeenCalledWith(expect.objectContaining({ sort: 'active', offset: 0 }));
    expect(track).not.toHaveBeenCalled();

    await act(async () => {
      answer({ data: rooms(2) });
    });
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('sort_change', { list: 'rooms', sort: 'active' });
  });

  it('is not sent for pressing the order already showing, nor when the list could not be loaded', async () => {
    render(<RoomListClient initialRooms={rooms(5)} />);
    fireEvent.click(screen.getByRole('button', { name: /^recent$/i }));
    expect(api.fetchRooms).not.toHaveBeenCalled();

    vi.mocked(api.fetchRooms).mockRejectedValueOnce(new Error('down'));
    fireEvent.click(screen.getByRole('button', { name: /active now/i }));
    await screen.findByRole('alert');
    expect(track).not.toHaveBeenCalled();
  });

  it('does not count as a new search while a term is showing', async () => {
    render(<RoomListClient initialRooms={rooms(5)} />);
    submitSearch('parser');
    await waitFor(() => expect(eventsNamed('search')).toHaveLength(1));

    fireEvent.click(screen.getByRole('button', { name: /active now/i }));
    await waitFor(() => expect(eventsNamed('sort_change')).toHaveLength(1));
    expect(api.fetchRooms).toHaveBeenLastCalledWith(expect.objectContaining({ q: 'parser', sort: 'active' }));
    expect(eventsNamed('search')).toHaveLength(1);
  });
});

describe('load_more on /rooms', () => {
  it('is sent with the page that was added, once it arrived', async () => {
    vi.mocked(api.fetchRooms).mockResolvedValueOnce({ data: rooms(20, 'second') });
    render(<RoomListClient initialRooms={rooms(20)} />);

    fireEvent.click(screen.getByRole('button', { name: /load more rooms/i }));
    await waitFor(() => expect(track).toHaveBeenCalledTimes(1));
    expect(api.fetchRooms).toHaveBeenCalledWith(expect.objectContaining({ offset: 20 }));
    expect(track).toHaveBeenCalledWith('load_more', { list: 'rooms', page: 2 });

    vi.mocked(api.fetchRooms).mockResolvedValueOnce({ data: rooms(4, 'third') });
    fireEvent.click(screen.getByRole('button', { name: /load more rooms/i }));
    await waitFor(() => expect(eventsNamed('load_more')).toHaveLength(2));
    expect(eventsNamed('load_more').map(([, params]) => params.page)).toEqual([2, 3]);
  });

  it('is not sent when the next page could not be loaded', async () => {
    vi.mocked(api.fetchRooms).mockRejectedValueOnce(new Error('down'));
    render(<RoomListClient initialRooms={rooms(20)} />);

    fireEvent.click(screen.getByRole('button', { name: /load more rooms/i }));
    await waitFor(() => expect(api.fetchRooms).toHaveBeenCalledTimes(1));
    await act(async () => {});
    expect(track).not.toHaveBeenCalled();
  });
});

describe('filter_change on /rooms', () => {
  it('says which rooms a signed-in person switched to', () => {
    render(<RoomsBrowser initialRooms={rooms(3)} />);
    expect(track).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: /my rooms/i }));
    expect(track).toHaveBeenCalledTimes(1);
    expect(track).toHaveBeenCalledWith('filter_change', { list: 'rooms', item: 'mine' });

    // Pressing the view already showing is not a change.
    fireEvent.click(screen.getByRole('button', { name: /my rooms/i }));
    fireEvent.click(screen.getByRole('button', { name: /public rooms/i }));
    expect(eventsNamed('filter_change').map(([, params]) => params.item)).toEqual(['mine', 'public']);
  });
});
