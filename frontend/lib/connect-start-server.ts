import type { APIConnectStart, APIConnectStartResponse } from '@/lib/api-types';

// /connect carries its default sentence in the HTML the server sends (SPEC.md 25.6): the use
// cases, the sentence, the line beside it and the Copy button are painted with the page, and
// the browser's own read of the contract replaces them when it arrives.
//
// That HTML is cached and shared, so it must never carry a flow code: no browser step stands
// behind a code minted for it. This read therefore asks for flow=none and nothing else (the
// API then mints nothing and every sentence has the plain skill link), and an answer that
// carries a flow id anyway is not used.
//
// Like the examples read (lib/connect-examples-server.ts) this also runs at BUILD time, so it
// never throws and never hangs: a refused, unreachable, slow or malformed answer is null, and
// the page shows the panel's loading state.

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

const START_TIMEOUT_MS = 5000;

// The default sentence changes only with a deploy; a read is reused for five minutes.
const START_REVALIDATE_SECONDS = 300;

export async function getDefaultConnectStart(): Promise<APIConnectStart | null> {
  try {
    const response = await fetch(`${API_BASE_URL}/v1/connect?flow=none`, {
      next: { revalidate: START_REVALIDATE_SECONDS },
      signal: AbortSignal.timeout(START_TIMEOUT_MS),
    });
    if (!response.ok) return null;
    const body = (await response.json()) as Partial<APIConnectStartResponse> | null;
    const start = body?.data;
    if (!start?.selected || !Array.isArray(start.presets) || start.presets.length === 0) return null;
    return start.selected.flow_id ? null : start;
  } catch {
    return null;
  }
}
