// @vitest-environment node
import { describe, it, expect, vi, afterEach } from 'vitest';
import { serverFetch, SERVER_USER_AGENT } from './server-fetch';

// The site's server reads of the API name their sender (SPEC.md 27.6), so the request log
// counts them as the web server's and not as an anonymous client's. In a browser the helper
// sets nothing: the browser sends its own User-Agent.

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('serverFetch', () => {
  it('names the web server as the sender, keeping every other option', async () => {
    const spy = vi.fn(() => Promise.resolve(new Response('{}')));
    vi.stubGlobal('fetch', spy);
    await serverFetch('https://api.solvr.dev/v1/overview', {
      cache: 'no-store',
      headers: { Accept: 'application/json' },
    });
    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe('https://api.solvr.dev/v1/overview');
    expect(init.cache).toBe('no-store');
    const headers = new Headers(init.headers);
    expect(headers.get('user-agent')).toBe(SERVER_USER_AGENT);
    expect(headers.get('accept')).toBe('application/json');
    expect(SERVER_USER_AGENT).toBe('solvr-web/1.0 (ssr)');
  });

  it('works without options', async () => {
    const spy = vi.fn(() => Promise.resolve(new Response('{}')));
    vi.stubGlobal('fetch', spy);
    await serverFetch('https://api.solvr.dev/v1/status');
    const [, init] = spy.mock.calls[0] as unknown as [string, RequestInit];
    expect(new Headers(init.headers).get('user-agent')).toBe(SERVER_USER_AGENT);
  });

  it('sets nothing in a browser', async () => {
    const spy = vi.fn(() => Promise.resolve(new Response('{}')));
    vi.stubGlobal('fetch', spy);
    vi.stubGlobal('window', {});
    await serverFetch('https://api.solvr.dev/v1/status', { cache: 'no-store' });
    const [, init] = spy.mock.calls[0] as unknown as [string, RequestInit | undefined];
    expect(new Headers(init?.headers).get('user-agent')).toBeNull();
  });
});
