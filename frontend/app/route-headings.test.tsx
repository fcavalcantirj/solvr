import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import type { ComponentType, ReactElement } from 'react';
import { INDEXABLE_ROUTES } from '@/lib/seo/route-policy';
import { WORKFLOW_GUIDES } from '@/lib/docs/workflow-guides';
import { OVERVIEW, STALE_META } from '@/components/homepage/overview-fixture';
import { CONNECT_EXAMPLES } from '@/components/connect/connect-fixture';

// SPEC.md 27.1 (Web routes): every indexable route serves exactly one <h1> in its server
// HTML, saying what the page is. /status served none (the lead's crawl, 2026-10-05): its
// server HTML is the loading state, which drew a bar where the heading goes. Each route of the
// policy is rendered here as the server renders it, header and footer included: client
// components in their first state, nothing read in the browser.

// React's request-scoped cache() only exists in the server build; pass through here.
vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('next/font/google', () => ({ Inter: () => ({}), JetBrains_Mono: () => ({}) }));
vi.mock('next/link', () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn(), back: vi.fn(), refresh: vi.fn() }),
  usePathname: () => '/',
  useSearchParams: () => new URLSearchParams(),
  useParams: () => ({}),
  notFound: () => {
    throw new Error('NEXT_NOT_FOUND');
  },
  redirect: () => {
    throw new Error('NEXT_REDIRECT');
  },
}));
// An anonymous visitor, as a crawler is.
vi.mock('@/hooks/use-auth', () => ({
  useAuth: () => ({
    user: null, isAuthenticated: false, isLoading: false,
    showAuthWall: vi.fn(), setShowAuthModal: vi.fn(), showAuthModal: false, authModalMessage: '',
    loginWithGitHub: vi.fn(), loginWithGoogle: vi.fn(), loginWithEmail: vi.fn(), register: vi.fn(), logout: vi.fn(), setToken: vi.fn(),
  }),
  AuthProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

// What the API answers the server reads of these pages: the home overview, the example
// sentences, and empty lists for the collections.
const emptyList = { data: [], meta: { total: 0, page: 1, per_page: 20, total_pages: 0, has_more: false } };
const ANSWERS: [RegExp, unknown][] = [
  [/^\/v1\/overview$/, { data: OVERVIEW, meta: STALE_META }],
  [/^\/v1\/connect\/examples$/, { data: { instruction_version: '2.1', presets: CONNECT_EXAMPLES } }],
  [/^\/v1\/(posts|rooms|agents|users|blog|leaderboard)$/, emptyList],
];
const fetchMock = vi.fn(async (input: unknown) => {
  const { pathname } = new URL(String(input));
  const hit = ANSWERS.find(([path]) => path.test(pathname));
  if (!hit) return { ok: false, status: 404, json: async () => ({ error: { code: 'NOT_FOUND' } }) };
  return { ok: true, status: 200, json: async () => hit[1] };
});
beforeEach(() => vi.stubGlobal('fetch', fetchMock));
afterEach(() => vi.unstubAllGlobals());

type Props = Record<string, unknown>;

// render is a route's page as the server renders it: an async server page is awaited, any
// other page is rendered as an element (a client page in its first state).
const render = (load: () => Promise<{ default: unknown }>, params: Record<string, string> = {}) => async () => {
  const Page = (await load()).default as (props: Props) => ReactElement | Promise<ReactElement>;
  const props = { params: Promise.resolve(params), searchParams: Promise.resolve({}) };
  if (Page.constructor.name === 'AsyncFunction') return renderToStaticMarkup(await Page(props));
  const Component = Page as unknown as ComponentType<Props>;
  return renderToStaticMarkup(<Component {...props} />);
};

// Every indexable route of the policy, and its page.
const routes: Record<string, () => Promise<string>> = {
  '/': render(() => import('./page')),
  '/posts': render(() => import('./posts/page')),
  '/rooms': render(() => import('./rooms/page')),
  '/connect': render(() => import('./connect/page')),
  '/docs': render(() => import('./docs/page')),
  '/docs/protocol': render(() => import('./docs/protocol/page')),
  '/docs/guides': render(() => import('./docs/guides/page')),
  ...Object.fromEntries(
    WORKFLOW_GUIDES.map((guide) => [`/docs/guides/${guide.slug}`, render(() => import('./docs/guides/[slug]/page'), { slug: guide.slug })])
  ),
  '/agents': render(() => import('./agents/page')),
  '/data': render(() => import('./data/page')),
  '/users': render(() => import('./users/page')),
  '/blog': render(() => import('./blog/page')),
  '/leaderboard': render(() => import('./leaderboard/page')),
  '/about': render(() => import('./about/page')),
  '/how-it-works': render(() => import('./how-it-works/page')),
  '/api-docs': render(() => import('./api-docs/page')),
  '/mcp': render(() => import('./mcp/page')),
  '/ipfs': render(() => import('./ipfs/page')),
  '/skill': render(() => import('./skill/page')),
  '/amcp': render(() => import('./amcp/page')),
  '/privacy': render(() => import('./privacy/page')),
  '/terms': render(() => import('./terms/page')),
  '/status': render(() => import('./status/page')),
};

const headings = (html: string) =>
  [...html.matchAll(/<h1\b[^>]*>([\s\S]*?)<\/h1>/g)].map((m) => m[1].replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim());

describe('the h1 of every indexable route', () => {
  it('are all checked here: every indexable route of the policy, and no other', () => {
    expect(Object.keys(routes).sort()).toEqual(INDEXABLE_ROUTES.map((r) => r.path).sort());
  });

  for (const [path, html] of Object.entries(routes)) {
    it(`${path} serves exactly one h1 in its server HTML`, async () => {
      const found = headings(await html());
      expect(found, `${path}: ${JSON.stringify(found)}`).toHaveLength(1);
      expect(found[0].length, path).toBeGreaterThan(0);
    });
  }

  it('/status says what the page is, and keeps saying it once the status is read', async () => {
    expect(headings(await routes['/status']())).toEqual(['Solvr Status']);
  });

  // The root layout wraps every page; it adds no heading of its own.
  it('the root layout adds no h1', async () => {
    const { default: RootLayout } = await import('./layout');
    expect(headings(renderToStaticMarkup(<RootLayout><main>page</main></RootLayout>))).toEqual([]);
  });
});
