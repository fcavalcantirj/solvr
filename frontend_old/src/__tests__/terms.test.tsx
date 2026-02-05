/**
 * Tests for Terms of Service page
 * Per PRD requirement: Create /terms page with Terms of Service content
 * Per SPEC.md Part 19.1: Legal pages
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
import TermsPage from '../app/terms/page';

describe('Terms of Service Page', () => {
  it('renders the main heading', () => {
    render(<TermsPage />);

    const heading = screen.getByRole('heading', { name: /terms of service/i, level: 1 });
    expect(heading).toBeInTheDocument();
  });

  it('renders the page with main element', () => {
    render(<TermsPage />);

    const main = screen.getByRole('main');
    expect(main).toBeInTheDocument();
  });

  it('displays last updated date', () => {
    render(<TermsPage />);

    expect(screen.getByText(/last updated/i)).toBeInTheDocument();
  });

  describe('Content Sections', () => {
    it('contains acceptance of terms section', () => {
      render(<TermsPage />);

      // Look for h2 heading specifically
      expect(screen.getByRole('heading', { name: /acceptance of terms/i })).toBeInTheDocument();
    });

    it('contains user-generated content section', () => {
      render(<TermsPage />);

      expect(screen.getByRole('heading', { name: /user-generated content/i })).toBeInTheDocument();
    });

    it('contains AI agent participation section', () => {
      render(<TermsPage />);

      expect(screen.getByRole('heading', { name: /ai agent/i })).toBeInTheDocument();
    });

    it('contains API usage section', () => {
      render(<TermsPage />);

      expect(screen.getByRole('heading', { name: /api usage/i })).toBeInTheDocument();
    });

    it('contains liability section', () => {
      render(<TermsPage />);

      expect(screen.getByRole('heading', { name: /limitation of liability/i })).toBeInTheDocument();
    });

    it('contains account termination section', () => {
      render(<TermsPage />);

      expect(screen.getByRole('heading', { name: /termination/i })).toBeInTheDocument();
    });
  });

  describe('Accessibility', () => {
    it('has proper heading hierarchy', () => {
      render(<TermsPage />);

      // Should have h1 for main title
      const h1 = screen.getByRole('heading', { level: 1 });
      expect(h1).toBeInTheDocument();

      // Should have section headings (h2)
      const h2s = screen.getAllByRole('heading', { level: 2 });
      expect(h2s.length).toBeGreaterThan(0);
    });

    it('is contained in article or main element for screen readers', () => {
      render(<TermsPage />);

      const main = screen.getByRole('main');
      expect(main).toBeInTheDocument();
    });
  });

  describe('Styling', () => {
    it('has max-width container for readability', () => {
      render(<TermsPage />);

      const main = screen.getByRole('main');
      expect(main.className).toMatch(/max-w-/);
    });

    it('has padding for proper spacing', () => {
      render(<TermsPage />);

      const main = screen.getByRole('main');
      expect(main.className).toMatch(/p-|px-|py-/);
    });
  });

  describe('Navigation', () => {
    it('contains link to privacy policy', () => {
      render(<TermsPage />);

      const privacyLink = screen.getByRole('link', { name: /privacy/i });
      expect(privacyLink).toBeInTheDocument();
      expect(privacyLink).toHaveAttribute('href', '/privacy');
    });
  });
});

describe('Terms Page Metadata', () => {
  it('exports correct metadata with title', async () => {
    const { metadata } = await import('../app/terms/metadata');
    expect(metadata.title).toBe('Terms of Service | Solvr');
  });

  it('exports metadata with description', async () => {
    const { metadata } = await import('../app/terms/metadata');
    expect(metadata.description).toBeDefined();
    expect(metadata.description).toContain('Terms');
  });

  it('has Open Graph metadata', async () => {
    const { metadata } = await import('../app/terms/metadata');
    expect(metadata.openGraph).toBeDefined();
    const og = metadata.openGraph as Record<string, unknown>;
    expect(og.title).toBe('Terms of Service | Solvr');
  });

  it('has Twitter card metadata', async () => {
    const { metadata } = await import('../app/terms/metadata');
    expect(metadata.twitter).toBeDefined();
    const twitter = metadata.twitter as Record<string, unknown>;
    expect(twitter.card).toBe('summary_large_image');
  });
});
