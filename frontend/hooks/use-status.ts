"use client";

import { useState, useEffect, useCallback, useRef } from 'react';
import { api } from '@/lib/api';
import type { APIStatusData } from '@/lib/api-types';

export interface UseStatusOptions {
  pollIntervalMs?: number;
}

export interface UseStatusResult {
  data: APIStatusData | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

const DEFAULT_POLL_INTERVAL = 60000; // 60 seconds

export function useStatus(
  options?: UseStatusOptions
): UseStatusResult {
  const [data, setData] = useState<APIStatusData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const pollInterval = options?.pollIntervalMs ?? DEFAULT_POLL_INTERVAL;

  const fetchStatus = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await api.getStatus();
      setData(response.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch status');
      setData(null);
    } finally {
      setLoading(false);
    }
  }, []);

  // Initial fetch
  useEffect(() => {
    fetchStatus();
  }, [fetchStatus]);

  // Polling
  useEffect(() => {
    if (pollInterval <= 0) return;

    intervalRef.current = setInterval(fetchStatus, pollInterval);

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    };
  }, [fetchStatus, pollInterval]);

  return { data, loading, error, refetch: fetchStatus };
}
