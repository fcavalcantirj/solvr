import type { Metadata } from 'next';
import { linkPreview } from './link-preview';
import { TITLE_TEMPLATE } from './site';

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
  // The evidence-backed workflow guides (task idx 84, lib/docs/workflow-guides.ts).
  { path: '/docs/guides/connect-planner-executor', sitemap: true, changefreq: 'monthly', priority: 0.6 },
  { path: '/docs/guides/share-context-between-agents', sitemap: true, changefreq: 'monthly', priority: 0.6 },
  { path: '/docs/guides/connect-builder-reviewer', sitemap: true, changefreq: 'monthly', priority: 0.6 },
  { path: '/docs/guides/resume-across-two-clis', sitemap: true, changefreq: 'monthly', priority: 0.6 },
  { path: '/agents', sitemap: true, changefreq: 'daily', priority: 0.8 },
  { path: '/data', sitemap: true, changefreq: 'hourly', priority: 0.7 },
  { path: '/users', sitemap: true, changefreq: 'daily', priority: 0.7 },
  { path: '/blog', sitemap: true, changefreq: 'daily', priority: 0.8 },
  { path: '/leaderboard', sitemap: true, changefreq: 'daily', priority: 0.7 },
  { path: '/about', sitemap: true, changefreq: 'monthly', priority: 0.3 },
  { path: '/how-it-works', sitemap: true, changefreq: 'monthly', priority: 0.3 },
  { path: '/api-docs', sitemap: true, changefreq: 'weekly', priority: 0.5 },
  { path: '/mcp', sitemap: true, changefreq: 'weekly', priority: 0.5 },
  { path: '/ipfs', sitemap: false },
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
  '/notifications',
  '/email',
  '/admin',
  '/connect/agent',
  '/blog/create',
  '/posts/new',
];

// The root title template (lib/seo/site.ts), restated by every route layout.
export { TITLE_TEMPLATE };

// The robots directive of a page that stays usable but out of search results.
export const NOINDEX: NonNullable<Metadata['robots']> = { index: false, follow: true };

// noindexMetadata is the metadata of a NOINDEX_ROUTES page. Its link preview carries its
// own title and no address: the page has no canonical.
export function noindexMetadata(title: string): Metadata {
  return { title: { default: title, template: TITLE_TEMPLATE }, robots: NOINDEX, ...linkPreview({ title }) };
}

// indexableMetadata is the metadata of an INDEXABLE_ROUTES page: its own title and
// description, a self-referencing canonical with no query string, and a link preview
// that says the same (lib/seo/link-preview.ts). A layout hands its preview to the pages
// below it, address included, so a page that states its own canonical states its own.
export function indexableMetadata(path: string, title: string, description: string): Metadata {
  return {
    title: { default: title, template: TITLE_TEMPLATE },
    description,
    alternates: { canonical: path },
    ...linkPreview({ title, description, path }),
  };
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
