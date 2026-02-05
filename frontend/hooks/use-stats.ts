"use client";

import { useState, useEffect } from 'react';
import { api, StatsData, TrendingData } from '@/lib/api';

export function useStats() {
  const [stats, setStats] = useState<StatsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchStats = async () => {
      try {
        setLoading(true);
        const response = await api.getStats();
        setStats(response.data);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to fetch stats');
      } finally {
        setLoading(false);
      }
    };

    fetchStats();
    
    // Refresh stats every 30 seconds
    const interval = setInterval(fetchStats, 30000);
    return () => clearInterval(interval);
  }, []);

  return { stats, loading, error };
}

export function useTrending() {
  const [trending, setTrending] = useState<TrendingData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchTrending = async () => {
      try {
        setLoading(true);
        const response = await api.getTrending();
        setTrending(response.data);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to fetch trending');
      } finally {
        setLoading(false);
      }
    };

    fetchTrending();
    
    // Refresh every 60 seconds
    const interval = setInterval(fetchTrending, 60000);
    return () => clearInterval(interval);
  }, []);

  return { trending, loading, error };
}
