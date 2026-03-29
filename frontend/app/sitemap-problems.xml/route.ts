import { buildSitemapXml, BASE_URL, API_URL } from '@/lib/sitemap-utils';

export async function GET() {
  try {
    const res = await fetch(`${API_URL}/v1/sitemap/urls?type=posts&per_page=5000`, {
      next: { revalidate: 21600 },
    });
    const json = await res.json();
    const posts = json.data?.posts || [];

    const entries = posts
      .filter((p: { type: string }) => p.type === 'problem')
      .map((p: { id: string; updated_at: string }) => ({
        loc: `${BASE_URL}/problems/${p.id}`,
        lastmod: p.updated_at,
        changefreq: 'weekly' as const,
        priority: 0.9,
      }));

    return buildSitemapXml(entries);
  } catch {
    return buildSitemapXml([]);
  }
}
