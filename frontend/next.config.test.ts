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
 * The header's primary navigation points at /posts and /connect. Both are real
 * pages now (app/posts/page.tsx, app/connect), so neither may be redirected
 * away: a redirect here would send the primary navigation somewhere other than
 * the page it advertises. The /posts -> /feed redirect that used to live here
 * was a placeholder from before the Posts collection page existed.
 */
describe('next.config redirects', () => {
  it('lets /posts serve its own collection page instead of redirecting it away', async () => {
    // Replaces the old 'resolves the /posts navigation destination' test, which
    // asserted the placeholder redirect /posts -> /feed. The real collection
    // page now exists, so the redirect was removed and this pins the new truth.
    expect(await redirectFor('/posts')).toBeUndefined();
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
