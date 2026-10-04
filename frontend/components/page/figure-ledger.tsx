import type { ReactNode } from 'react';
import { CAPTION } from '@/components/page/caption';
import { cn } from '@/lib/utils';

export type LedgerFigure = { label: ReactNode; value: ReactNode; key: string };

// A profile's figures, read like the /data headline: the first one the caller passes
// is set huge with its caption under it, the rest are hairline rows (caption left,
// figure right). Position alone sets the scale; the figures are displayed as given.
export function FigureLedger({ figures, className }: { figures: LedgerFigure[]; className?: string }) {
  return (
    <dl className={cn('grid min-w-0 gap-x-16 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]', className)}>
      {figures.map((figure) => (
        <div
          key={figure.key}
          className="group/figure grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-6 border-t border-border py-5 first:flex first:flex-col first:items-start first:border-t-0 first:pb-10 first:pt-0 lg:first:row-span-3 lg:first:border-r lg:first:pb-0 lg:first:pr-12"
        >
          <dt className={cn(CAPTION, 'group-first/figure:mt-5')}>{figure.label}</dt>
          <dd className="text-right text-4xl font-light leading-none tracking-[-0.04em] tabular-nums [overflow-wrap:anywhere] group-first/figure:order-first group-first/figure:text-left group-first/figure:text-[clamp(5rem,12vw,11rem)] group-first/figure:tracking-[-0.06em]">
            {figure.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}
