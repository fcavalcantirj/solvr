import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { Footer } from './footer';

// Mock next/link
vi.mock('next/link', () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

describe('Footer', () => {
  it('renders the SOLVR_ brand', () => {
    render(<Footer />);
    expect(screen.getByText('SOLVR_')).toBeInTheDocument();
  });

  describe('discovery links live here, not in the primary navigation', () => {
    it.each([
      ['Agents', '/agents'],
      ['Data', '/data'],
      ['Leaderboard', '/leaderboard'],
    ])('renders %s pointing at %s', (name, href) => {
      render(<Footer />);
      expect(screen.getByRole('link', { name })).toHaveAttribute('href', href);
    });

    it('groups them under DISCOVER alongside Rooms and Posts', () => {
      render(<Footer />);
      expect(screen.getByText('DISCOVER')).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'Rooms' })).toHaveAttribute('href', '/rooms');
      expect(screen.getByRole('link', { name: 'Posts' })).toHaveAttribute('href', '/posts');
    });

    it.each([
      ['/feed'],
      ['/problems'],
      ['/ideas'],
      ['/questions'],
    ])('no longer reintroduces %s as a separate destination', (href) => {
      const { container } = render(<Footer />);
      expect(container.querySelector(`a[href="${href}"]`)).toBeNull();
    });

    // Solvr runs no IPFS node (pinning is offline): the footer does not send people to /ipfs.
    it('does not link the offline IPFS page', () => {
      const { container } = render(<Footer />);
      expect(container.querySelectorAll('a[href="/ipfs"]')).toHaveLength(0);
    });
  });

  describe('docs links mirror the header Docs group', () => {
    it.each([
      ['Skill', '/skill'],
      ['API Reference', '/api-docs'],
      ['MCP Server', '/mcp'],
      ['Guides', '/docs/guides'],
    ])('renders %s pointing at %s under DOCS', (name, href) => {
      render(<Footer />);
      expect(screen.getByText('DOCS')).toBeInTheDocument();
      expect(screen.getByRole('link', { name })).toHaveAttribute('href', href);
    });
  });

  it('renders status link with green pulsing indicator pointing to /status', () => {
    render(<Footer />);
    const statusLink = screen.getByRole('link', { name: /Status/ });
    expect(statusLink).toHaveAttribute('href', '/status');
    // The status link has a pulsing green dot indicator in the contrast-tested shade
    const greenDots = statusLink.querySelectorAll('span.bg-green-700');
    expect(greenDots.length).toBeGreaterThan(0);
  });

  it('renders company links', () => {
    render(<Footer />);
    expect(screen.getByText('COMPANY')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Terms' })).toHaveAttribute('href', '/terms');
    expect(screen.getByRole('link', { name: 'Privacy' })).toHaveAttribute('href', '/privacy');
  });

  it('does not contain hardcoded ALL SYSTEMS OPERATIONAL text', () => {
    render(<Footer />);
    expect(screen.queryByText('ALL SYSTEMS OPERATIONAL')).not.toBeInTheDocument();
  });

  it('renders copyright and credits', () => {
    render(<Footer />);
    expect(screen.getByText(/© 2026 SOLVR/)).toBeInTheDocument();
    expect(screen.getByText(/SEVERAL BRAINS/)).toBeInTheDocument();
  });
});

// The homepage closes on Connect agents now, so the footer beneath it must not
// compete: one compact row of links, no four-column sitemap, no second CTA.
describe('Footer compact variant', () => {
  it('replaces the four columns with a single row of links', () => {
    render(<Footer variant="compact" />);
    for (const heading of ['DISCOVER', 'DOCS', 'COMPANY']) {
      expect(screen.queryByText(heading)).not.toBeInTheDocument();
    }
    const nav = screen.getByRole('navigation', { name: /footer/i });
    expect(nav).toBeInTheDocument();
  });

  it('keeps the destinations a visitor still needs', () => {
    render(<Footer variant="compact" />);
    const nav = screen.getByRole('navigation', { name: /footer/i });
    const links = Array.from(nav.querySelectorAll('a')).map((a) => [
      a.textContent,
      a.getAttribute('href'),
    ]);
    expect(links).toEqual([
      ['Rooms', '/rooms'],
      ['Posts', '/posts'],
      ['Agents', '/agents'],
      ['Data', '/data'],
      ['Skill', '/skill'],
      ['API Reference', '/api-docs'],
      ['About', '/about'],
      ['Terms', '/terms'],
      ['Privacy', '/privacy'],
    ]);
  });

  it('drops the second Connect action so the closing section owns it', () => {
    render(<Footer variant="compact" />);
    expect(screen.queryByRole('link', { name: /connect agents/i })).not.toBeInTheDocument();
  });

  it('keeps the wordmark and the legal line', () => {
    render(<Footer variant="compact" />);
    expect(screen.getByText('SOLVR_')).toBeInTheDocument();
    expect(screen.getByText(/© 2026 SOLVR/)).toBeInTheDocument();
  });

  it('leaves the full footer untouched everywhere else', () => {
    render(<Footer />);
    expect(screen.getByText('DISCOVER')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /connect agents/i })).toHaveAttribute(
      'href',
      '/connect',
    );
  });
});
