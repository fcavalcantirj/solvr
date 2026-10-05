import { readForPage } from './read-for-page';

// fetchSEO reads a page's search verdict (GET /v1/posts/{id}/seo or
// /v1/rooms/{slug}/seo, task idx 80) for server rendering, never from a stored copy.
//
// A refusal the API answers on purpose (404, 401, 403: the post or room is gone, or
// private) comes back as null, which pages render as noindex. An API failure (5xx, 429)
// or an unreachable API THROWS, like the page's own read (lib/seo/read-for-page.ts), so
// the page answers a retryable 500 (SPEC.md Part 27.4). It used to come back as null
// too: one bad minute of the API then told a crawler that a healthy page was noindex.
export async function fetchSEO<T>(path: string): Promise<T | null> {
  const { data } = await readForPage<{ data?: T }>(path);
  return data?.data ?? null;
}
