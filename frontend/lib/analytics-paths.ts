// Two pure rules about an address (SPEC.md 27.7): whether Google's tag may see the page at
// all, and which kind of page it is. No browser API here, so server components may call them.

// Google's tag sends the page address, query string included, with every hit, so these pages
// never load it and never send an event: their address carries a secret the page is about
// to use.
//   /claim             #token=<agent claim token>, usable for hours
//   /auth/callback     ?code=<one-time login code>
//   /email/unsubscribe ?email=&token=<unsubscribe token>
// gtag.js dropped the #fragment when measured (2026-09-28), but that is Google's code to
// change, so /claim is listed too.
const UNTRACKED_PATHS = ['/claim', '/auth/callback', '/email/unsubscribe'];

export function isUntrackedPath(pathname: string | null): boolean {
  if (pathname === null) return true;
  const path = pathname.length > 1 ? pathname.replace(/\/+$/, '') : pathname;
  return UNTRACKED_PATHS.includes(path);
}

// The kind of page, sent as content_group with every page view and every event.
export const CONTENT_GROUPS = [
  'home',
  'connect',
  'post',
  'room',
  'transcript',
  'agent',
  'user',
  'blog',
  'docs',
  'guide',
  'collection',
  'account',
  'other',
] as const;

export type ContentGroup = (typeof CONTENT_GROUPS)[number];

// Pages only a signed-in person, or a person signing in, uses.
const ACCOUNT_SECTIONS = new Set([
  'login',
  'join',
  'dashboard',
  'settings',
  'notifications',
  'pins',
  'referrals',
  'admin',
  'claim',
  'auth',
  'email',
]);

// Reference pages outside /docs itself.
const DOCS_SECTIONS = new Set(['api-docs', 'skill', 'mcp', 'amcp', 'ipfs']);

export function contentGroupForPath(pathname: string | null | undefined): ContentGroup {
  if (!pathname) return 'other';
  const [section, second, third] = pathname.split(/[?#]/)[0].split('/').filter(Boolean);
  if (section === undefined) return 'home';

  switch (section) {
    case 'connect':
      return 'connect';
    case 'posts':
      if (second === undefined) return 'collection';
      // The composer and the editor are a signed-in person's pages, not a post being read.
      if (second === 'new' || third === 'edit') return 'account';
      return 'post';
    case 'rooms':
      if (second === undefined) return 'collection';
      return third === 'history' ? 'transcript' : 'room';
    case 'agents':
      return second === undefined ? 'collection' : 'agent';
    case 'users':
      return second === undefined ? 'collection' : 'user';
    case 'leaderboard':
      return 'collection';
    case 'blog':
      return second === 'create' ? 'account' : 'blog';
    case 'docs':
      return second === 'guides' ? 'guide' : 'docs';
    default:
      if (DOCS_SECTIONS.has(section)) return 'docs';
      if (ACCOUNT_SECTIONS.has(section)) return 'account';
      return 'other';
  }
}
