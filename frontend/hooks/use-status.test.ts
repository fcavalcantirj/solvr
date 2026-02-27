import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { useStatus } from './use-status';

vi.mock('@/lib/api', () => ({
  api: {
    getStatus: vi.fn(),
  },
}));

import { api } from '@/lib/api';

const mockStatusData = {
  data: {
    overall_status: 'operational' as const,
    services: [
      {
        category: 'Core Services',
        items: [
          {
            name: 'REST API',
            description: 'Primary API endpoints',
            status: 'operational' as const,
            uptime: '99.98%',
            latency_ms: 45,
            last_checked: '2026-02-27T12:00:00Z',
          },
          {
            name: 'PostgreSQL',
            description: 'PostgreSQL data store',
            status: 'operational' as const,
            uptime: '99.98%',
            latency_ms: 8,
            last_checked: '2026-02-27T12:00:00Z',
          },
        ],
      },
      {
        category: 'Storage',
        items: [
          {
            name: 'IPFS Node',
            description: 'Decentralized content storage (Kubo)',
            status: 'operational' as const,
            uptime: '99.98%',
            latency_ms: 65,
            last_checked: '2026-02-27T12:00:00Z',
          },
        ],
      },
    ],
    summary: {
      uptime_30d: 99.97,
      avg_response_time_ms: 39.33,
      service_count: 3,
      last_checked: '2026-02-27T12:00:00Z',
    },
    uptime_history: [
      { date: '2026-02-27', status: 'operational' as const },
      { date: '2026-02-26', status: 'operational' as const },
    ],
    incidents: [],
  },
};

describe('useStatus', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('starts with loading state', () => {
    vi.mocked(api.getStatus).mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useStatus({ pollIntervalMs: 0 }));

    expect(result.current.loading).toBe(true);
    expect(result.current.data).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it('fetches status on mount', async () => {
    vi.mocked(api.getStatus).mockResolvedValue(mockStatusData);

    const { result } = renderHook(() => useStatus({ pollIntervalMs: 0 }));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.data).toEqual(mockStatusData.data);
    expect(result.current.error).toBeNull();
  });

  it('handles fetch errors', async () => {
    vi.mocked(api.getStatus).mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useStatus({ pollIntervalMs: 0 }));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.data).toBeNull();
    expect(result.current.error).toBe('Network error');
  });

  it('handles non-Error rejections', async () => {
    vi.mocked(api.getStatus).mockRejectedValue('string error');

    const { result } = renderHook(() => useStatus({ pollIntervalMs: 0 }));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe('Failed to fetch status');
  });

  it('provides refetch function', async () => {
    vi.mocked(api.getStatus).mockResolvedValue(mockStatusData);

    const { result } = renderHook(() => useStatus({ pollIntervalMs: 0 }));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(typeof result.current.refetch).toBe('function');
  });
});
