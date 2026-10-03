import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('@/lib/api', () => ({ api: { getRoomViewer: vi.fn() } }));

import { api } from '@/lib/api';
import { useRoomViewer } from './use-room-viewer';

// What the caller may do in a room is the API's answer (GET /v1/rooms/{slug}/viewer).
describe('useRoomViewer', () => {
  beforeEach(() => {
    vi.mocked(api.getRoomViewer).mockReset();
  });

  it('exposes the API answer', async () => {
    vi.mocked(api.getRoomViewer).mockResolvedValue({
      data: { can_pin: true, notifications: { available: true, subscribed: true, paused: false } },
    });
    const { result } = renderHook(() => useRoomViewer('ttt-room'));
    await waitFor(() => expect(result.current.canPin).toBe(true));
    expect(result.current.notifications).toEqual({ available: true, subscribed: true, paused: false });
    expect(api.getRoomViewer).toHaveBeenCalledWith('ttt-room');
  });

  it('offers nothing when the answer cannot be read', async () => {
    vi.mocked(api.getRoomViewer).mockRejectedValue(new Error('down'));
    const { result } = renderHook(() => useRoomViewer('ttt-room'));
    await waitFor(() => expect(api.getRoomViewer).toHaveBeenCalled());
    expect(result.current.canPin).toBe(false);
  });
});
