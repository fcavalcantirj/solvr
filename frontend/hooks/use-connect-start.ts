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
//
// The full /connect page also forwards the SOURCE it was linked with — a public
// room (?from_room=) or a post (?post=), "Try this workflow" — read once from its
// own address. The API validates it; an unknown or private source simply gives the
// ordinary contract. ?preset= is deliberately not forwarded: old links still carry
// a value the API refuses.
const TASK_DEBOUNCE_MS = 300;

interface ConnectSourceParams {
  from_room?: string;
  post?: string;
}

function readSourceFromLocation(): ConnectSourceParams {
  if (typeof window === 'undefined') return {};
  const params = new URLSearchParams(window.location.search);
  const fromRoom = params.get('from_room');
  const post = params.get('post');
  return {
    ...(fromRoom ? { from_room: fromRoom } : {}),
    ...(post ? { post } : {}),
  };
}

export function useConnectStart({ readLocation = false }: { readLocation?: boolean } = {}) {
  const [source] = useState<ConnectSourceParams>(() => (readLocation ? readSourceFromLocation() : {}));
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
          ...source,
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
  }, [source, debouncedTask, preset, visibility]);

  return { start, loading, error, task, setTask, setPreset, setVisibility };
}
