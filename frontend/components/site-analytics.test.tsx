import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, render } from '@testing-library/react';
import { renderToString } from 'react-dom/server';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative, sep } from 'node:path';

// Google's tag is requested only when three things hold at once (SPEC.md 27.7): the visitor
// accepted analytics, the page is not one whose address carries a secret (a claim token, a
// one-time login code, an unsubscribe token), and the page is idle. Without consent there is
// no tag, no request to Google and no cookie.
//
// This file pins WHEN SiteAnalytics acts. What it then queues for the tag (the consent
// default, the flags, the script) is pinned in lib/google-tag.test.ts, and the whole path
// through the real modules in site-analytics.integration.test.tsx.

const mockPathname = vi.fn<() => string | null>();
vi.mock('next/navigation', () => ({
  usePathname: () => mockPathname(),
}));

const tag = vi.hoisted(() => ({
  loadGoogleTag: vi.fn(),
  setGoogleTagEnabled: vi.fn(),
  setGoogleTagConsent: vi.fn(),
  setGoogleTagContentGroup: vi.fn(),
  deleteGoogleAnalyticsCookies: vi.fn(),
}));
vi.mock('@/lib/google-tag', () => tag);

const analytics = vi.hoisted(() => ({
  flushTrackedEvents: vi.fn(),
  dropTrackedEvents: vi.fn(),
}));
vi.mock('@/lib/analytics', () => analytics);

// The idle moment is handed to the test: nothing loads until it says so.
const idle = vi.hoisted(() => ({
  waiting: [] as Array<{ run: () => void; cancelled: boolean }>,
}));
vi.mock('@/lib/when-page-idle', () => ({
  whenPageIdle: vi.fn((callback: () => void) => {
    const entry = { run: callback, cancelled: false };
    idle.waiting.push(entry);
    return () => {
      entry.cancelled = true;
    };
  }),
}));

import { SiteAnalytics, isUntrackedPath } from './site-analytics';
import { setConsent } from '@/lib/consent';

/** The page becomes idle: every wait that was not cancelled is answered. */
function becomeIdle() {
  act(() => {
    for (const entry of idle.waiting.splice(0)) if (!entry.cancelled) entry.run();
  });
}

function goTo(path: string) {
  mockPathname.mockReturnValue(path);
  window.history.pushState({}, '', path);
}

describe('SiteAnalytics', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    idle.waiting.length = 0;
    window.localStorage.clear();
    goTo('/');
  });

  it('renders nothing of its own', () => {
    setConsent('granted');
    const { container } = render(<SiteAnalytics gaId="G-TEST" />);
    becomeIdle();
    expect(container).toBeEmptyDOMElement();
  });

  describe('before Accept', () => {
    it.each(['/', '/rooms/planner-room', '/posts', '/login'])('requests no tag on %s while the visitor has not chosen', (path) => {
      goTo(path);
      render(<SiteAnalytics gaId="G-TEST" />);
      becomeIdle();

      expect(tag.loadGoogleTag).not.toHaveBeenCalled();
      expect(idle.waiting).toHaveLength(0);
      // Google's own off switch is set for the id as well.
      expect(tag.setGoogleTagEnabled).toHaveBeenLastCalledWith('G-TEST', false);
    });

    it('requests no tag after Decline, deletes the Google Analytics cookies and forgets what was waiting', () => {
      setConsent('denied');
      render(<SiteAnalytics gaId="G-TEST" />);
      becomeIdle();

      expect(tag.loadGoogleTag).not.toHaveBeenCalled();
      expect(tag.setGoogleTagEnabled).toHaveBeenLastCalledWith('G-TEST', false);
      expect(tag.deleteGoogleAnalyticsCookies).toHaveBeenCalled();
      expect(analytics.dropTrackedEvents).toHaveBeenCalled();
    });

    it('deletes a Google Analytics cookie left from before anyone was asked, while nothing is chosen', () => {
      // No consent, no cookie: a visitor who came before the bar existed still carries _ga.
      render(<SiteAnalytics gaId="G-TEST" />);
      expect(tag.deleteGoogleAnalyticsCookies).toHaveBeenCalled();
    });

    it('touches nothing while the choice is not known yet (the server, and the first render in the browser)', () => {
      const html = renderToString(<SiteAnalytics gaId="G-TEST" />);
      expect(html).toBe('');
      expect(tag.setGoogleTagEnabled).not.toHaveBeenCalled();
      expect(tag.deleteGoogleAnalyticsCookies).not.toHaveBeenCalled();
      expect(tag.loadGoogleTag).not.toHaveBeenCalled();
    });
  });

  describe('after Accept', () => {
    it('waits for the page to be idle, then requests the tag with the kind of page shown', () => {
      setConsent('granted');
      goTo('/rooms/planner-room');
      render(<SiteAnalytics gaId="G-TEST" />);

      expect(tag.loadGoogleTag).not.toHaveBeenCalled();
      expect(idle.waiting).toHaveLength(1);
      expect(tag.setGoogleTagEnabled).toHaveBeenLastCalledWith('G-TEST', true);

      becomeIdle();
      expect(tag.loadGoogleTag).toHaveBeenCalledTimes(1);
      expect(tag.loadGoogleTag).toHaveBeenCalledWith('G-TEST', 'room');
    });

    it('flushes what was waiting right after the tag is requested', () => {
      setConsent('granted');
      render(<SiteAnalytics gaId="G-TEST" />);
      const flushesBefore = analytics.flushTrackedEvents.mock.invocationCallOrder.length;

      becomeIdle();
      const loaded = tag.loadGoogleTag.mock.invocationCallOrder[0];
      const flushes = analytics.flushTrackedEvents.mock.invocationCallOrder.slice(flushesBefore);
      expect(flushes.length).toBeGreaterThan(0);
      expect(Math.min(...flushes)).toBeGreaterThan(loaded);
    });

    it('requests the tag when the visitor accepts on the page, once it is idle again', () => {
      render(<SiteAnalytics gaId="G-TEST" />);
      expect(idle.waiting).toHaveLength(0);

      act(() => setConsent('granted'));
      expect(tag.loadGoogleTag).not.toHaveBeenCalled();
      expect(idle.waiting).toHaveLength(1);

      becomeIdle();
      expect(tag.loadGoogleTag).toHaveBeenCalledWith('G-TEST', 'home');
    });

    it('uses the page shown at the idle moment, not the one where the wait began', () => {
      setConsent('granted');
      goTo('/posts');
      const { rerender } = render(<SiteAnalytics gaId="G-TEST" />);

      goTo('/posts/abc');
      rerender(<SiteAnalytics gaId="G-TEST" />);
      becomeIdle();

      expect(tag.loadGoogleTag).toHaveBeenCalledTimes(1);
      expect(tag.loadGoogleTag).toHaveBeenCalledWith('G-TEST', 'post');
    });

    it('does not request it when consent goes before the page is idle', () => {
      setConsent('granted');
      render(<SiteAnalytics gaId="G-TEST" />);
      const waiting = idle.waiting[0];

      act(() => setConsent('denied'));
      expect(waiting.cancelled).toBe(true);

      becomeIdle();
      expect(tag.loadGoogleTag).not.toHaveBeenCalled();
    });
  });

  describe('untracked paths', () => {
    it.each(['/claim', '/claim/', '/auth/callback', '/email/unsubscribe'])(
      'never requests the tag on %s, whose address carries a secret, even after Accept',
      (path) => {
        setConsent('granted');
        goTo(path);
        render(<SiteAnalytics gaId="G-TEST" />);
        becomeIdle();

        expect(tag.loadGoogleTag).not.toHaveBeenCalled();
        expect(idle.waiting).toHaveLength(0);
        expect(tag.setGoogleTagEnabled).toHaveBeenLastCalledWith('G-TEST', false);
        expect(analytics.flushTrackedEvents).not.toHaveBeenCalled();
      },
    );

    it('never requests the tag when the page is unknown', () => {
      setConsent('granted');
      mockPathname.mockReturnValue(null);
      render(<SiteAnalytics gaId="G-TEST" />);
      becomeIdle();
      expect(tag.loadGoogleTag).not.toHaveBeenCalled();
    });

    it('requests the tag once the visitor moves on from a claim link', () => {
      setConsent('granted');
      goTo('/claim');
      const { rerender } = render(<SiteAnalytics gaId="G-TEST" />);
      becomeIdle();
      expect(tag.loadGoogleTag).not.toHaveBeenCalled();

      goTo('/dashboard');
      rerender(<SiteAnalytics gaId="G-TEST" />);
      becomeIdle();
      expect(tag.loadGoogleTag).toHaveBeenCalledWith('G-TEST', 'account');
      expect(tag.setGoogleTagEnabled).toHaveBeenLastCalledWith('G-TEST', true);
    });

    it('switches a tag that is already in the page off for an untracked page, and on again after it', () => {
      setConsent('granted');
      goTo('/rooms');
      const { rerender } = render(<SiteAnalytics gaId="G-TEST" />);
      becomeIdle();

      goTo('/claim');
      rerender(<SiteAnalytics gaId="G-TEST" />);
      expect(tag.setGoogleTagEnabled).toHaveBeenLastCalledWith('G-TEST', false);
      // The visitor did not withdraw anything: cookies and consent stay.
      expect(tag.deleteGoogleAnalyticsCookies).not.toHaveBeenCalled();
      expect(tag.setGoogleTagConsent).not.toHaveBeenCalledWith(false);

      goTo('/rooms');
      rerender(<SiteAnalytics gaId="G-TEST" />);
      expect(tag.setGoogleTagEnabled).toHaveBeenLastCalledWith('G-TEST', true);
    });
  });

  describe('client-side navigation', () => {
    it('tells the tag the kind of the new page and sends what was waiting, without requesting the tag again', () => {
      setConsent('granted');
      goTo('/');
      const { rerender } = render(<SiteAnalytics gaId="G-TEST" />);
      becomeIdle();
      expect(tag.loadGoogleTag).toHaveBeenCalledTimes(1);
      analytics.flushTrackedEvents.mockClear();

      goTo('/rooms/planner-room/history/2');
      rerender(<SiteAnalytics gaId="G-TEST" />);

      expect(tag.setGoogleTagContentGroup).toHaveBeenLastCalledWith('transcript');
      expect(analytics.flushTrackedEvents).toHaveBeenCalled();
      expect(idle.waiting).toHaveLength(0);
      becomeIdle();
      expect(tag.loadGoogleTag).toHaveBeenCalledTimes(1);
    });
  });

  describe('Decline after Accept', () => {
    it('switches the tag off first, withdraws consent from it, deletes its cookies and forgets what was waiting', () => {
      setConsent('granted');
      render(<SiteAnalytics gaId="G-TEST" />);
      becomeIdle();
      vi.clearAllMocks();

      act(() => setConsent('denied'));

      expect(tag.setGoogleTagEnabled).toHaveBeenCalledWith('G-TEST', false);
      expect(tag.setGoogleTagConsent).toHaveBeenCalledWith(false);
      expect(tag.deleteGoogleAnalyticsCookies).toHaveBeenCalledTimes(1);
      expect(analytics.dropTrackedEvents).toHaveBeenCalled();
      const order = (fn: { mock: { invocationCallOrder: number[] } }) => fn.mock.invocationCallOrder[0];
      expect(order(tag.setGoogleTagEnabled)).toBeLessThan(order(tag.setGoogleTagConsent));
      expect(order(tag.setGoogleTagConsent)).toBeLessThan(order(tag.deleteGoogleAnalyticsCookies));
    });

    it('switches it on again when the visitor accepts again', () => {
      setConsent('granted');
      render(<SiteAnalytics gaId="G-TEST" />);
      becomeIdle();
      act(() => setConsent('denied'));
      vi.clearAllMocks();

      act(() => setConsent('granted'));
      expect(tag.setGoogleTagEnabled).toHaveBeenLastCalledWith('G-TEST', true);
      expect(tag.setGoogleTagConsent).toHaveBeenLastCalledWith(true);
      expect(tag.deleteGoogleAnalyticsCookies).not.toHaveBeenCalled();
    });

    it('follows a Decline made in another tab', () => {
      setConsent('granted');
      render(<SiteAnalytics gaId="G-TEST" />);
      becomeIdle();
      vi.clearAllMocks();

      act(() => {
        window.localStorage.setItem('solvr_consent', JSON.stringify({ analytics: 'denied', at: '2026-10-05T00:00:00.000Z', v: 1 }));
        window.dispatchEvent(new StorageEvent('storage', { key: 'solvr_consent' }));
      });
      expect(tag.setGoogleTagEnabled).toHaveBeenLastCalledWith('G-TEST', false);
      expect(tag.deleteGoogleAnalyticsCookies).toHaveBeenCalled();
    });
  });
});

describe('root layout mounts Google Analytics only through SiteAnalytics', () => {
  const layout = readFileSync(join(__dirname, '..', 'app', 'layout.tsx'), 'utf8');

  it('renders SiteAnalytics and names neither the tag nor a component for it', () => {
    expect(layout).toMatch(/<SiteAnalytics\b/);
    expect(layout).not.toMatch(/GoogleAnalytics/);
    expect(layout).not.toMatch(/googletagmanager|gtag\(|dataLayer/);
  });

  it('takes the measurement id from NEXT_PUBLIC_GA_ID, with the fallback', () => {
    expect(layout).toMatch(/process\.env\.NEXT_PUBLIC_GA_ID \|\| 'G-HS74SKKSQY'/);
  });
});

// One file names Google's tag host, and nothing loads the tag around it.
describe('the tag is requested from one place', () => {
  const root = join(__dirname, '..');
  const sources: string[] = [];
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir)) {
      const full = join(dir, entry);
      if (statSync(full).isDirectory()) {
        if (entry !== 'node_modules' && !entry.startsWith('.')) walk(full);
      } else if (/\.(ts|tsx|js|jsx|mjs)$/.test(entry) && !/\.test\.(ts|tsx)$/.test(entry)) {
        sources.push(full);
      }
    }
  };
  for (const dir of ['app', 'components', 'lib', 'hooks']) walk(join(root, dir));
  const naming = (pattern: RegExp) =>
    sources.filter((file) => pattern.test(readFileSync(file, 'utf8'))).map((file) => relative(root, file).split(sep).join('/'));

  it('only lib/google-tag.ts names googletagmanager.com', () => {
    expect(sources.length).toBeGreaterThan(50);
    expect(naming(/googletagmanager\.com/)).toEqual(['lib/google-tag.ts']);
  });

  it('nothing imports a ready-made Google Analytics component', () => {
    expect(naming(/@next\/third-parties|@vercel\/analytics/)).toEqual([]);
  });

  it('only lib/google-tag.ts writes to the data layer', () => {
    expect(naming(/\bdataLayer\b/)).toEqual(['lib/google-tag.ts']);
  });
});

// Every page that reads a credential out of its own address must be untracked.
// A new page that starts reading ?token= fails here until it is listed.
describe('pages that read a secret from their address are untracked', () => {
  const SECRET_FROM_ADDRESS =
    /searchParams\??\.get\(\s*["'](token|code|key|secret|api_key|access_token)["']|location\.hash/;
  const appDir = join(__dirname, '..', 'app');

  const pages: string[] = [];
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir)) {
      const full = join(dir, entry);
      if (statSync(full).isDirectory()) {
        walk(full);
      } else if (entry === 'page.tsx' || entry === 'page.ts') {
        pages.push(full);
      }
    }
  };
  walk(appDir);

  const routeOf = (file: string) =>
    '/' +
    relative(appDir, file)
      .split(sep)
      .slice(0, -1)
      .filter((segment) => !/^\(.*\)$/.test(segment))
      .join('/');

  const readers = pages.filter((file) => SECRET_FROM_ADDRESS.test(readFileSync(file, 'utf8')));

  it('finds the pages known to read one', () => {
    expect(readers.map(routeOf).sort()).toEqual(
      expect.arrayContaining(['/auth/callback', '/claim', '/email/unsubscribe']),
    );
  });

  it.each(readers.map((file) => [routeOf(file)]))('%s is untracked', (route) => {
    expect(isUntrackedPath(route)).toBe(true);
  });
});
