import type { APIOverviewMeta } from '@/lib/api-types';

// OverviewMetaBanner states a degraded snapshot: the stale label and any
// partial-error notices from the meta envelope. The API owns all wording; the
// page never computes or re-words anything. A partial failure degrades to a
// non-blocking notice — the data that DID load is still shown below. A healthy
// snapshot has nothing to report, so nothing renders.
export function OverviewMetaBanner({ meta }: { meta: APIOverviewMeta }) {
  // A healthy API once sent partial_errors as null (Go nil slice), not []. Guard once
  // here so neither the .length check nor the .map below can throw, and so the page
  // survives a server that has not been redeployed yet.
  const partialErrors = meta.partial_errors ?? [];
  const staleLabel = meta.stale && meta.stale_label ? meta.stale_label : '';

  if (!staleLabel && partialErrors.length === 0) return null;

  return (
    <section
      data-testid="overview-meta-banner"
      className="px-4 sm:px-6 lg:px-12 py-4 border-t border-border"
    >
      {staleLabel ? (
        <div className="mx-auto flex max-w-[76rem] flex-wrap items-center gap-4 font-mono text-[11px] tracking-[0.06em] text-muted-foreground">
          <span
            data-testid="overview-stale-label"
            role="status"
            className="flex items-center gap-2 text-amber-700 dark:text-amber-400"
          >
            <svg
              data-testid="overview-stale-icon"
              width="12"
              height="12"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
              <line x1="12" y1="9" x2="12" y2="13" />
              <line x1="12" y1="17" x2="12.01" y2="17" />
            </svg>
            <span>{staleLabel}</span>
          </span>
        </div>
      ) : null}
      {partialErrors.length > 0 ? (
        <div
          data-testid="overview-partial-errors"
          className="mx-auto mt-2 flex max-w-[76rem] flex-col gap-1"
        >
          {partialErrors.map((err, i) => (
            <span
              key={i}
              className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground"
            >
              Temporarily unavailable: {err}
            </span>
          ))}
        </div>
      ) : null}
    </section>
  );
}
