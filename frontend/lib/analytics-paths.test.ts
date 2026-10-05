import { describe, it, expect } from 'vitest';
import { readdirSync, statSync } from 'node:fs';
import { join, relative, sep } from 'node:path';
import { CONTENT_GROUPS, contentGroupForPath, isUntrackedPath } from './analytics-paths';

// Two rules about an address, both pure: whether Google's tag may see the page at all, and
// which kind of page it is (the content_group every page view carries, SPEC.md 27.7).

describe('isUntrackedPath', () => {
  it.each(['/claim', '/claim/', '/auth/callback', '/auth/callback/', '/email/unsubscribe'])(
    '%s is untracked: its address carries a secret',
    (path) => {
      expect(isUntrackedPath(path)).toBe(true);
    },
  );

  it.each(['/', '/rooms/planner-room', '/claimed', '/auth', '/email', '/settings/agents', '/login'])(
    '%s is tracked',
    (path) => {
      expect(isUntrackedPath(path)).toBe(false);
    },
  );

  it('treats an unknown page as untracked', () => {
    expect(isUntrackedPath(null)).toBe(true);
  });
});

describe('contentGroupForPath', () => {
  it.each([
    ['/', 'home'],
    ['/connect', 'connect'],
    ['/connect/agent', 'connect'],
    ['/posts', 'collection'],
    ['/posts/3f2c1d1e-0000-4000-8000-000000000001', 'post'],
    ['/posts/3f2c1d1e-0000-4000-8000-000000000001/replies/2', 'post'],
    ['/posts/new', 'account'],
    // The post archive is a list of posts, not a post being read.
    ['/posts/page/1', 'collection'],
    ['/posts/page/12', 'collection'],
    ['/posts/3f2c1d1e-0000-4000-8000-000000000001/edit', 'account'],
    ['/rooms', 'collection'],
    ['/rooms/planner-room', 'room'],
    ['/rooms/planner-room/history/3', 'transcript'],
    ['/agents', 'collection'],
    ['/agents/agent_Claudius', 'agent'],
    ['/users', 'collection'],
    ['/users/26911295-5bf7-4c4e-91a1-03d483e78063', 'user'],
    ['/leaderboard', 'collection'],
    ['/blog', 'blog'],
    ['/blog/how-rooms-work', 'blog'],
    ['/blog/create', 'account'],
    ['/docs', 'docs'],
    ['/docs/protocol', 'docs'],
    ['/api-docs', 'docs'],
    ['/skill', 'docs'],
    ['/mcp', 'docs'],
    ['/amcp', 'docs'],
    ['/ipfs', 'docs'],
    ['/docs/guides', 'guide'],
    ['/docs/guides/connect-planner-executor', 'guide'],
    ['/login', 'account'],
    ['/join', 'account'],
    ['/dashboard', 'account'],
    ['/settings', 'account'],
    ['/settings/api-keys', 'account'],
    ['/notifications', 'account'],
    ['/pins', 'account'],
    ['/referrals', 'account'],
    ['/admin/system', 'account'],
    ['/claim', 'account'],
    ['/auth/callback', 'account'],
    ['/email/unsubscribe', 'account'],
    ['/about', 'other'],
    ['/how-it-works', 'other'],
    ['/data', 'other'],
    ['/status', 'other'],
    ['/terms', 'other'],
    ['/privacy', 'other'],
    ['/zh/promote', 'other'],
    ['/a-page-that-does-not-exist', 'other'],
  ] as const)('%s is %s', (path, group) => {
    expect(contentGroupForPath(path)).toBe(group);
  });

  it('ignores a trailing slash, a query string and a fragment', () => {
    expect(contentGroupForPath('/rooms/')).toBe('collection');
    expect(contentGroupForPath('/rooms/planner-room/')).toBe('room');
    expect(contentGroupForPath('/posts?q=deadlock')).toBe('collection');
    expect(contentGroupForPath('/docs/guides#top')).toBe('guide');
  });

  it('answers other when the page is unknown', () => {
    expect(contentGroupForPath(null)).toBe('other');
    expect(contentGroupForPath(undefined)).toBe('other');
    expect(contentGroupForPath('')).toBe('other');
  });

  it('is a closed list of thirteen', () => {
    expect([...CONTENT_GROUPS].sort()).toEqual(
      ['account', 'agent', 'blog', 'collection', 'connect', 'docs', 'guide', 'home', 'other', 'post', 'room', 'transcript', 'user'],
    );
  });

  // A new page must be given its kind on purpose. It fails here until the rule knows its
  // section, or until it is listed below as one of the pages that are simply "other".
  describe('every page of the app has a kind', () => {
    const OTHER_ON_PURPOSE = ['/about', '/data', '/how-it-works', '/privacy', '/status', '/terms', '/zh/promote'];
    const appDir = join(__dirname, '..', 'app');

    const routes: string[] = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir)) {
        const full = join(dir, entry);
        if (statSync(full).isDirectory()) walk(full);
        else if (entry === 'page.tsx' || entry === 'page.ts') {
          const segments = relative(appDir, full).split(sep).slice(0, -1).filter((s) => !/^\(.*\)$/.test(s));
          routes.push('/' + segments.map((s) => s.replace(/^\[.*\]$/, 'x')).join('/'));
        }
      }
    };
    walk(appDir);

    it('finds the pages', () => {
      expect(routes.length).toBeGreaterThan(40);
    });

    it.each(routes.map((route) => [route]))('%s', (route) => {
      const group = contentGroupForPath(route);
      expect(CONTENT_GROUPS).toContain(group);
      if (group === 'other') expect(OTHER_ON_PURPOSE).toContain(route);
    });
  });
});
