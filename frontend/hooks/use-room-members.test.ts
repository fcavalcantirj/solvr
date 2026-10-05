import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useRoomMembers } from './use-room-members';
import type { APIRoomMember } from '@/lib/api-types';

// Mock the api module
vi.mock('@/lib/api', () => ({
  api: {
    getMembers: vi.fn(),
  },
}));

const mockMembers: APIRoomMember[] = [
  {
    room_id: 'room-123',
    agent_id: 'agent-1',
    role: 'owner',
    added_by: 'system',
    created_at: '2026-09-20T00:00:00Z',
  },
  {
    room_id: 'room-123',
    agent_id: 'agent-2',
    role: 'member',
    added_by: 'agent-1',
    created_at: '2026-09-20T00:01:00Z',
  },
];

describe('useRoomMembers', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('fetches and returns members for a room', async () => {
    const { api } = await import('@/lib/api');
    vi.mocked(api.getMembers).mockResolvedValueOnce({ data: mockMembers });

    const { result } = renderHook(() => useRoomMembers('test-room'));

    expect(result.current.loading).toBe(true);
    expect(result.current.members).toEqual([]);

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.members).toEqual(mockMembers);
    expect(result.current.error).toBeNull();
  });

  it('handles fetch errors gracefully', async () => {
    const { api } = await import('@/lib/api');
    const mockError = new Error('Network error');
    vi.mocked(api.getMembers).mockRejectedValueOnce(mockError);

    const { result } = renderHook(() => useRoomMembers('test-room'));

    expect(result.current.loading).toBe(true);

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.members).toEqual([]);
    expect(result.current.error).toEqual(mockError);
  });

  it('refetches when slug changes', async () => {
    const { api } = await import('@/lib/api');
    vi.mocked(api.getMembers).mockResolvedValueOnce({ data: mockMembers });

    const { result, rerender } = renderHook(
      ({ slug }: { slug: string }) => useRoomMembers(slug),
      { initialProps: { slug: 'room-1' } }
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(vi.mocked(api.getMembers)).toHaveBeenCalledWith('room-1');

    // Change slug and verify it refetches
    vi.mocked(api.getMembers).mockResolvedValueOnce({ data: [] });
    rerender({ slug: 'room-2' });

    await waitFor(() => {
      expect(vi.mocked(api.getMembers)).toHaveBeenCalledWith('room-2');
    });
  });

  // The member list is an owner-only read (it answers 401 to an anonymous caller), so
  // a public room page must not ask for it on behalf of a visitor who is not signed in.
  describe('enabled option', () => {
    it('makes no request while it is disabled', async () => {
      const { api } = await import('@/lib/api');

      const { result } = renderHook(() => useRoomMembers('test-room', { enabled: false }));

      // Give a stray request the chance to go out before asserting there was none.
      await new Promise((resolve) => setTimeout(resolve, 20));
      expect(vi.mocked(api.getMembers)).not.toHaveBeenCalled();
      expect(result.current.members).toEqual([]);
      expect(result.current.loading).toBe(false);
      expect(result.current.error).toBeNull();
    });

    it('makes no request for a room it was not given', async () => {
      const { api } = await import('@/lib/api');

      const { result } = renderHook(() => useRoomMembers(''));

      await new Promise((resolve) => setTimeout(resolve, 20));
      expect(vi.mocked(api.getMembers)).not.toHaveBeenCalled();
      expect(result.current.loading).toBe(false);
    });

    it('reads the members once it is enabled', async () => {
      const { api } = await import('@/lib/api');
      vi.mocked(api.getMembers).mockResolvedValueOnce({ data: mockMembers });

      const { result, rerender } = renderHook(
        ({ enabled }: { enabled: boolean }) => useRoomMembers('test-room', { enabled }),
        { initialProps: { enabled: false } }
      );
      expect(vi.mocked(api.getMembers)).not.toHaveBeenCalled();

      rerender({ enabled: true });

      await waitFor(() => {
        expect(result.current.members).toEqual(mockMembers);
      });
      expect(vi.mocked(api.getMembers)).toHaveBeenCalledTimes(1);
      expect(vi.mocked(api.getMembers)).toHaveBeenCalledWith('test-room');
    });

    it('forgets the members when it is disabled again, even if an answer arrives late', async () => {
      const { api } = await import('@/lib/api');
      let answer: (value: { data: APIRoomMember[] }) => void = () => {};
      vi.mocked(api.getMembers).mockReturnValueOnce(
        new Promise((resolve) => {
          answer = resolve;
        })
      );

      const { result, rerender } = renderHook(
        ({ enabled }: { enabled: boolean }) => useRoomMembers('test-room', { enabled }),
        { initialProps: { enabled: true } }
      );
      await waitFor(() => expect(vi.mocked(api.getMembers)).toHaveBeenCalledTimes(1));

      // The viewer signs out while the read is still in flight.
      rerender({ enabled: false });
      await act(async () => {
        answer({ data: mockMembers });
      });

      expect(result.current.members).toEqual([]);
      expect(result.current.loading).toBe(false);
    });
  });
});
