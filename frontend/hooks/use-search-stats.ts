"use client";

import { useState, useEffect } from 'react';
import { api, PublicSearchStatsData } from '@/lib/api';

export function useSearchStats() {
  const [searchStats, setSearchStats] = useState<PublicSearchStatsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchSearchStats = async () => {
      try {
        setLoading(true);
        const response = await api.getPublicSearchStats();
        setSearchStats(response.data);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to fetch search stats');
      } finally {
        setLoading(false);
      }
    };

    fetchSearchStats();

    // Refresh every 120 seconds
    const interval = setInterval(fetchSearchStats, 120000);
    return () => clearInterval(interval);
  }, []);

  return { searchStats, loading, error };
}
