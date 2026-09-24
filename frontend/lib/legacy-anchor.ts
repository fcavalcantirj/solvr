// Legacy contribution anchors resolution.
//
// Before the unified Post/Reply model, a link into a contribution used a URL
// fragment such as #approach-<id>, #answer-<id>, #response-<id> or
// #comment-<id> (some links used a bare <id>). The contribution migration
// converted every legacy row into a canonical reply that carries its original
// identifier in `legacy_id`, so a legacy anchor can be resolved to the
// canonical reply the visitor was pointed at. Fragments never reach the server,
// so this runs client-side after the canonical post's replies have loaded.
//
// This is not business logic: the API owns the mapping (each reply reports its
// own legacy_id); the client only looks up which loaded reply matches so it can
// scroll to it.

const LEGACY_PREFIXES = ['approach-', 'answer-', 'response-', 'comment-'];

// extractLegacyId turns a URL hash into the bare legacy contribution id, or null
// when there is nothing to resolve.
export function extractLegacyId(hash: string): string | null {
  if (!hash) return null;
  let id = hash.replace(/^#/, '');
  if (!id) return null;
  for (const prefix of LEGACY_PREFIXES) {
    if (id.startsWith(prefix)) {
      id = id.slice(prefix.length);
      break;
    }
  }
  return id || null;
}

type ReplyRef = { id: string; legacy_id?: string | null };

// resolveLegacyAnchor returns the canonical reply id for a legacy anchor, or
// null when no loaded reply was migrated from that legacy contribution.
export function resolveLegacyAnchor(hash: string, replies: ReplyRef[]): string | null {
  const legacyId = extractLegacyId(hash);
  if (!legacyId) return null;
  const match = replies.find((r) => r.legacy_id != null && r.legacy_id === legacyId);
  return match ? match.id : null;
}
