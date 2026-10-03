import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import Home, { revalidate } from './page';
import { OVERVIEW, STALE_META } from '@/components/homepage/overview-fixture';
import { getInitialOverview } from '@/lib/overview-server';

// The index composes four things: the header, the compact proposition, the
// live overview, and the compact footer. The overview is read on the server,
// so the first HTML already carries the hero numbers. Everything else the old homepage
// carried — the marketing feature grid, the how-it-works explainer, the
// trending-problems showcase, the static API pitch and the separate CTA band —
// is gone, replaced by sections the API actually feeds.

vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock('@/components/header', () => ({
  Header: () => <header data-testid="page-header" />,
}));

vi.mock('@/components/hero-section', () => ({
  HeroSection: ({ heroNumbers }: { heroNumbers?: unknown[] }) => (
    <section data-testid="page-hero" data-hero-numbers={heroNumbers?.length ?? 0} />
  ),
}));

vi.mock('@/components/homepage/live-overview', () => ({
  LiveOverview: () => <div data-testid="page-overview" />,
}));

vi.mock('@/lib/overview-server', () => ({ getInitialOverview: vi.fn() }));

// The browser refresh is HomeOverview's own concern (home-overview.test.tsx);
// here it simply hands back what the server read.
vi.mock('@/hooks/use-homepage-overview', () => ({
  useHomepageOverview: (initial?: { data: unknown; meta: unknown } | null) => ({
    overview: initial?.data ?? null,
    meta: initial?.meta ?? null,
    loading: !initial,
    error: null,
  }),
}));

beforeEach(() => {
  vi.mocked(getInitialOverview).mockResolvedValue({ data: OVERVIEW, meta: STALE_META });
});

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');

describe('the index', () => {
  // Home is an async server component now: rendered as `await Home()`.
  it('renders the header, the proposition, the live overview and the footer in order', async () => {
    render(await Home());
    const header = screen.getByTestId('page-header');
    const hero = screen.getByTestId('page-hero');
    const overview = screen.getByTestId('page-overview');
    // DOCUMENT_POSITION_FOLLOWING = 4
    expect(header.compareDocumentPosition(hero) & 4).toBeTruthy();
    expect(hero.compareDocumentPosition(overview) & 4).toBeTruthy();
  });

  it('closes on the compact footer', async () => {
    render(await Home());
    const nav = screen.getByRole('navigation', { name: /footer/i });
    expect(nav).toBeInTheDocument();
    // The compact footer drops the four-column sitemap.
    expect(screen.queryByText('DISCOVER')).not.toBeInTheDocument();
  });

  it('passes the server-read hero numbers down, so the first HTML carries them', async () => {
    render(await Home());
    expect(screen.getByTestId('page-hero')).toHaveAttribute(
      'data-hero-numbers',
      String(OVERVIEW.hero_numbers.length),
    );
  });

  it('degrades to the browser read, not an error page, when the server read failed', async () => {
    vi.mocked(getInitialOverview).mockResolvedValue(null);
    render(await Home());
    expect(screen.getByTestId('page-hero')).toHaveAttribute('data-hero-numbers', '0');
    expect(screen.getByTestId('page-overview')).toBeInTheDocument();
  });

  it('is regenerated at most every 60 seconds', () => {
    expect(revalidate).toBe(60);
  });

  it('no longer mounts the marketing sections the API does not feed', () => {
    const source = read('app/page.tsx');
    for (const gone of [
      'HowItWorks',
      'FeaturesSection',
      'CollaborationShowcase',
      'ApiSection',
      'CtaSection',
    ]) {
      expect(source).not.toContain(gone);
    }
  });

  it('keeps navigation simple: the page mounts no second navigation of its own', () => {
    const source = read('app/page.tsx');
    expect(source).not.toContain('Sidebar');
    expect(source).not.toContain('<nav');
  });
});
