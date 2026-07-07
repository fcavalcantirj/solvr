import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useRoomSse } from './use-room-sse';

// Minimal EventSource stub that records the URL it was constructed with.
let capturedUrls: string[] = [];
class MockEventSource {
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(url: string) {
    capturedUrls.push(url);
  }
  addEventListener() {}
  close() {}
}

describe('useRoomSse — access_token (BART-156)', () => {
  const originalES = global.EventSource;
  beforeEach(() => {
    capturedUrls = [];
    (global as unknown as { EventSource: unknown }).EventSource = MockEventSource;
    localStorage.clear();
  });
  afterEach(() => {
    (global as unknown as { EventSource: unknown }).EventSource = originalES;
    vi.restoreAllMocks();
  });

  it('appends the JWT as ?access_token when a token is in localStorage', () => {
    localStorage.setItem('auth_token', 'jwt-abc');
    renderHook(() => useRoomSse('onvida-dev-20260706'));
    expect(capturedUrls.length).toBeGreaterThan(0);
    const url = new URL(capturedUrls[0]);
    expect(url.searchParams.get('access_token')).toBe('jwt-abc');
    expect(url.pathname).toContain('/v1/rooms/onvida-dev-20260706/stream');
  });

  it('omits access_token for an anonymous (logged-out) viewer', () => {
    renderHook(() => useRoomSse('public-room'));
    expect(capturedUrls.length).toBeGreaterThan(0);
    const url = new URL(capturedUrls[0]);
    expect(url.searchParams.get('access_token')).toBeNull();
  });
});
