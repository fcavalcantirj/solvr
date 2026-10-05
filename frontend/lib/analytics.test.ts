import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// lib/analytics.ts is the ONE way an event reaches Google Analytics (SPEC.md 27.7):
// track() for something that happened on this page, trackOnNextPage() for something
// followed by a full page load or done on a page the tag never sees. Both do nothing
// without consent. These tests run the real consent store and the real data layer.

type TagWindow = Window & { dataLayer?: unknown[] } & Record<string, unknown>;
const w = () => window as unknown as TagWindow;

const commands = () => (w().dataLayer ?? []).map((entry) => Array.from(entry as ArrayLike<unknown>));
const events = () => commands().filter(([name]) => name === 'event').map(([, name, params]) => [name, params]);

async function load() {
  vi.resetModules();
  const analytics = await import('./analytics');
  const tag = await import('./google-tag');
  const consent = await import('./consent');
  return { ...analytics, ...tag, ...consent };
}

function goTo(path: string) {
  window.history.pushState({}, '', path);
}

const PENDING_KEY = 'solvr_pending_events';

describe('analytics', () => {
  beforeEach(() => {
    window.localStorage.clear();
    window.sessionStorage.clear();
    delete w().dataLayer;
    document.querySelectorAll('script[src*="googletagmanager.com"]').forEach((s) => s.remove());
    goTo('/rooms');
  });

  afterEach(() => {
    vi.restoreAllMocks();
    goTo('/');
  });

  describe('track', () => {
    it('does nothing while the visitor has not chosen', async () => {
      const a = await load();
      a.track('nav_click', { item: 'rooms', location: 'header' });

      expect(w().dataLayer).toBeUndefined();
      // Nothing was kept for later either.
      a.setConsent('granted');
      a.loadGoogleTag('G-TEST', 'collection');
      a.flushTrackedEvents();
      expect(events()).toEqual([]);
    });

    it('does nothing after Decline', async () => {
      const a = await load();
      a.setConsent('denied');
      a.track('nav_click', { item: 'rooms', location: 'header' });
      expect(w().dataLayer).toBeUndefined();
    });

    it('does nothing on an untracked path, consent or not', async () => {
      const a = await load();
      a.setConsent('granted');
      a.loadGoogleTag('G-TEST', 'collection');
      goTo('/claim');
      a.track('cta_click', { item: 'claim', location: 'page' });
      expect(events()).toEqual([]);
    });

    it('hands the event to a loaded tag at once, with the kind of page it happened on', async () => {
      const a = await load();
      a.setConsent('granted');
      a.loadGoogleTag('G-TEST', 'collection');
      a.track('nav_click', { item: 'rooms', location: 'header' });

      expect(events()).toEqual([['nav_click', { item: 'rooms', location: 'header', content_group: 'collection' }]]);
    });

    it('sends an event with no parameters of its own', async () => {
      const a = await load();
      a.setConsent('granted');
      a.loadGoogleTag('G-TEST', 'collection');
      a.track('cta_click');
      expect(events()).toEqual([['cta_click', { content_group: 'collection' }]]);
    });

    it('queues in memory while the tag is not there, then flushes in order after the config', async () => {
      const a = await load();
      a.setConsent('granted');

      a.track('nav_click', { item: 'first', location: 'header' });
      goTo('/posts/abc');
      a.track('cta_click', { item: 'second', location: 'page' });
      a.track('nav_click', { item: 'third', location: 'footer' });
      expect(w().dataLayer).toBeUndefined();

      a.loadGoogleTag('G-TEST', 'post');
      a.flushTrackedEvents();

      expect(commands().map(([name]) => name)).toEqual(['consent', 'js', 'config', 'event', 'event', 'event']);
      // Each keeps the kind of page it happened on, not the one it was sent from.
      expect(events()).toEqual([
        ['nav_click', { item: 'first', location: 'header', content_group: 'collection' }],
        ['cta_click', { item: 'second', location: 'page', content_group: 'post' }],
        ['nav_click', { item: 'third', location: 'footer', content_group: 'post' }],
      ]);

      // Flushed once.
      a.flushTrackedEvents();
      expect(events()).toHaveLength(3);
    });

    it('drops what it queued when consent goes before the tag arrives', async () => {
      const a = await load();
      a.setConsent('granted');
      a.track('nav_click', { item: 'rooms', location: 'header' });

      a.setConsent('denied');
      a.dropTrackedEvents();
      a.setConsent('granted');
      a.loadGoogleTag('G-TEST', 'collection');
      a.flushTrackedEvents();
      expect(events()).toEqual([]);
    });

    it('never flushes into a withdrawn consent, even if nobody dropped the queue', async () => {
      const a = await load();
      a.setConsent('granted');
      a.track('nav_click', { item: 'rooms', location: 'header' });
      a.loadGoogleTag('G-TEST', 'collection');

      a.setConsent('denied');
      a.flushTrackedEvents();
      expect(events()).toEqual([]);
    });

    it('keeps at most fifty events waiting', async () => {
      const a = await load();
      a.setConsent('granted');
      for (let i = 0; i < 60; i++) a.track('nav_click', { item: `n${i}`, location: 'header' });
      a.loadGoogleTag('G-TEST', 'collection');
      a.flushTrackedEvents();

      expect(events()).toHaveLength(50);
      expect((events()[49][1] as { item: string }).item).toBe('n49');
    });

    it('strips an e-mail address, a Solvr key and a JWT from every string, cuts to 100 and drops undefined', async () => {
      const a = await load();
      a.setConsent('granted');
      a.loadGoogleTag('G-TEST', 'collection');
      const jwt = 'eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk';

      a.track('nav_click', {
        item: `who is ana@example.com with solvr_sk_abc123 and ${jwt}`,
        location: 'x'.repeat(300),
        surface: undefined,
        results: 7,
      });

      const [, params] = events()[0] as [string, Record<string, unknown>];
      expect(params.item).toBe('who is [redacted] with [redacted] and [redacted]');
      expect(params.location).toBe('x'.repeat(100));
      expect(params.results).toBe(7);
      expect('surface' in params).toBe(false);
      expect(JSON.stringify(params)).not.toMatch(/ana@|solvr_sk|eyJ/);
    });

    it('accepts only the names in the closed union', async () => {
      const a = await load();
      a.setConsent('granted');
      a.loadGoogleTag('G-TEST', 'collection');
      // @ts-expect-error an unknown event name is a type error
      a.track('made_up_event');
      // @ts-expect-error an unknown parameter name is a type error
      a.track('nav_click', { colour: 'red' });
      expect(a.ANALYTICS_EVENTS).toContain('nav_click');
      expect(a.ANALYTICS_EVENTS).toContain('cta_click');
    });
  });

  describe('trackOnNextPage', () => {
    it('keeps nothing while the visitor has not accepted', async () => {
      const a = await load();
      a.trackOnNextPage('cta_click', { item: 'login', location: 'page' });
      a.setConsent('denied');
      a.trackOnNextPage('cta_click', { item: 'login', location: 'page' });
      expect(window.sessionStorage.getItem(PENDING_KEY)).toBeNull();
    });

    it('stores the event, already stripped, and sends nothing yet', async () => {
      const a = await load();
      a.setConsent('granted');
      a.loadGoogleTag('G-TEST', 'account');
      goTo('/login');
      a.trackOnNextPage('cta_click', { item: 'ana@example.com', location: 'page' });

      expect(events()).toEqual([]);
      const stored = window.sessionStorage.getItem(PENDING_KEY) as string;
      expect(stored).not.toContain('ana@');
      expect(JSON.parse(stored)).toHaveLength(1);
    });

    it('sends it from the next tracked page, in order, with the kind of page it happened on', async () => {
      // The page where it happened: /claim never loads the tag.
      const first = await load();
      first.setConsent('granted');
      goTo('/claim');
      first.trackOnNextPage('cta_click', { item: 'claimed', location: 'page' });
      first.trackOnNextPage('nav_click', { item: 'second', location: 'page' });

      // A full page load later, on a tracked page: a fresh module, the tag arrives when idle.
      goTo('/settings/agents');
      const next = await load();
      next.flushTrackedEvents();
      expect(w().dataLayer).toBeUndefined(); // the tag is not there yet: they wait in storage
      expect(JSON.parse(window.sessionStorage.getItem(PENDING_KEY) as string)).toHaveLength(2);

      next.loadGoogleTag('G-TEST', 'account');
      next.flushTrackedEvents();
      expect(events()).toEqual([
        ['cta_click', { item: 'claimed', location: 'page', content_group: 'account' }],
        ['nav_click', { item: 'second', location: 'page', content_group: 'account' }],
      ]);
      expect(window.sessionStorage.getItem(PENDING_KEY)).toBeNull();

      next.flushTrackedEvents();
      expect(events()).toHaveLength(2);
    });

    it('sends an event kept from an earlier page before the ones queued on this page', async () => {
      const first = await load();
      first.setConsent('granted');
      goTo('/login');
      first.trackOnNextPage('cta_click', { item: 'earlier', location: 'page' });

      goTo('/');
      const next = await load();
      next.track('nav_click', { item: 'later', location: 'header' });
      next.loadGoogleTag('G-TEST', 'home');
      next.flushTrackedEvents();

      expect(events().map(([, params]) => (params as { item: string }).item)).toEqual(['earlier', 'later']);
    });

    it('keeps waiting while the page shown is untracked', async () => {
      const a = await load();
      a.setConsent('granted');
      a.loadGoogleTag('G-TEST', 'home');
      goTo('/auth/callback');
      a.trackOnNextPage('cta_click', { item: 'login', location: 'page' });

      a.flushTrackedEvents();
      expect(events()).toEqual([]);
      expect(window.sessionStorage.getItem(PENDING_KEY)).not.toBeNull();

      goTo('/posts');
      a.flushTrackedEvents();
      expect(events()).toHaveLength(1);
    });

    it('drops it when consent is no longer granted at the time it would be sent', async () => {
      const a = await load();
      a.setConsent('granted');
      a.loadGoogleTag('G-TEST', 'home');
      a.trackOnNextPage('cta_click', { item: 'login', location: 'page' });

      a.setConsent('denied');
      a.flushTrackedEvents();
      expect(events()).toEqual([]);
      expect(window.sessionStorage.getItem(PENDING_KEY)).toBeNull();

      a.setConsent('granted');
      a.flushTrackedEvents();
      expect(events()).toEqual([]);
    });

    it('is emptied by dropTrackedEvents', async () => {
      const a = await load();
      a.setConsent('granted');
      a.trackOnNextPage('cta_click', { item: 'login', location: 'page' });
      a.dropTrackedEvents();
      expect(window.sessionStorage.getItem(PENDING_KEY)).toBeNull();
    });

    it('ignores what it did not write: a broken value, an unknown event, a foreign parameter', async () => {
      const a = await load();
      a.setConsent('granted');
      a.loadGoogleTag('G-TEST', 'home');

      window.sessionStorage.setItem(PENDING_KEY, '{not json');
      expect(() => a.flushTrackedEvents()).not.toThrow();
      expect(events()).toEqual([]);

      window.sessionStorage.setItem(
        PENDING_KEY,
        JSON.stringify([
          { e: 'purchase', p: { item: 'x' }, g: 'home' },
          { e: 'nav_click', p: { item: 'ok', email: 'a@b.co', location: 'b@c.io' }, g: 'not-a-group' },
          'nonsense',
          null,
        ]),
      );
      a.flushTrackedEvents();
      expect(events()).toEqual([['nav_click', { item: 'ok', location: '[redacted]', content_group: 'other' }]]);
    });

    it('keeps at most twenty events waiting, the newest ones', async () => {
      const a = await load();
      a.setConsent('granted');
      for (let i = 0; i < 25; i++) a.trackOnNextPage('nav_click', { item: `n${i}`, location: 'page' });
      const stored = JSON.parse(window.sessionStorage.getItem(PENDING_KEY) as string) as Array<{ p: { item: string } }>;
      expect(stored).toHaveLength(20);
      expect(stored[0].p.item).toBe('n5');
      expect(stored[19].p.item).toBe('n24');
    });

    it('never breaks a page whose session storage throws', async () => {
      const a = await load();
      a.setConsent('granted');
      a.loadGoogleTag('G-TEST', 'home');
      vi.spyOn(Storage.prototype, 'getItem').mockImplementation((key: string) => {
        if (key === PENDING_KEY) throw new DOMException('denied', 'SecurityError');
        return JSON.stringify({ analytics: 'granted', at: '2026-10-05T00:00:00.000Z', v: 1 });
      });
      vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
        throw new DOMException('denied', 'SecurityError');
      });
      vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
        throw new DOMException('denied', 'SecurityError');
      });

      expect(() => a.trackOnNextPage('cta_click', { item: 'login', location: 'page' })).not.toThrow();
      expect(() => a.flushTrackedEvents()).not.toThrow();
      expect(() => a.dropTrackedEvents()).not.toThrow();
      expect(events()).toEqual([]);
    });
  });
});
