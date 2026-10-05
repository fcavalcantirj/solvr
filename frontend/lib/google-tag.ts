// Everything Solvr says to Google's tag goes through this file (SPEC.md 27.7). It is called
// from the browser only, and only by components/site-analytics.tsx and lib/analytics.ts.
// Collection is the whole job: commands go onto the data layer, nothing is read back.
//
// Measured against the real tag of this property, behind a network guard, on 2026-10-05:
//  - With the consent default below, or with the two flags, hits go to
//    www.google-analytics.com/g/collect. Without them a page view also goes to
//    stats.g.doubleclick.net and asks google.<tld>/ads/ga-audiences.
//  - When a page is left, the tag sends its last hit (user_engagement) twice: to
//    www.google-analytics.com and, as a copy without cookies, to www.google.com/g/collect.
//  - content_group reaches a hit only as a parameter of the config (or of one event). Stated
//    with gtag('set', …) it never arrives. That is why the config is written here: a config
//    with no parameters cannot say what kind of page this is.
//  - A second plain config is ignored. A config with `update: true` changes the group and
//    sends nothing.
//  - The page view the tag sends by itself after a history change is decided about a second
//    after the change, so a group updated right after a navigation is the one it carries.
//  - window['ga-disable-<id>'] = true silences a tag that is already in the page: no page
//    view, no event, no cookie. An event the tag accepted before the switch still goes out
//    (it batches for about five seconds).
//  - A consent update to denied does not silence it: the tag keeps sending, without cookies.
import type { ContentGroup } from './analytics-paths';
import type { CleanParams } from './analytics-redact';

type TagWindow = Window & { dataLayer?: unknown[] } & Record<string, unknown>;

const TAG_URL = 'https://www.googletagmanager.com/gtag/js';

// Analytics only: nothing for advertising, whatever the property's own settings say.
export const GOOGLE_TAG_CONSENT_DEFAULT = {
  analytics_storage: 'granted',
  ad_storage: 'denied',
  ad_user_data: 'denied',
  ad_personalization: 'denied',
} as const;

export const GOOGLE_TAG_SETTINGS = {
  allow_google_signals: false,
  allow_ad_personalization_signals: false,
} as const;

// The measurement id the tag was requested for; null until then. The tag cannot be taken
// out of a page again, so this never goes back to null.
let loadedId: string | null = null;
let tagConsentGranted = true;
let tagContentGroup: ContentGroup | null = null;

const gtag: (...command: unknown[]) => void = function gtag() {
  const w = window as unknown as TagWindow;
  w.dataLayer = w.dataLayer || [];
  // The tag reads `arguments` objects off the data layer and ignores a plain array.
  // eslint-disable-next-line prefer-rest-params
  w.dataLayer.push(arguments);
};

/** True once the tag was requested in this page: commands reach it, now or when it arrives. */
export function googleTagLoaded(): boolean {
  return loadedId !== null;
}

/**
 * Request Google's tag, once. The consent default and the one config are queued first, so
 * they are what the tag reads when it arrives. The caller decides WHEN: consent granted, a
 * tracked page, the page idle.
 */
export function loadGoogleTag(gaId: string, contentGroup: ContentGroup): void {
  if (typeof window === 'undefined' || loadedId !== null) return;
  loadedId = gaId;
  tagConsentGranted = true;
  tagContentGroup = contentGroup;

  gtag('consent', 'default', GOOGLE_TAG_CONSENT_DEFAULT);
  gtag('js', new Date());
  gtag('config', gaId, { ...GOOGLE_TAG_SETTINGS, content_group: contentGroup });

  const script = document.createElement('script');
  script.async = true;
  script.src = `${TAG_URL}?id=${encodeURIComponent(gaId)}`;
  document.head.appendChild(script);
}

/** Google's own off switch for one measurement id. It holds before and after the tag loads. */
export function setGoogleTagEnabled(gaId: string, enabled: boolean): void {
  if (typeof window === 'undefined') return;
  (window as unknown as TagWindow)[`ga-disable-${gaId}`] = !enabled;
}

/** Tell a loaded tag that consent went, or came back. Says nothing twice. */
export function setGoogleTagConsent(granted: boolean): void {
  if (loadedId === null || granted === tagConsentGranted) return;
  tagConsentGranted = granted;
  gtag('consent', 'update', { analytics_storage: granted ? 'granted' : 'denied' });
}

/** Tell a loaded tag the kind of page now shown (after a client-side navigation). */
export function setGoogleTagContentGroup(contentGroup: ContentGroup): void {
  if (loadedId === null || contentGroup === tagContentGroup) return;
  tagContentGroup = contentGroup;
  gtag('config', loadedId, { content_group: contentGroup, update: true });
}

/** Hand one event to a loaded tag. Dropped when there is none: lib/analytics.ts queues. */
export function sendGoogleTagEvent(name: string, params: CleanParams): void {
  if (loadedId === null) return;
  gtag('event', name, params);
}

/**
 * Delete Google Analytics' cookies (_ga and _ga_<id>). The tag sets them on the widest domain
 * the browser accepts, so this host and each of its parents is tried.
 */
export function deleteGoogleAnalyticsCookies(): void {
  if (typeof document === 'undefined') return;
  try {
    const names = document.cookie
      .split(';')
      .map((cookie) => cookie.split('=')[0].trim())
      .filter((name) => name === '_ga' || name.startsWith('_ga_'));
    if (names.length === 0) return;

    const labels = window.location.hostname.split('.');
    const domains = [''];
    for (let i = 0; i < labels.length - 1; i++) {
      const domain = labels.slice(i).join('.');
      domains.push(domain, `.${domain}`);
    }
    for (const name of names) {
      for (const domain of domains) {
        document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; max-age=0; path=/${domain ? `; domain=${domain}` : ''}`;
      }
    }
  } catch {
    // Cookies that cannot be read cannot be deleted from here either.
  }
}
