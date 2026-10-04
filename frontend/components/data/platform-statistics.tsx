"use client";

import { ArrowDownRight } from 'lucide-react';
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
          role="status"
          className="mx-auto max-w-[70rem] font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground"
        >
          READING THE STATISTICS...
        </p>
      </section>
    );
  }

  if (!overview) {
    return (
      <section className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border">
        <div className="mx-auto max-w-[70rem]">
          <p className="mb-3 font-mono text-[11px] uppercase tracking-[0.18em] text-foreground">
            STATISTICS UNAVAILABLE
          </p>
          <p className="max-w-[44rem] text-base leading-relaxed text-muted-foreground">
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
      <div className="mx-auto max-w-[76rem] px-4 sm:px-6 lg:px-12">
        <dl data-testid="data-hero-figures" className="grid gap-x-16 pb-12 pt-6 lg:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)] lg:pb-16 lg:pt-10">
          {overview.hero_numbers.map((number) => (
            <div key={number.key} className="group/hero grid min-w-0 grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)] items-center gap-x-5 border-t border-border py-6 first:mb-4 first:flex first:flex-col first:items-start first:border-0 first:pt-0 lg:first:row-span-3 lg:first:mb-0 lg:first:border-r lg:first:pr-12">
              <dt className="text-base font-light leading-snug tracking-[-0.015em] group-first/hero:mt-4 group-first/hero:text-3xl lg:group-first/hero:text-4xl">
                {number.label}
                <span className="mt-2 block font-mono text-[11px] font-normal tracking-normal text-muted-foreground group-first/hero:mt-4">{number.window}</span>
              </dt>
              <dd className="order-first text-6xl font-light leading-none tracking-[-0.04em] tabular-nums group-first/hero:text-[8rem] sm:group-first/hero:text-[11rem] lg:text-7xl lg:group-first/hero:text-[12rem]">
                {number.display}
              </dd>
            </div>
          ))}
        </dl>
        <nav className="grid grid-cols-2 gap-x-6 border-t border-border py-4 lg:grid-cols-4">
          {[
            { href: '#rooms', label: overview.rooms.heading },
            { href: '#api', label: overview.api_usage.heading },
            { href: '#search', label: overview.search.heading },
            { href: '#community', label: overview.community.heading },
          ].map((link) => (
            <a key={link.href} href={link.href} className="group flex items-center justify-between gap-3 py-3 text-xs text-muted-foreground transition-colors hover:text-foreground">
              {link.label}
              <ArrowDownRight aria-hidden="true" className="size-4 shrink-0 transition-transform group-hover:translate-y-0.5" />
            </a>
          ))}
        </nav>
      </div>
      <RoomStatsSection initial={overview.rooms} />
      <ApiUsageSection initial={overview.api_usage} />
      <SearchStatsSection initial={overview.search} />
      <CommunityTotalsSection data={overview.community} />
    </div>
  );
}
