import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
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
});
