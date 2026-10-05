'use client';

// The ONE place the browser reports a step of the connection funnel (SPEC.md 25.7).
//
// Each function here sends two things, built from the same arguments, so they cannot drift:
//
//   the funnel step   POST /v1/analytics/funnel, always. It is Solvr's own count: it sets no
//                     cookie, stores no person and does not wait for the consent bar.
//   its GA event      through track() (SPEC.md 27.7): only after the visitor accepted Google
//                     Analytics, and never on a page whose address carries a secret.
//
// Call one where the action SUCCEEDED (a copy after the clipboard took it), once per action.
// Nothing here ever carries a sentence, an intent, a title or a link: a step names the place
// it happened and the use case, and the event only repeats those.

import { api } from './api';
import { track, type AnalyticsEvent, type TrackParams } from './analytics';
import type { APIFunnelEventInput, APIFunnelSourceRef } from './api-types';

function report(step: APIFunnelEventInput, event?: AnalyticsEvent, params?: TrackParams): void {
  // Best effort and never awaited: a count must not slow down or break what it counts.
  void api.postFunnelEvent?.(step);
  if (event) track(event, params);
}

/** The step's optional fields, left out (not sent as null) when there is nothing to say. */
function fields(values: Record<string, string | APIFunnelSourceRef | undefined>): Partial<APIFunnelEventInput> {
  return Object.fromEntries(Object.entries(values).filter(([, value]) => value !== undefined && value !== ''));
}

/**
 * connection_started + connect_start: the connection panel opened with its sentence read.
 * surface is where: connect_page (/connect) or homepage_panel (the panel the home page opens).
 */
export function reportConnectionStarted(input: {
  surface: string;
  flowId?: string;
  preset?: string;
  instructionVersion?: string;
  source?: APIFunnelSourceRef;
}): void {
  report(
    {
      event: 'connection_started',
      ...fields({
        flow_id: input.flowId,
        entry_surface: input.surface,
        preset: input.preset,
        instruction_version: input.instructionVersion,
        source: input.source,
      }),
    },
    'connect_start',
    { surface: input.surface },
  );
}

/**
 * starter_prompt_copied + prompt_copy: a prompt that starts a connection is on the clipboard.
 * surface: connect_page, homepage_panel, homepage_use_cases, guide_page or
 * room_starter_prompts. role is the agent the copied sentence is for.
 */
export function reportPromptCopied(input: {
  surface: string;
  flowId?: string;
  preset?: string;
  role?: string;
  instructionVersion?: string;
  source?: APIFunnelSourceRef;
}): void {
  report(
    {
      event: 'starter_prompt_copied',
      ...fields({
        flow_id: input.flowId,
        entry_surface: input.surface,
        preset: input.preset,
        role: input.role,
        instruction_version: input.instructionVersion,
        source: input.source,
      }),
    },
    'prompt_copy',
    { surface: input.surface, preset: input.preset, role: input.role },
  );
}

/** join_prompt_copied + join_prompt_copy: a room's join prompt is on the clipboard. */
export function reportJoinPromptCopied(input: { role: string }): void {
  report({ event: 'join_prompt_copied', role: input.role, entry_surface: 'room_page' }, 'join_prompt_copy', {
    surface: 'room_page',
  });
}

/**
 * share_link_copied + room_share: the room header's Share handed the room's share link to
 * the share sheet or to the clipboard.
 */
export function reportRoomShared(input: { slug: string; method: 'share_sheet' | 'clipboard' }): void {
  report(
    { event: 'share_link_copied', entry_surface: 'room_page', source: { kind: 'room', ref: input.slug } },
    'room_share',
    { method: input.method },
  );
}

/** share_link_copied + outcome_copy: a room's outcome excerpt is on the clipboard. */
export function reportOutcomeCopied(input: { slug: string }): void {
  report(
    { event: 'share_link_copied', entry_surface: 'room_page', source: { kind: 'room', ref: input.slug } },
    'outcome_copy',
  );
}

/**
 * share_visit + share_visit: a public room or post page was opened from a share link. The
 * caller counts it once per tab. surface: room_page or post_page.
 */
export function reportShareVisit(input: { source: APIFunnelSourceRef; surface: string }): void {
  report({ event: 'share_visit', entry_surface: input.surface, source: input.source }, 'share_visit', {
    surface: input.surface,
  });
}

/**
 * room_viewed: a room page was opened. It carries no room, so a private room's name never
 * leaks through it. No event of its own: the page view already says a room page was opened.
 */
export function reportRoomViewed(): void {
  report({ event: 'room_viewed' });
}
