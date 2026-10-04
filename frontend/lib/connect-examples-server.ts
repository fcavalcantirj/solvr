import type { APIConnectExamplesResponse, APIConnectPreset } from '@/lib/api-types';

// The guides and the home cards show the API's three example sentences
// (GET /v1/connect/examples), read on the server so the first HTML carries them.
//
// Like the overview read (lib/overview-server.ts) this also runs at BUILD time, against
// the live API, so it never throws and never hangs: a refused, unreachable, slow or
// malformed answer is null, and the page renders without the sentences.

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

const EXAMPLES_TIMEOUT_MS = 5000;

// The examples change only with a deploy; a read is reused for five minutes.
const EXAMPLES_REVALIDATE_SECONDS = 300;

export async function getConnectExamples(): Promise<APIConnectPreset[] | null> {
  try {
    const response = await fetch(`${API_BASE_URL}/v1/connect/examples`, {
      next: { revalidate: EXAMPLES_REVALIDATE_SECONDS },
      signal: AbortSignal.timeout(EXAMPLES_TIMEOUT_MS),
    });
    if (!response.ok) return null;
    const body = (await response.json()) as Partial<APIConnectExamplesResponse> | null;
    const presets = body?.data?.presets;
    return Array.isArray(presets) && presets.length > 0 ? presets : null;
  } catch {
    return null;
  }
}
