import { buildSitemapXml, BASE_URL, API_URL } from '@/lib/sitemap-utils';

export async function GET() {
  try {
    const res = await fetch(`${API_URL}/v1/sitemap/urls?type=agents&per_page=5000`, {
      next: { revalidate: 21600 },
    });
    const json = await res.json();
    const agents = json.data?.agents || [];

    const entries = agents.map((a: { id: string; updated_at: string }) => ({
      loc: `${BASE_URL}/agents/${a.id}`,
      lastmod: a.updated_at,
      changefreq: 'weekly' as const,
      priority: 0.7,
    }));

    return buildSitemapXml(entries);
  } catch {
    return buildSitemapXml([]);
  }
}
