import { describe, it, expect, vi } from 'vitest';
import type { Metadata } from 'next';
import { INDEXABLE_ROUTES, NOINDEX, NOINDEX_ROUTES, TITLE_TEMPLATE } from '@/lib/seo/route-policy';
import { PREVIEW_CARD_PATH } from '@/lib/seo/link-preview';
import { previewProblems, readPreview, tagTitle } from '@/lib/seo/preview-check';
import { WORKFLOW_GUIDES } from '@/lib/docs/workflow-guides';

// Task idx 80: the client-rendered routes get their robots, title, description and
// canonical from a server layout. Noindex routes stay usable but out of search;
// indexable routes carry a self-referencing canonical with no query string.
//
// Link previews (recon finding F04): every route also states its own preview. An
// indexable route previews under its own title and canonical address; a noindex route
// under its own title with no address; each with the shared card as its picture.

// The pages below are read for their metadata only; what they render is left out.
vi.mock('react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react')>()),
  cache: <T,>(fn: T) => fn,
}));
vi.mock('next/font/google', () => ({ Inter: () => ({}), JetBrains_Mono: () => ({}) }));
vi.mock('@next/third-parties/google', () => ({ GoogleAnalytics: () => null }));
vi.mock('@/components/header', () => ({ Header: () => null }));
vi.mock('@/components/footer', () => ({ Footer: () => null }));
vi.mock('@/components/homepage/home-overview', () => ({ HomeOverview: () => null }));
vi.mock('@/components/connect/connect-panel', () => ({ ConnectPanel: () => null }));
vi.mock('@/components/connect/direct-create-panel', () => ({ DirectCreatePanel: () => null }));
vi.mock('@/components/posts/posts-page-client', () => ({ PostsPageClient: () => null }));
vi.mock('@/components/posts/post-composer', () => ({ PostComposer: () => null }));
vi.mock('@/components/rooms/rooms-browser', () => ({ RoomsBrowser: () => null }));
vi.mock('@/components/rooms/recently-viewed-rooms', () => ({ RecentlyViewedRooms: () => null }));
vi.mock('@/components/rooms/create-room-dialog', () => ({ CreateRoomDialog: () => null }));
vi.mock('@/components/agents/agents-page-client', () => ({ AgentsPageClient: () => null }));
vi.mock('@/components/users/users-page-client', () => ({ UsersPageClient: () => null }));
vi.mock('@/components/blog/blog-page-client', () => ({ BlogPageClient: () => null }));
vi.mock('@/components/leaderboard/leaderboard-page-client', () => ({ LeaderboardPageClient: () => null }));
vi.mock('@/components/prompt/guide-prompt', () => ({ GuidePrompt: () => null }));

import { metadata as rootLayout } from './layout';

const HOME_TITLE = (rootLayout.title as { default: string }).default;
const CARD = `https://solvr.dev${PREVIEW_CARD_PATH}`;

type Load = () => Promise<Metadata>;
const exported = (load: () => Promise<{ metadata: Metadata }>): Load => async () => (await load()).metadata;
const bare = { searchParams: Promise.resolve({}) };

const noindexLayouts: Record<string, () => Promise<{ metadata: Metadata }>> = {
  '/login': () => import('./login/layout'),
  '/join': () => import('./join/layout'),
  '/claim': () => import('./claim/layout'),
  '/auth': () => import('./auth/layout'),
  '/settings': () => import('./settings/layout'),
  '/dashboard': () => import('./dashboard/layout'),
  '/referrals': () => import('./referrals/layout'),
  '/pins': () => import('./pins/layout'),
  '/email': () => import('./email/layout'),
  '/admin': () => import('./admin/layout'),
  '/connect/agent': () => import('./connect/agent/layout'),
  '/blog/create': () => import('./blog/create/layout'),
  // The signed-in inbox answered 200 with the home page's title and no robots directive.
  '/notifications': () => import('./notifications/layout'),
};

// Every noindex route of the policy: the layouts above, and the composer, which is a
// server page with its own metadata.
const noindexRoutes: Record<string, Load> = {
  ...Object.fromEntries(Object.entries(noindexLayouts).map(([path, load]) => [path, exported(load)])),
  '/posts/new': exported(() => import('./posts/new/page')),
};

// Every indexable route of the policy, and where its metadata is stated: a route layout
// (the page is a client component), the page itself, or the page's generateMetadata.
const indexableRoutes: Record<string, Load> = {
  '/': exported(() => import('./page')),
  '/posts': async () => (await import('./posts/page')).generateMetadata(bare),
  '/rooms': async () => (await import('./rooms/page')).generateMetadata(bare),
  '/connect': exported(() => import('./connect/page')),
  '/docs': exported(() => import('./docs/layout')),
  '/docs/protocol': exported(() => import('./docs/protocol/page')),
  '/docs/guides': exported(() => import('./docs/guides/layout')),
  ...Object.fromEntries(
    WORKFLOW_GUIDES.map((guide): [string, Load] => [
      `/docs/guides/${guide.slug}`,
      async () => (await import('./docs/guides/[slug]/page')).generateMetadata({ params: Promise.resolve({ slug: guide.slug }) }),
    ])
  ),
  '/agents': exported(() => import('./agents/page')),
  '/data': exported(() => import('./data/layout')),
  '/users': exported(() => import('./users/page')),
  '/blog': exported(() => import('./blog/page')),
  '/leaderboard': exported(() => import('./leaderboard/page')),
  '/about': exported(() => import('./about/layout')),
  '/how-it-works': exported(() => import('./how-it-works/layout')),
  '/api-docs': exported(() => import('./api-docs/layout')),
  '/mcp': exported(() => import('./mcp/layout')),
  '/ipfs': exported(() => import('./ipfs/layout')),
  '/skill': exported(() => import('./skill/layout')),
  '/amcp': exported(() => import('./amcp/layout')),
  '/privacy': exported(() => import('./privacy/layout')),
  '/terms': exported(() => import('./terms/layout')),
  '/status': exported(() => import('./status/layout')),
};

describe('route layouts', () => {
  for (const [path, load] of Object.entries(noindexLayouts)) {
    it(`${path} stays usable but noindex`, async () => {
      const { metadata } = await load();
      expect(metadata.robots).toEqual(NOINDEX);
      expect(metadata.alternates?.canonical).toBeUndefined();
    });

    it(`${path} is named by the route policy as noindex`, () => {
      expect(NOINDEX_ROUTES).toContain(path);
    });
  }

  it('/notifications carries its own title, not the home page title', async () => {
    const { metadata } = await noindexLayouts['/notifications']();
    expect(metadata.title).toEqual({ default: 'Notifications', template: TITLE_TEMPLATE });
  });
});

describe('indexable routes', () => {
  it('are all checked here: every route of the policy, and no other', () => {
    expect(Object.keys(indexableRoutes).sort()).toEqual(INDEXABLE_ROUTES.map((r) => r.path).sort());
  });

  for (const [path, load] of Object.entries(indexableRoutes)) {
    it(`${path} is indexable with a self-referencing canonical`, async () => {
      const metadata = await load();
      expect(metadata.robots).toBeUndefined();
      expect(metadata.alternates?.canonical).toBe(path);
      expect(tagTitle(metadata.title)).not.toBe('');
      expect(String(metadata.description ?? '').length).toBeGreaterThan(40);
    });

    it(`${path} previews under its own title, description and address, with the card`, async () => {
      const metadata = await load();
      expect(previewProblems(metadata, { homeTitle: HOME_TITLE, home: path === '/' })).toEqual([]);
      const preview = readPreview(metadata);
      expect(preview.url).toBe(`https://solvr.dev${path}`);
      expect(preview.description).toBe(metadata.description);
      expect(preview.image).toBe(CARD);
      expect(preview.images[0]).toMatchObject({ width: 1200, height: 630, type: 'image/png' });
    });
  }

  it('the home title is the preview title of the home page only', async () => {
    const titles = await Promise.all(
      Object.entries(indexableRoutes).map(async ([path, load]) => [path, readPreview(await load()).title] as const)
    );
    expect(titles.filter(([, title]) => title === HOME_TITLE).map(([path]) => path)).toEqual(['/']);
  });

  it('no two routes share a preview title or a preview address', async () => {
    const previews = await Promise.all(Object.values(indexableRoutes).map(async (load) => readPreview(await load())));
    expect(new Set(previews.map((p) => p.title)).size).toBe(previews.length);
    expect(new Set(previews.map((p) => p.url)).size).toBe(previews.length);
  });

  it('a preview title is the title tag: the page name, then the brand once', async () => {
    expect(readPreview(await indexableRoutes['/docs']()).title).toBe('Docs | Solvr');
    expect(readPreview(await indexableRoutes['/docs/protocol']()).title).toBe('Agent-to-agent capabilities | Solvr');
    expect(readPreview(await indexableRoutes['/posts']()).title).toBe('Posts | Solvr');
    expect(readPreview(await indexableRoutes['/data']()).title).toBe('Statistics | Solvr');
    expect(readPreview(await indexableRoutes['/docs/guides/connect-planner-executor']()).title).toBe(
      'Connect a planner and an executor | Solvr'
    );
  });

  // A filtered collection is still the collection: it keeps the collection's preview.
  it('/posts and /rooms keep their preview under a query', async () => {
    const query = { searchParams: Promise.resolve({ q: 'planner' }) };
    for (const metadata of [
      await (await import('./posts/page')).generateMetadata(query),
      await (await import('./rooms/page')).generateMetadata(query),
    ]) {
      expect(metadata.robots).toEqual(NOINDEX);
      expect(previewProblems(metadata, { homeTitle: HOME_TITLE })).toEqual([]);
    }
  });
});

describe('noindex routes', () => {
  it('are all checked here: every route of the policy, and no other', () => {
    expect(Object.keys(noindexRoutes).sort()).toEqual([...NOINDEX_ROUTES].sort());
  });

  for (const [path, load] of Object.entries(noindexRoutes)) {
    it(`${path} previews under its own title, with the card`, async () => {
      const metadata = await load();
      expect(previewProblems(metadata, { homeTitle: HOME_TITLE })).toEqual([]);
      expect(readPreview(metadata).image).toBe(CARD);
    });
  }

  // A page with no canonical names no address; the composer states one, so it previews under it.
  it('state a preview address only where they state a canonical', async () => {
    for (const [path, load] of Object.entries(noindexRoutes)) {
      const metadata = await load();
      const canonical = metadata.alternates?.canonical;
      expect(readPreview(metadata).url, path).toBe(canonical ? `https://solvr.dev${canonical}` : undefined);
    }
    expect(readPreview(await noindexRoutes['/login']()).url).toBeUndefined();
    expect(readPreview(await noindexRoutes['/posts/new']()).url).toBe('https://solvr.dev/posts/new');
  });

  it('a preview title is the title tag', async () => {
    expect(readPreview(await noindexRoutes['/login']()).title).toBe('Sign in | Solvr');
    expect(readPreview(await noindexRoutes['/posts/new']()).title).toBe('New post | Solvr');
  });
});
