import type { APIOverviewMetric, APIOverviewTable } from '@/lib/api-types';

// The shared vocabulary of the live index: a strong monospaced number, the
// window it was measured over, and the definition of what it counts — all of
// them API strings. Thin dividers, square corners, no colour.

export function MetricGrid({ metrics }: { metrics: APIOverviewMetric[] }) {
  return (
    <dl className="grid grid-cols-2 lg:grid-cols-3 gap-px bg-border border border-border">
      {metrics.map((metric) => (
        <div
          key={metric.key}
          data-testid="overview-metric"
          className="bg-background p-5 sm:p-6"
        >
          <dd
            className="font-mono text-3xl sm:text-4xl font-light tracking-tight"
            title={metric.definition}
          >
            {metric.display}
          </dd>
          <dt className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground mt-3">
            {metric.label}
          </dt>
          <p className="font-mono text-[10px] tracking-wider text-muted-foreground/70 mt-1">
            {metric.window}
          </p>
        </div>
      ))}
    </dl>
  );
}

// SectionHeading carries the eyebrow, the API heading and its intro.
export function SectionHeading({
  eyebrow,
  heading,
  intro,
  definition,
}: {
  eyebrow: string;
  heading: string;
  intro?: string;
  definition?: string;
}) {
  return (
    <div className="max-w-3xl">
      <p className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-4">
        {eyebrow}
      </p>
      <h2 className="text-3xl md:text-4xl font-light tracking-tight" title={definition}>
        {heading}
      </h2>
      {intro ? (
        <p className="text-muted-foreground leading-relaxed mt-4">{intro}</p>
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
