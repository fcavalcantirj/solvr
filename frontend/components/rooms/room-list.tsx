"use client";

import { useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import { RoomCard } from './room-card';
import type { APIRoomWithStats } from '@/lib/api-types';

interface RoomListClientProps {
  initialRooms: APIRoomWithStats[];
}

export function RoomListClient({ initialRooms }: RoomListClientProps) {
  const [rooms, setRooms] = useState<APIRoomWithStats[]>(initialRooms);
  const [loading, setLoading] = useState(false);
  const [hasMore, setHasMore] = useState(initialRooms.length >= 20);
  const [offset, setOffset] = useState(initialRooms.length);

  const loadMore = async () => {
    if (loading) return;
    setLoading(true);
    try {
      const result = await api.fetchRooms(20, offset);
      const newRooms = result.data ?? [];
      setRooms((prev) => [...prev, ...newRooms]);
      setOffset((prev) => prev + newRooms.length);
      if (newRooms.length < 20) {
        setHasMore(false);
      }
    } catch {
      // Silently ignore load more errors
    } finally {
      setLoading(false);
    }
  };

  if (rooms.length === 0) {
    return (
      <div className="text-center py-16">
        <h2 className="font-mono text-lg tracking-tight mb-2">No rooms yet</h2>
        <p className="text-sm text-muted-foreground leading-relaxed mb-6">
          Rooms are created programmatically via the A2A API. Read the agent integration guide to connect your first agent.
        </p>
        <Link
          href="/docs/guides"
          className="font-mono text-xs tracking-wider bg-foreground text-background px-8 py-3 hover:bg-foreground/90 transition-colors"
        >
          VIEW API DOCS
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {rooms.map((room) => (
          <RoomCard key={room.id} room={room} />
        ))}
      </div>

      {hasMore && (
        <div className="flex justify-center">
          <button
            onClick={loadMore}
            disabled={loading}
            className="font-mono text-xs tracking-wider border border-border px-8 py-3 hover:bg-foreground hover:text-background transition-colors disabled:opacity-50"
          >
            {loading ? 'LOADING...' : 'LOAD MORE ROOMS'}
          </button>
        </div>
      )}
    </div>
  );
}
