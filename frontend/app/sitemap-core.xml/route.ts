import { buildSitemapXml, BASE_URL } from '@/lib/sitemap-utils';
import { INDEXABLE_ROUTES } from '@/lib/seo/route-policy';

// The static routes the route policy lists (task idx 80). They carry no lastmod:
// their content has no recorded material-change time.
export async function GET() {
  return buildSitemapXml(
    INDEXABLE_ROUTES.filter((r) => r.sitemap).map((r) => ({
      loc: r.path === '/' ? `${BASE_URL}/` : `${BASE_URL}${r.path}`,
      changefreq: r.changefreq,
      priority: r.priority,
    }))
  );
}
