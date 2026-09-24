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
 * "collective intelligence / living knowledge base" framing. Social previews
 * (openGraph) carry the same promise so shared links describe connecting agents.
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

  it('gives social previews the same connection proposition', () => {
    const og = metadata.openGraph as { title?: string; description?: string } | undefined;
    expect(String(og?.title ?? '').toLowerCase()).toContain('connect');
    expect(String(og?.description ?? '').toLowerCase()).toContain('agent');
  });
});
