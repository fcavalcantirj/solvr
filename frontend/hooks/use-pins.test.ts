"use client";

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { usePins } from './use-pins';

// Mock the api module
vi.mock('@/lib/api', () => ({
  api: {
    listPins: vi.fn(),
    createPin: vi.fn(),
    deletePin: vi.fn(),
    getStorageUsage: vi.fn(),
  },
}));

import { api } from '@/lib/api';

const mockPinResponse = {
  requestid: 'pin-1',
  status: 'pinned' as const,
  created: '2026-02-18T10:00:00Z',
  pin: {
    cid: 'QmTest123456789abcdef',
    name: 'test-file',
  },
  delegates: [],
  info: { size_bytes: 1024 },
};

const mockPinsListResponse = {
  count: 2,
  results: [
    mockPinResponse,
    {
      requestid: 'pin-2',
      status: 'queued' as const,
      created: '2026-02-18T11:00:00Z',
      pin: {
        cid: 'bafyTestCID123',
      },
      delegates: [],
    },
  ],
};

const mockStorageResponse = {
  data: {
    used: 1048576, // 1 MB
    quota: 1073741824, // 1 GB
    percentage: 0.1,
  },
};

describe('usePins', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.listPins).mockResolvedValue(mockPinsListResponse);
    vi.mocked(api.getStorageUsage).mockResolvedValue(mockStorageResponse);
  });

  it('fetches pins and storage on mount', async () => {
    const { result } = renderHook(() => usePins());

    expect(result.current.loading).toBe(true);

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.listPins).toHaveBeenCalledTimes(1);
    expect(api.getStorageUsage).toHaveBeenCalledTimes(1);
    expect(result.current.pins).toHaveLength(2);
    expect(result.current.pins[0].requestid).toBe('pin-1');
    expect(result.current.totalCount).toBe(2);
    expect(result.current.storage).toEqual(mockStorageResponse.data);
  });

  it('handles loading state correctly', async () => {
    const { result } = renderHook(() => usePins());

    expect(result.current.loading).toBe(true);
    expect(result.current.error).toBeNull();

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
  });

  it('handles error when fetching pins fails', async () => {
    vi.mocked(api.listPins).mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => usePins());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.error).toBe('Network error');
    expect(result.current.pins).toHaveLength(0);
  });

  it('filters pins by status', async () => {
    const { result } = renderHook(() => usePins({ status: 'pinned' }));

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.listPins).toHaveBeenCalledWith({ status: 'pinned', limit: 100 });
  });

  it('creates a pin and refreshes the list', async () => {
    vi.mocked(api.createPin).mockResolvedValue(mockPinResponse);

    const { result } = renderHook(() => usePins());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await act(async () => {
      await result.current.createPin('QmNewCID', 'my-pin');
    });

    expect(api.createPin).toHaveBeenCalledWith('QmNewCID', 'my-pin');
    // Should refetch after creating
    expect(api.listPins).toHaveBeenCalledTimes(2);
  });

  it('deletes a pin and refreshes the list', async () => {
    vi.mocked(api.deletePin).mockResolvedValue(undefined);

    const { result } = renderHook(() => usePins());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await act(async () => {
      await result.current.deletePin('pin-1');
    });

    expect(api.deletePin).toHaveBeenCalledWith('pin-1');
    // Should refetch after deleting
    expect(api.listPins).toHaveBeenCalledTimes(2);
  });

  it('handles create pin error', async () => {
    vi.mocked(api.createPin).mockRejectedValue(new Error('CID invalid'));

    const { result } = renderHook(() => usePins());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await expect(
      act(async () => {
        await result.current.createPin('bad-cid');
      })
    ).rejects.toThrow('CID invalid');
  });

  it('refetches pins when called', async () => {
    const { result } = renderHook(() => usePins());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await act(async () => {
      result.current.refetch();
    });

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(api.listPins).toHaveBeenCalledTimes(2);
  });

  it('handles storage fetch error gracefully', async () => {
    vi.mocked(api.getStorageUsage).mockRejectedValue(new Error('Forbidden'));

    const { result } = renderHook(() => usePins());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    // Pins should still load even if storage fails
    expect(result.current.pins).toHaveLength(2);
    expect(result.current.storage).toBeNull();
  });

  it('returns empty pins when API returns empty results', async () => {
    vi.mocked(api.listPins).mockResolvedValue({ count: 0, results: [] });

    const { result } = renderHook(() => usePins());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.pins).toHaveLength(0);
    expect(result.current.totalCount).toBe(0);
  });
});
