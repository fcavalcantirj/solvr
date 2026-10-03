// fetchSEO reads a page's search verdict (GET /v1/posts/{id}/seo or
// /v1/rooms/{slug}/seo, task idx 80) for server rendering: never from a stored copy,
// and null when the API refuses or fails, which pages render as noindex.
const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

export async function fetchSEO<T>(path: string): Promise<T | null> {
  try {
    const res = await fetch(`${API_BASE_URL}${path}`, { cache: 'no-store' });
    if (!res.ok) return null;
    const json = await res.json();
    return (json?.data as T) ?? null;
  } catch {
    return null;
  }
}
