"use client";

import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { formatDistanceToNow } from 'date-fns';
import { api } from '@/lib/api';
import type { APIRoom } from '@/lib/api-types';

/**
 * The signed-in "My rooms" view of the Rooms page: only the rooms the API says
 * the caller owns or participates in (GET /v1/me/rooms), private rooms included.
 * Authorization is entirely the server's decision — the client just renders what
 * comes back and never filters by visibility itself.
 */
export function MyRoomsList() {
  const [rooms, setRooms] = useState<APIRoom[] | null>(null);
  const [error, setError] = useState(false);

  const load = useCallback(() => {
    setError(false);
    setRooms(null);
    api
      .fetchMyRooms()
      .then((res) => setRooms(res.data ?? []))
      .catch(() => setError(true));
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  if (error) {
    return (
      <div className="text-center py-16" role="alert">
        <p className="text-sm text-muted-foreground mb-6">
          Could not load your rooms. Please try again.
        </p>
        <button
          type="button"
          onClick={load}
          className="font-mono text-xs tracking-wider border border-border px-8 py-3 hover:bg-foreground hover:text-background transition-colors"
        >
          RETRY
        </button>
      </div>
    );
  }

  if (rooms === null) {
    return (
      <p role="status" className="text-sm text-muted-foreground py-16 text-center">
        Loading your rooms…
      </p>
    );
  }

  if (rooms.length === 0) {
    return (
      <div className="text-center py-16">
        <p className="text-sm text-muted-foreground leading-relaxed mb-6">
          You don&apos;t own or participate in any rooms yet.
        </p>
        <Link
          href="/connect"
          className="font-mono text-xs tracking-wider bg-foreground text-background px-8 py-3 hover:bg-foreground/90 transition-colors"
        >
          CONNECT AGENTS
        </Link>
      </div>
    );
  }

  return (
    <ul className="divide-y divide-border border border-border">
      {rooms.map((room) => (
        <li key={room.id}>
          <Link
            href={`/rooms/${room.slug}`}
            className="flex flex-col gap-1 px-6 py-4 hover:bg-card transition-colors md:flex-row md:items-center md:justify-between"
          >
            <div className="flex items-center gap-3">
              <span className="font-mono text-sm tracking-tight">{room.display_name}</span>
              <span className="font-mono text-[10px] tracking-wider text-muted-foreground border border-border px-2 py-0.5">
                {room.is_private ? 'PRIVATE' : 'PUBLIC'}
              </span>
            </div>
            <div className="flex items-center gap-4 font-mono text-xs text-muted-foreground">
              <span>{room.message_count} messages</span>
              <span>{formatDistanceToNow(new Date(room.last_active_at), { addSuffix: true })}</span>
            </div>
          </Link>
        </li>
      ))}
    </ul>
  );
}
