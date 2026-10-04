import type { ReactNode } from 'react';
import { ChevronDown, Info } from 'lucide-react';
import { CAPTION } from '@/components/page/caption';
import { cn } from '@/lib/utils';

// Disclosure names are supplied by the API, just like their contents. A list the
// reader comes for opens by default; supporting detail starts closed.
export function StatisticsDetails({
  label,
  defaultOpen = false,
  children,
}: {
  label: string;
  defaultOpen?: boolean;
  children: ReactNode;
}) {
  return (
    <details open={defaultOpen} className="group/details min-w-0 border-t border-border">
      <summary
        className={cn(
          CAPTION,
          'flex min-h-12 cursor-pointer list-none items-center justify-between gap-4 py-3 text-foreground transition-colors hover:text-muted-foreground [&::-webkit-details-marker]:hidden',
          'focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground',
        )}
      >
        <span>{label}</span>
        <ChevronDown aria-hidden="true" className="size-4 shrink-0 transition-transform group-open/details:rotate-180" />
      </summary>
      <div className="pb-6">{children}</div>
    </details>
  );
}

// A figure's window is also the way to its fine print: what the number counts and the
// caveat it cannot state on its own sit one click away behind it, so the page reads as
// numbers first.
export function MetricNote({ window, definition, qualifier }: { window: string; definition: string; qualifier?: string }) {
  return (
    <details className="group/note">
      <summary
        className={cn(
          CAPTION,
          'inline-flex cursor-pointer list-none items-center gap-1.5 normal-case tracking-[0.06em] transition-colors hover:text-foreground [&::-webkit-details-marker]:hidden',
          'focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground',
        )}
      >
        {window}
        <Info aria-hidden="true" className="size-3 opacity-60 group-open/note:opacity-100" />
      </summary>
      <div className="mt-3 max-w-[60ch] space-y-2 text-[0.8125rem] leading-relaxed text-muted-foreground">
        <p>{definition}</p>
        {qualifier ? <p>{qualifier}</p> : null}
      </div>
    </details>
  );
}

export function StatisticsHeading({ heading, intro, children }: { heading: string; intro: string; children?: ReactNode }) {
  return (
    <div className="min-w-0 lg:sticky lg:top-28 lg:self-start">
      <h2 className="text-2xl font-light leading-tight tracking-[-0.025em] sm:text-3xl">{heading}</h2>
      <p className="mt-4 max-w-[34ch] text-sm leading-relaxed text-muted-foreground">{intro}</p>
      {children}
    </div>
  );
}
