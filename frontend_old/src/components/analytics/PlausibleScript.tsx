'use client';

/**
 * PlausibleScript component
 * Renders Plausible analytics script tag with privacy-focused configuration
 *
 * Per SPEC.md Part 19.3:
 * - Plausible is privacy-focused, GDPR-compliant
 * - No cookies required
 * - Self-hostable option supported via NEXT_PUBLIC_PLAUSIBLE_HOST
 *
 * Environment variables:
 * - NEXT_PUBLIC_PLAUSIBLE_DOMAIN: The domain to track (required for script to render)
 * - NEXT_PUBLIC_PLAUSIBLE_HOST: Custom Plausible host URL (optional, defaults to plausible.io)
 */

import Script from 'next/script';
import { useEffect } from 'react';

// Type declaration for window.plausible
declare global {
  interface Window {
    plausible?: (
      event: string,
      options?: { props?: Record<string, string | number | boolean> }
    ) => void;
  }
}

export function PlausibleScript() {
  const domain = process.env.NEXT_PUBLIC_PLAUSIBLE_DOMAIN;
  const host = process.env.NEXT_PUBLIC_PLAUSIBLE_HOST || 'https://plausible.io';

  // Set up window.plausible fallback function to queue events before script loads
  useEffect(() => {
    if (domain && !window.plausible) {
      // Create a queue function that stores calls until the real script loads
      window.plausible =
        window.plausible ||
        function (...args: Parameters<NonNullable<Window['plausible']>>) {
          ((window.plausible as unknown as { q?: unknown[] }).q =
            (window.plausible as unknown as { q?: unknown[] }).q || []).push(args);
        };
    }
  }, [domain]);

  // Don't render if no domain configured
  if (!domain) {
    return null;
  }

  return (
    <Script defer data-domain={domain} src={`${host}/js/script.js`} strategy="afterInteractive" />
  );
}
