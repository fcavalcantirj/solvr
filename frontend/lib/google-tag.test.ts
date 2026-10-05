/**
 * @vitest-environment jsdom
 * @vitest-environment-options { "url": "https://www.example.com/rooms" }
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';

// Everything Solvr says to Google's tag goes through lib/google-tag.ts: what is queued
// before the tag exists, the switch that silences a tag already in the page, the page type,
// and the cookies that go when consent goes (SPEC.md 27.7). The behaviours pinned here were
// measured against the real tag behind a network guard on 2026-10-05; the comments in the
// module say which.

type TagModule = typeof import('./google-tag');
type TagWindow = Window & { dataLayer?: unknown[] } & Record<string, unknown>;

const w = () => window as unknown as TagWindow;
const commands = () => (w().dataLayer ?? []).map((entry) => Array.from(entry as ArrayLike<unknown>));
const tagScripts = () => Array.from(document.querySelectorAll<HTMLScriptElement>('script[src*="googletagmanager.com"]'));

async function freshModule(): Promise<TagModule> {
  vi.resetModules();
  return import('./google-tag');
}

function cookieNames(): string[] {
  return document.cookie
    .split(';')
    .map((c) => c.split('=')[0].trim())
    .filter(Boolean)
    .sort();
}

describe('google tag', () => {
  beforeEach(() => {
    delete w().dataLayer;
    for (const key of Object.keys(window)) if (key.startsWith('ga-disable-')) delete w()[key];
    tagScripts().forEach((s) => s.remove());
    for (const name of cookieNames()) {
      document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
      document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/; domain=example.com`;
    }
  });

  describe('loadGoogleTag', () => {
    it('touches nothing until it is called', async () => {
      const tag = await freshModule();
      expect(tag.googleTagLoaded()).toBe(false);
      expect(w().dataLayer).toBeUndefined();
      expect(tagScripts()).toHaveLength(0);
    });

    it('queues the consent default, then js, then ONE config carrying the flags and the page type', async () => {
      const tag = await freshModule();
      tag.loadGoogleTag('G-TEST', 'post');

      const queued = commands();
      expect(queued).toHaveLength(3);
      expect(queued[0]).toEqual([
        'consent',
        'default',
        { analytics_storage: 'granted', ad_storage: 'denied', ad_user_data: 'denied', ad_personalization: 'denied' },
      ]);
      expect(queued[1][0]).toBe('js');
      expect(queued[1][1]).toBeInstanceOf(Date);
      expect(queued[2]).toEqual([
        'config',
        'G-TEST',
        { allow_google_signals: false, allow_ad_personalization_signals: false, content_group: 'post' },
      ]);
      expect(tag.googleTagLoaded()).toBe(true);
    });

    it('hands the tag real `arguments` objects: it ignores an array', async () => {
      const tag = await freshModule();
      tag.loadGoogleTag('G-TEST', 'home');
      for (const entry of w().dataLayer ?? []) {
        expect(Object.prototype.toString.call(entry)).toBe('[object Arguments]');
      }
    });

    it('requests the tag from Google once, asynchronously, after the commands are queued', async () => {
      const tag = await freshModule();
      tag.loadGoogleTag('G-TEST', 'home');

      const scripts = tagScripts();
      expect(scripts).toHaveLength(1);
      expect(scripts[0].src).toBe('https://www.googletagmanager.com/gtag/js?id=G-TEST');
      expect(scripts[0].async).toBe(true);
    });

    it('does nothing the second time: one config, one script', async () => {
      const tag = await freshModule();
      tag.loadGoogleTag('G-TEST', 'home');
      tag.loadGoogleTag('G-TEST', 'room');

      expect(commands().filter(([name]) => name === 'config')).toHaveLength(1);
      expect(tagScripts()).toHaveLength(1);
    });

    it('keeps what another script already queued', async () => {
      w().dataLayer = [{ event: 'something-else' }];
      const tag = await freshModule();
      tag.loadGoogleTag('G-TEST', 'home');
      expect(w().dataLayer).toHaveLength(4);
      expect(w().dataLayer?.[0]).toEqual({ event: 'something-else' });
    });
  });

  describe('setGoogleTagEnabled', () => {
    it("flips Google's own off switch for this measurement id", async () => {
      const tag = await freshModule();
      tag.setGoogleTagEnabled('G-TEST', false);
      expect(w()['ga-disable-G-TEST']).toBe(true);
      tag.setGoogleTagEnabled('G-TEST', true);
      expect(w()['ga-disable-G-TEST']).toBe(false);
    });

    it('works before the tag is in the page and queues nothing', async () => {
      const tag = await freshModule();
      tag.setGoogleTagEnabled('G-TEST', false);
      expect(w().dataLayer).toBeUndefined();
      expect(tagScripts()).toHaveLength(0);
    });
  });

  describe('setGoogleTagConsent', () => {
    it('says nothing to a tag that is not there', async () => {
      const tag = await freshModule();
      tag.setGoogleTagConsent(false);
      expect(w().dataLayer).toBeUndefined();
    });

    it('tells a loaded tag once when consent goes, and once when it comes back', async () => {
      const tag = await freshModule();
      tag.loadGoogleTag('G-TEST', 'home');

      tag.setGoogleTagConsent(true); // already the default
      expect(commands()).toHaveLength(3);

      tag.setGoogleTagConsent(false);
      tag.setGoogleTagConsent(false);
      expect(commands().slice(3)).toEqual([['consent', 'update', { analytics_storage: 'denied' }]]);

      tag.setGoogleTagConsent(true);
      expect(commands().slice(4)).toEqual([['consent', 'update', { analytics_storage: 'granted' }]]);
    });
  });

  describe('setGoogleTagContentGroup', () => {
    it('says nothing to a tag that is not there', async () => {
      const tag = await freshModule();
      tag.setGoogleTagContentGroup('room');
      expect(w().dataLayer).toBeUndefined();
    });

    it('updates the config, without a page view, when the page type changes', async () => {
      const tag = await freshModule();
      tag.loadGoogleTag('G-TEST', 'home');

      tag.setGoogleTagContentGroup('home'); // unchanged
      expect(commands()).toHaveLength(3);

      tag.setGoogleTagContentGroup('room');
      tag.setGoogleTagContentGroup('room');
      expect(commands().slice(3)).toEqual([['config', 'G-TEST', { content_group: 'room', update: true }]]);
    });
  });

  describe('sendGoogleTagEvent', () => {
    it('drops an event when the tag is not there', async () => {
      const tag = await freshModule();
      tag.sendGoogleTagEvent('nav_click', { item: 'rooms' });
      expect(w().dataLayer).toBeUndefined();
    });

    it('queues an event command for a loaded tag', async () => {
      const tag = await freshModule();
      tag.loadGoogleTag('G-TEST', 'home');
      tag.sendGoogleTagEvent('nav_click', { item: 'rooms', location: 'header' });
      expect(commands()[3]).toEqual(['event', 'nav_click', { item: 'rooms', location: 'header' }]);
    });
  });

  describe('deleteGoogleAnalyticsCookies', () => {
    it('removes _ga and every _ga_<id>, set for this host or for a parent domain, and nothing else', async () => {
      document.cookie = '_ga=GA1.1.1.1; path=/';
      document.cookie = '_ga_HS74SKKSQY=GS2.1.s1; path=/; domain=example.com';
      document.cookie = '_ga_OTHER=GS2.1.s2; path=/; domain=.www.example.com';
      document.cookie = '_gallery=keep; path=/';
      document.cookie = 'theme=dark; path=/';
      expect(cookieNames()).toEqual(['_ga', '_ga_HS74SKKSQY', '_ga_OTHER', '_gallery', 'theme']);

      const tag = await freshModule();
      tag.deleteGoogleAnalyticsCookies();

      expect(cookieNames()).toEqual(['_gallery', 'theme']);
    });

    it('never throws when there is nothing to delete', async () => {
      const tag = await freshModule();
      expect(() => tag.deleteGoogleAnalyticsCookies()).not.toThrow();
    });
  });
});
