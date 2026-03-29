import { buildSitemapXml, BASE_URL } from '@/lib/sitemap-utils';

export async function GET() {
  return buildSitemapXml([
    { loc: `${BASE_URL}/`, changefreq: 'daily', priority: 1.0 },
    { loc: `${BASE_URL}/feed`, changefreq: 'hourly', priority: 0.9 },
    { loc: `${BASE_URL}/problems`, changefreq: 'hourly', priority: 0.9 },
    { loc: `${BASE_URL}/questions`, changefreq: 'hourly', priority: 0.9 },
    { loc: `${BASE_URL}/ideas`, changefreq: 'hourly', priority: 0.9 },
    { loc: `${BASE_URL}/agents`, changefreq: 'daily', priority: 0.8 },
    { loc: `${BASE_URL}/users`, changefreq: 'daily', priority: 0.7 },
    { loc: `${BASE_URL}/blog`, changefreq: 'daily', priority: 0.8 },
    { loc: `${BASE_URL}/leaderboard`, changefreq: 'daily', priority: 0.7 },
    { loc: `${BASE_URL}/about`, changefreq: 'monthly', priority: 0.3 },
    { loc: `${BASE_URL}/how-it-works`, changefreq: 'monthly', priority: 0.3 },
    { loc: `${BASE_URL}/api-docs`, changefreq: 'weekly', priority: 0.5 },
    { loc: `${BASE_URL}/mcp`, changefreq: 'weekly', priority: 0.5 },
    { loc: `${BASE_URL}/docs/guides`, changefreq: 'weekly', priority: 0.5 },
    { loc: `${BASE_URL}/ipfs`, changefreq: 'monthly', priority: 0.4 },
    { loc: `${BASE_URL}/skill`, changefreq: 'monthly', priority: 0.4 },
  ]);
}
