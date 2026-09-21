import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useCollaborationExample } from './use-collaboration-example';
import { api } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  api: { getCollaborationExample: vi.fn() },
}));

const EXAMPLE = {
  kind: 'real' as const,
  state: 'completed',
  label: 'COMPLETED COLLABORATION',
  headline: 'A room',
  summary: 'A finished collaboration.',
  live_agent_count: 0,
  participants: [],
  steps: [],
  connect_url: '/connect?preset=planner-executor',
  connect_label: 'Try this workflow',
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe('useCollaborationExample', () => {
  it('returns whatever the API decided, untouched', async () => {
    vi.mocked(api.getCollaborationExample).mockResolvedValue({ data: EXAMPLE });

    const { result } = renderHook(() => useCollaborationExample());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.example).toEqual(EXAMPLE);
    expect(result.current.error).toBeNull();
    expect(api.getCollaborationExample).toHaveBeenCalledTimes(1);
  });

  it('reports a failed call instead of inventing an example', async () => {
    vi.mocked(api.getCollaborationExample).mockRejectedValue(new Error('network down'));

    const { result } = renderHook(() => useCollaborationExample());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.example).toBeNull();
    expect(result.current.error).toBe('network down');
  });

  it('drops a late answer after unmount instead of setting state', async () => {
    let resolve!: (v: { data: typeof EXAMPLE }) => void;
    vi.mocked(api.getCollaborationExample).mockReturnValue(
      new Promise((r) => {
        resolve = r;
      }),
    );

    const { result, unmount } = renderHook(() => useCollaborationExample());
    unmount();
    resolve({ data: EXAMPLE });
    await Promise.resolve();

    expect(result.current.example).toBeNull();
    expect(result.current.loading).toBe(true);
  });

  it('drops a late failure after unmount too', async () => {
    let reject!: (e: Error) => void;
    vi.mocked(api.getCollaborationExample).mockReturnValue(
      new Promise((_, r) => {
        reject = r;
      }),
    );

    const { result, unmount } = renderHook(() => useCollaborationExample());
    unmount();
    reject(new Error('too late'));
    await Promise.resolve();

    expect(result.current.error).toBeNull();
  });

  it('reports a non-Error rejection too', async () => {
    vi.mocked(api.getCollaborationExample).mockRejectedValue('boom');

    const { result } = renderHook(() => useCollaborationExample());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('Failed to fetch the example');
  });
});
