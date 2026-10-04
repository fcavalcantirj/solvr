// The square controls of the language, as class strings so a link, a button or a
// shadcn primitive at its call site can all wear them. Labels stay the caller's.

const FOCUS = 'focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground';

const BASE = `inline-flex items-center justify-center gap-2 border px-5 py-3 font-mono text-[11px] uppercase leading-none tracking-[0.18em] whitespace-nowrap transition-colors disabled:pointer-events-none disabled:opacity-50 [&_svg]:size-3.5 [&_svg]:shrink-0 ${FOCUS}`;

/** Filled in ink: the one action a block exists for. */
export const INK_BUTTON = `${BASE} border-foreground bg-foreground text-background hover:bg-background hover:text-foreground`;

/** A hairline frame: every other action. */
export const LINE_BUTTON = `${BASE} border-border bg-transparent text-foreground hover:border-foreground`;

/** An irreversible action, in the destructive token. */
export const DANGER_BUTTON = `${BASE} border-destructive bg-transparent text-destructive hover:bg-destructive hover:text-destructive-foreground`;

/** A quiet text action, underlined on hover. */
export const TEXT_LINK = `font-mono text-[11px] uppercase tracking-[0.18em] underline decoration-transparent underline-offset-4 transition-colors hover:decoration-current ${FOCUS}`;

/** A small icon-only button. */
export const ICON_BUTTON = `inline-flex size-9 shrink-0 items-center justify-center text-muted-foreground transition-colors hover:text-foreground ${FOCUS}`;

/** A text field on the warm white: a hairline that darkens with focus. */
export const FIELD =
  'w-full min-w-0 border border-border bg-transparent px-4 py-3 text-base placeholder:text-muted-foreground transition-colors focus-visible:border-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-0 focus-visible:outline-foreground';

/** The pressed state of a square toggle, shared by tab rows and filters. */
export function toggleClass(pressed: boolean): string {
  return `px-4 py-2.5 font-mono text-[11px] uppercase tracking-[0.18em] transition-colors disabled:opacity-50 focus-visible:relative focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground ${
    pressed ? 'bg-foreground text-background' : 'text-muted-foreground hover:bg-secondary hover:text-foreground'
  }`;
}
