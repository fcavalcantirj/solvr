import { buildUncachedSitemapXml, BASE_URL, readSitemapRows, sitemapUnavailable } from '@/lib/sitemap-utils';

// A post that is deleted, rejected or made family-only leaves the API's list at
// once; no stored copy of this index may keep naming it.
export const dynamic = 'force-dynamic';

export async function GET() {
  const posts = await readSitemapRows<{ id: string; updated_at: string }>('posts');
  // A failed read is a retryable 503, never an empty url set (SPEC.md 27.1, Sitemaps).
  if (!posts) return sitemapUnavailable();

  // Every knowledge item lives at the canonical /posts/{id} URL. The API already
  // excludes drafts, private, and deleted content, so we only map ids to URLs.
  return buildUncachedSitemapXml(
    posts.map((p) => ({
      loc: `${BASE_URL}/posts/${p.id}`,
      lastmod: p.updated_at,
      changefreq: 'weekly' as const,
      priority: 0.8,
    }))
  );
}
