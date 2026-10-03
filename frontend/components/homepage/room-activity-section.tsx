"use client";

import { useCallback, useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import type {
  APIOverviewActivity,
  APIOverviewActivityGroup,
  APIOverviewActivityItem,
} from '@/lib/api-types';
import { SectionHeading } from './metric';

// The public room activity stream.
//
// Everything on display was decided by the API: which entries are eligible,
// how a burst from one room is grouped, what each entry's action reads as, who
// posted it and whether that identity was ever authenticated, how far an
// excerpt is cut, and where the entry opens. This component groups nothing,
// words nothing and links nothing on its own.
//
// The one behaviour it owns is WHEN the list changes, and the answer is: only
// when the reader says so. On a fixed interval (REFRESH_INTERVAL_MS below) it
// asks the API whether anything arrived after the cursor it is holding — while
// the page is visible — and if something did, it offers the API's own label as
// a control. The entries a reader is looking at never move under them.

const REFRESH_INTERVAL_MS = 30_000;

export function RoomActivitySection({ initial }: { initial: APIOverviewActivity }) {
  const [groups, setGroups] = useState<APIOverviewActivityGroup[]>(initial.groups);
  const [hasMore, setHasMore] = useState(initial.has_more);
  const [nextOffset, setNextOffset] = useState(initial.next_offset);
  const [cursor, setCursor] = useState(initial.cursor);
  const [pending, setPending] = useState<APIOverviewActivity | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // The cursor the background check uses, without restarting the interval
  // every time it moves.
  const cursorRef = useRef(cursor);
  cursorRef.current = cursor;

  const loadMore = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await api.getHomepageActivity(nextOffset, initial.limit);
      const next = response.data;
      setGroups((current) => [...current, ...next.groups]);
      setHasMore(next.has_more);
      setNextOffset(next.next_offset);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not load more activity');
    } finally {
      setLoading(false);
    }
  };

  // A failed check is not an error the reader has to act on: the page they are
  // reading is still true, it simply has not learned anything newer.
  const checkForNewActivity = useCallback(async () => {
    if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return;
    try {
      const response = await api.getHomepageActivity(0, initial.limit, cursorRef.current);
      if (response.data.has_new) setPending(response.data);
    } catch {
      // Quiet on purpose.
    }
  }, [initial.limit]);

  useEffect(() => {
    const timer = setInterval(checkForNewActivity, REFRESH_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [checkForNewActivity]);

  const takeNewActivity = () => {
    if (!pending) return;
    setGroups(pending.groups);
    setHasMore(pending.has_more);
    setNextOffset(pending.next_offset);
    setCursor(pending.cursor);
    setPending(null);
  };

  return (
    <section
      data-testid="overview-section-activity"
      className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border bg-secondary"
    >
      <div className="max-w-7xl mx-auto">
        <SectionHeading
          eyebrow="ACTIVITY"
          heading={initial.heading}
          intro={initial.intro}
          definition={initial.definition}
        />

        <p className="mt-4 font-mono text-[10px] leading-relaxed tracking-wider text-muted-foreground max-w-3xl">
          {initial.outcome_note}
        </p>

        <div aria-live="polite" className="mt-8 min-h-[1px]">
          {pending?.new_label ? (
            <button
              type="button"
              data-testid="overview-activity-new"
              onClick={takeNewActivity}
              className="font-mono text-xs uppercase tracking-wider border border-foreground px-6 py-3 hover:bg-foreground hover:text-background transition-colors"
            >
              {pending.new_label}
            </button>
          ) : null}
        </div>

        {groups.length === 0 ? (
          <p className="mt-8 text-sm text-muted-foreground">{initial.empty_note}</p>
        ) : (
          <ul className="mt-4 border border-border bg-background divide-y divide-border">
            {groups.map((group, index) => (
              <li
                key={`${group.room_slug}-${group.items[0]?.id ?? index}`}
                data-testid="overview-activity-group"
                className="p-5 sm:p-6"
              >
                <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
                  <Link
                    href={group.room_url}
                    className="font-mono text-xs tracking-wider underline underline-offset-4 hover:no-underline"
                  >
                    {group.room_name}
                  </Link>
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                    {group.count_label} · {group.time_label}
                  </span>
                </div>

                {group.burst_note ? (
                  <p className="mt-1 font-mono text-[10px] tracking-wider text-muted-foreground">
                    {group.burst_note}
                  </p>
                ) : null}

                <ul className="mt-4 space-y-5 sm:space-y-6">
                  {group.items.map((item) => (
                    <ActivityEntry key={item.id} item={item} />
                  ))}
                </ul>
              </li>
            ))}
          </ul>
        )}

        {error ? (
          <p role="alert" className="mt-6 font-mono text-xs text-muted-foreground">
            {error}
          </p>
        ) : null}

        <p className="mt-6 font-mono text-[10px] leading-relaxed tracking-wider text-muted-foreground max-w-3xl">
          {initial.refresh_note}
        </p>

        {hasMore ? (
          <button
            type="button"
            onClick={loadMore}
            disabled={loading}
            className="mt-8 font-mono text-xs uppercase tracking-wider border border-foreground px-8 py-4 hover:bg-foreground hover:text-background transition-colors disabled:opacity-50 disabled:hover:bg-transparent disabled:hover:text-foreground"
          >
            {initial.load_more_label}
          </button>
        ) : null}
      </div>
    </section>
  );
}

// One entry: who posted it, what they said they were doing, the part of it the
// API chose to show, and where it opens.
function ActivityEntry({ item }: { item: APIOverviewActivityItem }) {
  return (
    <li data-testid="overview-activity-item" className="border-l border-border pl-4 sm:pl-5">
      <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
        {item.author} · {item.author_label}
        {item.author_note ? (
          <span className="block mt-1 normal-case">{item.author_note}</span>
        ) : null}
      </p>

      <p className="mt-2 font-mono text-xs tracking-wider">{item.action}</p>

      {item.excerpt ? (
        <p className="mt-2 text-sm leading-relaxed whitespace-pre-wrap">{item.excerpt}</p>
      ) : null}

      <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2">
        <time
          dateTime={item.timestamp}
          className="font-mono text-[10px] tracking-wider text-muted-foreground"
        >
          {item.time_label}
        </time>
        {item.is_excerpt && item.excerpt_note ? (
          <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
            {item.excerpt_note}
          </span>
        ) : null}
        <Link
          href={item.link_url}
          className="font-mono text-[10px] tracking-wider underline underline-offset-4 hover:no-underline"
        >
          {item.link_label}
        </Link>
      </div>
    </li>
  );
}
