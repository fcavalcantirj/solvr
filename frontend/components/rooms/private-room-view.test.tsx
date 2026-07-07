import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import React from 'react';
import { PrivateRoomView } from './private-room-view';

const mockUseAuth = vi.fn();
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockUseAuth(),
}));

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const mockFetchRoom = vi.fn();
vi.mock('@/lib/api', () => ({
  api: { fetchRoom: (slug: string) => mockFetchRoom(slug) },
}));

// Stub the heavy RoomDetailClient (pulls in SSE etc.) — assert we hand off to it.
vi.mock('./room-detail-client', () => ({
  RoomDetailClient: ({ room }: { room: { slug: string } }) => (
    <div data-testid="room-detail">{room.slug}</div>
  ),
}));

describe('PrivateRoomView', () => {
  beforeEach(() => vi.clearAllMocks());

  it('prompts login when unauthenticated', () => {
    mockUseAuth.mockReturnValue({ user: null, isAuthenticated: false, isLoading: false });
    render(<PrivateRoomView slug="onvida-dev-20260706" />);
    const link = screen.getByRole('link', { name: /log in/i });
    expect(link).toHaveAttribute('href', '/login');
    expect(mockFetchRoom).not.toHaveBeenCalled();
  });

  it('fetches with JWT and renders RoomDetailClient when authenticated', async () => {
    mockUseAuth.mockReturnValue({ user: { id: 'u1' }, isAuthenticated: true, isLoading: false });
    mockFetchRoom.mockResolvedValue({
      data: { room: { slug: 'onvida-dev-20260706' }, agents: [], recent_messages: [], owner_display_name: 'Felipe' },
    });
    render(<PrivateRoomView slug="onvida-dev-20260706" />);
    expect(await screen.findByTestId('room-detail')).toHaveTextContent('onvida-dev-20260706');
    expect(mockFetchRoom).toHaveBeenCalledWith('onvida-dev-20260706');
  });

  it('shows a not-a-member notice on 403', async () => {
    mockUseAuth.mockReturnValue({ user: { id: 'u1' }, isAuthenticated: true, isLoading: false });
    mockFetchRoom.mockRejectedValue({ statusCode: 403 });
    render(<PrivateRoomView slug="secret-room" />);
    await waitFor(() => expect(screen.getByText(/not a member/i)).toBeInTheDocument());
  });
});
