"use client";

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useCheckpoints } from './use-checkpoints';

vi.mock('@/lib/api', () => ({
  api: {
    getAgentCheckpoints: vi.fn(),
  },
}));

import { api } from '@/lib/api';

const mockCheckpointsResponse = {
  count: 2,
  results: [
    {
      requestid: 'cp-1',
      status: 'pinned' as const,
      created: '2026-02-20T10:00:00Z',
      pin: {
        cid: 'QmLatestCheckpoint123',
        name: 'checkpoint_QmLatest_20260220',
        meta: { type: 'amcp_checkpoint', agent_id: 'test-agent', death_count: '3' },
      },
      delegates: [],
    },
    {
      requestid: 'cp-2',
      status: 'pinned' as const,
      created: '2026-02-19T10:00:00Z',
      pin: {
        cid: 'QmOlderCheckpoint456',
        name: 'checkpoint_QmOlder_20260219',
        meta: { type: 'amcp_checkpoint', agent_id: 'test-agent' },
      },
      delegates: [],
    },
  ],
  latest: {
    requestid: 'cp-1',
    status: 'pinned' as const,
    created: '2026-02-20T10:00:00Z',
    pin: {
      cid: 'QmLatestCheckpoint123',
      name: 'checkpoint_QmLatest_20260220',
      meta: { type: 'amcp_checkpoint', agent_id: 'test-agent', death_count: '3' },
    },
    delegates: [],
  },
};

describe('useCheckpoints', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getAgentCheckpoints).mockResolvedValue(mockCheckpointsResponse);
  });

  it('fetches checkpoints on mount', async () => {
    const { result } = renderHook(() => useCheckpoints('test-agent'));

    expect(result.current.loading).toBe(true);

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getAgentCheckpoints).toHaveBeenCalledWith('test-agent');
    expect(result.current.checkpoints).toHaveLength(2);
    expect(result.current.count).toBe(2);
    expect(result.current.latest).toBeTruthy();
    expect(result.current.latest?.pin.cid).toBe('QmLatestCheckpoint123');
  });

  it('handles error gracefully', async () => {
    vi.mocked(api.getAgentCheckpoints).mockRejectedValue(new Error('Forbidden'));

    const { result } = renderHook(() => useCheckpoints('test-agent'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe('Forbidden');
    expect(result.current.checkpoints).toHaveLength(0);
    expect(result.current.latest).toBeNull();
    expect(result.current.count).toBe(0);
  });

  it('returns empty state for no checkpoints', async () => {
    vi.mocked(api.getAgentCheckpoints).mockResolvedValue({
      count: 0,
      results: [],
      latest: null,
    });

    const { result } = renderHook(() => useCheckpoints('test-agent'));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.checkpoints).toHaveLength(0);
    expect(result.current.latest).toBeNull();
    expect(result.current.count).toBe(0);
    expect(result.current.error).toBeNull();
  });
});
