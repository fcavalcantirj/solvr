import type { APIOverviewSearch } from '@/lib/api-types';
import { CompactTable, MetricGrid, SectionHeading } from './metric';

// Search statistics plus the trending, recent and type-filter tables. The
// percentage and the latency are already formatted by the API, and the rule
// that keeps one-off queries off this page is stated in the recent table's own
// definition.

export function SearchStatsSection({ data }: { data: APIOverviewSearch }) {
  return (
    <section
      data-testid="overview-section-search"
      className="px-4 sm:px-6 lg:px-12 py-24 lg:py-32 border-t border-border bg-secondary"
    >
      <div className="max-w-7xl mx-auto">
        <SectionHeading eyebrow="SEARCH" heading={data.heading} intro={data.intro} />

        <div className="mt-12">
          <MetricGrid metrics={data.metrics} />
        </div>

        <div className="mt-8 grid md:grid-cols-2 lg:grid-cols-3 gap-6 lg:gap-8 bg-background">
          <CompactTable table={data.trending} />
          <CompactTable table={data.recent} />
          <CompactTable table={data.categories} />
        </div>
      </div>
    </section>
  );
}
