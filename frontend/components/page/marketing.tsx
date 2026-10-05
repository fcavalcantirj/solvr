import type { ElementType, ReactNode } from "react";
import { CAPTION } from "@/components/page/caption";
import { CopyButton, type CopyReport } from "@/components/page/copy-button";
import { cn } from "@/lib/utils";

// The marketing family (/skill, /mcp, /api-docs, /amcp, /ipfs, /how-it-works) in the
// one-sentence language. Each page opens on its own thing set big — the line to run, the
// config to paste, the endpoint, the mechanism — and everything after it recedes into
// ledger sections: a heading column on the left, hairline rows on the right. Code is
// content: real, copyable JetBrains Mono, wrapped so the page only ever scrolls down.
//
// Every block that carries a Copy button takes a `report`: the surface and a stable id of
// what it copies, sent as code_copy once the copy worked (SPEC.md 27.7).

export const FRAME = "mx-auto w-full max-w-[76rem]";
export const SECTION = "border-b border-border px-4 py-14 sm:px-6 lg:px-12 lg:py-20";
export const LEDGER = cn(FRAME, "grid gap-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] lg:gap-16");

export const FOCUS =
  "focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-4 focus-visible:outline-foreground";

// The filled square action (the header's CONNECT AGENTS) and the caption link with an arrow.
export const ACTION = cn(
  "inline-flex min-h-12 items-center justify-center gap-3 border border-foreground bg-foreground px-6 py-3.5 font-mono text-[11px] uppercase tracking-[0.18em] text-background transition-colors hover:bg-background hover:text-foreground",
  FOCUS,
);
export const TEXT_LINK = cn(
  "group inline-flex min-h-10 items-center gap-2 font-mono text-[11px] uppercase tracking-[0.18em] text-foreground transition-colors hover:text-muted-foreground",
  FOCUS,
);

// Mono code that reads as code: wrapped, indentation kept, never a sideways scroll.
const CODE_TEXT = "whitespace-pre-wrap font-mono [overflow-wrap:anywhere] [tab-size:2]";

export function MarketingHero({ children }: { children: ReactNode }) {
  return (
    <section className="border-b border-border px-4 pb-14 pt-10 sm:px-6 lg:px-12 lg:pb-20 lg:pt-14">
      <div className={FRAME}>{children}</div>
    </section>
  );
}

// A one-line command or address set as the page's title: light mono on the paper, the
// accent band under it, and the word that copies it.
export function HeroCode({ code, className, report }: { code: string; className: string; report: CopyReport }) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-x-10 gap-y-4">
      <code className={cn("min-w-0 font-mono font-light leading-[1.18] tracking-[-0.04em] [overflow-wrap:anywhere]", className)}>
        <span className="prompt-swipe">{code}</span>
      </code>
      <CopyButton text={code} report={report} className="ml-auto" />
    </div>
  );
}

// Under the page's thing: the page's own heading and sentence on the left, its working
// detail on the right.
export function HeroLead({
  title,
  intro,
  actions,
  aside,
}: {
  title: ReactNode;
  intro?: ReactNode;
  actions?: ReactNode;
  aside?: ReactNode;
}) {
  return (
    <div className="mt-10 grid gap-12 border-t border-border pt-10 lg:mt-14 lg:grid-cols-2 lg:gap-16 lg:pt-12">
      <div className="min-w-0">
        <h1 className="text-[2rem] font-light leading-[1.08] tracking-[-0.035em] sm:text-[2.5rem] lg:text-5xl">
          {title}
        </h1>
        {intro ? (
          <p className="mt-6 max-w-[46ch] text-base leading-relaxed text-muted-foreground lg:text-lg">{intro}</p>
        ) : null}
        {actions ? <div className="mt-8 flex flex-wrap items-center gap-x-8 gap-y-3">{actions}</div> : null}
      </div>
      {aside ? <div className="min-w-0">{aside}</div> : null}
    </div>
  );
}

// A ledger section: the heading column (sticky while its rows scroll past on wide screens)
// and the rows themselves.
export function MarketingSection({
  heading,
  headingId,
  intro,
  aside,
  sticky = true,
  children,
}: {
  heading: ReactNode;
  headingId?: string;
  intro?: ReactNode;
  aside?: ReactNode;
  sticky?: boolean;
  children: ReactNode;
}) {
  return (
    <section className={SECTION}>
      <div className={LEDGER}>
        <div className={cn("min-w-0", sticky && "lg:sticky lg:top-28 lg:self-start")}>
          <h2 id={headingId} className="text-[1.75rem] font-light leading-tight tracking-[-0.03em] sm:text-3xl lg:text-[2.25rem]">
            {heading}
          </h2>
          {intro ? <p className="mt-4 max-w-[38ch] text-sm leading-relaxed text-muted-foreground">{intro}</p> : null}
          {aside}
        </div>
        <div className="min-w-0">{children}</div>
      </div>
    </section>
  );
}

// A block of code on an inverted ink tile, with its name and the word that copies it.
export function CodeTile({
  label,
  note,
  chip,
  code,
  copy = code,
  report,
  className,
  codeClassName,
}: {
  label?: ReactNode;
  note?: ReactNode;
  chip?: ReactNode;
  code: string;
  copy?: string | null;
  report: CopyReport;
  className?: string;
  codeClassName?: string;
}) {
  const header = label || note || chip;
  return (
    <figure className={cn("relative min-w-0 bg-foreground text-background", className)}>
      {header ? (
        <figcaption className="flex min-h-12 items-center justify-between gap-4 border-b border-background/15 py-1 pl-5 pr-3">
          <span className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-1">
            {label ? <span className={cn(CAPTION, "text-background/60")}>{label}</span> : null}
            {chip}
            {note ? <span className="text-xs text-background/60">{note}</span> : null}
          </span>
          {copy ? <CopyButton text={copy} report={report} onInk /> : null}
        </figcaption>
      ) : copy ? (
        <CopyButton text={copy} report={report} onInk className="absolute right-3 top-2" />
      ) : null}
      <pre className={cn(CODE_TEXT, "p-5 text-[13px] leading-[1.75] sm:text-sm", !header && copy && "pr-24", codeClassName)}>
        <code>{code}</code>
      </pre>
    </figure>
  );
}

// One line of code on ink, with the word that copies it.
export function CommandTile({ code, className, report }: { code: string; className?: string; report: CopyReport }) {
  return (
    <div className={cn("flex min-w-0 items-center justify-between gap-4 bg-foreground py-1.5 pl-5 pr-3 text-background", className)}>
      <code className={cn(CODE_TEXT, "min-w-0 py-2 text-[13px] leading-relaxed sm:text-sm")}>{code}</code>
      <CopyButton text={code} report={report} onInk />
    </div>
  );
}

// The accent's one other job: a chip of fill behind ink on the paper or on an ink tile.
export function AccentChip({ children }: { children: ReactNode }) {
  return (
    <span className="bg-prompt-accent px-2 py-0.5 font-mono text-[11px] uppercase tracking-[0.18em] text-prompt-accent-foreground">
      {children}
    </span>
  );
}

export type Step = { n: string; title: ReactNode; body?: ReactNode; extra?: ReactNode };

// Numbered steps as hairline rows: the numeral set light and big, the step beside it.
export function StepList({
  title,
  titleAs: Title = "h2",
  stepAs: StepTitle = "h3",
  steps,
}: {
  title?: string;
  titleAs?: ElementType;
  stepAs?: ElementType;
  steps: Step[];
}) {
  return (
    <div className="min-w-0">
      {title ? <Title className={cn(CAPTION, "mb-4 text-foreground")}>{title}</Title> : null}
      <ol className="border-b border-border">
        {steps.map((step) => (
          <li key={step.n} className="grid grid-cols-[3rem_minmax(0,1fr)] gap-x-4 border-t border-border py-5">
            <span aria-hidden="true" className="text-[2.5rem] font-light leading-[0.85] tracking-[-0.05em] tabular-nums">
              {step.n}
            </span>
            <div className="min-w-0">
              <StepTitle className="text-xl font-light leading-snug tracking-[-0.02em]">{step.title}</StepTitle>
              {step.body ? <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">{step.body}</p> : null}
              {step.extra}
            </div>
          </li>
        ))}
      </ol>
    </div>
  );
}

export type Feature = { title: ReactNode; body: ReactNode; extra?: ReactNode };

// What something gives, as open rows: a light heading and the sentence under it.
export function FeatureRows({ features }: { features: Feature[] }) {
  return (
    <div className="divide-y divide-border border-b border-border">
      {features.map((feature, index) => (
        <div key={index} className="py-7 first:pt-0">
          <h3 className="text-2xl font-light leading-tight tracking-[-0.025em] sm:text-[1.75rem]">{feature.title}</h3>
          <p className="mt-3 max-w-[60ch] text-sm leading-relaxed text-muted-foreground">{feature.body}</p>
          {feature.extra}
        </div>
      ))}
    </div>
  );
}

// A server's address and its state: the caption names it, the address is set as a figure,
// the state is said in words beside its dot.
export function StatusRow({ label, value, children }: { label: string; value: ReactNode; children: ReactNode }) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-x-6 gap-y-3 border-y border-border py-5">
      <div className="min-w-0">
        <p className={CAPTION}>{label}</p>
        <code className="mt-2 block font-mono text-lg [overflow-wrap:anywhere]">{value}</code>
      </div>
      <div className="flex items-center gap-2">{children}</div>
    </div>
  );
}
