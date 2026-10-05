import { describe, it, expect, vi } from 'vitest';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';

// The root layout pulls in web fonts and the analytics component at import time;
// neither matters to the metadata this file reads from it.
vi.mock('next/font/google', () => ({
  Inter: () => ({ variable: '--font-inter' }),
  JetBrains_Mono: () => ({ variable: '--font-jetbrains' }),
}));
vi.mock('@next/third-parties/google', () => ({ GoogleAnalytics: () => null }));

import robots from '@/app/robots';
import { metadata as rootMetadata } from '@/app/layout';
import { SITE_ORIGIN } from './site';
import { linkPreview, PREVIEW_CARD_ALT, PREVIEW_CARD_PATH } from './link-preview';

// A link pasted in a chat or a feed is shown as a card: a title, a line, an address and a
// picture. No page had a picture, and 55 pages showed the home page's title, because a
// page that states any Open Graph field replaces the whole object it would inherit.
// linkPreview builds that object, and the Twitter one, for every page.

const CARD = {
  url: `https://solvr.dev${PREVIEW_CARD_PATH}`,
  width: 1200,
  height: 630,
  alt: 'Connect your agents. Let them work together.',
  type: 'image/png',
};

describe('linkPreview', () => {
  it('builds Open Graph and Twitter from a page title, description and canonical path', () => {
    const { openGraph, twitter } = linkPreview({
      title: 'Docs',
      description: 'How to put two agents in one room.',
      path: '/docs',
    });
    expect(openGraph).toEqual({
      title: 'Docs | Solvr',
      description: 'How to put two agents in one room.',
      url: 'https://solvr.dev/docs',
      siteName: 'Solvr',
      locale: 'en_US',
      type: 'website',
      images: [CARD],
    });
    expect(twitter).toEqual({
      card: 'summary_large_image',
      site: '@solvrdev',
      title: 'Docs | Solvr',
      description: 'How to put two agents in one room.',
      images: [CARD],
    });
  });

  it('shows the title as the title tag does: a plain one under the site template, an absolute one as written', () => {
    expect(linkPreview({ title: 'Rooms' }).openGraph.title).toBe('Rooms | Solvr');
    const home = linkPreview({ title: { absolute: 'Solvr — Connect your agents. Let them work together.' }, path: '/' });
    expect(home.openGraph.title).toBe('Solvr — Connect your agents. Let them work together.');
    expect(home.twitter.title).toBe('Solvr — Connect your agents. Let them work together.');
    expect(home.openGraph.url).toBe('https://solvr.dev/');
  });

  it('states no address for a page with no canonical', () => {
    const { openGraph } = linkPreview({ title: 'Settings' });
    expect(openGraph).not.toHaveProperty('url');
    expect(openGraph.title).toBe('Settings | Solvr');
  });

  // Next copies a page's own title and description into a preview that has none, so the
  // preview a page inherits from the root layout must state neither.
  it('states no title, description or address when it is given none', () => {
    const { openGraph, twitter } = linkPreview();
    expect(openGraph).toEqual({ siteName: 'Solvr', locale: 'en_US', type: 'website', images: [CARD] });
    expect(twitter).toEqual({ card: 'summary_large_image', site: '@solvrdev', images: [CARD] });
  });

  it('names the card by what it says', () => {
    expect(PREVIEW_CARD_ALT).toBe('Connect your agents. Let them work together.');
  });

  it('marks an article with its dates and tags', () => {
    const { openGraph } = linkPreview({
      title: 'Hand a plan over',
      description: 'd',
      path: '/posts/p1',
      type: 'article',
      article: { publishedTime: '2026-09-01T00:00:00Z', modifiedTime: '2026-09-02T00:00:00Z', tags: ['agents'] },
    });
    expect(openGraph).toMatchObject({
      type: 'article',
      publishedTime: '2026-09-01T00:00:00Z',
      modifiedTime: '2026-09-02T00:00:00Z',
      tags: ['agents'],
      url: 'https://solvr.dev/posts/p1',
    });
  });

  it('keeps article dates off a page that is not an article', () => {
    const { openGraph } = linkPreview({ title: 'A room', article: { publishedTime: '2026-09-01T00:00:00Z' } });
    expect(openGraph).toMatchObject({ type: 'website' });
    expect(openGraph).not.toHaveProperty('publishedTime');
  });

  it('marks a profile page as a profile', () => {
    expect(linkPreview({ title: 'planner', path: '/agents/a1', type: 'profile' }).openGraph).toMatchObject({ type: 'profile' });
  });
});

describe("a page's own picture", () => {
  const cover = (image: string | null | undefined) =>
    linkPreview({ title: 'A blog post', description: 'd', path: '/blog/a-post', type: 'article', image });

  it('replaces the card, with no size or format claimed for it', () => {
    const { openGraph, twitter } = cover('https://cdn.example.test/covers/a-post.jpg');
    const own = [{ url: 'https://cdn.example.test/covers/a-post.jpg', alt: 'A blog post' }];
    expect(openGraph.images).toEqual(own);
    expect(twitter.images).toEqual(own);
    expect(twitter).toMatchObject({ card: 'summary_large_image' });
  });

  it('is made absolute on the site when the API serves a path', () => {
    expect(cover('/covers/a-post.jpg').openGraph.images).toEqual([{ url: 'https://solvr.dev/covers/a-post.jpg', alt: 'A blog post' }]);
    expect(cover('//cdn.example.test/a.png').openGraph.images).toEqual([{ url: 'https://cdn.example.test/a.png', alt: 'A blog post' }]);
  });

  it.each([
    ['nothing', undefined],
    ['null', null],
    ['an empty string', ''],
    ['blank space', '   '],
    ['a script address', 'javascript:alert(1)'],
    ['inline data', 'data:image/png;base64,AAAA'],
    ['a host-less address', 'http://'],
  ])('falls back to the card for %s', (_name, image) => {
    const { openGraph, twitter } = cover(image);
    expect(openGraph.images).toEqual([CARD]);
    expect(twitter.images).toEqual([CARD]);
  });
});

describe('the one origin', () => {
  it('is the origin the root layout resolves every canonical against', () => {
    expect(SITE_ORIGIN).toBe('https://solvr.dev');
    expect(rootMetadata.metadataBase?.origin).toBe(SITE_ORIGIN);
    expect(new URL(String(linkPreview({ path: '/rooms/kestrel' }).openGraph.url)).origin).toBe(SITE_ORIGIN);
  });

  it('names the address the way a canonical is resolved', () => {
    for (const path of ['/', '/posts/p1', '/rooms/kestrel/history/2']) {
      expect(linkPreview({ path }).openGraph.url).toBe(new URL(path, rootMetadata.metadataBase!).href);
    }
  });
});

describe('the root layout', () => {
  // A page that states no preview of its own inherits this one. It carries the site name
  // and the card only, so what Next fills in is that page's title, never the home page's.
  it('hands every page the card, and no title, description or address', () => {
    expect(rootMetadata.openGraph).toEqual(linkPreview().openGraph);
    expect(rootMetadata.twitter).toEqual(linkPreview().twitter);
    expect(rootMetadata.openGraph).not.toHaveProperty('title');
    expect(rootMetadata.openGraph).not.toHaveProperty('description');
    expect(rootMetadata.openGraph).not.toHaveProperty('url');
  });
});

describe('the card file', () => {
  // Platforms keep a preview picture by its address, so a new picture needs a new file
  // name (…-v2.png). The name is written once, in PREVIEW_CARD_PATH.
  it('is a versioned PNG under /og/', () => {
    expect(PREVIEW_CARD_PATH).toMatch(/^\/og\/solvr-card-v[0-9]+\.png$/);
  });

  it('exists in public/ and is 1200 by 630, as the preview says', () => {
    const png = readFileSync(join('public', PREVIEW_CARD_PATH));
    expect(png.subarray(0, 8).toString('hex')).toBe('89504e470d0a1a0a');
    expect(png.subarray(12, 16).toString('latin1')).toBe('IHDR');
    expect(png.readUInt32BE(16)).toBe(1200);
    expect(png.readUInt32BE(20)).toBe(630);
  });

  it('may be fetched by any crawler robots.txt lets in', () => {
    const result = robots();
    const rules = Array.isArray(result.rules) ? result.rules : [result.rules];
    const everyone = rules.find((r) => r.userAgent === '*');
    const disallowed = [everyone?.disallow ?? []].flat();
    expect(disallowed.filter((d) => PREVIEW_CARD_PATH.startsWith(d))).toEqual([]);
  });
});

// The sources a build ships, tests excluded.
function sources(root: string): string[] {
  const files: string[] = [];
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir)) {
      const full = join(dir, entry);
      if (statSync(full).isDirectory()) {
        if (entry !== 'node_modules' && entry !== '.next') walk(full);
      } else if (/\.(ts|tsx|mjs|js|jsx)$/.test(entry) && !/\.test\.(ts|tsx)$/.test(entry)) {
        files.push(full);
      }
    }
  };
  walk(root);
  return files;
}

describe('link previews are built in one place', () => {
  const HELPER = join('lib', 'seo', 'link-preview.ts');
  const shipped = ['app', 'components', 'lib', 'hooks'].flatMap(sources);

  it('finds the sources it is meant to check', () => {
    expect(shipped.length).toBeGreaterThan(50);
    expect(shipped).toContain(HELPER);
  });

  // A hand-built object is how the picture, the address and the page's own title went
  // missing: it replaces everything the page would have inherited. The pattern is the
  // object key (`openGraph: {`), not a tag name inside a string (`twitter:card`).
  const BY_HAND = /\b(openGraph|twitter)\s*:(?!\w)/;

  it('tells a hand-built object from a tag name', () => {
    expect(BY_HAND.test('openGraph: { title }')).toBe(true);
    expect(BY_HAND.test('  twitter : preview,')).toBe(true);
    expect(BY_HAND.test("push('twitter:card is wrong')")).toBe(false);
    expect(BY_HAND.test('...linkPreview({ title })')).toBe(false);
  });

  it('no page, layout or component builds an Open Graph or Twitter object by hand', () => {
    const byHand = shipped
      .filter((file) => file !== HELPER)
      .filter((file) => BY_HAND.test(readFileSync(file, 'utf8')));
    expect(byHand).toEqual([]);
  });

  // A segment that states a title but no preview shows the preview of the layout above
  // it, title and address included: the home page's on 55 pages, and "Guides" on a 404
  // under /docs/guides. So every segment that states metadata builds its own preview,
  // with linkPreview or with a route helper that calls it (lib/seo/route-policy.ts).
  it('every page and layout that states metadata builds its link preview', () => {
    const stating = sources('app').filter((file) =>
      /export\s+(const\s+(metadata|generateMetadata)\b|(async\s+)?function\s+generateMetadata\b)/.test(readFileSync(file, 'utf8'))
    );
    expect(stating.length).toBeGreaterThan(40);
    const without = stating.filter(
      (file) => !/\b(linkPreview|indexableMetadata|noindexMetadata)\(/.test(readFileSync(file, 'utf8'))
    );
    expect(without).toEqual([]);
  });

  // A page under a layout inherits that layout's preview, address included. A page that
  // states its own canonical must therefore state its own preview.
  it('every page that states a canonical builds its preview with the helper', () => {
    const withCanonical = sources('app').filter((file) => /\bcanonical\s*:/.test(readFileSync(file, 'utf8')));
    expect(withCanonical.length).toBeGreaterThan(5);
    const without = withCanonical.filter((file) => !readFileSync(file, 'utf8').includes('linkPreview('));
    expect(without).toEqual([]);
  });
});
