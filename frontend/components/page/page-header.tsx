import type { ReactNode } from 'react';
import { Caption } from '@/components/page/caption';
import { cn } from '@/lib/utils';

// The two ways a page opens. A collection (agents, people, pins) opens on its own
// name, set big over the chartreuse band as /posts and /rooms do, its purpose and
// actions beside it. A working page (settings, referrals, dashboards) opens like
// /status: a plain heading and its purpose, so the thing below can be the big one.
// Every word is the caller's; the size is fixed, never measured from the text.

export function CollectionHeader({
  title,
  lede,
  children,
  className,
}: {
  title: ReactNode;
  lede?: ReactNode;
  children?: ReactNode;
  className?: string;
}) {
  return (
    <header className={cn('grid items-end gap-8 px-4 py-10 sm:px-6 lg:grid-cols-2 lg:px-12 lg:pb-12 lg:pt-14', className)}>
      <h1 className="min-w-0 text-[clamp(4.5rem,10vw,9rem)] font-light leading-none tracking-[-0.065em] [overflow-wrap:anywhere]">
        <span className="prompt-swipe">{title}</span>
      </h1>
      {lede || children ? (
        <div className="flex min-w-0 flex-col items-start gap-6 lg:pb-2">
          {lede ? (
            <p className="max-w-[36ch] text-xl font-light leading-snug tracking-[-0.025em] lg:text-2xl">{lede}</p>
          ) : null}
          {children}
        </div>
      ) : null}
    </header>
  );
}

export function PageHeading({
  title,
  caption,
  lede,
  children,
}: {
  title: ReactNode;
  /** A short label the page already carries; it sits under the heading, never above it. */
  caption?: ReactNode;
  lede?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <header className="grid min-w-0 items-baseline gap-5 px-4 pb-10 pt-10 sm:px-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] lg:gap-16 lg:px-12 lg:pb-12 lg:pt-14">
      <div className="min-w-0">
        <h1 className="text-2xl font-normal tracking-[-0.025em] [overflow-wrap:anywhere]">{title}</h1>
        {caption ? <Caption className="mt-3">{caption}</Caption> : null}
      </div>
      {lede || children ? (
        <div className="flex min-w-0 flex-col items-start gap-6 sm:flex-row sm:items-baseline sm:justify-between">
          {lede ? <p className="max-w-[60ch] text-sm leading-relaxed text-muted-foreground">{lede}</p> : null}
          {children}
        </div>
      ) : null}
    </header>
  );
}
