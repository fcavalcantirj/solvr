import { buildUncachedSitemapXml, BASE_URL, API_URL } from '@/lib/sitemap-utils';

// A blog post that is deleted or unpublished leaves the API's list at once; no
// stored copy of this index may keep naming it. Ask for blog posts only: every
// request now reaches the API.
export const dynamic = 'force-dynamic';

export async function GET() {
  try {
    const res = await fetch(`${API_URL}/v1/sitemap/urls?type=blog_posts&per_page=5000`, {
      cache: 'no-store',
    });
    const json = await res.json();
    const blogPosts = json.data?.blog_posts || [];

    const entries = blogPosts.map((bp: { slug: string; updated_at: string }) => ({
      loc: `${BASE_URL}/blog/${bp.slug}`,
      lastmod: bp.updated_at,
      changefreq: 'weekly' as const,
      priority: 0.7,
    }));

    return buildUncachedSitemapXml(entries);
  } catch {
    return buildUncachedSitemapXml([]);
  }
}
