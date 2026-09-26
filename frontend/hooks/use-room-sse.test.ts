import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useRoomSse } from './use-room-sse';

vi.mock('@/lib/api', () => ({
  api: { createRoomStreamTicket: vi.fn() },
}));

import { api } from '@/lib/api';

// Minimal EventSource stub that records the URL it was constructed with.
let capturedUrls: string[] = [];
let sources: MockEventSource[] = [];
class MockEventSource {
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(url: string) {
    capturedUrls.push(url);
    sources.push(this);
  }
  addEventListener() {}
  close() {}
}

function ticketResponse(ticket: string) {
  return { data: { ticket, expires_at: '2026-09-25T00:00:00Z', ttl_seconds: 60, stream: '/v1/rooms/x/stream' } };
}

// idx 75 step 2 (was BART-156's ?access_token=): a browser EventSource cannot send an
// Authorization header, so a logged-in viewer opens the stream with a short-lived ticket
// the API mints, never with the long-lived JWT in the URL.
describe('useRoomSse — stream ticket', () => {
  const originalES = global.EventSource;
  beforeEach(() => {
    capturedUrls = [];
    sources = [];
    (global as unknown as { EventSource: unknown }).EventSource = MockEventSource;
    localStorage.clear();
    vi.mocked(api.createRoomStreamTicket).mockReset();
  });
  afterEach(() => {
    (global as unknown as { EventSource: unknown }).EventSource = originalES;
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('opens the stream with a ticket from the API and never puts the JWT in the URL', async () => {
    localStorage.setItem('auth_token', 'jwt-abc');
    vi.mocked(api.createRoomStreamTicket).mockResolvedValue(ticketResponse('solvr_st_ticket-1'));

    renderHook(() => useRoomSse('onvida-dev-20260706'));

    await waitFor(() => expect(capturedUrls.length).toBeGreaterThan(0));
    expect(api.createRoomStreamTicket).toHaveBeenCalledWith('onvida-dev-20260706');
    const url = new URL(capturedUrls[0]);
    expect(url.searchParams.get('ticket')).toBe('solvr_st_ticket-1');
    expect(url.searchParams.get('access_token')).toBeNull();
    expect(capturedUrls[0]).not.toContain('jwt-abc');
    expect(url.pathname).toContain('/v1/rooms/onvida-dev-20260706/stream');
  });

  it('opens the stream at once, with no ticket, for an anonymous (logged-out) viewer', () => {
    renderHook(() => useRoomSse('public-room'));
    expect(capturedUrls.length).toBeGreaterThan(0);
    const url = new URL(capturedUrls[0]);
    expect(url.searchParams.get('ticket')).toBeNull();
    expect(url.searchParams.get('access_token')).toBeNull();
    expect(api.createRoomStreamTicket).not.toHaveBeenCalled();
  });

  it('asks for a fresh ticket on every reconnect and resumes from the last event id', async () => {
    localStorage.setItem('auth_token', 'jwt-abc');
    vi.mocked(api.createRoomStreamTicket)
      .mockResolvedValueOnce(ticketResponse('solvr_st_first'))
      .mockResolvedValueOnce(ticketResponse('solvr_st_second'));
    renderHook(() => useRoomSse('room-a', 41));
    await waitFor(() => expect(capturedUrls.length).toBe(1));
    expect(new URL(capturedUrls[0]).searchParams.get('lastEventId')).toBe('41');

    vi.useFakeTimers();
    act(() => sources[0].onerror?.());
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3100);
    });

    expect(api.createRoomStreamTicket).toHaveBeenCalledTimes(2);
    expect(capturedUrls.length).toBe(2);
    const second = new URL(capturedUrls[1]);
    expect(second.searchParams.get('ticket')).toBe('solvr_st_second');
    expect(second.searchParams.get('lastEventId')).toBe('41');
  });

  it('retries instead of opening an unauthenticated stream when the ticket cannot be minted', async () => {
    localStorage.setItem('auth_token', 'jwt-abc');
    vi.mocked(api.createRoomStreamTicket).mockRejectedValue(new Error('403'));
    const { result } = renderHook(() => useRoomSse('private-room'));

    await waitFor(() => expect(result.current.status).toBe('reconnecting'));
    expect(capturedUrls).toEqual([]);
  });

  it('opens no stream when the viewer left before the ticket arrived', async () => {
    localStorage.setItem('auth_token', 'jwt-abc');
    let resolveTicket: (v: ReturnType<typeof ticketResponse>) => void = () => {};
    vi.mocked(api.createRoomStreamTicket).mockReturnValue(
      new Promise(resolve => {
        resolveTicket = resolve;
      }),
    );
    const { unmount } = renderHook(() => useRoomSse('room-a'));
    unmount();
    await act(async () => {
      resolveTicket(ticketResponse('solvr_st_late'));
    });
    expect(capturedUrls).toEqual([]);
  });
});
