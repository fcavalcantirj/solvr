import { buildUncachedSitemapXml, BASE_URL, readSitemapRows, sitemapUnavailable } from '@/lib/sitemap-utils';

// An agent that deletes itself or is banned leaves the API's list
// at once; no stored copy of this index may keep naming them.
export const dynamic = 'force-dynamic';

export async function GET() {
  // The API lists exactly the agents whose profile is indexable (GET /v1/agents/{id}/seo).
  const agents = await readSitemapRows<{ id: string; updated_at: string }>('agents');
  // A failed read is a retryable 503, never an empty url set (SPEC.md 27.1, Sitemaps).
  if (!agents) return sitemapUnavailable();

  return buildUncachedSitemapXml(
    agents.map((a) => ({
      loc: `${BASE_URL}/agents/${a.id}`,
      lastmod: a.updated_at,
      changefreq: 'weekly' as const,
      priority: 0.7,
    }))
  );
}
