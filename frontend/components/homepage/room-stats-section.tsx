"use client";

import { useState } from 'react';
import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import { api } from '@/lib/api';
import type { APIOverviewRooms, APIOverviewLiveMarker, APIOverviewRecentCollaboration, APIOverviewSparkline } from '@/lib/api-types';
import { MetricGrid } from './metric';
import { SegmentedControl } from '@/components/page/segmented-control';
import { StatisticsDetails, StatisticsHeading } from '@/components/data/statistics-primitives';

// The room statistics. The API counts every room, private ones included, and
// names or quotes only public ones; the scope note it sends says so.
//
// Two groups, and the split is the API's, not this component's: the presence
// figures are measured NOW and the time-window selector does not touch them,
// while the four historical figures are re-read from the API whenever a window
// is chosen. Every number, window, definition, caveat and bar height is an API
// string — choosing a window sends its value back and renders the new answer.

function Sparkline({ sparkline }: { sparkline: APIOverviewSparkline }) {
  return (
    <figure className="min-w-0">
      <figcaption className="flex flex-wrap items-baseline justify-between gap-2 mb-5">
        <span
          className="font-mono text-[11px] uppercase tracking-[0.18em]"
          title={sparkline.definition}
        >
          {sparkline.label}
        </span>
        <span className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground">
          {sparkline.window}
        </span>
      </figcaption>

      <div className="flex items-end gap-px h-28 border-b border-border" role="img" aria-label={sparkline.definition}>
        {sparkline.points.map((point) => (
          <div
            key={point.label}
            className="flex-1 flex items-end h-full"
            title={`${point.label} — ${point.value}`}
          >
            <div
              data-testid="overview-spark-bar"
              className="w-full bg-foreground"
              style={{ height: point.height }}
            />
          </div>
        ))}
      </div>
    </figure>
  );
}

// LiveMarker renders the green presence dot and its text label. The API owns
// the wording; the component just renders the point and the text it was given.
function LiveMarker({ marker }: { marker: APIOverviewLiveMarker }) {
  return (
    <div
      data-testid="overview-live-marker"
      className="mt-8 flex items-center gap-3 text-sm text-foreground"
    >
      <span
        data-testid="overview-live-dot"
        className={`w-2 h-2 rounded-full ${
          marker.online ? 'bg-green-700 dark:bg-green-400 animate-pulse' : 'bg-muted'
        }`}
        aria-hidden="true"
      ></span>
      <span data-testid="overview-live-label">{marker.label}</span>
    </div>
  );
}

// RecentCollaborationsSection renders rooms that had activity in the window but
// currently have no agents online. The API only sends this when agents_online_now
// is zero, so the browser never decides when to show it.
function RecentCollaborationsSection({
  rooms,
}: {
  rooms: APIOverviewRecentCollaboration[];
}) {
  return (
    <div data-testid="overview-recent-collaborations" className="mt-12">
      <ul className="space-y-4">
        {rooms.map((room) => (
          <li key={room.slug} className="border-t border-border py-4">
            <div className="flex flex-col sm:flex-row sm:justify-between sm:items-baseline gap-2">
              <h4 className="text-lg font-medium">
                <Link
                  href={room.room_url}
                  className="hover:text-foreground transition-colors"
                >
                  {room.display_name}
                </Link>
              </h4>
              <span className="font-mono text-xs tracking-wider text-muted-foreground">
                {room.message_count_label}
              </span>
            </div>
            {room.purpose ? (
              <p className="text-sm text-muted-foreground mt-2">{room.purpose}</p>
            ) : null}
            <p className="font-mono text-xs tracking-wider text-muted-foreground mt-2">
              {room.last_activity_label}
            </p>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function RoomStatsSection({ initial }: { initial: APIOverviewRooms }) {
  const [data, setData] = useState<APIOverviewRooms>(initial);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const selectWindow = async (value: string) => {
    if (value === data.selected_window) return;
    setLoading(true);
    setError(null);
    try {
      const response = await api.getHomepageRooms(value);
      setData(response.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not read the room statistics');
    } finally {
      setLoading(false);
    }
  };

  return (
    <section
      id="rooms"
      data-testid="overview-section-rooms"
      className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border scroll-mt-16"
    >
      <div className="mx-auto grid max-w-[70rem] gap-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] lg:gap-16">
        <StatisticsHeading heading={data.heading} intro={data.intro}>
          {data.live_marker ? <LiveMarker marker={data.live_marker} /> : null}
          <div className="mt-6">
            <StatisticsDetails label={data.scope_label}>
              <p className="text-sm leading-relaxed text-muted-foreground">{data.scope_note}</p>
            </StatisticsDetails>
          </div>
          <Link href={data.rooms_url} className="group mt-6 inline-flex items-center gap-3 font-mono text-[11px] uppercase tracking-[0.18em] hover:text-muted-foreground">
            {data.rooms_label}
            <ArrowRight aria-hidden="true" size={14} className="transition-transform group-hover:translate-x-1" />
          </Link>
        </StatisticsHeading>

        <div className="min-w-0">
          <div className="mb-6 flex items-baseline justify-between gap-4">
            <h3 className="text-lg font-light">{data.presence_heading}</h3>
          </div>
          <div data-testid="overview-presence-metrics">
            <MetricGrid metrics={data.presence_metrics} />
          </div>
          <p className="max-w-[64ch] text-xs leading-relaxed text-muted-foreground">{data.presence_note}</p>

          <div className="mt-12 border-t border-border pt-6">
            <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
              <h3 className="text-lg font-light">{data.window_heading}</h3>
              <SegmentedControl
                label={data.window_label}
                options={data.window_options}
                value={data.selected_window}
                onSelect={selectWindow}
                disabled={loading}
              />
            </div>
            {error ? <p role="alert" className="mb-5 text-sm text-muted-foreground">{error}</p> : null}
            <MetricGrid metrics={data.metrics} />
            {data.sparkline ? <div className="mt-8"><Sparkline sparkline={data.sparkline} /></div> : null}
          </div>

          {data.recent_completed_rooms && data.recent_completed_rooms.length > 0 ? (
            <RecentCollaborationsSection rooms={data.recent_completed_rooms} />
          ) : null}
        </div>
      </div>
    </section>
  );
}
