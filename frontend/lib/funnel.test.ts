import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative, sep } from 'node:path';

// lib/funnel.ts is the ONE place a browser step of the connection funnel is reported
// (SPEC.md 25.7), and each of its functions also sends the Google Analytics event that
// pairs with the step (27.7), built from the same arguments, so the two cannot drift.
//
//   the funnel step   always: it is first-party, sets no cookie and needs no consent
//   the GA event      only what track() lets through: consent granted, a tracked page
//
// These tests run the real consent store, the real track() and the real data layer. Only
// the API client is replaced.

const postFunnelEvent = vi.hoisted(() => vi.fn());
vi.mock('./api', () => ({ api: { postFunnelEvent } }));

type TagWindow = Window & { dataLayer?: unknown[] } & Record<string, unknown>;
const w = () => window as unknown as TagWindow;
const events = () =>
  (w().dataLayer ?? [])
    .map((entry) => Array.from(entry as ArrayLike<unknown>))
    .filter(([name]) => name === 'event')
    .map(([, name, params]) => [name, params]);

async function load() {
  vi.resetModules();
  const funnel = await import('./funnel');
  const tag = await import('./google-tag');
  const consent = await import('./consent');
  return { ...funnel, ...tag, ...consent };
}

function goTo(path: string) {
  window.history.pushState({}, '', path);
}

type Loaded = Awaited<ReturnType<typeof load>>;

// Every report: how it is called, the funnel step it must post, the GA event it must send.
const REPORTS: Array<{
  name: string;
  path: string;
  group: string;
  call: (f: Loaded) => void;
  step: Record<string, unknown>;
  ga: [string, Record<string, unknown>] | null;
}> = [
  {
    name: 'reportConnectionStarted on /connect',
    path: '/connect',
    group: 'connect',
    call: (f) => f.reportConnectionStarted({ flowId: 'abcdefgh', surface: 'connect_page', preset: 'plan-and-build', instructionVersion: '2.1' }),
    step: { event: 'connection_started', flow_id: 'abcdefgh', entry_surface: 'connect_page', preset: 'plan-and-build', instruction_version: '2.1' },
    ga: ['connect_start', { surface: 'connect_page' }],
  },
  {
    name: 'reportConnectionStarted from a public source',
    path: '/connect',
    group: 'connect',
    call: (f) =>
      f.reportConnectionStarted({
        flowId: 'abcdefgh',
        surface: 'connect_page',
        preset: 'plan-and-build',
        instructionVersion: '2.1',
        source: { kind: 'room', ref: 'demo-room' },
      }),
    step: {
      event: 'connection_started',
      flow_id: 'abcdefgh',
      entry_surface: 'connect_page',
      preset: 'plan-and-build',
      instruction_version: '2.1',
      source: { kind: 'room', ref: 'demo-room' },
    },
    ga: ['connect_start', { surface: 'connect_page' }],
  },
  {
    name: 'reportPromptCopied in the panel the home page opens',
    path: '/',
    group: 'home',
    call: (f) =>
      f.reportPromptCopied({ flowId: 'abcdefgh', surface: 'homepage_panel', preset: 'review-my-code', role: 'builder', instructionVersion: '2.1' }),
    step: {
      event: 'starter_prompt_copied',
      flow_id: 'abcdefgh',
      entry_surface: 'homepage_panel',
      preset: 'review-my-code',
      role: 'builder',
      instruction_version: '2.1',
    },
    ga: ['prompt_copy', { surface: 'homepage_panel', preset: 'review-my-code', role: 'builder' }],
  },
  {
    name: 'reportPromptCopied on a home page card (no flow)',
    path: '/',
    group: 'home',
    call: (f) => f.reportPromptCopied({ surface: 'homepage_use_cases', preset: 'plan-and-build', role: 'planner' }),
    step: { event: 'starter_prompt_copied', entry_surface: 'homepage_use_cases', preset: 'plan-and-build', role: 'planner' },
    ga: ['prompt_copy', { surface: 'homepage_use_cases', preset: 'plan-and-build', role: 'planner' }],
  },
  {
    name: 'reportPromptCopied on a guide',
    path: '/docs/guides/connect-planner-executor',
    group: 'guide',
    call: (f) => f.reportPromptCopied({ surface: 'guide_page', preset: 'plan-and-build', role: 'planner' }),
    step: { event: 'starter_prompt_copied', entry_surface: 'guide_page', preset: 'plan-and-build', role: 'planner' },
    ga: ['prompt_copy', { surface: 'guide_page', preset: 'plan-and-build', role: 'planner' }],
  },
  {
    name: "reportPromptCopied for a new room's starter prompt",
    path: '/rooms/demo-room',
    group: 'room',
    call: (f) => f.reportPromptCopied({ surface: 'room_starter_prompts', role: 'executor', source: { kind: 'room', ref: 'demo-room' } }),
    step: { event: 'starter_prompt_copied', entry_surface: 'room_starter_prompts', role: 'executor', source: { kind: 'room', ref: 'demo-room' } },
    ga: ['prompt_copy', { surface: 'room_starter_prompts', role: 'executor' }],
  },
  {
    name: 'reportJoinPromptCopied',
    path: '/rooms/demo-room',
    group: 'room',
    call: (f) => f.reportJoinPromptCopied({ role: 'collaborator' }),
    step: { event: 'join_prompt_copied', role: 'collaborator', entry_surface: 'room_page' },
    ga: ['join_prompt_copy', { surface: 'room_page' }],
  },
  {
    name: 'reportRoomShared through the share sheet',
    path: '/rooms/demo-room',
    group: 'room',
    call: (f) => f.reportRoomShared({ slug: 'demo-room', method: 'share_sheet' }),
    step: { event: 'share_link_copied', entry_surface: 'room_page', source: { kind: 'room', ref: 'demo-room' } },
    ga: ['room_share', { method: 'share_sheet' }],
  },
  {
    name: 'reportRoomShared through the clipboard',
    path: '/rooms/demo-room',
    group: 'room',
    call: (f) => f.reportRoomShared({ slug: 'demo-room', method: 'clipboard' }),
    step: { event: 'share_link_copied', entry_surface: 'room_page', source: { kind: 'room', ref: 'demo-room' } },
    ga: ['room_share', { method: 'clipboard' }],
  },
  {
    name: 'reportOutcomeCopied',
    path: '/rooms/demo-room',
    group: 'room',
    call: (f) => f.reportOutcomeCopied({ slug: 'demo-room' }),
    step: { event: 'share_link_copied', entry_surface: 'room_page', source: { kind: 'room', ref: 'demo-room' } },
    ga: ['outcome_copy', {}],
  },
  {
    name: 'reportShareVisit to a post',
    path: '/posts/p1',
    group: 'post',
    call: (f) => f.reportShareVisit({ source: { kind: 'post', ref: 'p1' }, surface: 'post_page' }),
    step: { event: 'share_visit', entry_surface: 'post_page', source: { kind: 'post', ref: 'p1' } },
    ga: ['share_visit', { surface: 'post_page' }],
  },
  {
    name: 'reportRoomViewed (the page view already says it: no second event)',
    path: '/rooms/demo-room',
    group: 'room',
    call: (f) => f.reportRoomViewed(),
    step: { event: 'room_viewed' },
    ga: null,
  },
];

describe('funnel reports', () => {
  beforeEach(() => {
    postFunnelEvent.mockReset();
    postFunnelEvent.mockResolvedValue(undefined);
    window.localStorage.clear();
    window.sessionStorage.clear();
    delete w().dataLayer;
    document.querySelectorAll('script[src*="googletagmanager.com"]').forEach((s) => s.remove());
  });

  afterEach(() => {
    goTo('/');
  });

  describe.each(REPORTS)('$name', ({ path, group, call, step, ga }) => {
    it('posts the funnel step, exactly, while nothing was chosen: and nothing goes to Google', async () => {
      goTo(path);
      const f = await load();
      call(f);

      expect(postFunnelEvent).toHaveBeenCalledTimes(1);
      expect(postFunnelEvent.mock.calls[0][0]).toStrictEqual(step);
      expect(w().dataLayer).toBeUndefined();
      // Nothing was kept for later either.
      f.setConsent('granted');
      f.loadGoogleTag('G-TEST', 'home');
      const { flushTrackedEvents } = await import('./analytics');
      flushTrackedEvents();
      expect(events()).toEqual([]);
    });

    it('posts the same funnel step after Decline, and still nothing goes to Google', async () => {
      goTo(path);
      const f = await load();
      f.setConsent('denied');
      call(f);

      expect(postFunnelEvent).toHaveBeenCalledTimes(1);
      expect(postFunnelEvent.mock.calls[0][0]).toStrictEqual(step);
      expect(w().dataLayer).toBeUndefined();
    });

    it('after Accept sends both: the same funnel step, and its Google Analytics event, once each', async () => {
      goTo(path);
      const f = await load();
      f.setConsent('granted');
      f.loadGoogleTag('G-TEST', 'home');
      call(f);

      expect(postFunnelEvent).toHaveBeenCalledTimes(1);
      expect(postFunnelEvent.mock.calls[0][0]).toStrictEqual(step);
      expect(events()).toEqual(ga ? [[ga[0], { ...ga[1], content_group: group }]] : []);
    });
  });

  it('pairs every step with the event SPEC.md 27.7 names for it', () => {
    const pairs = REPORTS.map((r) => `${r.step.event} -> ${r.ga ? r.ga[0] : '(none)'}`);
    expect([...new Set(pairs)].sort()).toEqual(
      [
        'connection_started -> connect_start',
        'join_prompt_copied -> join_prompt_copy',
        'room_viewed -> (none)',
        'share_link_copied -> outcome_copy',
        'share_link_copied -> room_share',
        'share_visit -> share_visit',
        'starter_prompt_copied -> prompt_copy',
      ].sort(),
    );
  });

  it('keeps counting funnel steps on a page Google never sees', async () => {
    goTo('/claim');
    const f = await load();
    f.setConsent('granted');
    f.loadGoogleTag('G-TEST', 'account');
    f.reportPromptCopied({ surface: 'homepage_use_cases', preset: 'plan-and-build', role: 'planner' });

    expect(postFunnelEvent).toHaveBeenCalledTimes(1);
    expect(events()).toEqual([]);
  });

  it('sends the Google Analytics event even where the API client has no funnel method, and never throws', async () => {
    vi.resetModules();
    vi.doMock('./api', () => ({ api: {} }));
    try {
      goTo('/connect');
      const funnel = await import('./funnel');
      const tag = await import('./google-tag');
      const consent = await import('./consent');
      consent.setConsent('granted');
      tag.loadGoogleTag('G-TEST', 'connect');

      expect(() => funnel.reportConnectionStarted({ surface: 'connect_page' })).not.toThrow();
      expect(events()).toEqual([['connect_start', { surface: 'connect_page', content_group: 'connect' }]]);
    } finally {
      vi.doMock('./api', () => ({ api: { postFunnelEvent } }));
    }
  });

  it('never puts a sentence, an intent, a title or a link into either report', async () => {
    goTo('/connect');
    const f = await load();
    f.setConsent('granted');
    f.loadGoogleTag('G-TEST', 'connect');
    for (const report of REPORTS) report.call(f);

    const sent = JSON.stringify([postFunnelEvent.mock.calls, events()]);
    expect(sent).not.toMatch(/skill\.md|https?:|Learn Solvr|prompt\.text|intent|title/i);
    // What a report may hold: the names below and short lowercase ids.
    for (const [, params] of events()) {
      for (const [key, value] of Object.entries(params as Record<string, unknown>)) {
        expect(['surface', 'preset', 'role', 'method', 'content_group']).toContain(key);
        expect(String(value)).toMatch(/^[a-z][a-z0-9_-]*$/);
      }
    }
  });
});

// Nothing else in the site reports a funnel step: a step sent from anywhere else would have
// no Google Analytics pair, which is exactly the drift this module exists to prevent.
describe('lib/funnel.ts is the only reporter of browser funnel steps', () => {
  const ROOT = join(__dirname, '..');
  const sources: string[] = [];
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir)) {
      const full = join(dir, entry);
      if (statSync(full).isDirectory()) {
        if (entry !== 'node_modules' && !entry.startsWith('.')) walk(full);
      } else if (/\.(ts|tsx)$/.test(entry) && !/\.test\.(ts|tsx)$/.test(entry)) {
        sources.push(full);
      }
    }
  };
  for (const dir of ['app', 'components', 'lib', 'hooks']) walk(join(ROOT, dir));
  const rel = (file: string) => relative(ROOT, file).split(sep).join('/');

  it('finds the source to check', () => {
    expect(sources.length).toBeGreaterThan(200);
  });

  it('no other shipped file calls api.postFunnelEvent', () => {
    const callers = sources.filter((file) => /postFunnelEvent\s*\??\.?\s*\(/.test(readFileSync(file, 'utf8'))).map(rel);
    // lib/api.ts defines the method; lib/funnel.ts is its one caller.
    expect(callers.sort()).toEqual(['lib/api.ts', 'lib/funnel.ts']);
  });

  it('every browser step the API accepts has a report here', () => {
    const funnel = readFileSync(join(ROOT, 'lib', 'funnel.ts'), 'utf8');
    for (const step of ['connection_started', 'starter_prompt_copied', 'room_viewed', 'join_prompt_copied', 'share_visit', 'share_link_copied']) {
      expect(funnel, step).toContain(`'${step}'`);
    }
    // skill_fetched is the web server's step (frontend/middleware.ts), never a browser's.
    expect(funnel).not.toContain("event: 'skill_fetched'");
  });
});
