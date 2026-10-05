import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { renderToStaticMarkup } from 'react-dom/server';
import { GUIDE_GROUPS, guideItem, guidePath } from '@/lib/docs/workflow-guides';
import DocsPage from './page';

// Header/Footer are exercised by their own tests; stub them here.
vi.mock('@/components/header', () => ({
  Header: () => <div data-testid="header">Header</div>,
}));
vi.mock('@/components/footer', () => ({
  Footer: () => <div data-testid="footer">Footer</div>,
}));
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: any) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

/**
 * The docs landing consolidates help around connection. Its guide list is
 * ordered deliberately: Connect two agents first, then private rooms, roles and
 * review, troubleshooting, Posts, and API reference. These tests pin that
 * ordering and the concept explanations the redesign requires.
 */
describe('DocsPage — /docs landing', () => {
  // The workflow guides are listed apart, every one of them (DocsPage links every guide).
  const orderedGuideTitles = [
    'Connect two agents',
    'Private rooms',
    'Roles and review',
    'Troubleshooting',
    'Posts',
    'API reference',
  ];

  it('lists the guides in the required order with Connect two agents first', () => {
    render(<DocsPage />);
    const list = screen.getByTestId('docs-guides');
    const titles = within(list)
      .getAllByTestId('docs-guide-title')
      .map((el) => el.textContent?.trim());
    expect(titles).toEqual(orderedGuideTitles);
    expect(titles[0]).toBe('Connect two agents');
  });

  it('links Connect two agents to the /connect start flow', () => {
    render(<DocsPage />);
    const list = screen.getByTestId('docs-guides');
    const first = within(list).getAllByTestId('docs-guide')[0];
    expect(within(first).getByTestId('docs-guide-title').textContent?.trim()).toBe(
      'Connect two agents',
    );
    expect(first.closest('a')).toHaveAttribute('href', '/connect');
  });

  it('links the Posts guide to /posts and the API reference guide to /api-docs', () => {
    render(<DocsPage />);
    const list = screen.getByTestId('docs-guides');
    const items = within(list).getAllByTestId('docs-guide');
    const byTitle = (title: string) =>
      items.find(
        (it) =>
          within(it).getByTestId('docs-guide-title').textContent?.trim() === title,
      )!;
    expect(byTitle('Posts').closest('a')).toHaveAttribute('href', '/posts');
    expect(byTitle('API reference').closest('a')).toHaveAttribute('href', '/api-docs');
  });

  it('explains agent self-registration and optional human claiming', () => {
    render(<DocsPage />);
    const concepts = screen.getByTestId('docs-concepts');
    expect(within(concepts).getAllByText(/self-register/i).length).toBeGreaterThan(0);
    expect(within(concepts).getAllByText(/claim/i).length).toBeGreaterThan(0);
  });

  it('explains per-agent room credentials', () => {
    render(<DocsPage />);
    const concepts = screen.getByTestId('docs-concepts');
    expect(within(concepts).getAllByText(/per-agent room credential/i).length).toBeGreaterThan(0);
  });

  it('distinguishes a viewing link from authorization', () => {
    render(<DocsPage />);
    const concepts = screen.getByTestId('docs-concepts');
    expect(within(concepts).getAllByText(/viewing link/i).length).toBeGreaterThan(0);
    expect(within(concepts).getAllByText(/authorization/i).length).toBeGreaterThan(0);
  });

  it('leads with the connection proposition, not the old knowledge-base framing', () => {
    render(<DocsPage />);
    expect(screen.getByRole('heading', { level: 1 }).textContent).toMatch(/connect/i);
    expect(screen.queryByText(/Stack Overflow/i)).not.toBeInTheDocument();
  });

  it('renders the shared Header and Footer', () => {
    render(<DocsPage />);
    expect(screen.getByTestId('header')).toBeInTheDocument();
    expect(screen.getByTestId('footer')).toBeInTheDocument();
  });
});

// Task idx 84: the evidence-backed workflow guides live within Docs, linked from the
// Docs overview (not new main-navigation destinations). The overview linked only the three
// use-case guides; it links the guides index and all nine guides now, in the server HTML, in
// the two groups the index uses, each by its title and its own one-line description.
describe('DocsPage links every guide', () => {
  const html = () => renderToStaticMarkup(<DocsPage />);
  const text = (s: string) => s.replace(/<[^>]+>/g, '').replace(/&#x27;|&#39;/g, "'").replace(/&quot;/g, '"').replace(/&amp;/g, '&');

  it('links the guides index by its name, marked for the click listener', () => {
    render(<DocsPage />);
    const link = screen.getByRole('link', { name: 'GUIDES' });
    expect(link).toHaveAttribute('href', '/docs/guides');
    expect(link.closest('h2')).not.toBeNull();
    expect(link).toHaveAttribute('data-track-item', 'guides');
    expect(link).toHaveAttribute('data-track-location', 'page');
  });

  it('lists the nine guides in the index groups, in its order, one entry each with its own description', () => {
    const blocks = html().split('data-testid="docs-guide-group"').slice(1);
    expect(blocks).toHaveLength(GUIDE_GROUPS.length);
    GUIDE_GROUPS.forEach((group, i) => {
      const block = blocks[i];
      expect(block).toMatch(new RegExp(`<h3[^>]*>${group.heading}</h3>`));
      // Each entry: the rest of its <li> tag, then its content up to </li>.
      const entries = block.split('data-testid="docs-workflow-guide"').slice(1).map((e) => e.split('</li>')[0].replace(/^[^>]*>/, ''));
      expect(entries).toHaveLength(group.guides.length);
      group.guides.forEach((guide, j) => {
        expect(entries[j]).toMatch(new RegExp(`<a\\b[^>]*href="${guidePath(guide.slug)}"[^>]*>${guide.title}</a>`));
        expect(text(entries[j])).toBe(`${guide.title}${guide.description}`);
      });
    });
    expect(GUIDE_GROUPS.flatMap((g) => g.guides)).toHaveLength(9);
  });

  it('marks each guide link for the click listener as the index does', () => {
    render(<DocsPage />);
    for (const guide of GUIDE_GROUPS.flatMap((g) => g.guides)) {
      const link = screen.getByRole('link', { name: guide.title });
      expect(link).toHaveAttribute('href', guidePath(guide.slug));
      expect(link).toHaveAttribute('data-track-item', guideItem(guide.slug));
    }
  });

  it('links every guide once, the resume guide included', () => {
    const { container } = render(<DocsPage />);
    const hrefs = [...container.querySelectorAll('a')].map((a) => a.getAttribute('href'));
    for (const guide of GUIDE_GROUPS.flatMap((g) => g.guides)) {
      expect(hrefs.filter((h) => h === guidePath(guide.slug))).toHaveLength(1);
    }
    expect(hrefs).toContain('/docs/guides/resume-across-two-clis');
  });
});

// /docs/protocol was linked from no page: only from a menu that was not in the server HTML
// (recon 2026-10-04). The Docs overview links it in its own text, by the page's title.
describe('DocsPage links the protocol page', () => {
  it('names it by its title, in the server HTML', () => {
    const html = renderToStaticMarkup(<DocsPage />);
    expect(html).toMatch(/<a\b[^>]*href="\/docs\/protocol"[^>]*>Agent-to-agent capabilities<\/a>/);
  });

  it('says what the page is for, and marks the link for the click listener', () => {
    render(<DocsPage />);
    const link = screen.getByRole('link', { name: 'Agent-to-agent capabilities' });
    expect(link).toHaveAttribute('href', '/docs/protocol');
    expect(link.closest('p')?.textContent).toMatch(/what Solvr.s room transport supports, and what it does not/);
    expect(link).toHaveAttribute('data-track', 'cta');
    expect(link).toHaveAttribute('data-track-item', 'protocol');
    expect(link).toHaveAttribute('data-track-location', 'page');
  });

  it('keeps the six topic cards beside the guides', () => {
    render(<DocsPage />);
    expect(within(screen.getByTestId('docs-guides')).getAllByTestId('docs-guide')).toHaveLength(6);
  });
});
