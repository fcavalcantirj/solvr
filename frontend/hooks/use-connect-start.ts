"use client";

import { useEffect, useRef, useState } from 'react';
import { api } from '@/lib/api';
import { APIError } from '@/lib/api-error';
import { useDebounce } from '@/hooks/use-debounce';
import type { APIConnectPreset, APIConnectStart } from '@/lib/api-types';

// The start-flow contract, re-read whenever the visitor changes the sentence.
//
// The hook holds what the visitor TYPED or CHOSE and nothing else: the sentence, its
// segments, the use cases and their lines all come back from GET /v1/connect. A choice
// not yet made is not sent, so the API's own defaults are the only defaults.
//
// Typing the intent and flipping the visibility re-read the API (the intent debounced).
// Switching the use case does not: every use case's sentence arrives in the same answer,
// so the switch only picks which one shows and nothing else on the page moves.
//
// The full /connect page also forwards the SOURCE it was linked with — a public room
// (?from_room=) or a post (?post=), "Try this workflow" — and the PRESET (?preset=, from
// the home cards), read once from its own address. The API is the only judge of both:
// a preset it refuses with a 400 is dropped once, and the contract is read again.
//
// One visit is one flow. The API mints a flow id with every answer unless it is handed
// one back, so the hook sends the flow id of the first answer it kept as `flow` on every
// later read. It only echoes what the API issued: it never makes, checks or changes one.
const INTENT_DEBOUNCE_MS = 300;

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
  const [intent, setIntent] = useState('');
  const [preset, setPreset] = useState<string | undefined>(linkedPreset);
  const [visibility, setVisibility] = useState<string | undefined>(undefined);
  const [start, setStart] = useState<APIConnectStart | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Bumped to read again after the API refused the linked preset (dropped only once).
  const [attempt, setAttempt] = useState(0);
  const linkedPresetRefused = useRef(false);
  // The preset in force when a read starts; switching it never triggers a read.
  const presetRef = useRef(preset);
  presetRef.current = preset;
  // The flow id of the first answer kept, echoed on every later read of this visit.
  const flowRef = useRef<string | undefined>(undefined);

  const debouncedIntent = useDebounce(intent, INTENT_DEBOUNCE_MS);

  useEffect(() => {
    let cancelled = false;
    const asked = presetRef.current;

    const fetchStart = async () => {
      let retrying = false;
      try {
        const response = await api.getConnectStart({
          ...source,
          ...(debouncedIntent.trim() ? { intent: debouncedIntent } : {}),
          ...(asked ? { preset: asked } : {}),
          ...(visibility ? { visibility } : {}),
          ...(flowRef.current ? { flow: flowRef.current } : {}),
        });
        if (!cancelled) {
          if (!flowRef.current) flowRef.current = response.data.selected?.flow_id || undefined;
          setStart(response.data);
          setError(null);
        }
      } catch (err) {
        if (cancelled) return;
        const refusedLinkedPreset =
          err instanceof APIError && err.statusCode === 400 &&
          asked !== undefined && asked === linkedPreset && !linkedPresetRefused.current;
        if (refusedLinkedPreset) {
          linkedPresetRefused.current = true;
          retrying = true;
          presetRef.current = undefined;
          setPreset(undefined);
          setAttempt((n) => n + 1);
          return;
        }
        setError(err instanceof Error ? err.message : 'The connection sentence could not be read');
      } finally {
        if (!cancelled && !retrying) setLoading(false);
      }
    };

    fetchStart();
    return () => {
      cancelled = true;
    };
  }, [source, linkedPreset, debouncedIntent, visibility, attempt]);

  // The use case showing: the visitor's pick, else the one the API selected.
  const presets = start?.presets ?? [];
  const active: APIConnectPreset | undefined =
    presets.find((p) => p.value === preset) ?? presets.find((p) => p.selected) ?? presets[0];

  return { start, active, loading, error, intent, setIntent, setPreset, setVisibility };
}
