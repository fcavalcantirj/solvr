"use client";

import Link from 'next/link';
import { CollaborationExample } from '@/components/collaboration-example';
import type { APIHomepageOverview, APIOverviewMeta } from '@/lib/api-types';
import { OverviewMetaBanner } from './overview-meta-banner';
import { RoomActivitySection } from './room-activity-section';
import { RoomPreviewsSection } from './room-previews-section';
import { ReusablePostsSection } from './reusable-posts-section';
import { ClosingSection } from './closing-section';

// The live index below the hero and the use cases, in the order it reads: the
// public activity stream, the rooms selected for a full read, the real
// planner/executor example, the Posts that survive a room, and the connection
// control. The deep statistics — rooms, API usage, searches and the all-time
// totals — live on /data (components/data/platform-statistics.tsx).
//
// One read of GET /v1/overview serves all of it: HomeOverview owns that read
// (seeded on the server, refreshed once in the browser) and hands its state here.

export interface LiveOverviewProps {
  overview: APIHomepageOverview | null;
  meta: APIOverviewMeta | null;
  loading: boolean;
  error: string | null;
}

export function LiveOverview({ overview, meta, loading, error }: LiveOverviewProps) {

  if (loading && !overview) {
    return (
      <section className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border">
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
      <section className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border">
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
      <RoomActivitySection initial={overview.activity} />
      <RoomPreviewsSection data={overview.previews} />
      <CollaborationExample />
      <ReusablePostsSection data={overview.posts} />
      <ClosingSection data={overview.closing} />
    </>
  );
}
