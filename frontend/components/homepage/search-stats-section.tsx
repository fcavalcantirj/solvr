"use client";

import { useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import type { APIOverviewSearch, APIOverviewSeries } from '@/lib/api-types';
import { MetricGrid, SectionHeading } from './metric';

// What people and agents search for.
//
// Every number, window, definition, caveat, label, bar height and link on this
// section is a string the API already decided. The browser sends the chosen
// window back and renders the new answer; it does not count, rank, format,
// escape, or judge whether a term may be shown. That last one matters most:
// which searches may be quoted at all is a publishing decision, and it is made
// in the API where it can be tested, not in a component.
//
// The chart ships with a table carrying the same rows, so the series is
// readable without seeing the bars.

function SearchChart({ series }: { series: APIOverviewSeries }) {
  if (!series.sparkline) return null;
  const { sparkline } = series;

  return (
    <figure className="border border-border p-4 sm:p-6">
      <figcaption className="flex flex-wrap items-baseline justify-between gap-2 mb-5">
        <span className="font-mono text-[10px] tracking-[0.2em]" title={sparkline.definition}>
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
              data-testid="search-spark-bar"
              className="w-full bg-foreground min-h-px"
              style={{ height: point.height }}
            />
          </div>
        ))}
      </div>

      {/* The accessible equivalent: the same rows, as a table. */}
      <table data-testid="search-series-table" className="w-full mt-6 text-left">
        <caption className="font-mono text-[10px] leading-relaxed text-muted-foreground/70 text-left mb-3 caption-bottom">
          {series.table_caption}
        </caption>
        <thead>
          <tr className="border-b border-border">
            <th scope="col" className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground py-2">
              {series.period_header}
            </th>
            <th scope="col" className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground py-2 text-right">
              {series.count_header}
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {series.rows.map((row) => (
            <tr key={row.label}>
              <th scope="row" className="font-mono text-xs font-normal py-2">
                {row.label}
              </th>
              <td className="font-mono text-xs text-right py-2">{row.display}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </figure>
  );
}

export function SearchStatsSection({ initial }: { initial: APIOverviewSearch }) {
  const [data, setData] = useState<APIOverviewSearch>(initial);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const selectWindow = async (value: string) => {
    if (value === data.selected_window) return;
    setLoading(true);
    setError(null);
    try {
      const response = await api.getHomepageSearch(value);
      setData(response.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not read the search statistics');
    } finally {
      setLoading(false);
    }
  };

  return (
    <section
      data-testid="overview-section-search"
      className="px-4 sm:px-6 lg:px-12 py-24 lg:py-32 border-t border-border bg-secondary"
    >
      <div className="max-w-7xl mx-auto">
        <SectionHeading eyebrow="SEARCH" heading={data.heading} intro={data.intro} />

        <div className="mt-8 flex flex-wrap items-center justify-end gap-4">
          <div
            role="group"
            aria-label={data.window_label}
            className="flex border border-border divide-x divide-border bg-background"
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
          <p role="alert" className="mt-5 font-mono text-xs text-muted-foreground">
            {error}
          </p>
        ) : null}

        <div data-testid="search-metrics" className="mt-6 bg-background">
          <MetricGrid metrics={data.metrics} />
        </div>

        {/* Known automated monitoring, stated apart from the totals above. */}
        {data.monitoring ? (
          <div
            data-testid="search-monitoring"
            className="mt-6 border border-border bg-background p-5 sm:p-6 flex flex-wrap items-baseline gap-x-6 gap-y-2"
          >
            <span className="font-mono text-2xl font-light tracking-tight">
              {data.monitoring.display}
            </span>
            <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
              {data.monitoring.label}
            </span>
            <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
              {data.monitoring.window}
            </span>
            <p className="font-mono text-[10px] leading-relaxed text-muted-foreground/70 basis-full">
              {data.monitoring.definition}
            </p>
          </div>
        ) : null}

        <div className="mt-8 grid lg:grid-cols-12 gap-6 lg:gap-8 items-start">
          <div className="lg:col-span-7 bg-background">
            {/* Top searches */}
            <div data-testid="search-top-table" className="border border-border">
              <div className="px-4 py-3 border-b border-border flex flex-wrap items-baseline justify-between gap-2">
                <p className="font-mono text-[10px] tracking-[0.2em]" title={data.top.definition}>
                  {data.top.heading}
                </p>
                <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
                  {data.top.window}
                </p>
              </div>

              {data.top.rows.length === 0 ? (
                <p className="px-4 py-4 text-sm text-muted-foreground">{data.top.empty_note}</p>
              ) : (
                <table className="w-full text-left">
                  <thead>
                    <tr className="border-b border-border">
                      <th scope="col" className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground px-4 py-2">
                        {data.top.query_header}
                      </th>
                      <th scope="col" className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground px-4 py-2 text-right">
                        {data.top.count_header}
                      </th>
                      <th scope="col" className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground px-4 py-2 text-right">
                        {data.top.with_results_header}
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {data.top.rows.map((row) => (
                      <tr key={row.query}>
                        <th scope="row" className="font-normal px-4 py-3">
                          <Link
                            href={row.search_url}
                            title={row.search_label}
                            className="font-mono text-xs break-words hover:underline"
                          >
                            {row.query}
                          </Link>
                        </th>
                        <td className="font-mono text-xs text-muted-foreground px-4 py-3 text-right whitespace-nowrap">
                          {row.count_label}
                        </td>
                        <td className="font-mono text-[10px] tracking-wider text-muted-foreground px-4 py-3 text-right whitespace-nowrap">
                          {row.with_results_label}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}

              {data.top.withheld_note ? (
                <p
                  data-testid="search-withheld-note"
                  className="px-4 py-3 border-t border-border font-mono text-[10px] leading-relaxed text-muted-foreground/70"
                >
                  {data.top.withheld_note}
                </p>
              ) : null}
            </div>

            {/* Recent searches */}
            <div data-testid="search-recent-list" className="border border-border border-t-0">
              <div className="px-4 py-3 border-b border-border flex flex-wrap items-baseline justify-between gap-2">
                <p className="font-mono text-[10px] tracking-[0.2em]">{data.recent.heading}</p>
                <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
                  {data.recent.window}
                </p>
              </div>

              {data.recent.rows.length === 0 ? (
                <p className="px-4 py-4 text-sm text-muted-foreground">{data.recent.empty_note}</p>
              ) : (
                <ul className="divide-y divide-border">
                  {data.recent.rows.map((row) => (
                    <li
                      key={row.query}
                      className="px-4 py-3 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1"
                    >
                      <Link
                        href={row.search_url}
                        title={row.search_label}
                        className="font-mono text-xs break-words min-w-0 hover:underline"
                      >
                        {row.query}
                      </Link>
                      <span className="flex items-baseline gap-3 shrink-0">
                        <span className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground">
                          {row.searcher_label}
                        </span>
                        <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                          {row.time_label}
                        </span>
                        <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                          {row.results_label}
                        </span>
                      </span>
                    </li>
                  ))}
                </ul>
              )}

              <p className="px-4 py-3 border-t border-border font-mono text-[10px] leading-relaxed text-muted-foreground/70">
                {data.recent.definition}
              </p>
            </div>
          </div>

          <div className="lg:col-span-5 bg-background">
            <SearchChart series={data.series} />
          </div>
        </div>

        <p className="mt-8 max-w-3xl text-sm text-muted-foreground leading-relaxed">
          {data.privacy_note}
        </p>
      </div>
    </section>
  );
}
