'use client';

// The ONE way an event reaches Google Analytics (SPEC.md 27.7).
//
//   track(event, params?)            something happened on this page
//   trackOnNextPage(event, params?)  something happened that a full page load follows, or
//                                    on a page Google's tag never sees
//
// Both do nothing unless the visitor accepted analytics. Every string parameter is stripped
// of e-mail addresses, Solvr keys, bearer tokens and JWTs and cut to 100 characters, and
// every event carries the kind of page it happened on. Collection is the whole job: events
// go onto the tag's data layer (lib/google-tag.ts), nothing is ever read back.
//
// Wiring an event is one line at the place where the action SUCCEEDED:
//   track('nav_click', { item: 'rooms', location: 'header' });
// A new event is one more name in ANALYTICS_EVENTS and one more row in SPEC.md 27.7.

import { CONTENT_GROUPS, contentGroupForPath, isUntrackedPath, type ContentGroup } from './analytics-paths';
import { cleanParams, type CleanParams, type TrackParams } from './analytics-redact';
import { getConsent } from './consent';
import { googleTagLoaded, sendGoogleTagEvent } from './google-tag';

// The closed list of event names. An unknown name is a type error, and is ignored at run time.
export const ANALYTICS_EVENTS = ['nav_click', 'cta_click'] as const;

export type AnalyticsEvent = (typeof ANALYTICS_EVENTS)[number];
export type { TrackParams, TrackParamName } from './analytics-redact';

// sessionStorage: events waiting for the next tracked page. Listed on /privacy.
export const PENDING_EVENTS_KEY = 'solvr_pending_events';

const QUEUE_LIMIT = 50;
const PENDING_LIMIT = 20;

interface QueuedEvent {
  event: AnalyticsEvent;
  params: CleanParams;
}

// What sessionStorage holds for one waiting event: its name, its cleaned parameters and
// the kind of page it happened on. Short keys, the value is rewritten on every call.
interface PendingEvent {
  e: AnalyticsEvent;
  p: CleanParams;
  g: ContentGroup;
}

// Events tracked while consent is granted and Google's tag is not in the page yet (it is
// requested only when the page is idle). Flushed in order once it is.
const queue: QueuedEvent[] = [];

function isEvent(name: unknown): name is AnalyticsEvent {
  return typeof name === 'string' && (ANALYTICS_EVENTS as readonly string[]).includes(name);
}

function deliver(event: AnalyticsEvent, params: CleanParams): void {
  if (googleTagLoaded()) {
    sendGoogleTagEvent(event, params);
  } else if (queue.length < QUEUE_LIMIT) {
    queue.push({ event, params });
  }
}

/**
 * Report something that happened on this page. A no-op unless consent is granted and the
 * page is a tracked one. Waits in memory while Google's tag is still on its way.
 */
export function track(event: AnalyticsEvent, params?: TrackParams): void {
  if (typeof window === 'undefined' || !isEvent(event)) return;
  if (getConsent() !== 'granted' || isUntrackedPath(window.location.pathname)) return;
  deliver(event, { ...cleanParams(params), content_group: contentGroupForPath(window.location.pathname) });
}

function readPending(): PendingEvent[] {
  try {
    const raw = window.sessionStorage.getItem(PENDING_EVENTS_KEY);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    const pending: PendingEvent[] = [];
    for (const entry of parsed) {
      if (!entry || typeof entry !== 'object') continue;
      const { e, p, g } = entry as Record<string, unknown>;
      if (!isEvent(e)) continue;
      pending.push({
        e,
        // Cleaned again on the way out: this value may have been written by anything.
        p: cleanParams(p && typeof p === 'object' ? (p as TrackParams) : undefined),
        g: (CONTENT_GROUPS as readonly unknown[]).includes(g) ? (g as ContentGroup) : 'other',
      });
    }
    return pending;
  } catch {
    return [];
  }
}

function clearPending(): void {
  try {
    window.sessionStorage.removeItem(PENDING_EVENTS_KEY);
  } catch {
    // Storage that throws holds nothing to clear.
  }
}

/**
 * Report something that is followed by a full page load, or that happened on a page the
 * tag never sees: the event waits in sessionStorage and is sent from the next tracked page.
 * Nothing is kept about an action taken without consent, and consent is asked again at
 * the time of sending.
 */
export function trackOnNextPage(event: AnalyticsEvent, params?: TrackParams): void {
  if (typeof window === 'undefined' || !isEvent(event)) return;
  if (getConsent() !== 'granted') return;
  const pending = readPending();
  pending.push({ e: event, p: cleanParams(params), g: contentGroupForPath(window.location.pathname) });
  try {
    window.sessionStorage.setItem(PENDING_EVENTS_KEY, JSON.stringify(pending.slice(-PENDING_LIMIT)));
  } catch {
    // A browser that keeps nothing loses this one event, never the page.
  }
}

/**
 * Send what is waiting: first the events kept from an earlier page, then the ones queued on
 * this page, each in its own order. Called by SiteAnalytics when the tag arrives and on
 * every tracked page after that. Without consent everything waiting is dropped instead.
 */
export function flushTrackedEvents(): void {
  if (typeof window === 'undefined') return;
  if (getConsent() !== 'granted') {
    dropTrackedEvents();
    return;
  }
  // Not yet: they wait for a tracked page that has the tag.
  if (!googleTagLoaded() || isUntrackedPath(window.location.pathname)) return;

  const pending = readPending();
  if (pending.length > 0) {
    clearPending();
    for (const { e, p, g } of pending) sendGoogleTagEvent(e, { ...p, content_group: g });
  }
  for (const { event, params } of queue.splice(0)) sendGoogleTagEvent(event, params);
}

/** Forget everything waiting, in memory and in sessionStorage. Called when consent is not granted. */
export function dropTrackedEvents(): void {
  queue.length = 0;
  if (typeof window !== 'undefined') clearPending();
}
