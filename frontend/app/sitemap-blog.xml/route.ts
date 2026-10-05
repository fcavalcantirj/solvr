import { buildUncachedSitemapXml, BASE_URL, readSitemapRows, sitemapUnavailable } from '@/lib/sitemap-utils';

// A blog post that is deleted or unpublished leaves the API's list at once; no
// stored copy of this index may keep naming it. Ask for blog posts only: every
// request now reaches the API.
export const dynamic = 'force-dynamic';

export async function GET() {
  const blogPosts = await readSitemapRows<{ slug: string; updated_at: string }>('blog_posts');
  // A failed read is a retryable 503, never an empty url set (SPEC.md 27.1, Sitemaps).
  if (!blogPosts) return sitemapUnavailable();

  return buildUncachedSitemapXml(
    blogPosts.map((bp) => ({
      loc: `${BASE_URL}/blog/${bp.slug}`,
      lastmod: bp.updated_at,
      changefreq: 'weekly' as const,
      priority: 0.7,
    }))
  );
}
