import { describe, it, expect } from 'vitest';
import type { Metadata } from 'next';
import { NOINDEX, NOINDEX_ROUTES, TITLE_TEMPLATE } from '@/lib/seo/route-policy';

// Task idx 80: the client-rendered routes get their robots, title, description and
// canonical from a server layout. Noindex routes stay usable but out of search;
// indexable routes carry a self-referencing canonical with no query string.

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

const indexableLayouts: Record<string, () => Promise<{ metadata: Metadata }>> = {
  '/about': () => import('./about/layout'),
  '/amcp': () => import('./amcp/layout'),
  '/api-docs': () => import('./api-docs/layout'),
  '/how-it-works': () => import('./how-it-works/layout'),
  '/ipfs': () => import('./ipfs/layout'),
  '/mcp': () => import('./mcp/layout'),
  '/privacy': () => import('./privacy/layout'),
  '/terms': () => import('./terms/layout'),
  '/skill': () => import('./skill/layout'),
  '/status': () => import('./status/layout'),
  '/docs': () => import('./docs/layout'),
  '/docs/guides': () => import('./docs/guides/layout'),
  '/data': () => import('./data/layout'),
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

  for (const [path, load] of Object.entries(indexableLayouts)) {
    it(`${path} is indexable with a self-referencing canonical`, async () => {
      const { metadata } = await load();
      expect(metadata.robots).toBeUndefined();
      expect(metadata.alternates?.canonical).toBe(path);
      expect(String(metadata.title ?? '')).not.toBe('');
      expect(String(metadata.description ?? '').length).toBeGreaterThan(40);
    });
  }
});
