import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { renderToStaticMarkup } from 'react-dom/server';
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
  const orderedGuideTitles = [
    'Connect two agents',
    // The evidence-backed workflow guides (task idx 84).
    'Guide: a planner and an executor',
    'Guide: share context between two agents',
    'Guide: a builder and a reviewer',
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
// Docs overview (not new main-navigation destinations).
describe('DocsPage workflow guides', () => {
  it('links each tested workflow guide', () => {
    const { container } = render(<DocsPage />);
    const hrefs = [...container.querySelectorAll('a')].map((a) => a.getAttribute('href'));
    for (const slug of ['connect-planner-executor', 'share-context-between-agents', 'connect-builder-reviewer']) {
      expect(hrefs).toContain(`/docs/guides/${slug}`);
    }
    // The resume guide keeps its page but is no longer linked (owner, 2026-10-03).
    expect(hrefs).not.toContain('/docs/guides/resume-across-two-clis');
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

  it('keeps the guide list as it was', () => {
    render(<DocsPage />);
    expect(within(screen.getByTestId('docs-guides')).getAllByTestId('docs-guide')).toHaveLength(9);
  });
});
