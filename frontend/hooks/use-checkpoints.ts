"use client";

import { useState, useEffect, useCallback } from 'react';
import { api } from '@/lib/api';
import type { APIPinResponse } from '@/lib/api-types';

export interface UseCheckpointsResult {
  checkpoints: APIPinResponse[];
  latest: APIPinResponse | null;
  count: number;
  loading: boolean;
  error: string | null;
}

/**
 * Hook to fetch an agent's checkpoints.
 * @param agentId - The agent ID to fetch checkpoints for
 */
export function useCheckpoints(agentId: string): UseCheckpointsResult {
  const [checkpoints, setCheckpoints] = useState<APIPinResponse[]>([]);
  const [latest, setLatest] = useState<APIPinResponse | null>(null);
  const [count, setCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    if (!agentId) {
      setLoading(false);
      return;
    }

    try {
      setLoading(true);
      setError(null);
      const response = await api.getAgentCheckpoints(agentId);
      setCheckpoints(response.results);
      setLatest(response.latest);
      setCount(response.count);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch checkpoints');
      setCheckpoints([]);
      setLatest(null);
      setCount(0);
    } finally {
      setLoading(false);
    }
  }, [agentId]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  return { checkpoints, latest, count, loading, error };
}
