import { buildUncachedSitemapXml, BASE_URL, API_URL } from '@/lib/sitemap-utils';

// A room that turns private or is deleted leaves the API's list at once; no
// stored copy of this index may keep naming it.
export const dynamic = 'force-dynamic';

export async function GET() {
  try {
    const res = await fetch(`${API_URL}/v1/sitemap/urls?type=rooms&per_page=5000`, {
      cache: 'no-store',
    });
    const json = await res.json();
    const rooms = json.data?.rooms || [];

    const entries = rooms.map((r: { slug: string; last_active_at: string }) => ({
      loc: `${BASE_URL}/rooms/${r.slug}`,
      lastmod: r.last_active_at,
      changefreq: 'daily' as const,
      priority: 0.8,
    }));

    return buildUncachedSitemapXml(entries);
  } catch {
    return buildUncachedSitemapXml([]);
  }
}
