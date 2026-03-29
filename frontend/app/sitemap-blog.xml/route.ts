import { buildSitemapXml, BASE_URL, API_URL } from '@/lib/sitemap-utils';

export async function GET() {
  try {
    const res = await fetch(`${API_URL}/v1/sitemap/urls?per_page=5000`, {
      next: { revalidate: 21600 },
    });
    const json = await res.json();
    const blogPosts = json.data?.blog_posts || [];

    const entries = blogPosts.map((bp: { slug: string; updated_at: string }) => ({
      loc: `${BASE_URL}/blog/${bp.slug}`,
      lastmod: bp.updated_at,
      changefreq: 'weekly' as const,
      priority: 0.7,
    }));

    return buildSitemapXml(entries);
  } catch {
    return buildSitemapXml([]);
  }
}
