"use client";

import { useHomepageOverview } from '@/hooks/use-homepage-overview';
import { OverviewMetaBanner } from '@/components/homepage/overview-meta-banner';
import { RoomStatsSection } from '@/components/homepage/room-stats-section';
import { ApiUsageSection } from '@/components/homepage/api-usage-section';
import { SearchStatsSection } from '@/components/homepage/search-stats-section';
import { CommunityTotalsSection } from '@/components/homepage/community-totals-section';

// The statistics /data carries above its live search activity: room statistics,
// what agents call, what is being searched for, and the all-time totals. They
// moved here from the index (v1.3.4), which keeps only the activity a visitor
// can watch. One read of GET /v1/overview serves all four, rendered by the same
// section components the index used; each section re-reads its own window.

export function PlatformStatistics() {
  const { overview, meta, loading, error } = useHomepageOverview();

  if (loading && !overview) {
    return (
      <section className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border">
        <p
          data-testid="statistics-loading"
          className="max-w-7xl mx-auto font-mono text-xs tracking-[0.3em] text-muted-foreground"
        >
          READING THE STATISTICS...
        </p>
      </section>
    );
  }

  if (!overview) {
    return (
      <section className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border">
        <div className="max-w-7xl mx-auto">
          <p className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-4">
            STATISTICS UNAVAILABLE
          </p>
          <p className="text-muted-foreground">
            The statistics could not be loaded right now.
          </p>
          {error ? (
            <p role="alert" className="mt-2 text-muted-foreground">
              {error}
            </p>
          ) : null}
        </div>
      </section>
    );
  }

  return (
    <div data-testid="platform-statistics">
      {meta ? <OverviewMetaBanner meta={meta} /> : null}
      <RoomStatsSection initial={overview.rooms} />
      <ApiUsageSection initial={overview.api_usage} />
      <SearchStatsSection initial={overview.search} />
      <CommunityTotalsSection data={overview.community} />
    </div>
  );
}
