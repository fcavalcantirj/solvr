"use client";

import { useEffect, useRef, useState } from 'react';
import { api } from '@/lib/api';
import { APIError } from '@/lib/api-error';
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
// ordinary contract.
//
// It forwards the PRESET it was linked with too (?preset=, from the homepage use
// cases), so the API answers with that preset selected. The API is the only judge
// of a preset: one it refuses with a 400 (old links still carry the retired
// planner-executor) is dropped once, and the contract is read again with the
// API's own default. Nothing here knows the preset values or maps one to another.
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

function readPresetFromLocation(): string | undefined {
  if (typeof window === 'undefined') return undefined;
  return new URLSearchParams(window.location.search).get('preset') || undefined;
}

export function useConnectStart({ readLocation = false }: { readLocation?: boolean } = {}) {
  const [source] = useState<ConnectSourceParams>(() => (readLocation ? readSourceFromLocation() : {}));
  const [linkedPreset] = useState<string | undefined>(() => (readLocation ? readPresetFromLocation() : undefined));
  const [task, setTask] = useState('');
  const [preset, setPreset] = useState<string | undefined>(linkedPreset);
  // Set once the API has refused the linked preset, so it is dropped only once.
  const linkedPresetRefused = useRef(false);
  const [visibility, setVisibility] = useState<string | undefined>(undefined);
  const [start, setStart] = useState<APIConnectStart | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const debouncedTask = useDebounce(task, TASK_DEBOUNCE_MS);

  useEffect(() => {
    let cancelled = false;

    const fetchStart = async () => {
      let retrying = false;
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
        if (cancelled) return;
        const refusedLinkedPreset =
          err instanceof APIError && err.statusCode === 400 &&
          preset !== undefined && preset === linkedPreset && !linkedPresetRefused.current;
        if (refusedLinkedPreset) {
          // Read the contract again without it: the API's default is the only default.
          linkedPresetRefused.current = true;
          retrying = true;
          setPreset(undefined);
          return;
        }
        setError(err instanceof Error ? err.message : 'The connection instructions could not be read');
      } finally {
        if (!cancelled && !retrying) setLoading(false);
      }
    };

    fetchStart();
    return () => {
      cancelled = true;
    };
  }, [source, linkedPreset, debouncedTask, preset, visibility]);

  return { start, loading, error, task, setTask, setPreset, setVisibility };
}
