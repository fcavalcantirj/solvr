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
    <section aria-label="Recently viewed rooms" className="mb-8 border border-border bg-card px-6 py-4">
      <div className="flex items-center justify-between gap-4">
        <h2 className="font-mono text-xs tracking-wider text-muted-foreground">
          RECENTLY VIEWED
        </h2>
        <button
          type="button"
          onClick={handleClear}
          className="font-mono text-[10px] tracking-wider text-muted-foreground hover:text-foreground transition-colors"
        >
          CLEAR
        </button>
      </div>
      <ul className="mt-3 flex flex-wrap gap-x-6 gap-y-2">
        {rooms.map((room) => (
          <li key={room.slug}>
            <Link
              href={`/rooms/${room.slug}`}
              className="font-mono text-sm hover:underline underline-offset-4"
            >
              {room.displayName}
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
