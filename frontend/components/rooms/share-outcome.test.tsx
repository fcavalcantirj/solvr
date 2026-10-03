import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('@/lib/api', () => ({ api: { getRoomShare: vi.fn(), postFunnelEvent: vi.fn() } }));

import { api } from '@/lib/api';
import { ShareOutcome } from './share-outcome';

// "Copy outcome" (idx 88 step 3): the API composes the excerpt and links; the browser
// shows them and copies them on request. Nothing is ever posted anywhere.

const SHARE = {
  data: {
    room_url: 'https://solvr.dev/rooms/ttt-room',
    share_url: 'https://solvr.dev/rooms/ttt-room?via=share',
    try_url: 'https://solvr.dev/connect?from_room=ttt-room',
    excerpt: { title: 'Tic-tac-toe', text: 'Shipped a scoreboard.', source: 'result' },
    copy_text: 'Tic-tac-toe\nShipped a scoreboard.\nhttps://solvr.dev/rooms/ttt-room?via=share',
    note: 'Copy only — Solvr never posts this anywhere.',
  },
};

beforeEach(() => {
  vi.mocked(api.getRoomShare).mockReset();
  vi.mocked(api.postFunnelEvent).mockReset();
  Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
});

describe('ShareOutcome', () => {
  it('reads the share contract only when opened and shows it', async () => {
    vi.mocked(api.getRoomShare).mockResolvedValue(SHARE);
    render(<ShareOutcome slug="ttt-room" />);
    expect(api.getRoomShare).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: /copy outcome/i }));
    expect(await screen.findByText('Shipped a scoreboard.')).toBeInTheDocument();
    expect(api.getRoomShare).toHaveBeenCalledWith('ttt-room');
    expect(screen.getByText(/never posts this anywhere/i)).toBeInTheDocument();
    expect(screen.getByTestId('share-outcome-text')).toHaveTextContent('https://solvr.dev/rooms/ttt-room?via=share');
  });

  it('copies the composed text and only then reports share_link_copied', async () => {
    vi.mocked(api.getRoomShare).mockResolvedValue(SHARE);
    render(<ShareOutcome slug="ttt-room" />);
    fireEvent.click(screen.getByRole('button', { name: /copy outcome/i }));
    fireEvent.click(await screen.findByRole('button', { name: /^copy$/i }));

    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith(SHARE.data.copy_text));
    await waitFor(() =>
      expect(api.postFunnelEvent).toHaveBeenCalledWith({
        event: 'share_link_copied',
        entry_surface: 'room_page',
        source: { kind: 'room', ref: 'ttt-room' },
      }),
    );
  });

  it('reports nothing when the clipboard refuses', async () => {
    vi.mocked(api.getRoomShare).mockResolvedValue(SHARE);
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) } });
    render(<ShareOutcome slug="ttt-room" />);
    fireEvent.click(screen.getByRole('button', { name: /copy outcome/i }));
    fireEvent.click(await screen.findByRole('button', { name: /^copy$/i }));
    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalled());
    expect(api.postFunnelEvent).not.toHaveBeenCalled();
  });

  it('says so when the share contract cannot be read', async () => {
    vi.mocked(api.getRoomShare).mockRejectedValue(new Error('boom'));
    render(<ShareOutcome slug="ttt-room" />);
    fireEvent.click(screen.getByRole('button', { name: /copy outcome/i }));
    expect(await screen.findByRole('alert')).toBeInTheDocument();
  });
});
