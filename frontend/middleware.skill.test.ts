import { describe, it, expect, vi, afterEach } from 'vitest';
import { NextRequest } from 'next/server';
import type { NextFetchEvent } from 'next/server';
import { middleware, config } from './middleware';

// The sentence of GET /v1/connect links the skill as /skill.md?f=<flow code>. The web
// server reports every GET of that link to the API (funnel step skill_fetched, SPEC.md
// 25.7) beside the response: it never delays the skill, never fails it, and judges
// nothing itself (the API validates the code and decides what the fetch was).

// The API base the rest of the web client uses (inlined at build time).
const FUNNEL = `${process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev'}/v1/analytics/funnel`;

function fakeEvent() {
  const waitUntil = vi.fn<(promise: Promise<unknown>) => void>();
  return { event: { waitUntil } as unknown as NextFetchEvent, waitUntil };
}

function stubFetch(impl?: (...args: unknown[]) => unknown) {
  const spy = vi.fn(impl ?? (() => Promise.resolve(new Response(null, { status: 202 }))));
  vi.stubGlobal('fetch', spy);
  return spy;
}

function sentBody(spy: ReturnType<typeof stubFetch>, call = 0) {
  const init = spy.mock.calls[call][1] as RequestInit;
  return JSON.parse(init.body as string);
}

// A pass-through is NextResponse.next(): the request goes on to the file in public/.
function expectPassThrough(res: Response) {
  expect(res.status).toBe(200);
  expect(res.headers.get('x-middleware-next')).toBe('1');
  expect(res.headers.get('location')).toBeNull();
}

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe('middleware: a fetch of the skill link is reported to the API', () => {
  it('runs on /skill.md', () => {
    expect(config.matcher).toContain('/skill.md');
  });

  it('reports once for /skill.md?f=<code>, beside the response, and passes the request through', async () => {
    const spy = stubFetch();
    const { event, waitUntil } = fakeEvent();

    const res = middleware(new NextRequest('https://solvr.dev/skill.md?f=k7m2p9xq'), event);

    expectPassThrough(res);
    expect(spy).toHaveBeenCalledTimes(1);
    const [url, init] = spy.mock.calls[0] as [string, RequestInit];
    expect(url).toBe(FUNNEL);
    expect(init.method).toBe('POST');
    expect(new Headers(init.headers).get('content-type')).toBe('application/json');
    expect(new Headers(init.headers).get('user-agent')).toBe('solvr-web/1.0 (skill-fetch report)');
    expect(sentBody(spy)).toEqual({ event: 'skill_fetched', flow_id: 'k7m2p9xq', request_mode: '', user_agent: '' });
    // The report outlives the response through waitUntil, and that promise never rejects.
    expect(waitUntil).toHaveBeenCalledTimes(1);
    await expect(waitUntil.mock.calls[0][0]).resolves.toBeUndefined();
  });

  it('hands the request\'s Sec-Fetch-Mode to the API as request_mode, untouched', () => {
    const spy = stubFetch();
    for (const mode of ['navigate', 'cors', 'no-cors']) {
      middleware(
        new NextRequest('https://solvr.dev/skill.md?f=k7m2p9xq', { headers: { 'Sec-Fetch-Mode': mode } }),
        fakeEvent().event,
      );
    }
    expect(spy.mock.calls.map((_, i) => sentBody(spy, i).request_mode)).toEqual(['navigate', 'cors', 'no-cors']);
  });

  // A bot is not an agent: the app a sentence was pasted into fetches the link to build a
  // preview, and a crawler may follow it. The web server does not tell them apart; it hands
  // the request's User-Agent to the API, which does (SPEC.md 25.7).
  it('hands the request\'s User-Agent to the API as user_agent, raw', () => {
    const spy = stubFetch();
    const agents = [
      'Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)',
      'Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)',
      'curl/8.7.1',
      'Mixed CASE,  two  spaces; and "quotes" \\ kept/1.0',
    ];
    for (const agent of agents) {
      middleware(
        new NextRequest('https://solvr.dev/skill.md?f=k7m2p9xq', { headers: { 'User-Agent': agent } }),
        fakeEvent().event,
      );
    }
    expect(spy.mock.calls.map((_, i) => sentBody(spy, i).user_agent)).toEqual(agents);
    // Nothing else in the report changes with it.
    expect(sentBody(spy, 0)).toEqual({
      event: 'skill_fetched',
      flow_id: 'k7m2p9xq',
      request_mode: '',
      user_agent: agents[0],
    });
  });

  it('sends an empty user_agent when the request names none', () => {
    const spy = stubFetch();
    middleware(new NextRequest('https://solvr.dev/skill.md?f=k7m2p9xq'), fakeEvent().event);
    expect(sentBody(spy).user_agent).toBe('');
  });

  it('cuts a long User-Agent to its first 200 characters', () => {
    const spy = stubFetch();
    const long = `${'a'.repeat(150)} Googlebot/2.1 ${'z'.repeat(300)}`;
    const exact = 'b'.repeat(200);
    // Googlebot's smartphone agent is 200 characters long: it is sent whole.
    const googlebotPhone =
      'Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) ' +
      'Chrome/141.0.7390.122 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)';
    for (const agent of [long, exact, googlebotPhone, `${exact}c`]) {
      middleware(
        new NextRequest('https://solvr.dev/skill.md?f=k7m2p9xq', { headers: { 'User-Agent': agent } }),
        fakeEvent().event,
      );
    }
    const sent = spy.mock.calls.map((_, i) => sentBody(spy, i).user_agent as string);
    expect(sent[0]).toHaveLength(200);
    expect(sent[0]).toBe(long.slice(0, 200));
    expect(sent[0]).toContain('Googlebot');
    expect(sent[1]).toBe(exact);
    expect(googlebotPhone).toHaveLength(200);
    expect(sent[2]).toBe(googlebotPhone);
    expect(sent[3]).toBe(exact);
  });

  it('reports under its own name: the fetcher\'s User-Agent travels in the body only', () => {
    const spy = stubFetch();
    middleware(
      new NextRequest('https://solvr.dev/skill.md?f=k7m2p9xq', { headers: { 'User-Agent': 'Twitterbot/1.0' } }),
      fakeEvent().event,
    );
    const init = spy.mock.calls[0][1] as RequestInit;
    expect(new Headers(init.headers).get('user-agent')).toBe('solvr-web/1.0 (skill-fetch report)');
    expect(sentBody(spy).user_agent).toBe('Twitterbot/1.0');
  });

  it('judges nothing: whatever f holds is sent, and only the API decides what it is worth', () => {
    const spy = stubFetch();
    middleware(new NextRequest('https://solvr.dev/skill.md?f=NOT-A-CODE&utm_source=x'), fakeEvent().event);
    middleware(new NextRequest(`https://solvr.dev/skill.md?f=${'a'.repeat(64)}`), fakeEvent().event);
    expect(sentBody(spy, 0)).toEqual({ event: 'skill_fetched', flow_id: 'NOT-A-CODE', request_mode: '', user_agent: '' });
    expect(sentBody(spy, 1).flow_id).toBe('a'.repeat(64));
    expect(spy).toHaveBeenCalledTimes(2);
  });

  it('reports nothing for /skill.md without a usable f, and still passes the request through', () => {
    const spy = stubFetch();
    for (const url of [
      'https://solvr.dev/skill.md',
      'https://solvr.dev/skill.md?f=',
      'https://solvr.dev/skill.md?flow=k7m2p9xq',
      'https://solvr.dev/skill.md?F=k7m2p9xq',
      `https://solvr.dev/skill.md?f=${'a'.repeat(65)}`,
    ]) {
      const { event, waitUntil } = fakeEvent();
      expectPassThrough(middleware(new NextRequest(url), event));
      expect(waitUntil).not.toHaveBeenCalled();
    }
    expect(spy).not.toHaveBeenCalled();
  });

  it('reports only a GET', () => {
    const spy = stubFetch();
    for (const method of ['HEAD', 'POST', 'OPTIONS']) {
      expectPassThrough(
        middleware(new NextRequest('https://solvr.dev/skill.md?f=k7m2p9xq', { method }), fakeEvent().event),
      );
    }
    expect(spy).not.toHaveBeenCalled();
  });

  it('reports only the skill itself, not a path that looks like it', () => {
    const spy = stubFetch();
    for (const path of ['/skill.md/extra', '/skill', '/skill.mdx', '/heartbeat.md', '/docs/skill.md']) {
      middleware(new NextRequest(`https://solvr.dev${path}?f=k7m2p9xq`), fakeEvent().event);
    }
    expect(spy).not.toHaveBeenCalled();
  });

  it('does not read the API\'s answer and releases it', async () => {
    const answer = new Response('{"data":{"recorded":true}}', { status: 202 });
    const cancel = vi.spyOn(answer.body as ReadableStream, 'cancel');
    stubFetch(() => Promise.resolve(answer));
    const { event, waitUntil } = fakeEvent();

    expectPassThrough(middleware(new NextRequest('https://solvr.dev/skill.md?f=k7m2p9xq'), event));

    await expect(waitUntil.mock.calls[0][0]).resolves.toBeUndefined();
    expect(cancel).toHaveBeenCalledTimes(1);
  });

  it('never fails the response when the report is rejected', async () => {
    stubFetch(() => Promise.reject(new Error('the API is away')));
    const { event, waitUntil } = fakeEvent();

    const res = middleware(new NextRequest('https://solvr.dev/skill.md?f=k7m2p9xq'), event);

    expectPassThrough(res);
    await expect(waitUntil.mock.calls[0][0]).resolves.toBeUndefined();
  });

  it('never throws when the report cannot even be started', async () => {
    stubFetch(() => {
      throw new Error('fetch is broken');
    });
    const { event, waitUntil } = fakeEvent();

    let res: Response | undefined;
    expect(() => {
      res = middleware(new NextRequest('https://solvr.dev/skill.md?f=k7m2p9xq'), event);
    }).not.toThrow();

    expectPassThrough(res as Response);
    await expect(waitUntil.mock.calls[0][0]).resolves.toBeUndefined();
  });

  it('does not wait for the report: the response is ready while the API has not answered', () => {
    const spy = stubFetch(() => new Promise(() => {})); // an API that never answers
    const { event, waitUntil } = fakeEvent();

    const res = middleware(new NextRequest('https://solvr.dev/skill.md?f=k7m2p9xq'), event);

    // middleware() is synchronous: it returned a response, not a promise of one.
    expect(res).not.toBeInstanceOf(Promise);
    expectPassThrough(res);
    expect(spy).toHaveBeenCalledTimes(1);
    expect(waitUntil).toHaveBeenCalledTimes(1);
  });

  it('gives up on the report after about 3 seconds', async () => {
    vi.useFakeTimers();
    let signal: AbortSignal | undefined;
    stubFetch((_url, init) => {
      signal = (init as RequestInit).signal as AbortSignal;
      return new Promise((_resolve, reject) => {
        signal?.addEventListener('abort', () => reject(new Error('aborted')));
      });
    });
    const { event, waitUntil } = fakeEvent();
    middleware(new NextRequest('https://solvr.dev/skill.md?f=k7m2p9xq'), event);
    const report = waitUntil.mock.calls[0][0];

    await vi.advanceTimersByTimeAsync(2900);
    expect(signal?.aborted).toBe(false);
    await vi.advanceTimersByTimeAsync(200);
    expect(signal?.aborted).toBe(true);
    await expect(report).resolves.toBeUndefined();
  });

  it('still reports when the runtime hands it no event to extend', () => {
    const spy = stubFetch();
    const res = middleware(new NextRequest('https://solvr.dev/skill.md?f=k7m2p9xq'));
    expectPassThrough(res);
    expect(spy).toHaveBeenCalledTimes(1);
  });

  it('leaves the legacy redirects and the block exactly as they were, without reporting', () => {
    const spy = stubFetch();
    const redirect = middleware(new NextRequest('https://solvr.dev/problems/abc-123?f=k7m2p9xq&sort=top'), fakeEvent().event);
    expect(redirect.status).toBe(308);
    const location = new URL(redirect.headers.get('location') as string);
    expect(location.pathname).toBe('/posts/abc-123');
    expect(location.search).toBe('?f=k7m2p9xq&sort=top');
    expect(middleware(new NextRequest('https://solvr.dev/feed'), fakeEvent().event).status).toBe(308);
    expect(middleware(new NextRequest('https://solvr.dev/new?f=k7m2p9xq'), fakeEvent().event).status).toBe(308);
    expect(middleware(new NextRequest('https://solvr.dev/adfa?f=k7m2p9xq'), fakeEvent().event).status).toBe(404);
    expect(spy).not.toHaveBeenCalled();
  });
});
