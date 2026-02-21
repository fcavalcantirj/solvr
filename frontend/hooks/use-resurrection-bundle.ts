"use client";

import { useState, useEffect, useCallback } from 'react';
import { api } from '@/lib/api';
import type { APIResurrectionBundle } from '@/lib/api-types';

export interface UseResurrectionBundleResult {
  bundle: APIResurrectionBundle | null;
  loading: boolean;
  error: string | null;
}

/**
 * Hook to fetch an agent's resurrection bundle.
 * Only fetches when enabled is true (lazy load for tab activation).
 * @param agentId - The agent ID to fetch the bundle for
 * @param enabled - Whether to fetch (lazy load control)
 */
export function useResurrectionBundle(agentId: string, enabled: boolean): UseResurrectionBundleResult {
  const [bundle, setBundle] = useState<APIResurrectionBundle | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    if (!agentId || !enabled) {
      return;
    }

    try {
      setLoading(true);
      setError(null);
      const response = await api.getResurrectionBundle(agentId);
      setBundle(response);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch resurrection bundle');
      setBundle(null);
    } finally {
      setLoading(false);
    }
  }, [agentId, enabled]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  return { bundle, loading, error };
}
