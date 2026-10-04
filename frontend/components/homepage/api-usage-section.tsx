"use client";

import { useState } from 'react';
import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import { api } from '@/lib/api';
import type { APIOverviewAPIUsage, APIOverviewSeries } from '@/lib/api-types';
import { MetricGrid } from './metric';
import { SegmentedControl } from '@/components/page/segmented-control';
import { StatisticsDetails, StatisticsHeading } from '@/components/data/statistics-primitives';

// How much the API is called.
//
// Every figure, window, definition, caveat, bar height and note on this
// section is a string the API already decided. The browser sends the chosen
// window back and renders the new answer. In particular it never decides that
// a missing measurement is a zero: when the API says a figure is unavailable
// it has already put its own placeholder in `display`, and this component
// prints that placeholder.
//
// The chart ships with a table carrying the same rows, so the series is
// readable without seeing the bars.

function UsageChart({ series }: { series: APIOverviewSeries }) {
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
              data-testid="api-usage-spark-bar"
              className="w-full bg-foreground"
              style={{ height: point.height }}
            />
          </div>
        ))}
      </div>

      <StatisticsDetails label={series.table_heading}>
      <table data-testid="api-usage-series-table" className="w-full mt-6 text-left">
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

export function ApiUsageSection({ initial }: { initial: APIOverviewAPIUsage }) {
  const [data, setData] = useState<APIOverviewAPIUsage>(initial);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const selectWindow = async (value: string) => {
    if (value === data.selected_window) return;
    setLoading(true);
    setError(null);
    try {
      const response = await api.getHomepageApiUsage(value);
      setData(response.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not read the API activity');
    } finally {
      setLoading(false);
    }
  };

  return (
    <section
      id="api"
      data-testid="overview-section-api"
      className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border scroll-mt-16"
    >
      <div className="mx-auto grid max-w-[70rem] gap-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] lg:gap-16">
        <StatisticsHeading heading={data.heading} intro={data.intro}>
          <p className="mt-6 max-w-[34ch] text-[0.8125rem] leading-relaxed text-muted-foreground">{data.scope_note}</p>
          {data.start_note ? (
            <p data-testid="api-usage-start-note" className="mt-3 max-w-[34ch] text-[0.8125rem] leading-relaxed text-muted-foreground">{data.start_note}</p>
          ) : null}
          <Link href={data.docs_url} className="group mt-6 inline-flex items-center gap-3 font-mono text-[11px] uppercase tracking-[0.18em] hover:text-muted-foreground">
            {data.docs_label}
            <ArrowRight aria-hidden="true" size={14} className="transition-transform group-hover:translate-x-1" />
          </Link>
        </StatisticsHeading>

        <div className="min-w-0">
          <div className="mb-8 flex justify-end">
            <SegmentedControl
              label={data.window_label}
              options={data.window_options}
              value={data.selected_window}
              onSelect={selectWindow}
              disabled={loading}
            />
          </div>
          {error ? <p role="alert" className="mb-5 text-sm text-muted-foreground">{error}</p> : null}
          <div data-testid="api-usage-metrics"><MetricGrid metrics={data.metrics} /></div>
          <div className="mt-8"><UsageChart series={data.series} /></div>
          <div className="mt-10">
            <ul className="divide-y divide-border border-t border-border">
              {data.endpoints.map((endpoint) => (
                <li key={`${endpoint.method} ${endpoint.path}`} data-testid="overview-endpoint" className="py-4">
                  <p className="break-words font-mono text-xs">
                    <span className="text-muted-foreground">{endpoint.method}</span>{' '}{endpoint.path}
                  </p>
                  <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{endpoint.summary}</p>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </section>
  );
}
