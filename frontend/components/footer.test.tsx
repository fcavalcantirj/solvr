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
      ['IPFS', '/ipfs'],
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

    it('stays compact — a single IPFS entry, not one per section', () => {
      const { container } = render(<Footer />);
      expect(container.querySelectorAll('a[href="/ipfs"]')).toHaveLength(1);
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
    // The status link has a pulsing green dot indicator (two spans with emerald colors)
    const greenDots = statusLink.querySelectorAll('span.bg-emerald-500');
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
