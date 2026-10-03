import type { MetadataRoute } from 'next';

// robots.txt only spares crawl budget. It disallows nothing on the site itself:
// account, sign-in and composer pages carry a noindex that a crawler must be able
// to fetch to obey (lib/seo/route-policy.ts), and private data is protected by
// authorization, never by robots.txt.
export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: 'semrushbot',
        disallow: '/',
      },
      {
        userAgent: 'ahrefsbot',
        disallow: '/',
      },
      {
        userAgent: 'MJ12bot',
        disallow: '/',
      },
      {
        userAgent: '*',
        allow: '/',
      },
    ],
    sitemap: 'https://solvr.dev/sitemap.xml',
    host: 'https://solvr.dev',
  };
}
