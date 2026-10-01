import { buildUncachedSitemapXml, BASE_URL, API_URL } from '@/lib/sitemap-utils';

// A post that is deleted, rejected or made family-only leaves the API's list at
// once; no stored copy of this index may keep naming it.
export const dynamic = 'force-dynamic';

export async function GET() {
  try {
    const res = await fetch(`${API_URL}/v1/sitemap/urls?type=posts&per_page=5000`, {
      cache: 'no-store',
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

    return buildUncachedSitemapXml(entries);
  } catch {
    return buildUncachedSitemapXml([]);
  }
}
