import { describe, it, expect, vi } from 'vitest';

// layout.tsx pulls in web fonts and the GA component at import time; neither is
// relevant to the metadata contract this test pins.
vi.mock('next/font/google', () => ({
  Inter: () => ({ variable: '--font-inter' }),
  JetBrains_Mono: () => ({ variable: '--font-jetbrains' }),
}));
vi.mock('@next/third-parties/google', () => ({
  GoogleAnalytics: () => null,
}));

import { metadata } from './layout';

/**
 * The site-wide metadata must lead with the connection proposition, not the old
 * "collective intelligence / living knowledge base" framing.
 *
 * The link preview is each page's own (lib/seo/link-preview.ts). The root layout hands
 * down the site name and the card only: when it also stated the home page's title and
 * description, every page that stated no preview showed the home page's.
 */
describe('root layout metadata — connection proposition', () => {
  const title =
    typeof metadata.title === 'object' && metadata.title !== null
      ? (metadata.title as { default?: string }).default ?? ''
      : String(metadata.title ?? '');

  it('titles the site around connecting agents', () => {
    expect(title.toLowerCase()).toContain('connect');
    expect(title.toLowerCase()).not.toContain('collective intelligence');
  });

  it('describes the two-agent connection promise, not a knowledge base', () => {
    const description = String(metadata.description ?? '').toLowerCase();
    expect(description).toContain('agent');
    expect(description).toMatch(/connect|room|work together/);
    expect(description).not.toContain('living knowledge base');
  });

  it('keeps connection-focused keywords', () => {
    const keywords = String(metadata.keywords ?? '').toLowerCase();
    expect(keywords).toMatch(/connect|collaborat|agent/);
    expect(keywords).not.toContain('programming q&a');
  });

  it('hands every page the site name and the preview card, with a large-image card for Twitter', () => {
    const og = metadata.openGraph as { siteName?: string; images?: { url: string; width: number; height: number }[] } | undefined;
    expect(og?.siteName).toBe('Solvr');
    expect(og?.images).toHaveLength(1);
    expect(og?.images?.[0]).toMatchObject({ width: 1200, height: 630 });
    expect(og?.images?.[0].url).toMatch(/^https:\/\/solvr\.dev\/og\/.+\.png$/);
    expect(metadata.twitter).toMatchObject({ card: 'summary_large_image' });
  });

  // Next copies a page's own title and description into a preview that states none. A
  // title here would win over that, on every page without a preview of its own.
  it('states no preview title, description or address for the pages below it to inherit', () => {
    for (const preview of [metadata.openGraph, metadata.twitter]) {
      expect(preview).toBeDefined();
      expect(preview).not.toHaveProperty('title');
      expect(preview).not.toHaveProperty('description');
      expect(preview).not.toHaveProperty('url');
    }
  });
});
