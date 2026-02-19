"use client";

import { useState, useEffect, useCallback, useRef } from 'react';
import { api } from '@/lib/api';
import type { APIIPFSHealthResponse } from '@/lib/api-types';

export interface UseIPFSHealthOptions {
  pollIntervalMs?: number;
}

export interface UseIPFSHealthResult {
  data: APIIPFSHealthResponse | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

const DEFAULT_POLL_INTERVAL = 30000; // 30 seconds

export function useIPFSHealth(
  options?: UseIPFSHealthOptions
): UseIPFSHealthResult {
  const [data, setData] = useState<APIIPFSHealthResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const pollInterval = options?.pollIntervalMs ?? DEFAULT_POLL_INTERVAL;

  const fetchHealth = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await api.getIPFSHealth();
      setData(response);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch IPFS health');
      setData(null);
    } finally {
      setLoading(false);
    }
  }, []);

  // Initial fetch
  useEffect(() => {
    fetchHealth();
  }, [fetchHealth]);

  // Polling
  useEffect(() => {
    if (pollInterval <= 0) return;

    intervalRef.current = setInterval(fetchHealth, pollInterval);

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    };
  }, [fetchHealth, pollInterval]);

  return { data, loading, error, refetch: fetchHealth };
}
