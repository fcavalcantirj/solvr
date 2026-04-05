import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { CreateRoomDialog } from './create-room-dialog';

// Mock auth
const mockAuth: { user: { id: string; type: string; displayName: string } | null; isAuthenticated: boolean; setShowAuthModal: ReturnType<typeof vi.fn> } = { user: null, isAuthenticated: false, setShowAuthModal: vi.fn() };
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockAuth,
}));

// Mock api
vi.mock('@/lib/api', () => ({
  api: {
    createRoom: vi.fn(),
  },
}));

// Mock next/navigation
const mockPush = vi.fn();
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
}));

import { api } from '@/lib/api';

describe('CreateRoomDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.user = null;
    mockAuth.isAuthenticated = false;
  });

  it('shows auth modal when unauthenticated user clicks create', () => {
    render(<CreateRoomDialog />);
    const button = screen.getByRole('button', { name: /create room/i });
    fireEvent.click(button);
    expect(mockAuth.setShowAuthModal).toHaveBeenCalledWith(true);
  });

  it('opens dialog when authenticated user clicks create', async () => {
    mockAuth.user = { id: 'u1', type: 'human', displayName: 'Test' };
    mockAuth.isAuthenticated = true;
    render(<CreateRoomDialog />);

    fireEvent.click(screen.getByRole('button', { name: /create room/i }));
    expect(screen.getByText(/room name/i)).toBeInTheDocument();
  });

  it('disables submit when name is empty', async () => {
    mockAuth.user = { id: 'u1', type: 'human', displayName: 'Test' };
    mockAuth.isAuthenticated = true;
    render(<CreateRoomDialog />);

    fireEvent.click(screen.getByRole('button', { name: /create room/i }));
    const submit = screen.getByRole('button', { name: /^create$/i });
    expect(submit).toBeDisabled();
  });

  it('calls api.createRoom and navigates on success', async () => {
    mockAuth.user = { id: 'u1', type: 'human', displayName: 'Test' };
    mockAuth.isAuthenticated = true;

    vi.mocked(api.createRoom).mockResolvedValue({
      data: { slug: 'my-new-room', id: 'room-1', display_name: 'My New Room' }, token: 'tok_123',
    });

    render(<CreateRoomDialog />);
    fireEvent.click(screen.getByRole('button', { name: /create room/i }));

    const nameInput = screen.getByPlaceholderText(/room name/i);
    fireEvent.change(nameInput, { target: { value: 'My New Room' } });

    const submit = screen.getByRole('button', { name: /^create$/i });
    await act(async () => {
      fireEvent.click(submit);
    });

    await waitFor(() => {
      expect(api.createRoom).toHaveBeenCalledWith(
        expect.objectContaining({ display_name: 'My New Room' })
      );
    });

    // Success dialog should appear with "Room Created"
    await waitFor(() => {
      expect(screen.getByText(/room created/i)).toBeInTheDocument();
    });

    // Click "Go to Room" to navigate
    const goButton = screen.getByRole('button', { name: /go to room/i });
    fireEvent.click(goButton);
    expect(mockPush).toHaveBeenCalledWith('/rooms/my-new-room');
  });

  it('shows error on API failure', async () => {
    mockAuth.user = { id: 'u1', type: 'human', displayName: 'Test' };
    mockAuth.isAuthenticated = true;

    vi.mocked(api.createRoom).mockRejectedValue(new Error('Server error'));

    render(<CreateRoomDialog />);
    fireEvent.click(screen.getByRole('button', { name: /create room/i }));

    const nameInput = screen.getByPlaceholderText(/room name/i);
    fireEvent.change(nameInput, { target: { value: 'Test Room' } });

    const submit = screen.getByRole('button', { name: /^create$/i });
    await act(async () => {
      fireEvent.click(submit);
    });

    await waitFor(() => {
      expect(screen.getByText(/failed to create room/i)).toBeInTheDocument();
    });
  });
});
