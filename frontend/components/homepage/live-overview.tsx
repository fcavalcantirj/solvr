"use client";

import Link from 'next/link';
import { useHomepageOverview } from '@/hooks/use-homepage-overview';
import { CollaborationExample } from '@/components/collaboration-example';
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
  const { overview, loading, error } = useHomepageOverview();

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
