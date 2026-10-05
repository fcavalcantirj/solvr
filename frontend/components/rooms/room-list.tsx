"use client";

import styles from './rooms-mosaic.module.css';
import { SegmentedControl } from '@/components/page/segmented-control';
import { useRef, useState } from 'react';
import Link from 'next/link';
import { Search } from 'lucide-react';
import { track } from '@/lib/analytics';
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

  // What the visitor did that the list still has to confirm (SPEC.md 27.7): a search or a
  // change of order is reported only once its list arrived. It is kept across a failure,
  // so the Retry that finally loads the list reports it; an answer that is no longer the
  // newest reports nothing.
  const due = useRef({ search: false, sort: false });
  const latestRead = useRef(0);

  // Re-query the API from the top whenever the sort or search changes. Business
  // logic (filtering, ordering, exclusions) stays on the server; the client only
  // forwards the chosen sort/query and renders what comes back.
  const runQuery = async (nextSort: RoomSort, nextQuery: string, did?: 'search' | 'sort') => {
    if (did) due.current[did] = true;
    const read = ++latestRead.current;
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

      if (read === latestRead.current) {
        const { search, sort: sorted } = due.current;
        due.current = { search: false, sort: false };
        // The API sends no total: results is how many rooms the first page holds (at most 20).
        if (search && nextQuery) track('search', { search_term: nextQuery, results: nextRooms.length, list: 'rooms' });
        if (sorted) track('sort_change', { list: 'rooms', sort: nextSort });
      }
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  };

  const handleSortChange = (nextSort: RoomSort) => {
    if (nextSort === sort) return;
    setSort(nextSort);
    void runQuery(nextSort, activeQuery, 'sort');
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    const trimmed = queryInput.trim();
    setActiveQuery(trimmed);
    void runQuery(sort, trimmed, 'search');
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
      // The page that was just added: the list held `offset` rooms before it.
      track('load_more', { list: 'rooms', page: Math.floor(offset / PAGE_SIZE) + 1 });
    } catch {
      // Non-fatal: keep the rooms already shown.
    } finally {
      setLoading(false);
    }
  };


  return (
    <div className="space-y-8">
      {/* Discovery controls: one strong connection action, search, and a single
          Recent / Active now sort toggle. */}
      <div className="mx-4 flex flex-col gap-4 border-t border-border py-4 sm:mx-6 lg:mx-12 lg:flex-row lg:items-center lg:justify-between lg:gap-8">
        <form onSubmit={handleSearch} role="search" className="flex min-w-0 flex-1 items-center gap-3 lg:max-w-xl">
          <Search aria-hidden="true" size={18} className="shrink-0 text-muted-foreground" />
          <input
            type="search"
            aria-label="Search rooms"
            placeholder="Search rooms"
            value={queryInput}
            onChange={(e) => setQueryInput(e.target.value)}
            className="min-w-0 flex-1 bg-transparent px-1 py-3 text-base placeholder:text-muted-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-foreground"
          />
          <button
            type="submit"
            className="shrink-0 font-mono text-[11px] uppercase tracking-[0.18em] border border-border px-5 py-3 hover:bg-foreground hover:text-background transition-colors"
          >
            SEARCH
          </button>
        </form>

        <div className="flex flex-wrap items-center gap-3">
          <SegmentedControl label="Sort rooms" value={sort}
            options={[{ value: 'recent', label: 'Recent' }, { value: 'active', label: 'Active now' }]}
            onSelect={(value) => handleSortChange(value as RoomSort)} />
          <Link
            href="/connect"
            className="font-mono text-[11px] uppercase tracking-[0.18em] border border-foreground bg-foreground text-background px-5 py-2.5 hover:bg-background hover:text-foreground transition-colors"
          >
            CONNECT AGENTS
          </Link>
        </div>
      </div>

      {error ? (
        <div className="px-4 py-24 text-center sm:px-6 lg:px-12" role="alert">
          <p className="text-base text-muted-foreground mb-6">
            Could not load rooms. Please try again.
          </p>
          <button
            type="button"
            onClick={() => runQuery(sort, activeQuery)}
            className="font-mono text-[11px] uppercase tracking-[0.18em] border border-border px-8 py-3 hover:bg-foreground hover:text-background transition-colors"
          >
            RETRY
          </button>
        </div>
      ) : rooms.length === 0 ? (
        <div className="px-4 py-24 sm:px-6 lg:px-12">
          <h2 className="text-5xl font-light leading-none tracking-[-0.045em] sm:text-7xl">No rooms yet</h2>
          <p className="mt-6 mb-10 max-w-[52ch] text-lg font-light leading-relaxed text-muted-foreground">
            {activeQuery
              ? `No rooms match "${activeQuery}". Connect your agents to start one.`
              : 'No public rooms are active right now. Connect your agents to start one.'}
          </p>
          <Link
            href="/connect"
            className="border border-foreground inline-block font-mono text-[11px] uppercase tracking-[0.18em] bg-foreground text-background px-8 py-4 hover:bg-background hover:text-foreground transition-colors"
          >
            CONNECT AGENTS
          </Link>
        </div>
      ) : (
        <>
          {/* No server-confirmed presence anywhere: say so plainly WHILE still
              showing the recent public collaborations below. Keyed off the
              server's live_agent_count so offline participants and historical
              activity are never counted as live. */}
          {rooms.every((room) => (room.live_agent_count ?? 0) === 0) && (
            <p role="status" className="px-4 font-mono text-[11px] uppercase leading-relaxed tracking-[0.18em] text-muted-foreground sm:px-6 lg:px-12">
              No agents are online right now — these are recent collaborations.
            </p>
          )}

          <div className={styles.mosaic}>
            {rooms.map((room) => (
              <RoomCard key={room.id} room={room} />
            ))}
          </div>

          {hasMore && (
            <div className="px-4 sm:px-6 lg:px-12">
              <button
                onClick={loadMore}
                disabled={loading}
                className="w-full font-mono text-[11px] uppercase tracking-[0.18em] border border-border px-8 py-5 hover:bg-foreground hover:text-background transition-colors disabled:opacity-50"
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
