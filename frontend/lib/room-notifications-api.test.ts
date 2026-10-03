import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { api } from './api';

// Opt-in room notifications (idx 92): the client only calls the API's routes.
describe('room notification requests', () => {
  let fetchMock: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ data: {} }) });
    global.fetch = fetchMock as unknown as typeof global.fetch;
  });
  afterEach(() => vi.restoreAllMocks());

  const call = (i = 0) => ({ url: String(fetchMock.mock.calls[i][0]), init: fetchMock.mock.calls[i][1] as RequestInit });

  it('opts in and out of a room', async () => {
    await api.setRoomNotifications('ttt-room', true);
    await api.setRoomNotifications('ttt-room', false);
    expect(call(0).url).toMatch(/\/v1\/rooms\/ttt-room\/notifications$/);
    expect(call(0).init.method).toBe('PUT');
    expect(call(1).init.method).toBe('DELETE');
  });

  it('reads and sets the global switch', async () => {
    await api.getNotificationSettings();
    await api.updateNotificationSettings('paused');
    expect(call(0).url).toMatch(/\/v1\/me\/notification-settings$/);
    expect(call(1).init.method).toBe('PATCH');
    expect(call(1).init.body).toBe(JSON.stringify({ room_notifications: 'paused' }));
  });

  it('lists, reads one and reads all of the inbox', async () => {
    await api.listNotifications();
    await api.markNotificationRead('n-1');
    await api.markAllNotificationsRead();
    expect(call(0).url).toMatch(/\/v1\/notifications\?per_page=50$/);
    expect(call(1).url).toMatch(/\/v1\/notifications\/n-1\/read$/);
    expect(call(1).init.method).toBe('POST');
    expect(call(2).url).toMatch(/\/v1\/notifications\/read-all$/);
  });
});
