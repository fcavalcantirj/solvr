import Link from 'next/link';
import type { APIOverviewRooms, APIOverviewSparkline } from '@/lib/api-types';
import { MetricGrid, SectionHeading } from './metric';

// Live room statistics. Every number, window, definition and bar height is an
// API string — the sparkline arrives with each bar's CSS height already
// decided, so nothing here multiplies, rounds or scales anything.

function Sparkline({ sparkline }: { sparkline: APIOverviewSparkline }) {
  return (
    <figure className="border border-border p-4 sm:p-6">
      <figcaption className="flex flex-wrap items-baseline justify-between gap-2 mb-5">
        <span
          className="font-mono text-[10px] tracking-[0.2em]"
          title={sparkline.definition}
        >
          {sparkline.label}
        </span>
        <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
          {sparkline.window}
        </span>
      </figcaption>

      <div className="flex items-end gap-px h-24" role="img" aria-label={sparkline.definition}>
        {sparkline.points.map((point) => (
          <div
            key={point.label}
            className="flex-1 flex items-end h-full"
            title={`${point.label} — ${point.value}`}
          >
            <div
              data-testid="overview-spark-bar"
              className="w-full bg-foreground min-h-px"
              style={{ height: point.height }}
            />
          </div>
        ))}
      </div>
    </figure>
  );
}

export function RoomStatsSection({ data }: { data: APIOverviewRooms }) {
  return (
    <section
      data-testid="overview-section-rooms"
      className="px-4 sm:px-6 lg:px-12 py-24 lg:py-32 border-t border-border"
    >
      <div className="max-w-7xl mx-auto">
        <SectionHeading eyebrow="LIVE" heading={data.heading} intro={data.intro} />

        <div className="mt-12 grid lg:grid-cols-12 gap-8 lg:gap-12 items-start">
          <div className="lg:col-span-7">
            <MetricGrid metrics={data.metrics} />
          </div>
          <div className="lg:col-span-5">
            {data.sparkline ? <Sparkline sparkline={data.sparkline} /> : null}
          </div>
        </div>

        <Link
          href={data.rooms_url}
          className="inline-block mt-10 font-mono text-xs uppercase tracking-wider border border-foreground px-8 py-4 hover:bg-foreground hover:text-background transition-colors"
        >
          {data.rooms_label}
        </Link>
      </div>
    </section>
  );
}
