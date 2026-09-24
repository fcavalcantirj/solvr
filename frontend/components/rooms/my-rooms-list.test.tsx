import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MyRoomsList } from './my-rooms-list';
import { api } from '@/lib/api';
import type { APIRoom } from '@/lib/api-types';

vi.mock('next/link', () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => (
    <a href={href}>{children}</a>
  ),
}));

vi.mock('@/lib/api', () => ({
  api: {
    fetchMyRooms: vi.fn(),
  },
}));

const room = (over: Partial<APIRoom> = {}): APIRoom => ({
  id: 'r1',
  slug: 'my-room',
  display_name: 'My Room',
  tags: [],
  is_private: false,
  message_count: 5,
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-20T00:00:00Z',
  last_active_at: '2026-09-20T00:00:00Z',
  ...over,
});

describe('MyRoomsList', () => {
  beforeEach(() => {
    vi.mocked(api.fetchMyRooms).mockReset();
  });

  it('shows a loading state, then the rooms owned/participated by the caller', async () => {
    let resolve!: (v: { data: APIRoom[] }) => void;
    vi.mocked(api.fetchMyRooms).mockReturnValue(
      new Promise((r) => {
        resolve = r;
      }),
    );
    render(<MyRoomsList />);
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
    resolve({ data: [room({ id: 'a', slug: 'alpha', display_name: 'Alpha' }), room({ id: 'b', slug: 'beta', display_name: 'Beta', is_private: true })] });
    await waitFor(() => expect(screen.getByText('Alpha')).toBeInTheDocument());
    expect(screen.getByText('Alpha').closest('a')).toHaveAttribute('href', '/rooms/alpha');
    // Private rooms the caller owns are legitimately shown here.
    expect(screen.getByText('Beta')).toBeInTheDocument();
    expect(screen.getByText(/private/i)).toBeInTheDocument();
  });

  it('shows an empty state when the caller has no rooms', async () => {
    vi.mocked(api.fetchMyRooms).mockResolvedValue({ data: [] });
    render(<MyRoomsList />);
    await waitFor(() =>
      expect(screen.getByText(/don't own or participate in any rooms/i)).toBeInTheDocument(),
    );
  });

  it('shows an error with retry when the request fails', async () => {
    vi.mocked(api.fetchMyRooms).mockRejectedValueOnce(new Error('boom'));
    render(<MyRoomsList />);
    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());

    vi.mocked(api.fetchMyRooms).mockResolvedValueOnce({ data: [room({ display_name: 'Recovered' })] });
    fireEvent.click(screen.getByRole('button', { name: /retry/i }));
    await waitFor(() => expect(screen.getByText('Recovered')).toBeInTheDocument());
  });
});
