import { buildSitemapXml, BASE_URL, API_URL } from '@/lib/sitemap-utils';

export async function GET() {
  try {
    const res = await fetch(`${API_URL}/v1/sitemap/urls?type=users&per_page=5000`, {
      next: { revalidate: 21600 },
    });
    const json = await res.json();
    const users = json.data?.users || [];

    const entries = users.map((u: { id: string; updated_at: string }) => ({
      loc: `${BASE_URL}/users/${u.id}`,
      lastmod: u.updated_at,
      changefreq: 'monthly' as const,
      priority: 0.5,
    }));

    return buildSitemapXml(entries);
  } catch {
    return buildSitemapXml([]);
  }
}
