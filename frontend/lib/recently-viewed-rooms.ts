// Per-browser "recently viewed" public rooms list.
//
// This is local UI convenience state (like the browser's own history), NOT domain
// data. The API stays the sole authority on what a room contains and who may read
// it — this store only remembers which PUBLIC rooms this browser opened so the
// Rooms page can offer a quick way back. It persists ONLY non-secret public room
// references (slug + display name + a local timestamp); never tokens, message
// bodies, private-room references, or any other field a caller might pass.

const STORAGE_KEY = 'solvr:recently-viewed-rooms';

/** Keep the list small — this is a quick return path, not a history log. */
export const RECENT_ROOMS_MAX = 6;

export interface RecentRoomRef {
  slug: string;
  displayName: string;
  /** ISO timestamp of when this browser last opened the room. */
  viewedAt: string;
}

function isBrowser(): boolean {
  return typeof window !== 'undefined' && typeof window.localStorage !== 'undefined';
}

function isValidRef(value: unknown): value is RecentRoomRef {
  if (!value || typeof value !== 'object') return false;
  const r = value as Record<string, unknown>;
  return (
    typeof r.slug === 'string' &&
    r.slug.length > 0 &&
    typeof r.displayName === 'string' &&
    typeof r.viewedAt === 'string'
  );
}

/** Read the stored list, newest first. Tolerates absent/corrupt storage. */
export function getRecentRooms(): RecentRoomRef[] {
  if (!isBrowser()) return [];
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(isValidRef).slice(0, RECENT_ROOMS_MAX);
  } catch {
    return [];
  }
}

function write(rooms: RecentRoomRef[]): void {
  if (!isBrowser()) return;
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(rooms));
  } catch {
    // Storage full or unavailable — a lost convenience entry is non-fatal.
  }
}

/**
 * Record that this browser opened a public room. Only the slug and a display
 * name are persisted — any other field on the argument is deliberately dropped
 * so a secret can never leak into local storage. Deduplicates by slug and keeps
 * the most recent view at the front, capped at RECENT_ROOMS_MAX.
 */
export function recordRoomView(ref: { slug: string; displayName: string }): RecentRoomRef[] {
  if (!isBrowser()) return [];
  if (!ref || typeof ref.slug !== 'string' || ref.slug.length === 0) {
    return getRecentRooms();
  }
  const entry: RecentRoomRef = {
    slug: ref.slug,
    displayName: typeof ref.displayName === 'string' && ref.displayName.length > 0 ? ref.displayName : ref.slug,
    viewedAt: new Date().toISOString(),
  };
  const rest = getRecentRooms().filter((r) => r.slug !== entry.slug);
  const next = [entry, ...rest].slice(0, RECENT_ROOMS_MAX);
  write(next);
  return next;
}

/** Remove a single reference — used when the API reports a room is no longer accessible. */
export function removeRecentRoom(slug: string): RecentRoomRef[] {
  if (!isBrowser()) return [];
  const next = getRecentRooms().filter((r) => r.slug !== slug);
  write(next);
  return next;
}

/** Clear the whole list (the user's explicit "clear" action). */
export function clearRecentRooms(): void {
  if (!isBrowser()) return;
  try {
    window.localStorage.removeItem(STORAGE_KEY);
  } catch {
    // non-fatal
  }
}
