// readForPage is a page's server read of its own resource (task idx 83, SPEC.md
// Part 27.4). A refusal the API answers on purpose (404, 401, 403...) comes back as
// { status, data: null } for the page to turn into a real 404 or a gate. An API
// failure (5xx, 429) or an unreachable API throws, so the page answers a retryable 5xx:
// a crawler that sees a 404 drops the page, one that sees a 5xx comes back later.
const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

export class UpstreamError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'UpstreamError';
  }
}

export async function readForPage<T>(path: string): Promise<{ status: number; data: T | null }> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE_URL}${path}`, { cache: 'no-store' });
  } catch {
    throw new UpstreamError(`${path}: API unreachable`);
  }
  if (res.ok) return { status: res.status, data: (await res.json()) as T };
  if (res.status >= 500 || res.status === 429) throw new UpstreamError(`${path}: API answered ${res.status}`);
  return { status: res.status, data: null };
}
