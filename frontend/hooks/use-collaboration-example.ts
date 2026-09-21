"use client";

import { useState, useEffect } from 'react';
import { api } from '@/lib/api';
import type { APICollaborationExample } from '@/lib/api-types';

// The homepage example is served whole by the API — including its own fallback
// when the room is private, deleted or unavailable. This hook only fetches it.
export function useCollaborationExample() {
  const [example, setExample] = useState<APICollaborationExample | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const fetchExample = async () => {
      try {
        const response = await api.getCollaborationExample();
        if (!cancelled) {
          setExample(response.data);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to fetch the example');
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    fetchExample();
    return () => {
      cancelled = true;
    };
  }, []);

  return { example, loading, error };
}
