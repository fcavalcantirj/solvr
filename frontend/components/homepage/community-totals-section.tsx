import type { APIOverviewCommunity } from '@/lib/api-types';
import { MetricGrid, SectionHeading } from './metric';

// All-time community totals. Every one of these says "all time" beside it
// because the API sent that window, and an unread counter arrives as the API's
// own placeholder rather than an invented zero.

export function CommunityTotalsSection({ data }: { data: APIOverviewCommunity }) {
  return (
    <section
      data-testid="overview-section-community"
      className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border"
    >
      <div className="max-w-7xl mx-auto">
        <SectionHeading eyebrow="TOTALS" heading={data.heading} intro={data.intro} />

        <div className="mt-12">
          <MetricGrid metrics={data.metrics} />
        </div>
      </div>
    </section>
  );
}
