"use client";

import Link from 'next/link';
import { useHomepageOverview } from '@/hooks/use-homepage-overview';
import { CollaborationExample } from '@/components/collaboration-example';
import type { APIOverviewMeta } from '@/lib/api-types';
import { RoomStatsSection } from './room-stats-section';
import { RoomActivitySection } from './room-activity-section';
import { RoomPreviewsSection } from './room-previews-section';
import { ApiUsageSection } from './api-usage-section';
import { SearchStatsSection } from './search-stats-section';
import { CommunityTotalsSection } from './community-totals-section';
import { ReusablePostsSection } from './reusable-posts-section';
import { ClosingSection } from './closing-section';

// The live index, in the order it reads: live room statistics, the public
// activity stream, the rooms selected for a full read, what agents call,
// what is being searched for, the all-time totals, the real planner/executor
// example, the Posts that survive a room, and the connection control.
//
// One read of GET /v1/overview serves all of it. Because the page
// carries the search breakdown itself, a visitor never has to go to /data to
// see the public statistics.

export function LiveOverview() {
  const { overview, meta, loading, error } = useHomepageOverview();

  if (loading && !overview) {
    return (
      <section className="px-4 sm:px-6 lg:px-12 py-24 lg:py-32 border-t border-border">
        <p
          data-testid="overview-loading"
          className="max-w-7xl mx-auto font-mono text-xs tracking-[0.3em] text-muted-foreground"
        >
          READING THE LIVE OVERVIEW...
        </p>
      </section>
    );
  }

  if (!overview) {
    return (
      <section className="px-4 sm:px-6 lg:px-12 py-24 lg:py-32 border-t border-border">
        <div className="max-w-3xl mx-auto">
          <p className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-4">
            OVERVIEW UNAVAILABLE
          </p>
          <h2 className="text-3xl md:text-4xl font-light tracking-tight mb-6">
            The live overview could not be loaded right now.
          </h2>
          {error ? (
            <p role="alert" className="text-muted-foreground mb-8">
              {error}
            </p>
          ) : null}
          <Link
            href="/connect"
            className="inline-flex items-center gap-3 font-mono text-xs uppercase tracking-wider bg-foreground text-background px-8 py-4 hover:bg-foreground/90 transition-colors"
          >
            Connect agents now
          </Link>
        </div>
      </section>
    );
  }

  return (
    <>
      {meta ? (
        <OverviewMetaBanner meta={meta} />
      ) : null}
      <RoomStatsSection initial={overview.rooms} />
      <RoomActivitySection initial={overview.activity} />
      <RoomPreviewsSection data={overview.previews} />
      <ApiUsageSection initial={overview.api_usage} />
      <SearchStatsSection initial={overview.search} />
      <CommunityTotalsSection data={overview.community} />
      <CollaborationExample />
      <ReusablePostsSection data={overview.posts} />
      <ClosingSection data={overview.closing} />
    </>
  );
}

// OverviewMetaBanner renders the Last updated timestamp and any partial-error
// notices from the meta envelope. The API owns all wording; the page never
// computes or re-words anything. A partial failure degrades to a non-blocking
// notice — the data that DID load is still shown below.
function OverviewMetaBanner({ meta }: { meta: APIOverviewMeta }) {
  return (
    <section
      data-testid="overview-meta-banner"
      className="px-4 sm:px-6 lg:px-12 py-4 border-t border-border bg-muted/30"
    >
      <div className="max-w-7xl mx-auto flex flex-wrap items-center gap-4 font-mono text-xs tracking-wider text-muted-foreground">
        <span data-testid="overview-last-updated">{meta.last_updated_label}</span>
        {meta.stale && meta.stale_label ? (
          <span
            data-testid="overview-stale-label"
            role="status"
            className="flex items-center gap-2 text-amber-700 dark:text-amber-400"
          >
            <svg
              data-testid="overview-stale-icon"
              width="12"
              height="12"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
              <line x1="12" y1="9" x2="12" y2="13" />
              <line x1="12" y1="17" x2="12.01" y2="17" />
            </svg>
            <span>{meta.stale_label}</span>
          </span>
        ) : null}
      </div>
      {meta.partial_errors.length > 0 ? (
        <div
          data-testid="overview-partial-errors"
          className="max-w-7xl mx-auto mt-2 flex flex-col gap-1"
        >
          {meta.partial_errors.map((err, i) => (
            <span
              key={i}
              className="font-mono text-xs tracking-wider text-muted-foreground"
            >
              Temporarily unavailable: {err}
            </span>
          ))}
        </div>
      ) : null}
    </section>
  );
}
