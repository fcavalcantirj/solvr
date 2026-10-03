import { NextResponse } from 'next/server';
import { API_URL, BASE_URL } from '@/lib/sitemap-utils';

// Each sub-sitemap with the type whose newest material change dates it (task idx 83).
// The core sitemap lists static routes that have no recorded change time.
const SUB_SITEMAPS: { name: string; type?: 'posts' | 'agents' | 'blog_posts' | 'rooms' }[] = [
  { name: 'sitemap-core.xml' },
  { name: 'sitemap-posts.xml', type: 'posts' },
  { name: 'sitemap-agents.xml', type: 'agents' },
  { name: 'sitemap-blog.xml', type: 'blog_posts' },
  { name: 'sitemap-rooms.xml', type: 'rooms' },
];

// The listings change whenever content does, so the index is read fresh too.
export const dynamic = 'force-dynamic';

type Lastmods = Partial<Record<'posts' | 'agents' | 'blog_posts' | 'rooms', string | null>>;

// lastmods reads each type's newest material change from GET /v1/sitemap/counts. When
// the API cannot answer, the index names no lastmod at all rather than a made-up one.
async function lastmods(): Promise<Lastmods> {
  try {
    const res = await fetch(`${API_URL}/v1/sitemap/counts`, { cache: 'no-store' });
    if (!res.ok) return {};
    const json = await res.json();
    return json.data?.lastmod ?? {};
  } catch {
    return {};
  }
}

export async function GET() {
  const dates = await lastmods();
  const xml = `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${SUB_SITEMAPS.map(({ name, type }) => {
  const lastmod = type ? dates[type] : null;
  return `  <sitemap>
    <loc>${BASE_URL}/${name}</loc>${lastmod ? `\n    <lastmod>${lastmod}</lastmod>` : ''}
  </sitemap>`;
}).join('\n')}
</sitemapindex>`;

  return new NextResponse(xml, {
    headers: {
      'Content-Type': 'text/xml; charset=utf-8',
      'Cache-Control': 'private, no-cache, no-store, max-age=0, must-revalidate',
    },
  });
}
