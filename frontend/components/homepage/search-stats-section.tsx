"use client";

import { useState } from 'react';
import Link from 'next/link';
import { api } from '@/lib/api';
import type { APIOverviewSearch, APIOverviewSeries } from '@/lib/api-types';
import { MetricGrid } from './metric';
import { SegmentedControl } from '@/components/page/segmented-control';
import { StatisticsDetails, StatisticsHeading } from '@/components/data/statistics-primitives';

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
    <figure className="min-w-0">
      <figcaption className="flex flex-wrap items-baseline justify-between gap-2 mb-5">
        <span className="font-mono text-[11px] uppercase tracking-[0.18em]" title={sparkline.definition}>
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
              data-testid="search-spark-bar"
              className="w-full bg-foreground"
              style={{ height: point.height }}
            />
          </div>
        ))}
      </div>

      <StatisticsDetails label={series.table_heading}>
      <table data-testid="search-series-table" className="w-full mt-6 text-left">
        <caption className="text-[0.8125rem] leading-relaxed text-muted-foreground text-left mb-3 caption-bottom">
          {series.table_caption}
        </caption>
        <thead>
          <tr className="border-b border-border">
            <th scope="col" className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground py-2">
              {series.period_header}
            </th>
            <th scope="col" className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground py-2 text-right">
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
      </StatisticsDetails>
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
      id="search"
      data-testid="overview-section-search"
      className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border scroll-mt-16"
    >
      <div className="mx-auto grid max-w-[70rem] gap-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] lg:gap-16">
        <StatisticsHeading heading={data.heading} intro={data.intro}>
          <p className="mt-6 max-w-[34ch] text-[0.8125rem] leading-relaxed text-muted-foreground">{data.privacy_note}</p>
          {/* Known automated monitoring, stated apart from the totals. */}
          {data.monitoring ? (
            <div data-testid="search-monitoring" className="mt-6 max-w-[34ch] border-t border-border pt-4">
              <p className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <span className="text-3xl font-light tabular-nums tracking-[-0.04em]">{data.monitoring.display}</span>
                <span className="font-mono text-[11px] uppercase tracking-[0.18em] text-foreground">{data.monitoring.label}</span>
                <span className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground">{data.monitoring.window}</span>
              </p>
              <p className="mt-2 text-[0.8125rem] leading-relaxed text-muted-foreground">{data.monitoring.definition}</p>
            </div>
          ) : null}
        </StatisticsHeading>
        <div className="min-w-0">
          <div className="mb-8 flex flex-wrap items-center justify-end gap-4">
            <SegmentedControl
              label={data.window_label}
              options={data.window_options}
              value={data.selected_window}
              onSelect={selectWindow}
              disabled={loading}
            />
          </div>

          {error ? (
            <p role="alert" className="mt-5 font-mono text-xs text-muted-foreground">
              {error}
            </p>
          ) : null}

          <div data-testid="search-metrics" className="min-w-0">
            <MetricGrid metrics={data.metrics} />
          </div>

          <div className="mt-8"><SearchChart series={data.series} /></div>

          <div className="mt-4">
            <div className="min-w-0">
              {/* Top searches */}
              <div data-testid="search-top-table">
                <StatisticsDetails label={data.top.heading} defaultOpen>
                  <div className="py-3 border-b border-border flex flex-wrap items-baseline justify-between gap-2">
                    <p className="text-xs leading-relaxed text-muted-foreground" title={data.top.definition}>{data.top.definition}</p>
                    <p className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground">
                      {data.top.window}
                    </p>
                  </div>

                  {data.top.rows.length === 0 ? (
                    <p className="py-4 text-sm text-muted-foreground">{data.top.empty_note}</p>
                  ) : (
                    <table className="w-full table-fixed text-left">
                      <thead>
                        <tr className="border-b border-border">
                          <th scope="col" className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground py-2">
                            {data.top.query_header}
                          </th>
                          <th scope="col" className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground py-2 text-right">
                            {data.top.count_header}
                          </th>
                          <th scope="col" className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground py-2 text-right">
                            {data.top.with_results_header}
                          </th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border">
                        {data.top.rows.map((row) => (
                          <tr key={row.query}>
                            <th scope="row" className="font-normal py-3">
                              <Link
                                href={row.search_url}
                                title={row.search_label}
                                className="text-[0.9375rem] underline-offset-4 [overflow-wrap:anywhere] hover:underline"
                              >
                                {row.query}
                              </Link>
                            </th>
                            <td className="font-mono text-xs text-muted-foreground py-3 text-right">
                              {row.count_label}
                            </td>
                            <td className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground py-3 text-right">
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
                      className="py-3 border-t border-border text-[0.8125rem] leading-relaxed text-muted-foreground"
                    >
                      {data.top.withheld_note}
                    </p>
                  ) : null}
                </StatisticsDetails>
              </div>

              {/* Recent searches */}
              <div data-testid="search-recent-list">
                <StatisticsDetails label={data.recent.heading} defaultOpen>
                  <div className="py-3 border-b border-border flex flex-wrap items-baseline justify-between gap-2">
                    <p className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground">
                      {data.recent.window}
                    </p>
                  </div>

                  {data.recent.rows.length === 0 ? (
                    <p className="py-4 text-sm text-muted-foreground">{data.recent.empty_note}</p>
                  ) : (
                    <ul className="divide-y divide-border">
                      {data.recent.rows.map((row) => (
                        <li
                          key={row.query}
                          className="py-3 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1"
                        >
                          <Link
                            href={row.search_url}
                            title={row.search_label}
                            className="min-w-0 text-[0.9375rem] underline-offset-4 [overflow-wrap:anywhere] hover:underline"
                          >
                            {row.query}
                          </Link>
                          <span className="flex flex-wrap items-baseline gap-3">
                            <span className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
                              {row.searcher_label}
                            </span>
                            <span className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground">
                              {row.time_label}
                            </span>
                            <span className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground">
                              {row.results_label}
                            </span>
                          </span>
                        </li>
                      ))}
                    </ul>
                  )}

                  <p className="py-3 border-t border-border text-[0.8125rem] leading-relaxed text-muted-foreground">
                    {data.recent.definition}
                  </p>
                </StatisticsDetails>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
