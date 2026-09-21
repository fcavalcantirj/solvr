"use client";

import { useState, useEffect } from 'react';
import { api } from '@/lib/api';
import type { APIHomepageOverview } from '@/lib/api-types';

// The index is served whole by GET /v1/homepage/overview — headings, windows,
// definitions, excerpts, relative times and normalised sparkline heights
// included. This hook only fetches it.
export function useHomepageOverview() {
  const [overview, setOverview] = useState<APIHomepageOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const fetchOverview = async () => {
      try {
        const response = await api.getHomepageOverview();
        if (!cancelled) {
          setOverview(response.data);
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

  return { overview, loading, error };
}
