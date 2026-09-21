"use client";

import { useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import type { APIOverviewActivity, APIOverviewActivityItem } from '@/lib/api-types';
import { SectionHeading } from './metric';

// The public room activity stream. The API sends the six most recent
// activities, already excerpted and time-worded, and says whether another page
// exists and where it is. Load more appends the next page verbatim — this
// component never slices, sorts or re-words anything.

export function RoomActivitySection({ initial }: { initial: APIOverviewActivity }) {
  const [items, setItems] = useState<APIOverviewActivityItem[]>(initial.items);
  const [hasMore, setHasMore] = useState(initial.has_more);
  const [nextOffset, setNextOffset] = useState(initial.next_offset);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadMore = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await api.getHomepageActivity(nextOffset, initial.limit);
      const page = response.data;
      setItems((current) => [...current, ...page.items]);
      setHasMore(page.has_more);
      setNextOffset(page.next_offset);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not load more activity');
    } finally {
      setLoading(false);
    }
  };

  return (
    <section
      data-testid="overview-section-activity"
      className="px-4 sm:px-6 lg:px-12 py-24 lg:py-32 border-t border-border bg-secondary"
    >
      <div className="max-w-7xl mx-auto">
        <SectionHeading
          eyebrow="ACTIVITY"
          heading={initial.heading}
          intro={initial.intro}
          definition={initial.definition}
        />

        {items.length === 0 ? (
          <p className="mt-12 text-sm text-muted-foreground">{initial.empty_note}</p>
        ) : (
          <ul className="mt-12 border border-border bg-background divide-y divide-border">
            {items.map((item) => (
              <li
                key={`${item.room_slug}-${item.message_url ?? item.excerpt}`}
                data-testid="overview-activity-item"
                className="p-5 sm:p-6"
              >
                <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 mb-3">
                  <Link
                    href={item.room_url}
                    className="font-mono text-xs tracking-wider underline underline-offset-4 hover:no-underline"
                  >
                    {item.room_name}
                  </Link>
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                    {item.time_label}
                  </span>
                </div>

                <p className="font-mono text-[10px] tracking-wider text-muted-foreground mb-2">
                  {item.author} · {item.author_role}
                </p>
                <p className="text-sm leading-relaxed whitespace-pre-wrap">
                  {item.excerpt}
                </p>

                <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2">
                  {item.is_excerpt && item.excerpt_note ? (
                    <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                      {item.excerpt_note}
                    </span>
                  ) : null}
                  {item.message_url ? (
                    <Link
                      href={item.message_url}
                      className="font-mono text-[10px] tracking-wider underline underline-offset-4 hover:no-underline"
                    >
                      Open the original message
                    </Link>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        )}

        {error ? (
          <p role="alert" className="mt-6 font-mono text-xs text-muted-foreground">
            {error}
          </p>
        ) : null}

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
