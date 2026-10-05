// The marks the site's one click listener reads (components/track-clicks.tsx, SPEC.md 27.7).
// Pure, with no browser API: server components spread them too.
//
//   <Link href="/rooms" {...trackNav('rooms', 'header')}>ROOMS</Link>
//   <button {...trackCta('connect_agents', 'hero')}>Connect agents now</button>
//
// item is a stable lowercase id for WHAT was pressed, never the visible label, so a change of
// wording does not start a new series. location is WHERE it sits, one of the seven below.

export const TRACK_LOCATIONS = ['header', 'mobile_menu', 'docs_menu', 'account_menu', 'footer', 'hero', 'page'] as const;

export type TrackLocation = (typeof TRACK_LOCATIONS)[number];

export interface TrackAttrs {
  'data-track': 'nav' | 'cta';
  'data-track-item': string;
  'data-track-location': TrackLocation;
}

/** A link that takes the visitor somewhere: header, menus, footer. Sends nav_click. */
export function trackNav(item: string, location: TrackLocation): TrackAttrs {
  return { 'data-track': 'nav', 'data-track-item': item, 'data-track-location': location };
}

/** A call to action: the thing a block exists for. Sends cta_click. */
export function trackCta(item: string, location: TrackLocation): TrackAttrs {
  return { 'data-track': 'cta', 'data-track-item': item, 'data-track-location': location };
}
