import { describe, it, expect, vi, afterEach } from 'vitest';
import { act, render } from '@testing-library/react';

// One visit, through the real consent store, the real track() and the real data layer:
// nothing for Google exists before Accept, the tag is requested only when the page is
// idle, and Decline after Accept silences it (SPEC.md 27.7). Nothing is mocked but the
// router's pathname and the browser's idle moment. jsdom never fetches the script.

vi.mock('next/navigation', () => ({
  usePathname: () => window.location.pathname,
}));

import { SiteAnalytics } from './site-analytics';
import { track } from '@/lib/analytics';
import { setConsent } from '@/lib/consent';

type TagWindow = {
  dataLayer?: unknown[];
  requestIdleCallback?: (callback: () => void, options?: { timeout: number }) => number;
} & Record<string, unknown>;
const w = () => window as unknown as TagWindow;

const commands = () => (w().dataLayer ?? []).map((entry) => Array.from(entry as ArrayLike<unknown>));
const tagScripts = () => Array.from(document.querySelectorAll<HTMLScriptElement>('script[src*="googletagmanager.com"]'));
const cookieNames = () => document.cookie.split(';').map((c) => c.split('=')[0].trim()).filter(Boolean);

describe('SiteAnalytics with the real store and data layer', () => {
  afterEach(() => {
    delete w().requestIdleCallback;
  });

  it('asks Google for nothing before Accept, loads when idle after it, and stops on Decline', () => {
    window.localStorage.clear();
    const idle: Array<() => void> = [];
    w().requestIdleCallback = (callback) => idle.push(callback);
    const becomeIdle = () => act(() => idle.splice(0).forEach((run) => run()));

    render(<SiteAnalytics gaId="G-TEST" />);

    // 1. Nothing chosen: no tag, no data layer, and an event goes nowhere.
    track('nav_click', { item: 'rooms', location: 'header' });
    becomeIdle();
    expect(tagScripts()).toHaveLength(0);
    expect(w().dataLayer).toBeUndefined();
    expect(cookieNames()).toEqual([]);

    // 2. Accept: still nothing until the page is idle. An event meanwhile waits.
    act(() => setConsent('granted'));
    track('cta_click', { item: 'connect_agents', location: 'hero' });
    expect(tagScripts()).toHaveLength(0);
    expect(w().dataLayer).toBeUndefined();

    // 3. Idle: the consent default, js and ONE config with the flags are queued, the tag
    // is requested, and the waiting event follows the config.
    becomeIdle();
    expect(tagScripts().map((s) => s.src)).toEqual(['https://www.googletagmanager.com/gtag/js?id=G-TEST']);
    expect(commands().map(([name]) => name)).toEqual(['consent', 'js', 'config', 'event']);
    expect(commands()[0]).toEqual([
      'consent',
      'default',
      { analytics_storage: 'granted', ad_storage: 'denied', ad_user_data: 'denied', ad_personalization: 'denied' },
    ]);
    expect(commands()[2]).toEqual([
      'config',
      'G-TEST',
      { allow_google_signals: false, allow_ad_personalization_signals: false, content_group: 'home' },
    ]);
    expect(commands()[3]).toEqual(['event', 'cta_click', { item: 'connect_agents', location: 'hero', content_group: 'home' }]);
    expect(w()['ga-disable-G-TEST']).toBe(false);

    // 4. Decline after Accept: off switch, consent withdrawn from the tag, its cookies
    // deleted, and no further event.
    document.cookie = '_ga=GA1.1.1.1; path=/';
    document.cookie = '_ga_TEST=GS2.1.s1; path=/';
    document.cookie = 'theme=dark; path=/';
    act(() => setConsent('denied'));

    expect(w()['ga-disable-G-TEST']).toBe(true);
    expect(commands().at(-1)).toEqual(['consent', 'update', { analytics_storage: 'denied' }]);
    expect(cookieNames()).toEqual(['theme']);
    const before = commands().length;
    track('nav_click', { item: 'posts', location: 'header' });
    expect(commands()).toHaveLength(before);

    // 5. Accept again: the same tag is switched back on; it is not requested twice.
    act(() => setConsent('granted'));
    becomeIdle();
    expect(w()['ga-disable-G-TEST']).toBe(false);
    expect(commands().at(-1)).toEqual(['consent', 'update', { analytics_storage: 'granted' }]);
    expect(tagScripts()).toHaveLength(1);
    track('nav_click', { item: 'posts', location: 'header' });
    expect(commands().at(-1)).toEqual(['event', 'nav_click', { item: 'posts', location: 'header', content_group: 'home' }]);

    document.cookie = 'theme=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/';
  });
});
