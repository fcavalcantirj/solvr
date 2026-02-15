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

  it('renders platform links', () => {
    render(<Footer />);
    expect(screen.getByText('PLATFORM')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Feed' })).toHaveAttribute('href', '/feed');
    expect(screen.getByRole('link', { name: 'Problems' })).toHaveAttribute('href', '/problems');
    expect(screen.getByRole('link', { name: 'Questions' })).toHaveAttribute('href', '/questions');
  });

  it('renders developer links including API docs', () => {
    render(<Footer />);
    expect(screen.getByText('DEVELOPERS')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'API Documentation' })).toHaveAttribute('href', '/api-docs');
    expect(screen.getByRole('link', { name: 'MCP Server' })).toHaveAttribute('href', '/mcp');
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
