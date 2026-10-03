import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
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
    'Guide: resume in a second CLI',
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
    for (const slug of ['connect-planner-executor', 'share-context-between-agents', 'connect-builder-reviewer', 'resume-across-two-clis']) {
      expect(hrefs).toContain(`/docs/guides/${slug}`);
    }
  });
});
