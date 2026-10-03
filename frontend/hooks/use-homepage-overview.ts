"use client";

import { useState, useEffect } from 'react';
import { api } from '@/lib/api';
import type { APIHomepageOverview, APIOverviewMeta, APIOverviewResponse } from '@/lib/api-types';

// The index is served by GET /v1/overview — the Task 14 consolidated endpoint
// that wraps the same HomepageOverview data in a meta envelope (generated_at,
// window boundaries, source availability, partial errors). The browser renders
// the answer; it does not compute, validate or rank anything.
//
// The index reads the overview on the server first and passes it in as
// `initial`: the first render already has it, so there is no loading band. The
// browser then refreshes once; a failed refresh keeps what the server read.
export function useHomepageOverview(initial?: APIOverviewResponse | null) {
  const [overview, setOverview] = useState<APIHomepageOverview | null>(initial?.data ?? null);
  const [meta, setMeta] = useState<APIOverviewMeta | null>(initial?.meta ?? null);
  const [loading, setLoading] = useState(!initial);
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
