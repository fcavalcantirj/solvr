"use client";

import { useEffect, useState } from 'react';
import { api } from '@/lib/api';
import { useDebounce } from '@/hooks/use-debounce';
import type { APIConnectStart } from '@/lib/api-types';

// The start-flow contract, re-read whenever the visitor changes something.
//
// The hook holds what the visitor TYPED or CHOSE and nothing else: the prompt,
// the labels, the explanations and which option is selected all come back from
// GET /v1/connect. A choice not yet made is not sent at all, so the API's own
// defaults are the only defaults anywhere.
const TASK_DEBOUNCE_MS = 300;

export function useConnectStart() {
  const [task, setTask] = useState('');
  const [preset, setPreset] = useState<string | undefined>(undefined);
  const [visibility, setVisibility] = useState<string | undefined>(undefined);
  const [start, setStart] = useState<APIConnectStart | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const debouncedTask = useDebounce(task, TASK_DEBOUNCE_MS);

  useEffect(() => {
    let cancelled = false;

    const fetchStart = async () => {
      try {
        const response = await api.getConnectStart({
          ...(debouncedTask ? { task: debouncedTask } : {}),
          ...(preset ? { preset } : {}),
          ...(visibility ? { visibility } : {}),
        });
        if (!cancelled) {
          setStart(response.data);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'The connection instructions could not be read');
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    fetchStart();
    return () => {
      cancelled = true;
    };
  }, [debouncedTask, preset, visibility]);

  return { start, loading, error, task, setTask, setPreset, setVisibility };
}
