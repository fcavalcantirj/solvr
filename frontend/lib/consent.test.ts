import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// The visitor's choice about Google Analytics lives in this browser and nowhere else: pages
// are served from shared caches, so no server ever reads it. Three states (unset, granted,
// denied), one storage key, and a module that owns reading, writing and telling the page.

type ConsentModule = typeof import('./consent');

async function freshModule(): Promise<ConsentModule> {
  vi.resetModules();
  return import('./consent');
}

function setGpc(value: unknown) {
  Object.defineProperty(window.navigator, 'globalPrivacyControl', { value, configurable: true });
}

describe('consent store', () => {
  beforeEach(() => {
    window.localStorage.clear();
    setGpc(undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    setGpc(undefined);
  });

  it('is unset until the visitor chooses', async () => {
    const consent = await freshModule();
    expect(consent.getConsent()).toBe('unset');
    expect(window.localStorage.getItem('solvr_consent')).toBeNull();
  });

  it('stores Accept as {"analytics":"granted","at":<ISO time>,"v":1} under solvr_consent', async () => {
    const consent = await freshModule();
    const before = Date.now();
    consent.setConsent('granted');

    expect(consent.getConsent()).toBe('granted');
    const stored = JSON.parse(window.localStorage.getItem('solvr_consent') as string);
    expect(Object.keys(stored).sort()).toEqual(['analytics', 'at', 'v']);
    expect(stored.analytics).toBe('granted');
    expect(stored.v).toBe(1);
    expect(new Date(stored.at).toISOString()).toBe(stored.at);
    expect(new Date(stored.at).getTime()).toBeGreaterThanOrEqual(before);
  });

  it('stores Decline the same way', async () => {
    const consent = await freshModule();
    consent.setConsent('denied');

    expect(consent.getConsent()).toBe('denied');
    expect(JSON.parse(window.localStorage.getItem('solvr_consent') as string).analytics).toBe('denied');
  });

  it('lets the visitor change the choice', async () => {
    const consent = await freshModule();
    consent.setConsent('granted');
    consent.setConsent('denied');
    expect(consent.getConsent()).toBe('denied');
    consent.setConsent('granted');
    expect(consent.getConsent()).toBe('granted');
  });

  it('remembers the choice on the next page, however old it is', async () => {
    window.localStorage.setItem('solvr_consent', JSON.stringify({ analytics: 'granted', at: '2020-01-01T00:00:00.000Z', v: 1 }));
    const consent = await freshModule();
    expect(consent.getConsent()).toBe('granted');
  });

  it.each([
    ['text that is not JSON', 'granted'],
    ['a value that is not a choice', JSON.stringify({ analytics: 'maybe', at: '2026-10-05T00:00:00.000Z', v: 1 })],
    ['another version of the record', JSON.stringify({ analytics: 'granted', at: '2026-10-05T00:00:00.000Z', v: 2 })],
    ['a record with no time', JSON.stringify({ analytics: 'granted', v: 1 })],
    ['a JSON null', 'null'],
  ])('reads %s as unset', async (_name, raw) => {
    window.localStorage.setItem('solvr_consent', raw);
    const consent = await freshModule();
    expect(consent.getConsent()).toBe('unset');
  });

  describe('Global Privacy Control', () => {
    it('counts as denied when the browser sends it and nothing is stored', async () => {
      setGpc(true);
      const consent = await freshModule();
      expect(consent.getConsent()).toBe('denied');
      // Nothing was chosen, so nothing is written.
      expect(window.localStorage.getItem('solvr_consent')).toBeNull();
    });

    it.each([false, undefined, 'true', 1])('does not count when the signal is %s', async (value) => {
      setGpc(value);
      const consent = await freshModule();
      expect(consent.getConsent()).toBe('unset');
    });

    it('loses to a stored choice: Accept stays granted', async () => {
      setGpc(true);
      const consent = await freshModule();
      consent.setConsent('granted');
      expect(consent.getConsent()).toBe('granted');
    });

    it('loses to a stored choice read on a later page', async () => {
      window.localStorage.setItem('solvr_consent', JSON.stringify({ analytics: 'granted', at: '2026-10-05T00:00:00.000Z', v: 1 }));
      setGpc(true);
      const consent = await freshModule();
      expect(consent.getConsent()).toBe('granted');
    });
  });

  describe('storage that throws (private mode, blocked site data)', () => {
    it('reads as unset and never throws', async () => {
      vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
        throw new DOMException('denied', 'SecurityError');
      });
      const consent = await freshModule();
      expect(() => consent.getConsent()).not.toThrow();
      expect(consent.getConsent()).toBe('unset');
    });

    it('keeps a choice for this page when it cannot be written, and tells the page', async () => {
      vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
        throw new DOMException('full', 'QuotaExceededError');
      });
      const consent = await freshModule();
      const listener = vi.fn();
      consent.subscribeConsent(listener);

      expect(() => consent.setConsent('granted')).not.toThrow();
      expect(consent.getConsent()).toBe('granted');
      expect(listener).toHaveBeenCalledTimes(1);

      consent.setConsent('denied');
      expect(consent.getConsent()).toBe('denied');
    });

    it('prefers the choice made on this page over an older one it could not replace', async () => {
      window.localStorage.setItem('solvr_consent', JSON.stringify({ analytics: 'granted', at: '2026-10-05T00:00:00.000Z', v: 1 }));
      const consent = await freshModule();
      vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
        throw new DOMException('full', 'QuotaExceededError');
      });
      consent.setConsent('denied');
      expect(consent.getConsent()).toBe('denied');
    });

    it('is unset again on the next page: nothing was kept', async () => {
      const blocked = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
        throw new DOMException('full', 'QuotaExceededError');
      });
      const first = await freshModule();
      first.setConsent('granted');
      blocked.mockRestore();

      const next = await freshModule();
      expect(next.getConsent()).toBe('unset');
    });
  });

  describe('telling the page', () => {
    it('calls every subscriber on a change and stops after unsubscribe', async () => {
      const consent = await freshModule();
      const a = vi.fn();
      const b = vi.fn();
      const stopA = consent.subscribeConsent(a);
      consent.subscribeConsent(b);

      consent.setConsent('granted');
      expect(a).toHaveBeenCalledTimes(1);
      expect(b).toHaveBeenCalledTimes(1);

      stopA();
      consent.setConsent('denied');
      expect(a).toHaveBeenCalledTimes(1);
      expect(b).toHaveBeenCalledTimes(2);
    });

    it('hears a choice made in another tab', async () => {
      const consent = await freshModule();
      const listener = vi.fn();
      const stop = consent.subscribeConsent(listener);

      window.localStorage.setItem('solvr_consent', JSON.stringify({ analytics: 'denied', at: '2026-10-05T00:00:00.000Z', v: 1 }));
      window.dispatchEvent(new StorageEvent('storage', { key: 'solvr_consent' }));
      expect(listener).toHaveBeenCalledTimes(1);
      expect(consent.getConsent()).toBe('denied');

      // Another key is nobody's business here.
      window.dispatchEvent(new StorageEvent('storage', { key: 'auth_token' }));
      expect(listener).toHaveBeenCalledTimes(1);

      stop();
      window.dispatchEvent(new StorageEvent('storage', { key: 'solvr_consent' }));
      expect(listener).toHaveBeenCalledTimes(1);
    });
  });

  describe('Cookie settings', () => {
    it('asks whoever shows the choice to show it again', async () => {
      const consent = await freshModule();
      const listener = vi.fn();
      const stop = consent.onConsentSettingsRequest(listener);

      consent.openConsentSettings();
      expect(listener).toHaveBeenCalledTimes(1);

      stop();
      consent.openConsentSettings();
      expect(listener).toHaveBeenCalledTimes(1);
    });
  });
});
