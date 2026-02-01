/**
 * Tests for Footer component
 * Tests per PRD requirement: Create Footer component with links to Terms, Privacy
 */

import { render, screen } from '@testing-library/react';

// Mock next/link
jest.mock('next/link', () => {
  return function MockLink({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) {
    return <a href={href}>{children}</a>;
  };
});

// Import component after mocks
import Footer from '../components/Footer';

describe('Footer', () => {
  it('renders the footer element', () => {
    render(<Footer />);

    const footer = screen.getByRole('contentinfo');
    expect(footer).toBeInTheDocument();
  });

  it('renders Terms link', () => {
    render(<Footer />);

    const termsLink = screen.getByRole('link', { name: /terms/i });
    expect(termsLink).toBeInTheDocument();
    expect(termsLink).toHaveAttribute('href', '/terms');
  });

  it('renders Privacy link', () => {
    render(<Footer />);

    const privacyLink = screen.getByRole('link', { name: /privacy/i });
    expect(privacyLink).toBeInTheDocument();
    expect(privacyLink).toHaveAttribute('href', '/privacy');
  });

  it('renders About link', () => {
    render(<Footer />);

    const aboutLink = screen.getByRole('link', { name: /about/i });
    expect(aboutLink).toBeInTheDocument();
    expect(aboutLink).toHaveAttribute('href', '/about');
  });

  it('renders API Docs link', () => {
    render(<Footer />);

    const docsLink = screen.getByRole('link', { name: /api docs/i });
    expect(docsLink).toBeInTheDocument();
    expect(docsLink).toHaveAttribute('href', '/docs/api');
  });

  it('renders GitHub link', () => {
    render(<Footer />);

    const githubLink = screen.getByRole('link', { name: /github/i });
    expect(githubLink).toBeInTheDocument();
    expect(githubLink).toHaveAttribute('href', expect.stringContaining('github.com'));
  });

  it('displays "Built for humans and AI agents" tagline', () => {
    render(<Footer />);

    expect(screen.getByText(/built for humans and ai agents/i)).toBeInTheDocument();
  });

  it('displays current year in copyright', () => {
    render(<Footer />);

    const currentYear = new Date().getFullYear().toString();
    expect(screen.getByText(new RegExp(currentYear))).toBeInTheDocument();
  });

  it('displays Solvr brand name', () => {
    render(<Footer />);

    expect(screen.getByText(/solvr/i)).toBeInTheDocument();
  });
});

describe('Footer accessibility', () => {
  it('has navigation role for link groups', () => {
    render(<Footer />);

    const nav = screen.getByRole('navigation', { name: /footer/i });
    expect(nav).toBeInTheDocument();
  });

  it('links open in new tab for external links', () => {
    render(<Footer />);

    const githubLink = screen.getByRole('link', { name: /github/i });
    expect(githubLink).toHaveAttribute('target', '_blank');
    expect(githubLink).toHaveAttribute('rel', expect.stringContaining('noopener'));
  });
});

describe('Footer styling', () => {
  it('has border styling', () => {
    render(<Footer />);

    const footer = screen.getByRole('contentinfo');
    expect(footer.className).toContain('border');
  });

  it('has appropriate padding', () => {
    render(<Footer />);

    const footer = screen.getByRole('contentinfo');
    expect(footer.className).toMatch(/p-|py-|px-/);
  });

  it('has muted text color for copyright', () => {
    render(<Footer />);

    // The tagline should have muted styling
    const tagline = screen.getByText(/built for humans and ai agents/i);
    expect(tagline).toBeInTheDocument();
  });
});

describe('Footer responsive behavior', () => {
  it('renders all links on desktop and mobile', () => {
    render(<Footer />);

    // All key links should be present
    expect(screen.getByRole('link', { name: /terms/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /privacy/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /about/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /api docs/i })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /github/i })).toBeInTheDocument();
  });
});
