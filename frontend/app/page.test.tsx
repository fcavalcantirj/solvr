import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

import Home from './page';

// The index composes four things: the header, the compact proposition, the
// live overview, and the compact footer. Everything else the old homepage
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
  HeroSection: () => <section data-testid="page-hero" />,
}));

vi.mock('@/components/homepage/live-overview', () => ({
  LiveOverview: () => <div data-testid="page-overview" />,
}));

const read = (file: string) => readFileSync(join(process.cwd(), file), 'utf8');

describe('the index', () => {
  it('renders the header, the proposition, the live overview and the footer in order', () => {
    render(<Home />);
    const header = screen.getByTestId('page-header');
    const hero = screen.getByTestId('page-hero');
    const overview = screen.getByTestId('page-overview');
    // DOCUMENT_POSITION_FOLLOWING = 4
    expect(header.compareDocumentPosition(hero) & 4).toBeTruthy();
    expect(hero.compareDocumentPosition(overview) & 4).toBeTruthy();
  });

  it('closes on the compact footer', () => {
    render(<Home />);
    const nav = screen.getByRole('navigation', { name: /footer/i });
    expect(nav).toBeInTheDocument();
    // The compact footer drops the four-column sitemap.
    expect(screen.queryByText('DISCOVER')).not.toBeInTheDocument();
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
