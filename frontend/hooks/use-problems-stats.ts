"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIProblemsStatsResponse } from '@/lib/api';

export type ProblemsStatsData = APIProblemsStatsResponse['data'];

export interface UseProblemsStatsResult {
  stats: ProblemsStatsData | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function useProblemsStats(): UseProblemsStatsResult {
  const [stats, setStats] = useState<ProblemsStatsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchStats = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await api.getProblemsStats();
      setStats(response.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch problems stats');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  return { stats, loading, error, refetch: fetchStats };
}
