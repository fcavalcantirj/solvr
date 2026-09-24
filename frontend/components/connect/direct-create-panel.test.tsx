import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { DirectCreatePanel } from './direct-create-panel';

// The direct-create panel is the SECONDARY, signed-in-only path on /connect:
// a human who prefers to make the room themselves instead of pasting the
// planner prompt into an agent. Logged-out visitors keep the prompt-first flow
// and never see this form (task: "Retain a fast direct-create option for
// authenticated humans").

const mockAuth: { isAuthenticated: boolean } = { isAuthenticated: false };
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => mockAuth,
}));

vi.mock('@/lib/api', () => ({
  api: {
    createRoom: vi.fn(),
  },
}));

const mockPush = vi.fn();
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush }),
}));

import { api } from '@/lib/api';

describe('DirectCreatePanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.isAuthenticated = false;
  });

  it('renders nothing for logged-out visitors (they keep the prompt-first path)', () => {
    const { container } = render(<DirectCreatePanel />);
    expect(container).toBeEmptyDOMElement();
    expect(screen.queryByTestId('direct-create-toggle')).toBeNull();
  });

  it('offers a secondary Create here action and a minimal form when signed in', () => {
    mockAuth.isAuthenticated = true;
    render(<DirectCreatePanel />);

    const toggle = screen.getByTestId('direct-create-toggle');
    expect(toggle).toBeInTheDocument();
    fireEvent.click(toggle);

    // Only a short purpose/name and a visibility choice are asked up front.
    expect(screen.getByTestId('direct-create-name')).toBeInTheDocument();
    const publicRadio = screen.getByTestId('direct-create-visibility-public') as HTMLInputElement;
    const privateRadio = screen.getByTestId('direct-create-visibility-private') as HTMLInputElement;
    expect(publicRadio.checked).toBe(true);
    expect(privateRadio.checked).toBe(false);

    // Extra metadata lives under an Optional settings disclosure, not up front.
    const optional = screen.getByTestId('direct-create-optional');
    expect(optional.tagName.toLowerCase()).toBe('details');
    expect(optional).not.toHaveAttribute('open');
  });

  it('creates the room with the chosen visibility and lands on its room page', async () => {
    mockAuth.isAuthenticated = true;
    vi.mocked(api.createRoom).mockResolvedValue({
      data: { slug: 'debug-the-parser', id: 'room-1', display_name: 'Debug the parser' },
    });

    render(<DirectCreatePanel />);
    fireEvent.click(screen.getByTestId('direct-create-toggle'));

    fireEvent.change(screen.getByTestId('direct-create-name'), {
      target: { value: 'Debug the parser' },
    });
    fireEvent.click(screen.getByTestId('direct-create-visibility-private'));

    await act(async () => {
      fireEvent.click(screen.getByTestId('direct-create-submit'));
    });

    await waitFor(() => {
      expect(api.createRoom).toHaveBeenCalledWith(
        expect.objectContaining({ display_name: 'Debug the parser', is_private: true }),
      );
    });
    // Lands on the new room's page, flagged so it can show the starter prompts.
    expect(mockPush).toHaveBeenCalledWith('/rooms/debug-the-parser?created=1');
  });

  it('keeps a validation error next to the form and does not duplicate the room on retry', async () => {
    mockAuth.isAuthenticated = true;
    // A retry after a lost response hits the deterministic-slug uniqueness and
    // comes back 409 — no second room is created, and the error stays by the form.
    vi.mocked(api.createRoom).mockRejectedValue({ status: 409 });

    render(<DirectCreatePanel />);
    fireEvent.click(screen.getByTestId('direct-create-toggle'));
    fireEvent.change(screen.getByTestId('direct-create-name'), {
      target: { value: 'Debug the parser' },
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId('direct-create-submit'));
    });

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
    expect(screen.getByRole('alert').textContent?.toLowerCase()).toContain('already exists');
    expect(mockPush).not.toHaveBeenCalled();
  });
});
