import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { RoomsBrowser } from './rooms-browser';
import { useAuth } from '@/hooks/use-auth';

vi.mock('@/hooks/use-auth', () => ({
  useAuth: vi.fn(),
}));

vi.mock('./room-list', () => ({
  RoomListClient: () => <div data-testid="public-list">public rooms</div>,
}));

vi.mock('./my-rooms-list', () => ({
  MyRoomsList: () => <div data-testid="my-list">my rooms</div>,
}));

describe('RoomsBrowser', () => {
  beforeEach(() => {
    vi.mocked(useAuth).mockReset();
  });

  it('shows only public rooms and no filter when logged out', () => {
    vi.mocked(useAuth).mockReturnValue({ isAuthenticated: false } as never);
    render(<RoomsBrowser initialRooms={[]} initialSort="recent" />);
    expect(screen.getByTestId('public-list')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /my rooms/i })).not.toBeInTheDocument();
    expect(screen.queryByTestId('my-list')).not.toBeInTheDocument();
  });

  it('offers a My rooms filter when signed in and switches views', () => {
    vi.mocked(useAuth).mockReturnValue({ isAuthenticated: true } as never);
    render(<RoomsBrowser initialRooms={[]} initialSort="recent" />);
    // Defaults to public.
    expect(screen.getByTestId('public-list')).toBeInTheDocument();
    expect(screen.queryByTestId('my-list')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /my rooms/i }));
    expect(screen.getByTestId('my-list')).toBeInTheDocument();
    expect(screen.queryByTestId('public-list')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /public rooms/i }));
    expect(screen.getByTestId('public-list')).toBeInTheDocument();
    expect(screen.queryByTestId('my-list')).not.toBeInTheDocument();
  });
});
