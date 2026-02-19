import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useIPFSHealth } from './use-ipfs-health';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getIPFSHealth: vi.fn(),
  },
}));

import { api } from '@/lib/api';

describe('useIPFSHealth', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('basic fetching', () => {
    it('starts with loading state', () => {
      vi.mocked(api.getIPFSHealth).mockReturnValue(new Promise(() => {})); // Never resolves
      const { result } = renderHook(() => useIPFSHealth({ pollIntervalMs: 0 }));
      expect(result.current.loading).toBe(true);
      expect(result.current.data).toBeNull();
      expect(result.current.error).toBeNull();
    });

    it('fetches IPFS health on mount', async () => {
      vi.mocked(api.getIPFSHealth).mockResolvedValue({
        connected: true,
        peer_id: '12D3KooWTest',
        version: 'kubo/0.39.0',
      });

      const { result } = renderHook(() => useIPFSHealth({ pollIntervalMs: 0 }));

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      expect(result.current.data).toEqual({
        connected: true,
        peer_id: '12D3KooWTest',
        version: 'kubo/0.39.0',
      });
      expect(result.current.error).toBeNull();
    });

    it('handles disconnected IPFS node', async () => {
      vi.mocked(api.getIPFSHealth).mockResolvedValue({
        connected: false,
        peer_id: '',
        version: '',
        error: 'timeout',
      });

      const { result } = renderHook(() => useIPFSHealth({ pollIntervalMs: 0 }));

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      expect(result.current.data).toEqual({
        connected: false,
        peer_id: '',
        version: '',
        error: 'timeout',
      });
    });

    it('handles fetch error', async () => {
      vi.mocked(api.getIPFSHealth).mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() => useIPFSHealth({ pollIntervalMs: 0 }));

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      expect(result.current.data).toBeNull();
      expect(result.current.error).toBe('Network error');
    });

    it('supports manual refetch', async () => {
      vi.mocked(api.getIPFSHealth)
        .mockResolvedValueOnce({
          connected: true,
          peer_id: '12D3KooWTest',
          version: 'kubo/0.39.0',
        })
        .mockResolvedValueOnce({
          connected: false,
          peer_id: '',
          version: '',
          error: 'disconnected',
        });

      const { result } = renderHook(() => useIPFSHealth({ pollIntervalMs: 0 }));

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      expect(result.current.data?.connected).toBe(true);

      await act(async () => {
        result.current.refetch();
      });

      await waitFor(() => {
        expect(result.current.data?.connected).toBe(false);
      });
    });
  });

  describe('polling', () => {
    beforeEach(() => {
      vi.useFakeTimers();
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it('polls at configured interval', async () => {
      vi.mocked(api.getIPFSHealth).mockResolvedValue({
        connected: true,
        peer_id: '12D3KooWTest',
        version: 'kubo/0.39.0',
      });

      renderHook(() => useIPFSHealth({ pollIntervalMs: 5000 }));

      // Initial fetch
      expect(api.getIPFSHealth).toHaveBeenCalledTimes(1);

      // Advance timer by poll interval
      await act(async () => {
        vi.advanceTimersByTime(5000);
      });

      expect(api.getIPFSHealth).toHaveBeenCalledTimes(2);

      // Another interval
      await act(async () => {
        vi.advanceTimersByTime(5000);
      });

      expect(api.getIPFSHealth).toHaveBeenCalledTimes(3);
    });

    it('stops polling on unmount', async () => {
      vi.mocked(api.getIPFSHealth).mockResolvedValue({
        connected: true,
        peer_id: '12D3KooWTest',
        version: 'kubo/0.39.0',
      });

      const { unmount } = renderHook(() => useIPFSHealth({ pollIntervalMs: 5000 }));

      expect(api.getIPFSHealth).toHaveBeenCalledTimes(1);
      unmount();

      await act(async () => {
        vi.advanceTimersByTime(10000);
      });

      // Should only have the initial call
      expect(api.getIPFSHealth).toHaveBeenCalledTimes(1);
    });

    it('disables polling when pollIntervalMs is 0', async () => {
      vi.mocked(api.getIPFSHealth).mockResolvedValue({
        connected: true,
        peer_id: '12D3KooWTest',
        version: 'kubo/0.39.0',
      });

      renderHook(() => useIPFSHealth({ pollIntervalMs: 0 }));

      // Only initial fetch
      expect(api.getIPFSHealth).toHaveBeenCalledTimes(1);

      await act(async () => {
        vi.advanceTimersByTime(60000);
      });

      // Still only initial fetch
      expect(api.getIPFSHealth).toHaveBeenCalledTimes(1);
    });
  });
});
