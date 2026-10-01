import { buildUncachedSitemapXml, BASE_URL, API_URL } from '@/lib/sitemap-utils';

// A user who deletes their account or is banned leaves the API's list
// at once; no stored copy of this index may keep naming them.
export const dynamic = 'force-dynamic';

export async function GET() {
  try {
    const res = await fetch(`${API_URL}/v1/sitemap/urls?type=users&per_page=5000`, {
      cache: 'no-store',
    });
    const json = await res.json();
    const users = json.data?.users || [];

    const entries = users.map((u: { id: string; updated_at: string }) => ({
      loc: `${BASE_URL}/users/${u.id}`,
      lastmod: u.updated_at,
      changefreq: 'monthly' as const,
      priority: 0.5,
    }));

    return buildUncachedSitemapXml(entries);
  } catch {
    return buildUncachedSitemapXml([]);
  }
}
