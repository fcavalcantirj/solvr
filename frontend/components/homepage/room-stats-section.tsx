"use client";

import { useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import type { APIOverviewRooms, APIOverviewLiveMarker, APIOverviewRecentCollaboration, APIOverviewSparkline } from '@/lib/api-types';
import { MetricGrid, SectionHeading } from './metric';

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

// LiveMarker renders the green presence dot and its text label. The API owns
// the wording; the component just renders the point and the text it was given.
function LiveMarker({ marker }: { marker: APIOverviewLiveMarker }) {
  return (
    <div
      data-testid="overview-live-marker"
      className="mt-8 flex items-center gap-3 font-mono text-xs tracking-wider"
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
      <p className="font-mono text-xs tracking-[0.2em] text-muted-foreground mb-4">
        RECENTLY COMPLETED
      </p>
      <ul className="space-y-4">
        {rooms.map((room) => (
          <li key={room.slug} className="border border-border p-4 sm:p-6">
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
      data-testid="overview-section-rooms"
      className="px-4 sm:px-6 lg:px-12 py-24 lg:py-32 border-t border-border"
    >
      <div className="max-w-7xl mx-auto">
        <SectionHeading eyebrow="LIVE" heading={data.heading} intro={data.intro} />

        {data.live_marker ? (
          <LiveMarker marker={data.live_marker} />
        ) : null}

        <div className="mt-8 max-w-3xl">
          <p className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
            {data.scope_label}
          </p>
          <p className="text-sm text-muted-foreground leading-relaxed mt-2">
            {data.scope_note}
          </p>
        </div>

        {/* The "now" half. No selector reaches these. */}
        <div className="mt-12">
          <div className="flex flex-wrap items-baseline gap-x-6 gap-y-2 mb-5">
            <h3 className="font-mono text-xs tracking-[0.3em]">{data.presence_heading}</h3>
            <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
              {data.presence_note}
            </p>
          </div>
          <div data-testid="overview-presence-metrics">
            <MetricGrid metrics={data.presence_metrics} />
          </div>

          {data.recent_collaborations && data.recent_collaborations.length > 0 ? (
            <RecentCollaborationsSection rooms={data.recent_collaborations} />
          ) : null}
        </div>

        {/* The windowed half, and the selector that drives it. */}
        <div className="mt-16">
          <div className="flex flex-wrap items-center justify-between gap-4 mb-5">
            <h3 className="font-mono text-xs tracking-[0.3em]">{data.window_heading}</h3>
            <div
              role="group"
              aria-label={data.window_label}
              className="flex border border-border divide-x divide-border"
            >
              {data.window_options.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  aria-pressed={option.value === data.selected_window}
                  disabled={loading}
                  onClick={() => selectWindow(option.value)}
                  className={`font-mono text-[10px] uppercase tracking-[0.2em] px-4 py-3 transition-colors disabled:opacity-50 ${
                    option.value === data.selected_window
                      ? 'bg-foreground text-background'
                      : 'hover:bg-secondary'
                  }`}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>

          {error ? (
            <p role="alert" className="mb-5 font-mono text-xs text-muted-foreground">
              {error}
            </p>
          ) : null}

          <div className="grid lg:grid-cols-12 gap-8 lg:gap-12 items-start">
            <div className="lg:col-span-7">
              <MetricGrid metrics={data.metrics} />
            </div>
            <div className="lg:col-span-5">
              {data.sparkline ? <Sparkline sparkline={data.sparkline} /> : null}
            </div>
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
