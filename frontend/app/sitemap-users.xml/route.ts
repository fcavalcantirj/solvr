import { buildUncachedSitemapXml, BASE_URL, readSitemapRows, sitemapUnavailable } from '@/lib/sitemap-utils';

// The people whose profile is indexable (SPEC.md 27.1): the API lists exactly them, by the
// rule of GET /v1/users/{id}/seo. A person who deletes their account or loses their last
// public content leaves the list at once; no stored copy of this index may keep naming them.
export const dynamic = 'force-dynamic';

export async function GET() {
  const users = await readSitemapRows<{ id: string; updated_at: string }>('users');
  // A failed read is a retryable 503, never an empty url set (SPEC.md 27.1, Sitemaps).
  if (!users) return sitemapUnavailable();

  return buildUncachedSitemapXml(
    users.map((u) => ({
      loc: `${BASE_URL}/users/${u.id}`,
      lastmod: u.updated_at,
      changefreq: 'weekly' as const,
      // A profile, under the agent profile's rule: the agent profile's priority.
      priority: 0.7,
    }))
  );
}
