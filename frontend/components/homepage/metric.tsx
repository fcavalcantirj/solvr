import type { APIOverviewMetric, APIOverviewTable } from '@/lib/api-types';

import { MetricNote } from '@/components/data/statistics-primitives';

// Open rows: what a figure counts on the left, the figure set big on the right, and
// the window it was measured over under its label. The window opens the definition
// and any caveat, by keyboard or touch. Every word and display value is the API's.
export function MetricGrid({ metrics }: { metrics: APIOverviewMetric[] }) {
  return (
    <dl className="divide-y divide-border">
      {metrics.map((metric) => (
        <div
          key={metric.key}
          data-testid="overview-metric"
          className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-6 py-5 first:pt-0"
        >
          <dt className="max-w-[30ch] font-mono text-[11px] uppercase leading-relaxed tracking-[0.18em] text-foreground">
            {metric.label}
          </dt>
          <dd
            className="row-span-2 text-[2.5rem] font-light leading-none tracking-[-0.04em] tabular-nums sm:text-5xl"
            title={metric.definition}
          >
            {metric.display}
          </dd>
          <dd className="mt-1.5 min-w-0">
            <MetricNote window={metric.window} definition={metric.definition} qualifier={metric.qualifier} />
          </dd>
        </div>
      ))}
    </dl>
  );
}

// SectionHeading opens a homepage section on the API heading and its intro.
// No kicker above it (v1.3.7): the heading carries its own weight.
export function SectionHeading({
  heading,
  intro,
  definition,
}: {
  heading: string;
  intro?: string;
  definition?: string;
}) {
  return (
    <div className="max-w-[44rem]">
      <h2
        className="text-[2rem] font-light leading-[1.08] tracking-[-0.03em] sm:text-[2.75rem]"
        title={definition}
      >
        {heading}
      </h2>
      {intro ? (
        <p className="mt-4 text-base leading-relaxed text-muted-foreground sm:text-lg">{intro}</p>
      ) : null}
    </div>
  );
}

// CompactTable renders one of the API's tables: its heading, the window it
// covers, its definition, and its rows — or the API's own empty note.
export function CompactTable({ table }: { table: APIOverviewTable }) {
  return (
    <div
      data-testid={`overview-table-${table.heading}`}
      className="border border-border"
    >
      <div className="px-4 py-3 border-b border-border flex flex-wrap items-baseline justify-between gap-2">
        <p
          className="font-mono text-[10px] tracking-[0.2em]"
          title={table.definition}
        >
          {table.heading}
        </p>
        <p className="font-mono text-[10px] tracking-wider text-muted-foreground">
          {table.window}
        </p>
      </div>

      {table.rows.length === 0 ? (
        <p className="px-4 py-4 text-sm text-muted-foreground">{table.empty_note}</p>
      ) : (
        <ul className="divide-y divide-border">
          {table.rows.map((row) => (
            <li
              key={row.label}
              className="px-4 py-3 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1"
            >
              <span className="font-mono text-xs break-words min-w-0">{row.label}</span>
              <span className="flex items-baseline gap-3 shrink-0">
                {row.time_label ? (
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                    {row.time_label}
                  </span>
                ) : null}
                {row.detail ? (
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                    {row.detail}
                  </span>
                ) : null}
                <span className="font-mono text-xs text-muted-foreground">
                  {row.count_label}
                </span>
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
