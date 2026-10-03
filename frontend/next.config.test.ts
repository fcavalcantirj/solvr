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
  // The index is server-rendered with the hero numbers and revalidates every
  // ~60s (app/page.tsx). A shared cache may keep it only as long, so a cached
  // page never shows numbers much older than the page itself would.
  it('lets a shared cache keep the index for 60 seconds, matching its revalidate', async () => {
    const index = await headerRuleFor('/');

    expect(index?.headers).toEqual([
      { key: 'Cache-Control', value: 'public, s-maxage=60, stale-while-revalidate=300' },
    ]);
  });

  it('caches /posts like the other list pages', async () => {
    const posts = await headerRuleFor('/posts');

    expect(posts).toBeDefined();
    expect(posts?.headers[0].key).toBe('Cache-Control');
  });
});

/**
 * A room can turn private or be deleted at any moment, and the API refuses it to
 * anonymous readers from that moment on. A shared cache holding the room page
 * would keep serving the old transcript (s-maxage=3600 + stale-while-revalidate
 * let a proxy serve it for a day), so the room page must never be stored by one.
 */
describe('next.config room page cache headers', () => {
  it('never lets a shared cache store a room page', async () => {
    const room = await headerRuleFor('/rooms/:slug');

    expect(room).toBeDefined();
    const value = room!.headers.find((h) => h.key === 'Cache-Control')?.value ?? '';
    expect(value).toContain('no-store');
    expect(value).toContain('private');
    expect(value).not.toMatch(/public|s-maxage|stale-while-revalidate/);
  });
});

/**
 * The rooms list and the rooms sitemap name public rooms. A room that turns
 * private or is deleted leaves the API's list at once; a shared cache holding
 * /rooms (s-maxage=300 + stale-while-revalidate=3600) would keep naming it.
 */
describe('next.config rooms list cache headers', () => {
  it('never lets a shared cache store the rooms list', async () => {
    const rooms = await headerRuleFor('/rooms');

    expect(rooms).toBeDefined();
    const value = rooms!.headers.find((h) => h.key === 'Cache-Control')?.value ?? '';
    expect(value).toContain('no-store');
    expect(value).not.toMatch(/public|s-maxage|stale-while-revalidate/);
  });
});

/**
 * A post that is deleted or turns family-only (a blog post deleted or
 * unpublished) is refused by the API at once and leaves its lists. A shared cache
 * holding the list (s-maxage=300 + stale-while-revalidate=3600) or the page
 * (s-maxage=3600 + stale-while-revalidate=86400) would keep showing its title and
 * text, so none of them may be stored by one.
 */
describe('next.config posts and blog cache headers', () => {
  it.each(['/posts', '/posts/:id', '/blog', '/blog/:slug'])('never lets a shared cache store %s', async (source) => {
    const rule = await headerRuleFor(source);

    expect(rule).toBeDefined();
    const value = rule!.headers.find((h) => h.key === 'Cache-Control')?.value ?? '';
    expect(value).toContain('no-store');
    expect(value).not.toMatch(/public|s-maxage|stale-while-revalidate/);
  });
});

/**
 * An account that deletes itself or is banned is refused by the API at once and
 * leaves its lists and the leaderboard. A shared cache holding a profile (s-maxage=
 * 3600 + stale-while-revalidate=86400) or a list (s-maxage=300 + stale-while-
 * revalidate=3600) would keep showing the removed account, so none may be stored.
 */
describe('next.config account page cache headers', () => {
  it.each(['/agents', '/agents/:id', '/users', '/users/:id', '/leaderboard'])('never lets a shared cache store %s', async (source) => {
    const rule = await headerRuleFor(source);

    expect(rule).toBeDefined();
    const value = rule!.headers.find((h) => h.key === 'Cache-Control')?.value ?? '';
    expect(value).toContain('no-store');
    expect(value).not.toMatch(/public|s-maxage|stale-while-revalidate/);
  });
});
