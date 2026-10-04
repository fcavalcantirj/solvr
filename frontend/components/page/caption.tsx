import type { ComponentPropsWithoutRef, ElementType, ReactNode } from 'react';
import { cn } from '@/lib/utils';

// The one small monospaced label of the language: a caption beside a figure, a
// window, a table's title, a link. Never a kicker above a heading.

export const CAPTION = 'font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground';

type CaptionProps<T extends ElementType> = {
  as?: T;
  className?: string;
  children: ReactNode;
} & Omit<ComponentPropsWithoutRef<T>, 'as' | 'className' | 'children'>;

export function Caption<T extends ElementType = 'p'>({ as, className, children, ...rest }: CaptionProps<T>) {
  const Tag: ElementType = as ?? 'p';
  return (
    <Tag className={cn(CAPTION, className)} {...rest}>
      {children}
    </Tag>
  );
}
