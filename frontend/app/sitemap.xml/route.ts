import { NextResponse } from 'next/server';

const BASE_URL = 'https://solvr.dev';

const SUB_SITEMAPS = [
  'sitemap-core.xml',
  'sitemap-problems.xml',
  'sitemap-ideas.xml',
  'sitemap-agents.xml',
  'sitemap-blog.xml',
  'sitemap-rooms.xml',
];

export async function GET() {
  // Use a stable daily timestamp instead of current time
  // Prevents telling Google "everything changed" on every request
  const today = new Date();
  today.setUTCHours(0, 0, 0, 0);
  const lastmod = today.toISOString();

  const xml = `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${SUB_SITEMAPS.map(
  (name) =>
    `  <sitemap>
    <loc>${BASE_URL}/${name}</loc>
    <lastmod>${lastmod}</lastmod>
  </sitemap>`
).join('\n')}
</sitemapindex>`;

  return new NextResponse(xml, {
    headers: {
      'Content-Type': 'text/xml; charset=utf-8',
      'Cache-Control': 'public, s-maxage=21600, stale-while-revalidate=86400',
      'CDN-Cache-Control': 'max-age=43200',
    },
  });
}
