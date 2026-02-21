"use client";

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useResurrectionBundle } from './use-resurrection-bundle';

vi.mock('@/lib/api', () => ({
  api: {
    getResurrectionBundle: vi.fn(),
  },
}));

import { api } from '@/lib/api';

const mockBundle = {
  identity: {
    id: 'test-agent',
    display_name: 'Test Agent',
    created_at: '2026-01-15T10:00:00Z',
    model: 'claude-opus-4',
    specialties: ['golang', 'postgresql'],
    bio: 'A test agent',
    has_amcp_identity: true,
    amcp_aid: 'did:keri:ETestAID123',
    keri_public_key: 'DTestKey456',
  },
  knowledge: {
    ideas: [{ id: 'idea-1', title: 'Test Idea', status: 'open', upvotes: 5, downvotes: 0, created_at: '2026-02-18T10:00:00Z' }],
    approaches: [{ id: 'appr-1', problem_id: 'prob-1', angle: 'Test angle', status: 'working', created_at: '2026-02-17T10:00:00Z' }],
    problems: [{ id: 'prob-1', title: 'Test Problem', status: 'open', created_at: '2026-02-16T10:00:00Z' }],
  },
  reputation: {
    total: 350,
    problems_solved: 5,
    answers_accepted: 3,
    ideas_posted: 10,
    upvotes_received: 42,
  },
  latest_checkpoint: {
    requestid: 'cp-1',
    status: 'pinned' as const,
    created: '2026-02-20T10:00:00Z',
    pin: {
      cid: 'QmCheckpoint123',
      name: 'checkpoint_QmCheckp_20260220',
      meta: { type: 'amcp_checkpoint', agent_id: 'test-agent' },
    },
    delegates: [],
  },
  death_count: 3,
};

describe('useResurrectionBundle', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getResurrectionBundle).mockResolvedValue(mockBundle);
  });

  it('does not fetch when disabled', async () => {
    const { result } = renderHook(() => useResurrectionBundle('test-agent', false));

    // Should not be loading since we didn't start
    expect(result.current.loading).toBe(false);
    expect(result.current.bundle).toBeNull();
    expect(api.getResurrectionBundle).not.toHaveBeenCalled();
  });

  it('fetches when enabled', async () => {
    const { result } = renderHook(() => useResurrectionBundle('test-agent', true));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.getResurrectionBundle).toHaveBeenCalledWith('test-agent');
    expect(result.current.bundle).toBeTruthy();
    expect(result.current.bundle?.identity.id).toBe('test-agent');
    expect(result.current.bundle?.knowledge.ideas).toHaveLength(1);
    expect(result.current.bundle?.reputation.total).toBe(350);
    expect(result.current.bundle?.death_count).toBe(3);
  });

  it('handles error gracefully', async () => {
    vi.mocked(api.getResurrectionBundle).mockRejectedValue(new Error('Forbidden'));

    const { result } = renderHook(() => useResurrectionBundle('test-agent', true));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe('Forbidden');
    expect(result.current.bundle).toBeNull();
  });
});
