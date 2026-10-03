import type { APIOverviewResponse } from '@/lib/api-types';

// The index reads GET /v1/overview on the server, so its first HTML already
// carries the hero numbers and the sections below them: no blank band while the
// browser asks, and a crawler sees the numbers. The browser still refreshes once
// (hooks/use-homepage-overview.ts).
//
// This read also runs at BUILD time: production images run `next build` against
// the live API, and the index is pre-rendered. So it never throws and never
// hangs — a refused, unreachable, slow or malformed answer is null, and the page
// falls back to reading the overview in the browser.

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

// How long the index waits for the API before rendering without the overview.
const OVERVIEW_TIMEOUT_MS = 5000;

// How long a read is reused before the next request reads again (seconds). It
// matches the index's own revalidate (app/page.tsx).
const OVERVIEW_REVALIDATE_SECONDS = 60;

export async function getInitialOverview(): Promise<APIOverviewResponse | null> {
  try {
    const response = await fetch(`${API_BASE_URL}/v1/overview`, {
      next: { revalidate: OVERVIEW_REVALIDATE_SECONDS },
      signal: AbortSignal.timeout(OVERVIEW_TIMEOUT_MS),
    });
    if (!response.ok) return null;
    const body = (await response.json()) as Partial<APIOverviewResponse> | null;
    if (!body?.data) return null;
    return body as APIOverviewResponse;
  } catch {
    return null;
  }
}
