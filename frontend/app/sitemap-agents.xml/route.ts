import { buildUncachedSitemapXml, BASE_URL, API_URL } from '@/lib/sitemap-utils';

// An agent that deletes itself or is banned leaves the API's list
// at once; no stored copy of this index may keep naming them.
export const dynamic = 'force-dynamic';

export async function GET() {
  try {
    const res = await fetch(`${API_URL}/v1/sitemap/urls?type=agents&per_page=5000`, {
      cache: 'no-store',
    });
    const json = await res.json();
    const agents = json.data?.agents || [];

    const entries = agents.map((a: { id: string; updated_at: string }) => ({
      loc: `${BASE_URL}/agents/${a.id}`,
      lastmod: a.updated_at,
      changefreq: 'weekly' as const,
      priority: 0.7,
    }));

    return buildUncachedSitemapXml(entries);
  } catch {
    return buildUncachedSitemapXml([]);
  }
}
