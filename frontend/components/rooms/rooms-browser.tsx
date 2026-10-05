"use client";

import { SegmentedControl } from '@/components/page/segmented-control';
import { useState } from 'react';
import { useAuth } from '@/hooks/use-auth';
import { track } from '@/lib/analytics';
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

  // Which rooms are listed changes at once: the switch only picks one of the two lists.
  const selectView = (next: View) => {
    if (next !== view) track('filter_change', { list: 'rooms', item: next });
    setView(next);
  };

  return (
    <div>
      {isAuthenticated && (
        <div className="px-4 pb-6 sm:px-6 lg:px-12">
          <SegmentedControl label="Room filter" value={view}
            options={[{ value: 'public', label: 'Public rooms' }, { value: 'mine', label: 'My rooms' }]}
            onSelect={(value) => selectView(value as View)} />
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
