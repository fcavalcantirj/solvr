import type { MetadataRoute } from 'next';

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
        disallow: ['/settings/', '/auth/', '/login', '/join', '/new', '/admin/', '/dashboard/'],
      },
    ],
    sitemap: 'https://solvr.dev/sitemap.xml',
    host: 'https://solvr.dev',
  };
}
