import { NextResponse } from 'next/server';
import { BASE_URL, readSitemapAPI, sitemapUnavailable } from '@/lib/sitemap-utils';

type DatedType = 'posts' | 'agents' | 'users' | 'blog_posts' | 'rooms';

// Each sub-sitemap with the type whose newest material change dates it (task idx 83).
// The core sitemap lists static routes that have no recorded change time. Every
// app/sitemap-*.xml route is named here (app/sitemap-failures.test.ts).
const SUB_SITEMAPS: { name: string; type?: DatedType }[] = [
  { name: 'sitemap-core.xml' },
  { name: 'sitemap-posts.xml', type: 'posts' },
  { name: 'sitemap-agents.xml', type: 'agents' },
  { name: 'sitemap-users.xml', type: 'users' },
  { name: 'sitemap-blog.xml', type: 'blog_posts' },
  { name: 'sitemap-rooms.xml', type: 'rooms' },
];

// The listings change whenever content does, so the index is read fresh too.
export const dynamic = 'force-dynamic';

type Lastmods = Partial<Record<DatedType, string | null>>;

// lastmods reads each type's newest material change from GET /v1/sitemap/counts, or null
// when the read is not an answer (an answer without data.lastmod included).
function lastmods(): Promise<Lastmods | null> {
  return readSitemapAPI('/v1/sitemap/counts', (body) => {
    const lastmod = (body as { data?: { lastmod?: unknown } } | null)?.data?.lastmod;
    return lastmod && typeof lastmod === 'object' && !Array.isArray(lastmod) ? (lastmod as Lastmods) : null;
  });
}

export async function GET() {
  const dates = await lastmods();
  // The API cannot answer: a retryable 503, never an index without dates or with the
  // current time (SPEC.md 27.1, Sitemaps). The content sub-sitemaps answer 503 then too.
  if (!dates) return sitemapUnavailable();

  const xml = `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${SUB_SITEMAPS.map(({ name, type }) => {
  const lastmod = type ? dates[type] : null;
  return `  <sitemap>
    <loc>${BASE_URL}/${name}</loc>${lastmod ? `\n <lastmod>${lastmod}</lastmod>` : ''}
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
