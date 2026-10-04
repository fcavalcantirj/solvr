import type { ElementType, ReactNode } from 'react';
import { cn } from '@/lib/utils';

// The section grammar of /data and /status: a hairline across the page, the heading
// (and its purpose, and any action that belongs to it) in a left column, the ledger
// on the right. On a phone the two simply stack.

export const SECTION =
  'grid min-w-0 gap-8 border-t border-border px-4 py-12 sm:px-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] lg:gap-16 lg:px-12 lg:py-16';

export const SECTION_HEADING = 'text-2xl font-light leading-tight tracking-[-0.025em] sm:text-3xl [overflow-wrap:anywhere]';

export function PageSection({
  heading,
  headingAs,
  headingClassName,
  intro,
  aside,
  id,
  className,
  children,
}: {
  heading: ReactNode;
  headingAs?: ElementType;
  headingClassName?: string;
  intro?: ReactNode;
  /** An action or note that belongs with the heading, under its purpose. */
  aside?: ReactNode;
  id?: string;
  className?: string;
  children: ReactNode;
}) {
  const Heading: ElementType = headingAs ?? 'h2';
  return (
    <section id={id} className={cn(SECTION, className)}>
      <div className="min-w-0 lg:sticky lg:top-24 lg:self-start">
        <Heading className={cn(SECTION_HEADING, headingClassName)}>{heading}</Heading>
        {intro ? <p className="mt-4 max-w-[40ch] text-sm leading-relaxed text-muted-foreground">{intro}</p> : null}
        {aside ? <div className="mt-6">{aside}</div> : null}
      </div>
      <div className="min-w-0">{children}</div>
    </section>
  );
}
