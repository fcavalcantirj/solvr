import type { ReactNode } from 'react';
import { FigureLedger, type LedgerFigure } from '@/components/page/figure-ledger';

// A profile opens on the agent or the person themself: their mark, their name set
// big over the chartreuse band, what they say about themselves, and then their
// figures read like the /data headline. The name's size is fixed by the viewport,
// never by its length; a long handle simply wraps.
export function ProfileHero({
  avatar,
  name,
  aside,
  children,
  figures,
  footer,
}: {
  avatar: ReactNode;
  name: ReactNode;
  /** Actions that belong to the profile (follow), set against the mark. */
  aside?: ReactNode;
  /** What the profile says: status, dates, bio. */
  children?: ReactNode;
  figures: LedgerFigure[];
  /** A last hairline row under the figures (external links). */
  footer?: ReactNode;
}) {
  return (
    <header className="px-4 pt-10 sm:px-6 lg:px-12 lg:pt-14">
      <div className="flex min-w-0 items-start justify-between gap-6">
        <div className="flex size-16 shrink-0 items-center justify-center overflow-hidden border border-foreground sm:size-20">
          {avatar}
        </div>
        {aside ? <div className="flex min-w-0 flex-wrap items-center justify-end gap-3">{aside}</div> : null}
      </div>
      <h1 className="mt-8 text-[clamp(3rem,7.4vw,8rem)] font-light leading-none tracking-[-0.055em] [overflow-wrap:anywhere] lg:mt-10">
        <span className="prompt-swipe">{name}</span>
      </h1>
      {children ? <div className="mt-8 min-w-0 lg:mt-10">{children}</div> : null}
      <div className="mt-12 border-t border-border pb-12 pt-10 lg:pb-16 lg:pt-12">
        <FigureLedger figures={figures} />
      </div>
      {footer ? <div className="border-t border-border py-5">{footer}</div> : null}
    </header>
  );
}
