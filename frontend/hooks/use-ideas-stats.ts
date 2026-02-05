"use client";

import { useState, useEffect, useCallback } from 'react';
import { api, APIIdeasStatsResponse } from '@/lib/api';

export interface IdeasStats {
  total: number;
  countsByStatus: Record<string, number>;
  freshSparks: Array<{
    id: string;
    title: string;
    support: number;
    createdAt: string;
  }>;
  readyToDevelop: Array<{
    id: string;
    title: string;
    support: number;
    validationScore: number;
  }>;
  topSparklers: Array<{
    id: string;
    name: string;
    type: 'human' | 'agent';
    ideasCount: number;
    realizedCount: number;
  }>;
  trendingTags: Array<{
    name: string;
    count: number;
    growth: number;
  }>;
  pipelineStats: {
    sparkToDeveloping: number;
    developingToMature: number;
    matureToRealized: number;
    avgDaysToRealization: number;
  };
  recentlyRealized: Array<{
    id: string;
    title: string;
    evolvedInto?: string;
  }>;
}

// Transform API response to frontend format
function transformStats(data: APIIdeasStatsResponse['data']): IdeasStats {
  return {
    total: data.counts_by_status.total || 0,
    countsByStatus: data.counts_by_status,
    freshSparks: data.fresh_sparks.map(s => ({
      id: s.id,
      title: s.title,
      support: s.support,
      createdAt: s.created_at,
    })),
    readyToDevelop: data.ready_to_develop.map(r => ({
      id: r.id,
      title: r.title,
      support: r.support,
      validationScore: r.validation_score,
    })),
    topSparklers: data.top_sparklers.map(t => ({
      id: t.id,
      name: t.name,
      type: t.type,
      ideasCount: t.ideas_count,
      realizedCount: t.realized_count,
    })),
    trendingTags: data.trending_tags.map(t => ({
      name: t.name,
      count: t.count,
      growth: t.growth,
    })),
    pipelineStats: {
      sparkToDeveloping: data.pipeline_stats.spark_to_developing,
      developingToMature: data.pipeline_stats.developing_to_mature,
      matureToRealized: data.pipeline_stats.mature_to_realized,
      avgDaysToRealization: data.pipeline_stats.avg_days_to_realization,
    },
    recentlyRealized: data.recently_realized.map(r => ({
      id: r.id,
      title: r.title,
      evolvedInto: r.evolved_into,
    })),
  };
}

export interface UseIdeasStatsResult {
  stats: IdeasStats | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function useIdeasStats(): UseIdeasStatsResult {
  const [stats, setStats] = useState<IdeasStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchStats = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await api.getIdeasStats();
      setStats(transformStats(response.data));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch ideas stats');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStats();

    // Refresh stats every 60 seconds
    const interval = setInterval(fetchStats, 60000);
    return () => clearInterval(interval);
  }, [fetchStats]);

  const refetch = useCallback(() => {
    fetchStats();
  }, [fetchStats]);

  return {
    stats,
    loading,
    error,
    refetch,
  };
}
