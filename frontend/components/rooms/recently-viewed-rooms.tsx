"use client";

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import {
  getRecentRooms,
  removeRecentRoom,
  clearRecentRooms,
  type RecentRoomRef,
} from '@/lib/recently-viewed-rooms';

/**
 * A small return path on the Rooms page: the public rooms this browser opened
 * before. The list is stored locally (see lib/recently-viewed-rooms) and holds
 * only non-secret public references. The API remains the authority on whether a
 * room is still reachable — any entry it no longer serves is pruned here rather
 * than the client deciding accessibility on its own.
 */
export function RecentlyViewedRooms() {
  const [rooms, setRooms] = useState<RecentRoomRef[]>([]);

  useEffect(() => {
    const initial = getRecentRooms();
    setRooms(initial);

    // Ask the API about each remembered room; if it can no longer be read
    // (deleted, or turned private), drop the local reference.
    let cancelled = false;
    initial.forEach((ref) => {
      api.fetchRoom(ref.slug).catch(() => {
        if (cancelled) return;
        setRooms((prev) => prev.filter((r) => r.slug !== ref.slug));
        removeRecentRoom(ref.slug);
      });
    });
    return () => {
      cancelled = true;
    };
  }, []);

  if (rooms.length === 0) return null;

  const handleClear = () => {
    clearRecentRooms();
    setRooms([]);
  };

  return (
    <section
      aria-label="Recently viewed rooms"
      className="mx-4 mb-8 flex flex-wrap items-baseline gap-x-8 gap-y-3 border-t border-border py-5 sm:mx-6 lg:mx-12"
    >
      <h2 className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
        RECENTLY VIEWED
      </h2>
      <ul className="flex min-w-0 flex-1 flex-wrap gap-x-6 gap-y-2">
        {rooms.map((room) => (
          <li key={room.slug} className="min-w-0">
            <Link
              href={`/rooms/${room.slug}`}
              className="text-lg font-light tracking-[-0.015em] underline decoration-transparent underline-offset-4 transition-colors hover:decoration-current"
            >
              {room.displayName}
            </Link>
          </li>
        ))}
      </ul>
      <button
        type="button"
        onClick={handleClear}
        className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground hover:text-foreground transition-colors"
      >
        CLEAR
      </button>
    </section>
  );
}
