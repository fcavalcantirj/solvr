import type { APIRoomMessage } from "@/lib/api-types";

// The server message id is a monotonic per-insert sequence. Ordering the
// transcript by it yields a stable oldest->newest order no matter which page a
// message arrived on — the initial recent window, an older-history page, an SSE
// push, or a single deep-link fetch. The API owns authorship, content, and
// permissions; these helpers only assemble the already-authoritative rows into a
// consistent view (no domain logic lives here).

/**
 * Merge message batches into one ordered, de-duplicated transcript.
 *
 * - De-duplicates by persistent message id so refreshed, replayed, or locally
 *   echoed copies collapse to a single row, keeping the FIRST occurrence.
 * - Orders ascending by server id (oldest first, newest last) for conventional
 *   top-to-bottom reading.
 */
export function mergeMessages(...batches: APIRoomMessage[][]): APIRoomMessage[] {
  const byId = new Map<number, APIRoomMessage>();
  for (const batch of batches) {
    for (const m of batch) {
      if (!byId.has(m.id)) byId.set(m.id, m);
    }
  }
  return Array.from(byId.values()).sort((a, b) => a.id - b.id);
}

/**
 * "Following the latest exchange" means the reader is at (or near) the bottom,
 * where the newest message lives once the transcript reads oldest->newest.
 */
export function isNearBottom(
  el: Pick<HTMLElement, "scrollTop" | "scrollHeight" | "clientHeight">,
  threshold = 64,
): boolean {
  return el.scrollHeight - el.scrollTop - el.clientHeight <= threshold;
}

// Messages longer than this collapse behind an explicit expand control so a long
// specification or verification dump does not dominate the transcript.
export const LONG_MESSAGE_CHARS = 1200;

export function isLongMessage(content: string): boolean {
  return content.length > LONG_MESSAGE_CHARS;
}
