"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIQuestionsStatsResponse } from '@/lib/api';

export type QuestionsStatsData = APIQuestionsStatsResponse['data'];

export interface UseQuestionsStatsResult {
  stats: QuestionsStatsData | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function useQuestionsStats(): UseQuestionsStatsResult {
  const [stats, setStats] = useState<QuestionsStatsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchStats = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await api.getQuestionsStats();
      setStats(response.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch questions stats');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStats();
  }, [fetchStats]);

  return { stats, loading, error, refetch: fetchStats };
}
