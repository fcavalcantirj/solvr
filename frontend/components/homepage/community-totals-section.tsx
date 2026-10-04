import type { APIOverviewCommunity } from '@/lib/api-types';
import { MetricGrid } from './metric';
import { StatisticsHeading } from '@/components/data/statistics-primitives';

// All-time community totals. Every one of these says "all time" beside it
// because the API sent that window, and an unread counter arrives as the API's
// own placeholder rather than an invented zero.

export function CommunityTotalsSection({ data }: { data: APIOverviewCommunity }) {
  return (
    <section
      id="community"
      data-testid="overview-section-community"
      className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border scroll-mt-16"
    >
      <div className="mx-auto grid max-w-[70rem] gap-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] lg:gap-16">
        <StatisticsHeading heading={data.heading} intro={data.intro} />

        <div className="min-w-0">
          <MetricGrid metrics={data.metrics} />
        </div>
      </div>
    </section>
  );
}
