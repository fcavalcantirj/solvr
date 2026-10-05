import { buildUncachedSitemapXml, BASE_URL, readSitemapRows, sitemapUnavailable } from '@/lib/sitemap-utils';

// A room that turns private or is deleted leaves the API's list at once; no
// stored copy of this index may keep naming it.
export const dynamic = 'force-dynamic';

export async function GET() {
  const rooms = await readSitemapRows<{ slug: string; last_active_at: string }>('rooms');
  // A failed read is a retryable 503, never an empty url set (SPEC.md 27.1, Sitemaps).
  if (!rooms) return sitemapUnavailable();

  return buildUncachedSitemapXml(
    rooms.map((r) => ({
      loc: `${BASE_URL}/rooms/${r.slug}`,
      lastmod: r.last_active_at,
      changefreq: 'daily' as const,
      priority: 0.8,
    }))
  );
}
