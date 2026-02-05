/**
 * Tests for Privacy Policy page
 * Per PRD requirement: Create /privacy page with Privacy Policy content
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
import PrivacyPage from '../app/privacy/page';

describe('Privacy Policy Page', () => {
  it('renders the main heading', () => {
    render(<PrivacyPage />);

    const heading = screen.getByRole('heading', { name: /privacy policy/i, level: 1 });
    expect(heading).toBeInTheDocument();
  });

  it('renders the page with main element', () => {
    render(<PrivacyPage />);

    const main = screen.getByRole('main');
    expect(main).toBeInTheDocument();
  });

  it('displays last updated date', () => {
    render(<PrivacyPage />);

    expect(screen.getByText(/last updated/i)).toBeInTheDocument();
  });

  describe('Content Sections - Per SPEC.md 19.1', () => {
    it('contains data collection section', () => {
      render(<PrivacyPage />);

      expect(screen.getByRole('heading', { name: /data we collect/i })).toBeInTheDocument();
    });

    it('contains how data is used section', () => {
      render(<PrivacyPage />);

      expect(screen.getByRole('heading', { name: /how we use your data/i })).toBeInTheDocument();
    });

    it('contains third-party sharing section', () => {
      render(<PrivacyPage />);

      expect(screen.getByRole('heading', { name: /data sharing/i })).toBeInTheDocument();
    });

    it('contains data retention section', () => {
      render(<PrivacyPage />);

      expect(screen.getByRole('heading', { name: /data retention/i })).toBeInTheDocument();
    });

    it('contains user rights section', () => {
      render(<PrivacyPage />);

      expect(screen.getByRole('heading', { name: /your rights/i })).toBeInTheDocument();
    });

    it('contains cookie policy section', () => {
      render(<PrivacyPage />);

      expect(screen.getByRole('heading', { name: /cookie/i })).toBeInTheDocument();
    });
  });

  describe('AI Agent Context - Per SPEC.md 19.1', () => {
    it('mentions AI agents as content creators', () => {
      render(<PrivacyPage />);

      expect(screen.getByRole('heading', { name: /ai agent privacy/i })).toBeInTheDocument();
    });
  });

  describe('Accessibility', () => {
    it('has proper heading hierarchy', () => {
      render(<PrivacyPage />);

      // Should have h1 for main title
      const h1 = screen.getByRole('heading', { level: 1 });
      expect(h1).toBeInTheDocument();

      // Should have section headings (h2)
      const h2s = screen.getAllByRole('heading', { level: 2 });
      expect(h2s.length).toBeGreaterThan(0);
    });

    it('is contained in main element for screen readers', () => {
      render(<PrivacyPage />);

      const main = screen.getByRole('main');
      expect(main).toBeInTheDocument();
    });
  });

  describe('Styling', () => {
    it('has max-width container for readability', () => {
      render(<PrivacyPage />);

      const main = screen.getByRole('main');
      expect(main.className).toMatch(/max-w-/);
    });

    it('has padding for proper spacing', () => {
      render(<PrivacyPage />);

      const main = screen.getByRole('main');
      expect(main.className).toMatch(/p-|px-|py-/);
    });
  });

  describe('Navigation', () => {
    it('contains link to terms of service', () => {
      render(<PrivacyPage />);

      const termsLink = screen.getByRole('link', { name: /terms/i });
      expect(termsLink).toBeInTheDocument();
      expect(termsLink).toHaveAttribute('href', '/terms');
    });
  });

  describe('Contact Information', () => {
    it('provides contact method for privacy concerns', () => {
      render(<PrivacyPage />);

      // Should have contact section heading
      expect(screen.getByRole('heading', { name: /contact us/i })).toBeInTheDocument();
    });
  });
});

describe('Privacy Page Metadata', () => {
  it('exports correct metadata with title', async () => {
    const { metadata } = await import('../app/privacy/metadata');
    expect(metadata.title).toBe('Privacy Policy | Solvr');
  });

  it('exports metadata with description', async () => {
    const { metadata } = await import('../app/privacy/metadata');
    expect(metadata.description).toBeDefined();
    expect(metadata.description).toContain('Privacy');
  });

  it('has Open Graph metadata', async () => {
    const { metadata } = await import('../app/privacy/metadata');
    expect(metadata.openGraph).toBeDefined();
    const og = metadata.openGraph as Record<string, unknown>;
    expect(og.title).toBe('Privacy Policy | Solvr');
  });

  it('has Twitter card metadata', async () => {
    const { metadata } = await import('../app/privacy/metadata');
    expect(metadata.twitter).toBeDefined();
    const twitter = metadata.twitter as Record<string, unknown>;
    expect(twitter.card).toBe('summary_large_image');
  });
});
