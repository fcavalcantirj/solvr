"use client";

import { useState } from 'react';
import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import { api } from '@/lib/api';
import type { APIOverviewAPIUsage, APIOverviewSeries } from '@/lib/api-types';
import { MetricGrid, SectionHeading } from './metric';

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
              data-testid="api-usage-spark-bar"
              className="w-full bg-foreground min-h-px"
              style={{ height: point.height }}
            />
          </div>
        ))}
      </div>

      {/* The accessible equivalent: the same rows, as a table. */}
      <table data-testid="api-usage-series-table" className="w-full mt-6 text-left">
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
      data-testid="overview-section-api"
      className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border"
    >
      <div className="max-w-7xl mx-auto">
        <SectionHeading eyebrow="API" heading={data.heading} intro={data.intro} />

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

        <div className="mt-6 grid lg:grid-cols-12 gap-8 lg:gap-12 items-start">
          <div className="lg:col-span-7">
            <div data-testid="api-usage-metrics">
              <MetricGrid metrics={data.metrics} />
            </div>

            {/* When measurement began, when that falls inside the window. */}
            {data.start_note ? (
              <p
                data-testid="api-usage-start-note"
                className="mt-4 font-mono text-[10px] leading-relaxed text-muted-foreground/70"
              >
                {data.start_note}
              </p>
            ) : null}
          </div>

          <div className="lg:col-span-5">
            <UsageChart series={data.series} />
          </div>
        </div>

        <p className="mt-8 max-w-3xl text-sm text-muted-foreground leading-relaxed">
          {data.scope_note}
        </p>

        <ul className="mt-10 border border-border divide-y divide-border">
          {data.endpoints.map((endpoint) => (
            <li
              key={`${endpoint.method} ${endpoint.path}`}
              data-testid="overview-endpoint"
              className="p-4 sm:p-5"
            >
              <p className="font-mono text-xs tracking-wider break-words">
                <span className="text-muted-foreground">{endpoint.method}</span>{' '}
                {endpoint.path}
              </p>
              <p className="text-sm text-muted-foreground leading-relaxed mt-1">
                {endpoint.summary}
              </p>
            </li>
          ))}
        </ul>

        <Link
          href={data.docs_url}
          className="group inline-flex items-center gap-3 mt-10 font-mono text-xs uppercase tracking-wider border border-foreground px-8 py-4 hover:bg-foreground hover:text-background transition-colors"
        >
          {data.docs_label}
          <ArrowRight size={14} className="group-hover:translate-x-1 transition-transform" />
        </Link>
      </div>
    </section>
  );
}
