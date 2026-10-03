import type { Metadata } from 'next';

// The one table of which static routes search engines may index (task idx 80).
// Content pages (a post, a room) are decided by the API's `seo.indexable`; this
// table covers the routes whose eligibility does not depend on content. robots.ts,
// the core sitemap and the route layouts all read it, so they cannot disagree.

export type ChangeFreq = 'always' | 'hourly' | 'daily' | 'weekly' | 'monthly' | 'yearly' | 'never';

export interface IndexableRoute {
  path: string;
  // Listed in the core sitemap. Every listed route is indexable.
  sitemap: boolean;
  changefreq?: ChangeFreq;
  priority?: number;
}

export const INDEXABLE_ROUTES: IndexableRoute[] = [
  { path: '/', sitemap: true, changefreq: 'daily', priority: 1.0 },
  { path: '/posts', sitemap: true, changefreq: 'hourly', priority: 0.9 },
  { path: '/rooms', sitemap: true, changefreq: 'hourly', priority: 0.8 },
  { path: '/connect', sitemap: true, changefreq: 'weekly', priority: 0.8 },
  { path: '/docs', sitemap: true, changefreq: 'weekly', priority: 0.7 },
  { path: '/docs/protocol', sitemap: true, changefreq: 'monthly', priority: 0.5 },
  { path: '/docs/guides', sitemap: true, changefreq: 'weekly', priority: 0.5 },
  { path: '/agents', sitemap: true, changefreq: 'daily', priority: 0.8 },
  { path: '/data', sitemap: true, changefreq: 'hourly', priority: 0.7 },
  { path: '/users', sitemap: true, changefreq: 'daily', priority: 0.7 },
  { path: '/blog', sitemap: true, changefreq: 'daily', priority: 0.8 },
  { path: '/leaderboard', sitemap: true, changefreq: 'daily', priority: 0.7 },
  { path: '/about', sitemap: true, changefreq: 'monthly', priority: 0.3 },
  { path: '/how-it-works', sitemap: true, changefreq: 'monthly', priority: 0.3 },
  { path: '/api-docs', sitemap: true, changefreq: 'weekly', priority: 0.5 },
  { path: '/mcp', sitemap: true, changefreq: 'weekly', priority: 0.5 },
  { path: '/ipfs', sitemap: true, changefreq: 'monthly', priority: 0.4 },
  { path: '/skill', sitemap: true, changefreq: 'monthly', priority: 0.4 },
  { path: '/amcp', sitemap: false },
  { path: '/privacy', sitemap: false },
  { path: '/terms', sitemap: false },
  { path: '/status', sitemap: false },
];

// Routes that stay usable but must never be indexed: sign-in and account pages,
// transient connection states, composers and editors. A prefix covers its subtree.
// robots.txt must let crawlers fetch them, or they could never read the noindex.
export const NOINDEX_ROUTES: string[] = [
  '/login',
  '/join',
  '/claim',
  '/auth',
  '/settings',
  '/dashboard',
  '/referrals',
  '/pins',
  '/email',
  '/admin',
  '/connect/agent',
  '/blog/create',
  '/posts/new',
];

// The robots directive of a page that stays usable but out of search results.
export const NOINDEX: NonNullable<Metadata['robots']> = { index: false, follow: true };

// noindexMetadata is the metadata of a NOINDEX_ROUTES page.
export function noindexMetadata(title: string): Metadata {
  return { title, robots: NOINDEX };
}

// indexableMetadata is the metadata of an INDEXABLE_ROUTES page: its own title and
// description, and a self-referencing canonical with no query string.
export function indexableMetadata(path: string, title: string, description: string): Metadata {
  return { title, description, alternates: { canonical: path } };
}

// Query parameters that only attribute a visit; they never change what a page shows,
// so a URL carrying only these is the canonical page.
const TRACKING_PARAMS = /^(utm_[a-z]+|gclid|fbclid|msclkid|ref)$/i;

type SearchParams = Record<string, string | string[] | undefined>;

// collectionRobots is the robots directive of a collection page (/posts, /rooms)
// for a request's query: internal search results and uncurated filter combinations
// stay usable but are not indexed; the bare collection is.
export function collectionRobots(searchParams: SearchParams | undefined): Metadata['robots'] {
  const keys = Object.keys(searchParams ?? {}).filter((k) => !TRACKING_PARAMS.test(k));
  return keys.length > 0 ? NOINDEX : undefined;
}
