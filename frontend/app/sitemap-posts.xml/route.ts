import { buildSitemapXml, BASE_URL, API_URL } from '@/lib/sitemap-utils';

export async function GET() {
  try {
    const res = await fetch(`${API_URL}/v1/sitemap/urls?type=posts&per_page=5000`, {
      next: { revalidate: 21600 },
    });
    const json = await res.json();
    const posts = json.data?.posts || [];

    // Every knowledge item lives at the canonical /posts/{id} URL. The API already
    // excludes drafts, private, and deleted content, so we only map ids to URLs.
    const entries = posts.map((p: { id: string; updated_at: string }) => ({
      loc: `${BASE_URL}/posts/${p.id}`,
      lastmod: p.updated_at,
      changefreq: 'weekly' as const,
      priority: 0.8,
    }));

    return buildSitemapXml(entries);
  } catch {
    return buildSitemapXml([]);
  }
}
