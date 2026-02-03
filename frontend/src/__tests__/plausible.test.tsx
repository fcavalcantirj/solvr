/**
 * Tests for Plausible Analytics integration
 * Per SPEC.md Part 19.3 - Plausible is privacy-focused, GDPR-compliant
 *
 * Requirements:
 * - Add Plausible script tag to layout.tsx
 * - Set data-domain from config (env var)
 * - Verify no PII sent to Plausible
 */

import { render } from '@testing-library/react';

// Mock next/script since it uses portals and doesn't render normally in tests
jest.mock('next/script', () => {
  return function MockScript(props: {
    src?: string;
    defer?: boolean;
    'data-domain'?: string;
    strategy?: string;
  }) {
    // Return a div with data attributes to simulate the script for testing
    return (
      <script
        data-testid="plausible-script"
        src={props.src}
        defer={props.defer}
        data-domain={props['data-domain']}
        data-strategy={props.strategy}
      />
    );
  };
});

import { PlausibleScript } from '../components/analytics/PlausibleScript';

describe('PlausibleScript component', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    jest.resetModules();
    process.env = { ...originalEnv };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  it('renders script tag when NEXT_PUBLIC_PLAUSIBLE_DOMAIN is set', () => {
    process.env.NEXT_PUBLIC_PLAUSIBLE_DOMAIN = 'solvr.dev';
    const { container } = render(<PlausibleScript />);

    const script = container.querySelector('script');
    expect(script).toBeInTheDocument();
    expect(script).toHaveAttribute('data-domain', 'solvr.dev');
    expect(script).toHaveAttribute('src', 'https://plausible.io/js/script.js');
    expect(script).toHaveAttribute('defer');
  });

  it('uses custom Plausible host if NEXT_PUBLIC_PLAUSIBLE_HOST is set', () => {
    process.env.NEXT_PUBLIC_PLAUSIBLE_DOMAIN = 'solvr.dev';
    process.env.NEXT_PUBLIC_PLAUSIBLE_HOST = 'https://analytics.solvr.dev';
    const { container } = render(<PlausibleScript />);

    const script = container.querySelector('script');
    expect(script).toHaveAttribute('src', 'https://analytics.solvr.dev/js/script.js');
  });

  it('does not render script tag when NEXT_PUBLIC_PLAUSIBLE_DOMAIN is not set', () => {
    delete process.env.NEXT_PUBLIC_PLAUSIBLE_DOMAIN;
    const { container } = render(<PlausibleScript />);

    const script = container.querySelector('script');
    expect(script).not.toBeInTheDocument();
  });

  it('does not render script tag when NEXT_PUBLIC_PLAUSIBLE_DOMAIN is empty string', () => {
    process.env.NEXT_PUBLIC_PLAUSIBLE_DOMAIN = '';
    const { container } = render(<PlausibleScript />);

    const script = container.querySelector('script');
    expect(script).not.toBeInTheDocument();
  });

  it('script has defer attribute for non-blocking load', () => {
    process.env.NEXT_PUBLIC_PLAUSIBLE_DOMAIN = 'solvr.dev';
    const { container } = render(<PlausibleScript />);

    const script = container.querySelector('script');
    // Plausible recommends defer, we ensure it's set
    expect(script).toHaveAttribute('defer');
  });

  it('does not include any PII in the script', () => {
    process.env.NEXT_PUBLIC_PLAUSIBLE_DOMAIN = 'solvr.dev';
    const { container } = render(<PlausibleScript />);

    const script = container.querySelector('script');
    // Ensure no user-related data is added as attributes
    expect(script).not.toHaveAttribute('data-user');
    expect(script).not.toHaveAttribute('data-email');
    expect(script).not.toHaveAttribute('data-user-id');
    // Only expected attributes should be present
    const attrs = script?.getAttributeNames() || [];
    expect(attrs).toContain('src');
    expect(attrs).toContain('data-domain');
  });

  it('uses afterInteractive strategy for optimal loading', () => {
    process.env.NEXT_PUBLIC_PLAUSIBLE_DOMAIN = 'solvr.dev';
    const { container } = render(<PlausibleScript />);

    const script = container.querySelector('script');
    expect(script).toHaveAttribute('data-strategy', 'afterInteractive');
  });
});

describe('Plausible global function', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    jest.resetModules();
    process.env = { ...originalEnv };
    // Clear any global plausible function
    delete (window as { plausible?: unknown }).plausible;
  });

  afterEach(() => {
    process.env = originalEnv;
    delete (window as { plausible?: unknown }).plausible;
  });

  it('PlausibleScript sets up window.plausible fallback function', () => {
    process.env.NEXT_PUBLIC_PLAUSIBLE_DOMAIN = 'solvr.dev';
    render(<PlausibleScript />);

    // The script should set up a queue for events before the script loads
    expect(typeof (window as { plausible?: unknown }).plausible).toBe('function');
  });
});

describe('trackEvent helper', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    jest.resetModules();
    process.env = { ...originalEnv };
    // Clear any global plausible function
    delete (window as { plausible?: unknown }).plausible;
  });

  afterEach(() => {
    process.env = originalEnv;
    delete (window as { plausible?: unknown }).plausible;
  });

  it('trackEvent calls window.plausible with event name and props', async () => {
    const mockPlausible = jest.fn();
    (window as { plausible?: unknown }).plausible = mockPlausible;

    const { trackEvent } = await import('../lib/analytics');

    trackEvent('Search', { query_length: 15, results: 10 });

    expect(mockPlausible).toHaveBeenCalledWith('Search', {
      props: { query_length: 15, results: 10 },
    });
  });

  it('trackEvent does not throw when window.plausible is not defined', async () => {
    delete (window as { plausible?: unknown }).plausible;

    const { trackEvent } = await import('../lib/analytics');

    // Should not throw
    expect(() => trackEvent('Search', { query_length: 15 })).not.toThrow();
  });

  it('trackEvent strips PII from props before sending', async () => {
    const mockPlausible = jest.fn();
    (window as { plausible?: unknown }).plausible = mockPlausible;

    const { trackEvent } = await import('../lib/analytics');

    // Attempt to pass PII - should be stripped
    trackEvent('Search', {
      query_length: 15,
      email: 'user@example.com', // PII - should be removed
      user_id: '12345', // PII - should be removed
      results: 5,
    });

    expect(mockPlausible).toHaveBeenCalledWith('Search', {
      props: { query_length: 15, results: 5 },
    });
  });

  it('trackEvent strips query content (only length allowed)', async () => {
    const mockPlausible = jest.fn();
    (window as { plausible?: unknown }).plausible = mockPlausible;

    const { trackEvent } = await import('../lib/analytics');

    // Attempt to pass query content - should be removed
    trackEvent('Search', {
      query: 'postgres async', // Content - should be removed
      query_length: 14,
      results: 5,
    });

    expect(mockPlausible).toHaveBeenCalledWith('Search', {
      props: { query_length: 14, results: 5 },
    });
  });

  it('trackEvent can be called without props', async () => {
    const mockPlausible = jest.fn();
    (window as { plausible?: unknown }).plausible = mockPlausible;

    const { trackEvent } = await import('../lib/analytics');

    trackEvent('PageView');

    expect(mockPlausible).toHaveBeenCalledWith('PageView');
  });
});
