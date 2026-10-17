import { describe, it, expect, vi, afterEach } from 'vitest';
import { NextRequest } from 'next/server';
import type { NextFetchEvent } from 'next/server';
import { unstable_doesMiddlewareMatch } from 'next/experimental/testing/server';
import { middleware, config } from './middleware';

// Consent-free page views (SPEC.md 27.6, 27.7). The web server reports every page GET to the
// API beside the response: curl and crawlers included, since classifying them is the point.
// It forwards what the request said about itself and judges nothing: the API drops a
// prefetch, reduces the path to the kind of page and classifies the view. It never delays
// the page, never fails it, and never sends the query string (it can carry a secret).

const PAGEVIEW = `${process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev'}/v1/analytics/pageview`;

const BROWSER_UA =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36';

function fakeEvent() {
  const waitUntil = vi.fn<(promise: Promise<unknown>) => void>();
  return { event: { waitUntil } as unknown as NextFetchEvent, waitUntil };
}

function stubFetch(impl?: (...args: unknown[]) => unknown) {
  const spy = vi.fn(impl ?? (() => Promise.resolve(new Response(null, { status: 202 }))));
  vi.stubGlobal('fetch', spy);
  return spy;
}

function pageViewCalls(spy: ReturnType<typeof stubFetch>) {
  return spy.mock.calls.filter(([url]) => url === PAGEVIEW) as Array<[string, RequestInit]>;
}

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('middleware: a page the site serves is reported to the API', () => {
  it('reports a browser opening a room, with what the request said about itself, and passes it through', async () => {
    const spy = stubFetch();
    const { event, waitUntil } = fakeEvent();
    const res = middleware(
      new NextRequest('https://solvr.dev/rooms/planner-room?ref=share&token=abc', {
        headers: {
          'User-Agent': BROWSER_UA,
          'Sec-Fetch-Mode': 'navigate',
          'Sec-Fetch-Dest': 'document',
          'Sec-CH-UA': '"Chromium";v="149", "Google Chrome";v="149"',
          'Accept-Language': 'pt-BR,pt;q=0.9',
        },
      }),
      event,
    );

    expect(res.headers.get('x-middleware-next')).toBe('1');
    const calls = pageViewCalls(spy);
    expect(calls).toHaveLength(1);
    const [, init] = calls[0];
    expect(init.method).toBe('POST');
    expect(new Headers(init.headers).get('user-agent')).toBe('solvr-web/1.0 (pageview report)');
    expect(JSON.parse(init.body as string)).toEqual({
      path: '/rooms/planner-room',
      user_agent: BROWSER_UA,
      request_mode: 'navigate',
      fetch_dest: 'document',
      sec_ch_ua: '"Chromium";v="149", "Google Chrome";v="149"',
      accept_language: true,
      rsc: false,
      prefetch: false,
    });
    expect(waitUntil).toHaveBeenCalledTimes(1);
    await expect(waitUntil.mock.calls[0][0]).resolves.toBeUndefined();
  });

  it('reports curl too, with nothing it did not send', () => {
    const spy = stubFetch();
    middleware(new NextRequest('https://solvr.dev/', { headers: { 'User-Agent': 'curl/8.7.1' } }), fakeEvent().event);
    const calls = pageViewCalls(spy);
    expect(calls).toHaveLength(1);
    expect(JSON.parse(calls[0][1].body as string)).toEqual({
      path: '/',
      user_agent: 'curl/8.7.1',
      request_mode: '',
      fetch_dest: '',
      sec_ch_ua: '',
      accept_language: false,
      rsc: false,
      prefetch: false,
    });
  });

  it('marks a client-side navigation, and a browser speculation as a prefetch for the API to drop', () => {
    const spy = stubFetch();
    middleware(new NextRequest('https://solvr.dev/posts', { headers: { RSC: '1', 'Sec-Fetch-Mode': 'cors' } }), fakeEvent().event);
    middleware(new NextRequest('https://solvr.dev/blog', { headers: { 'Sec-Purpose': 'prefetch;prerender' } }), fakeEvent().event);
    middleware(new NextRequest('https://solvr.dev/docs', { headers: { Purpose: 'prefetch' } }), fakeEvent().event);
    const bodies = pageViewCalls(spy).map(([, init]) => JSON.parse(init.body as string));
    expect(bodies.map((b) => [b.path, b.rsc, b.prefetch])).toEqual([
      ['/posts', true, false],
      ['/blog', false, true],
      ['/docs', false, true],
    ]);
  });

  it("never reports the app's own link prefetches", () => {
    const spy = stubFetch();
    middleware(new NextRequest('https://solvr.dev/rooms', { headers: { RSC: '1', 'Next-Router-Prefetch': '1' } }), fakeEvent().event);
    expect(pageViewCalls(spy)).toHaveLength(0);
  });

  it('cuts a long User-Agent and Sec-CH-UA to the 200 characters the API accepts', () => {
    const spy = stubFetch();
    middleware(
      new NextRequest('https://solvr.dev/', { headers: { 'User-Agent': 'a'.repeat(300), 'Sec-CH-UA': 'b'.repeat(300) } }),
      fakeEvent().event,
    );
    const body = JSON.parse(pageViewCalls(spy)[0][1].body as string);
    expect(body.user_agent).toHaveLength(200);
    expect(body.sec_ch_ua).toHaveLength(200);
  });

  it('reports nothing for a request that is not a page view', () => {
    const spy = stubFetch();
    middleware(new NextRequest('https://solvr.dev/rooms', { method: 'HEAD' }), fakeEvent().event);
    middleware(new NextRequest('https://solvr.dev/rooms', { method: 'POST' }), fakeEvent().event);
    middleware(new NextRequest('https://solvr.dev/skill.md?f=k7m2p9xq'), fakeEvent().event);
    middleware(new NextRequest('https://solvr.dev/adfa'), fakeEvent().event);
    middleware(new NextRequest('https://solvr.dev/problems/2f0c6a3e-0000-4000-8000-000000000001'), fakeEvent().event);
    expect(pageViewCalls(spy)).toHaveLength(0);
  });

  it('never fails or holds the page when the API is down or slow', async () => {
    vi.useFakeTimers();
    let signal: AbortSignal | undefined;
    stubFetch((_url, init) => {
      signal = (init as RequestInit).signal as AbortSignal;
      return new Promise((_resolve, reject) => {
        signal?.addEventListener('abort', () => reject(new Error('aborted')));
      });
    });
    const { event, waitUntil } = fakeEvent();
    const res = middleware(new NextRequest('https://solvr.dev/connect'), event);
    expect(res.headers.get('x-middleware-next')).toBe('1');
    const settled = waitUntil.mock.calls[0][0];
    await vi.advanceTimersByTimeAsync(2900);
    expect(signal?.aborted).toBe(false);
    await vi.advanceTimersByTimeAsync(200);
    expect(signal?.aborted).toBe(true);
    await expect(settled).resolves.toBeUndefined();

    stubFetch(() => Promise.reject(new TypeError('fetch failed')));
    const failing = fakeEvent();
    middleware(new NextRequest('https://solvr.dev/connect'), failing.event);
    await expect(failing.waitUntil.mock.calls[0][0]).resolves.toBeUndefined();
  });
});

describe('the middleware runs on every page and on no asset', () => {
  it.each([
    '/',
    '/rooms',
    '/rooms/planner-room',
    '/rooms/planner-room/history/2',
    '/posts/3f2c1d1e-0000-4000-8000-000000000001',
    '/api-docs',
    '/docs/guides/codex',
    '/connect',
    '/new',
    '/problems/2f0c6a3e-0000-4000-8000-000000000001',
    '/skill.md',
  ])('runs on %s', (path) => {
    expect(unstable_doesMiddlewareMatch({ config, url: path })).toBe(true);
  });

  it.each([
    '/_next/static/chunks/main.js',
    '/_next/image',
    '/favicon.ico',
    '/robots.txt',
    '/sitemap.xml',
    '/sitemap-rooms.xml',
    '/llms.txt',
    '/skill.json',
    '/scripts/solvr.sh',
    '/api/anything',
  ])('does not run on %s', (path) => {
    expect(unstable_doesMiddlewareMatch({ config, url: path })).toBe(false);
  });
});
