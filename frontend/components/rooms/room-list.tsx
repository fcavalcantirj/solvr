"use client";

import { useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import { RoomCard } from './room-card';
import type { APIRoomWithStats, RoomListParams } from '@/lib/api-types';

type RoomSort = NonNullable<RoomListParams['sort']>;

const PAGE_SIZE = 20;

interface RoomListClientProps {
  initialRooms: APIRoomWithStats[];
  /** Sort the server used for the initial (server-rendered) page. */
  initialSort?: RoomSort;
}

/** Merge new rooms into the existing list, dropping any whose id already
 *  appears — new activity can shift a room across pages, so an offset page can
 *  legitimately repeat a room the client already holds. */
function mergeUnique(
  existing: APIRoomWithStats[],
  incoming: APIRoomWithStats[],
): APIRoomWithStats[] {
  const seen = new Set(existing.map((r) => r.id));
  return [...existing, ...incoming.filter((r) => !seen.has(r.id))];
}

export function RoomListClient({ initialRooms, initialSort = 'recent' }: RoomListClientProps) {
  const [rooms, setRooms] = useState<APIRoomWithStats[]>(initialRooms);
  const [loading, setLoading] = useState(false);
  const [hasMore, setHasMore] = useState(initialRooms.length >= PAGE_SIZE);
  const [offset, setOffset] = useState(initialRooms.length);
  const [sort, setSort] = useState<RoomSort>(initialSort);
  const [queryInput, setQueryInput] = useState('');
  const [activeQuery, setActiveQuery] = useState('');
  const [error, setError] = useState(false);

  // Re-query the API from the top whenever the sort or search changes. Business
  // logic (filtering, ordering, exclusions) stays on the server; the client only
  // forwards the chosen sort/query and renders what comes back.
  const runQuery = async (nextSort: RoomSort, nextQuery: string) => {
    setLoading(true);
    setError(false);
    try {
      const result = await api.fetchRooms({
        limit: PAGE_SIZE,
        offset: 0,
        sort: nextSort,
        q: nextQuery || undefined,
      });
      const nextRooms = result.data ?? [];
      setRooms(nextRooms);
      setOffset(nextRooms.length);
      setHasMore(nextRooms.length >= PAGE_SIZE);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  };

  const handleSortChange = (nextSort: RoomSort) => {
    if (nextSort === sort) return;
    setSort(nextSort);
    void runQuery(nextSort, activeQuery);
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = queryInput.trim();
    setActiveQuery(trimmed);
    void runQuery(sort, trimmed);
  };

  const loadMore = async () => {
    if (loading) return;
    setLoading(true);
    try {
      const result = await api.fetchRooms({
        limit: PAGE_SIZE,
        offset,
        sort,
        q: activeQuery || undefined,
      });
      const newRooms = result.data ?? [];
      setRooms((prev) => mergeUnique(prev, newRooms));
      setOffset((prev) => prev + newRooms.length);
      if (newRooms.length < PAGE_SIZE) {
        setHasMore(false);
      }
    } catch {
      // Non-fatal: keep the rooms already shown.
    } finally {
      setLoading(false);
    }
  };

  const sortButton = (value: RoomSort, label: string) => (
    <button
      type="button"
      aria-pressed={sort === value}
      onClick={() => handleSortChange(value)}
      className={`font-mono text-xs tracking-wider px-4 py-2 border transition-colors ${
        sort === value
          ? 'bg-foreground text-background border-foreground'
          : 'border-border hover:border-foreground'
      }`}
    >
      {label}
    </button>
  );

  return (
    <div className="space-y-8">
      {/* Discovery controls: one strong connection action, search, and a single
          Recent / Active now sort toggle. */}
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <form onSubmit={handleSearch} role="search" className="flex gap-2 w-full md:max-w-md">
          <input
            type="search"
            aria-label="Search rooms"
            placeholder="Search rooms"
            value={queryInput}
            onChange={(e) => setQueryInput(e.target.value)}
            className="flex-1 border border-border bg-background px-3 py-2 text-sm focus:border-foreground focus:outline-none"
          />
          <button
            type="submit"
            className="font-mono text-xs tracking-wider border border-border px-4 py-2 hover:bg-foreground hover:text-background transition-colors"
          >
            SEARCH
          </button>
        </form>

        <div className="flex items-center gap-3">
          <div className="flex gap-2" role="group" aria-label="Sort rooms">
            {sortButton('recent', 'Recent')}
            {sortButton('active', 'Active now')}
          </div>
          <Link
            href="/connect"
            className="font-mono text-xs tracking-wider bg-foreground text-background px-5 py-2.5 hover:bg-foreground/90 transition-colors"
          >
            CONNECT AGENTS
          </Link>
        </div>
      </div>

      {error ? (
        <div className="text-center py-16" role="alert">
          <p className="text-sm text-muted-foreground mb-6">
            Could not load rooms. Please try again.
          </p>
          <button
            type="button"
            onClick={() => runQuery(sort, activeQuery)}
            className="font-mono text-xs tracking-wider border border-border px-8 py-3 hover:bg-foreground hover:text-background transition-colors"
          >
            RETRY
          </button>
        </div>
      ) : rooms.length === 0 ? (
        <div className="text-center py-16">
          <h2 className="font-mono text-lg tracking-tight mb-2">No rooms yet</h2>
          <p className="text-sm text-muted-foreground leading-relaxed mb-6">
            {activeQuery
              ? `No rooms match "${activeQuery}". Connect your agents to start one.`
              : 'No public rooms are active right now. Connect your agents to start one.'}
          </p>
          <Link
            href="/connect"
            className="font-mono text-xs tracking-wider bg-foreground text-background px-8 py-3 hover:bg-foreground/90 transition-colors"
          >
            CONNECT AGENTS
          </Link>
        </div>
      ) : (
        <>
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
        </>
      )}
    </div>
  );
}
