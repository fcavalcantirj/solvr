"use client";

import { useState } from 'react';
import { useAuth } from '@/hooks/use-auth';
import { RoomListClient } from './room-list';
import { MyRoomsList } from './my-rooms-list';
import type { APIRoomWithStats, RoomListParams } from '@/lib/api-types';

type View = 'public' | 'mine';
type RoomSort = NonNullable<RoomListParams['sort']>;

interface RoomsBrowserProps {
  initialRooms: APIRoomWithStats[];
  initialSort?: RoomSort;
}

/**
 * Wraps the public room discovery list and adds a "My rooms" filter for signed-in
 * users on the same Rooms page — deliberately NOT a separate feed-style dashboard.
 * Logged-out visitors keep the plain public discovery experience with no filter.
 */
export function RoomsBrowser({ initialRooms, initialSort = 'recent' }: RoomsBrowserProps) {
  const { isAuthenticated } = useAuth();
  const [view, setView] = useState<View>('public');
  const showMine = isAuthenticated && view === 'mine';

  const filterButton = (value: View, label: string) => (
    <button
      type="button"
      aria-pressed={view === value}
      onClick={() => setView(value)}
      className={`font-mono text-xs tracking-wider px-4 py-2 border transition-colors ${
        view === value
          ? 'bg-foreground text-background border-foreground'
          : 'border-border hover:border-foreground'
      }`}
    >
      {label}
    </button>
  );

  return (
    <div className="space-y-8">
      {isAuthenticated && (
        <div className="flex gap-2" role="group" aria-label="Room filter">
          {filterButton('public', 'Public rooms')}
          {filterButton('mine', 'My rooms')}
        </div>
      )}
      {showMine ? (
        <MyRoomsList />
      ) : (
        <RoomListClient initialRooms={initialRooms} initialSort={initialSort} />
      )}
    </div>
  );
}
