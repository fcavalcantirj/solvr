/**
 * Robots.txt configuration for Solvr
 * Per SPEC.md Part 19.2 - SEO
 *
 * Rules per specification:
 * - Allow: /
 * - Disallow: /admin/
 * - Disallow: /api/
 * - Disallow: /auth/
 * - Sitemap: https://solvr.dev/sitemap.xml
 */

import type { MetadataRoute } from 'next';

/**
 * Get base URL from environment or default
 */
function getBaseUrl(): string {
  return process.env.NEXT_PUBLIC_APP_URL || 'https://solvr.dev';
}

/**
 * Generate robots.txt configuration
 * Per SPEC.md Part 19.2:
 * - Allow: / (allow root)
 * - Disallow: /admin/ (admin pages not indexable)
 * - Disallow: /api/ (API endpoints not indexable)
 * - Disallow: /auth/ (auth pages not indexable)
 */
export default function robots(): MetadataRoute.Robots {
  const baseUrl = getBaseUrl();

  return {
    rules: {
      userAgent: '*',
      allow: '/',
      disallow: ['/admin/', '/api/', '/auth/'],
    },
    sitemap: `${baseUrl}/sitemap.xml`,
  };
}
