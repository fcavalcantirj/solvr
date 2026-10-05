"use client";

import { useEffect } from 'react';
import Link from 'next/link';
import { ArrowRight } from 'lucide-react';

import { useConnectStart } from '@/hooks/use-connect-start';
import { api } from '@/lib/api';
import type { APIConnectPreset, APIConnectStart, APIFunnelSourceRef } from '@/lib/api-types';
import { Prompt } from '@/components/prompt/prompt';
import { RolePairSwitch } from '@/components/prompt/role-pair-switch';
import { roles } from '@/components/prompt/prompt-align';
import { trackCta, type TrackLocation } from '@/lib/track-attrs';

// The connection panel: the one surface that starts a connection.
//
// The index opens it inline and /connect renders the same component full-page, so the
// two can never drift apart — both read GET /v1/connect and render what it answers.
// The sentence, its deciding words, the use cases and their lines are the API's; this
// file sends back what the visitor types or flips, and copies the text it was given.

type ConnectPanelVariant = 'panel' | 'page';

// funnelSourceOf names the public source a seeded flow's funnel steps are attributed
// to, from the source the API already resolved (idx 88). None for an ordinary start.
function funnelSourceOf(start: APIConnectStart): APIFunnelSourceRef | undefined {
  const src = start.source;
  if (src?.kind === 'room' && src.room_slug) return { kind: 'room', ref: src.room_slug };
  if (src?.kind === 'post' && src.post_id) return { kind: 'post', ref: src.post_id };
  return undefined;
}

// entrySurfaceFor names where a connection-funnel browser step was reported from, so
// the funnel can tell an index-panel open apart from the full /connect page.
function entrySurfaceFor(variant: ConnectPanelVariant): string {
  return variant === 'page' ? 'connect_page' : 'homepage_panel';
}

// Where the panel's calls to action sit for the click listener (SPEC.md 27.7): on the
// /connect page itself, or inside the home page's hero, which opens the panel in place.
function trackLocationFor(variant: ConnectPanelVariant): TrackLocation {
  return variant === 'page' ? 'page' : 'hero';
}

export function ConnectPanel({ variant = 'panel' }: { variant?: ConnectPanelVariant }) {
  // Only the full page forwards the source and preset it was linked with.
  const { start, active, loading, error, setIntent, setPreset, setVisibility } = useConnectStart({
    readLocation: variant === 'page',
  });

  if (!start || !active) {
    if (loading) {
      return (
        <div data-testid="connect-loading" className="font-mono text-xs tracking-[0.3em] text-muted-foreground py-8">
          READING THE SENTENCE...
        </div>
      );
    }
    return (
      <p role="alert" className="text-muted-foreground py-8">
        {error ?? 'The connection sentence could not be read.'}
      </p>
    );
  }

  return (
    <ConnectPanelContent
      start={start}
      active={active}
      variant={variant}
      error={error}
      onIntent={setIntent}
      onPreset={setPreset}
      onVisibility={setVisibility}
    />
  );
}

function ConnectPanelContent({
  start,
  active,
  variant,
  error,
  onIntent,
  onPreset,
  onVisibility,
}: {
  start: APIConnectStart;
  active: APIConnectPreset;
  variant: ConnectPanelVariant;
  error: string | null;
  onIntent: (value: string) => void;
  onPreset: (value: string) => void;
  onVisibility: (value: string) => void;
}) {
  const entrySurface = entrySurfaceFor(variant);
  const place = trackLocationFor(variant);

  // connection_started: the panel/page meaningfully opened (its contract loaded). Fired
  // once per open — this component mounts once the contract exists and stays mounted
  // across re-reads — not on every keystroke.
  useEffect(() => {
    const source = funnelSourceOf(start);
    void api.postFunnelEvent?.({
      event: 'connection_started',
      flow_id: start.selected.flow_id,
      entry_surface: entrySurface,
      preset: start.selected.preset,
      instruction_version: start.instruction_version,
      ...(source ? { source } : {}),
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // starter_prompt_copied: reported ONLY after the clipboard write succeeded, for the
  // use case and the agent the copied sentence is for.
  const onCopied = () => {
    void api.postFunnelEvent?.({
      event: 'starter_prompt_copied',
      flow_id: start.selected.flow_id,
      entry_surface: entrySurface,
      preset: active.value,
      role: roles(active.prompt).a?.toLowerCase(),
      instruction_version: start.instruction_version,
    });
  };

  // The flip asks for the other visibility of the one the sentence names now.
  const shown = active.prompt.segments.find((s) => s.kind === 'visibility')?.value;
  const flip = () => onVisibility(shown === 'private' ? 'public' : 'private');

  const Heading = variant === 'page' ? 'h1' : 'h2';

  return (
    <div data-testid="connect-panel" data-variant={variant}>
      <Heading className="text-xl font-normal tracking-[-0.01em] text-foreground sm:text-2xl">{start.heading}</Heading>

      {start.source && (
        <p data-testid="connect-source" className="mt-3 max-w-[60ch] text-sm leading-relaxed text-muted-foreground">
          Starting from{' '}
          <Link href={start.source.url} className="text-foreground underline underline-offset-4">
            {start.source.title}
          </Link>
          . {start.source.detail}
        </p>
      )}

      <div className="mt-6 lg:mt-8">
        <RolePairSwitch presets={start.presets} value={active.value} onChange={onPreset} more={start.more} />
      </div>

      <div className="mt-10 lg:mt-12">
        <Prompt
          variant="connect"
          preset={active}
          others={start.presets.filter((p) => p.value !== active.value)}
          onIntentChange={onIntent}
          onVisibilityToggle={flip}
          onCopied={onCopied}
          copyTrack={trackCta('copy_prompt', place)}
          intentLabel={start.intent_field.label}
          intentMaxChars={start.intent_field.max_chars}
          aside={
            <Link
              href={start.example.url}
              {...trackCta('example', place)}
              className="group inline-flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground transition-colors hover:text-foreground"
            >
              {start.example.label}
              <ArrowRight aria-hidden="true" className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
            </Link>
          }
        />
      </div>

      {error ? (
        <p role="alert" className="mt-6 text-sm text-muted-foreground">
          {error}
        </p>
      ) : null}
    </div>
  );
}
