import { NextResponse } from 'next/server';

interface SitemapEntry {
  loc: string;
  lastmod?: string;
  changefreq?: 'always' | 'hourly' | 'daily' | 'weekly' | 'monthly' | 'yearly' | 'never';
  priority?: number;
}

export function buildSitemapXml(entries: SitemapEntry[]): NextResponse {
  const xml = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${entries
  .map(
    (e) =>
      `  <url>
    <loc>${e.loc}</loc>${e.lastmod ? `\n    <lastmod>${e.lastmod}</lastmod>` : ''}${e.changefreq ? `\n    <changefreq>${e.changefreq}</changefreq>` : ''}${e.priority !== undefined ? `\n    <priority>${e.priority}</priority>` : ''}
  </url>`
  )
  .join('\n')}
</urlset>`;

  return new NextResponse(xml, {
    headers: {
      'Content-Type': 'text/xml; charset=utf-8',
      'Cache-Control': 'public, s-maxage=21600, stale-while-revalidate=86400',
      'CDN-Cache-Control': 'max-age=43200',
    },
  });
}

const NO_STORE = 'private, no-cache, no-store, max-age=0, must-revalidate';

// buildUncachedSitemapXml is buildSitemapXml for an index whose entries can be
// withdrawn at any moment (a post deleted or made family-only, a blog post
// unpublished, a room turned private): no shared cache may store it, so the index
// names exactly what the API lists now.
export function buildUncachedSitemapXml(entries: SitemapEntry[]): NextResponse {
  const res = buildSitemapXml(entries);
  res.headers.set('Cache-Control', NO_STORE);
  res.headers.delete('CDN-Cache-Control');
  return res;
}

export const BASE_URL = 'https://solvr.dev';
export const API_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

// How long a crawler is asked to wait when a sitemap cannot be read (SPEC.md 27.1, Sitemaps).
export const SITEMAP_RETRY_AFTER_SECONDS = 120;

// sitemapUnavailable is a sitemap's answer when its API read fails: a retryable 503 with no
// url set. An empty url set at 200 told a crawler that read it in that minute the site had no
// pages; a 503 with Retry-After tells it to come back. No cache may keep it.
export function sitemapUnavailable(): NextResponse {
  return new NextResponse('The sitemap cannot be read right now. Retry later.\n', {
    status: 503,
    headers: {
      'Content-Type': 'text/plain; charset=utf-8',
      'Retry-After': String(SITEMAP_RETRY_AFTER_SECONDS),
      'Cache-Control': NO_STORE,
    },
  });
}

// readSitemapAPI reads one API answer for a sitemap, on every request. Only a 2xx answer
// whose body `pick` recognises is an answer; anything else (a 5xx or 429, any other status, an
// unreachable API, a body that is not JSON or not the expected shape) is null, which the
// sitemap turns into sitemapUnavailable().
export async function readSitemapAPI<T>(path: string, pick: (body: unknown) => T | null): Promise<T | null> {
  try {
    const res = await fetch(`${API_URL}${path}`, { cache: 'no-store' });
    if (!res.ok) return null;
    return pick(await res.json());
  } catch {
    return null;
  }
}

// The content types GET /v1/sitemap/urls lists. The API decides what each one lists.
export type SitemapType = 'posts' | 'agents' | 'users' | 'blog_posts' | 'rooms';

// readSitemapRows reads one type's rows from GET /v1/sitemap/urls (one page of at most 5000,
// the API's largest), or null when the read is not an answer (readSitemapAPI).
export function readSitemapRows<T>(type: SitemapType): Promise<T[] | null> {
  return readSitemapAPI(`/v1/sitemap/urls?type=${type}&per_page=5000`, (body) => {
    const rows = (body as { data?: Record<string, unknown> } | null)?.data?.[type];
    return Array.isArray(rows) ? (rows as T[]) : null;
  });
}
