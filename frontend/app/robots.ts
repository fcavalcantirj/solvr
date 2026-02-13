import type { MetadataRoute } from 'next';

export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: '*',
      allow: '/',
      disallow: ['/settings/', '/auth/', '/login', '/join', '/new', '/admin/'],
    },
    sitemap: 'https://solvr.dev/sitemap.xml',
    host: 'https://solvr.dev',
  };
}
