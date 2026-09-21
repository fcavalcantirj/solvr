import { describe, it, expect } from 'vitest';
import nextConfig from './next.config.mjs';

type Redirect = { source: string; destination: string; permanent: boolean };
type HeaderRule = { source: string; headers: { key: string; value: string }[] };

async function redirectFor(source: string): Promise<Redirect | undefined> {
  const redirects = (await nextConfig.redirects!()) as Redirect[];
  return redirects.find((r) => r.source === source);
}

async function headerRuleFor(source: string): Promise<HeaderRule | undefined> {
  const headers = (await nextConfig.headers!()) as HeaderRule[];
  return headers.find((h) => h.source === source);
}

/**
 * The header's primary navigation points at /posts and its prominent action
 * points at /connect. Neither has its own page yet, so both must still resolve
 * rather than 404 — a navigation destination that dead-ends is a broken one.
 */
describe('next.config redirects', () => {
  it('resolves the /posts navigation destination', async () => {
    const posts = await redirectFor('/posts');

    expect(posts).toBeDefined();
    expect(posts?.destination).toBe('/feed');
    // Temporary: task 44 replaces this with the real Posts collection page.
    expect(posts?.permanent).toBe(false);
  });

  it('resolves the /connect primary action', async () => {
    const connect = await redirectFor('/connect');

    expect(connect).toBeDefined();
    expect(connect?.destination).toBe('/connect/agent');
    // Temporary: task 16 replaces this with the prompt-first start flow.
    expect(connect?.permanent).toBe(false);
  });
});

describe('next.config cache headers', () => {
  it('caches /posts like the other list pages', async () => {
    const posts = await headerRuleFor('/posts');

    expect(posts).toBeDefined();
    expect(posts?.headers[0].key).toBe('Cache-Control');
  });
});
