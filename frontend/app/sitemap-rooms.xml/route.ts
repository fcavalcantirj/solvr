import { buildSitemapXml, BASE_URL, API_URL } from '@/lib/sitemap-utils';

// A room that turns private or is deleted leaves the API's list at once; no
// stored copy of this index may keep naming it.
export const dynamic = 'force-dynamic';

const NO_STORE = 'private, no-cache, no-store, max-age=0, must-revalidate';

function withoutSharedCache(res: ReturnType<typeof buildSitemapXml>) {
  res.headers.set('Cache-Control', NO_STORE);
  res.headers.delete('CDN-Cache-Control');
  return res;
}

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

    return withoutSharedCache(buildSitemapXml(entries));
  } catch {
    return withoutSharedCache(buildSitemapXml([]));
  }
}
