import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('@/lib/api', () => ({ api: { setRoomNotifications: vi.fn() } }));

import { api } from '@/lib/api';
import { RoomNotifyToggle } from './room-notify-toggle';

// The per-room opt-in (idx 92): rendered only when the API says it is available to this
// viewer, it shows the API's state and changes it on request.
beforeEach(() => vi.mocked(api.setRoomNotifications).mockReset());

const state = (over = {}) => ({ available: true, subscribed: false, paused: false, ...over });

describe('RoomNotifyToggle', () => {
  it('renders nothing when notifications are not available to this viewer', () => {
    const { container } = render(<RoomNotifyToggle slug="r" initial={state({ available: false })} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('opts in, then shows the state the API answered', async () => {
    vi.mocked(api.setRoomNotifications).mockResolvedValue({
      data: { subscribed: true, paused: false, events: ['room.reply', 'room.review_requested'], off: 'DELETE /v1/rooms/r/notifications' },
    });
    render(<RoomNotifyToggle slug="r" initial={state()} />);
    fireEvent.click(screen.getByRole('button', { name: /notify me about replies/i }));
    await waitFor(() => expect(api.setRoomNotifications).toHaveBeenCalledWith('r', true));
    expect(await screen.findByRole('button', { name: /turn off room notifications/i })).toBeInTheDocument();
  });

  it('says when every room is paused', () => {
    render(<RoomNotifyToggle slug="r" initial={state({ subscribed: true, paused: true })} />);
    expect(screen.getByText(/paused for every room/i)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /notifications/i })).toHaveAttribute('href', '/notifications');
  });
});
