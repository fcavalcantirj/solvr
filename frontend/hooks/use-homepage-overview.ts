"use client";

import { useState, useEffect } from 'react';
import { api } from '@/lib/api';
import type { APIHomepageOverview, APIOverviewMeta } from '@/lib/api-types';

// The index is served by GET /v1/overview — the Task 14 consolidated endpoint
// that wraps the same HomepageOverview data in a meta envelope (generated_at,
// window boundaries, source availability, partial errors). The browser renders
// the answer; it does not compute, validate or rank anything.
export function useHomepageOverview() {
  const [overview, setOverview] = useState<APIHomepageOverview | null>(null);
  const [meta, setMeta] = useState<APIOverviewMeta | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const fetchOverview = async () => {
      try {
        const response = await api.getOverview();
        if (!cancelled) {
          setOverview(response.data);
          setMeta(response.meta);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to fetch the overview');
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    fetchOverview();
    return () => {
      cancelled = true;
    };
  }, []);

  return { overview, meta, loading, error };
}
