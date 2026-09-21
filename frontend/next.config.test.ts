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
 * The header's primary navigation points at /posts, which has no page of its
 * own yet and must still resolve rather than 404 — a navigation destination
 * that dead-ends is a broken one. /connect, by contrast, IS a page now.
 */
describe('next.config redirects', () => {
  it('resolves the /posts navigation destination', async () => {
    const posts = await redirectFor('/posts');

    expect(posts).toBeDefined();
    expect(posts?.destination).toBe('/feed');
    // Temporary: task 44 replaces this with the real Posts collection page.
    expect(posts?.permanent).toBe(false);
  });

  it('lets /connect serve its own page instead of redirecting it away', async () => {
    // /connect is now a real page: the prompt-first start flow, rendering the
    // same panel the index opens inline. A redirect here would send the
    // primary action to the old agent-claim flow, which demands a login.
    expect(await redirectFor('/connect')).toBeUndefined();
  });
});

describe('next.config cache headers', () => {
  it('caches /posts like the other list pages', async () => {
    const posts = await headerRuleFor('/posts');

    expect(posts).toBeDefined();
    expect(posts?.headers[0].key).toBe('Cache-Control');
  });
});
