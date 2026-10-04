import type { ReactNode } from "react";

// One briefing block, shared by the agent's own briefing and the platform briefing:
// a hairline, its heading in light Inter with whatever the API counts beside it,
// then its rows, each on a hairline of its own.
export function BriefingBlock({ title, icon, extra, children }: { title: string; icon?: ReactNode; extra?: ReactNode; children: ReactNode }) {
  return (
    <section className="min-w-0 border-t border-border py-6">
      <div className="mb-4 flex flex-wrap items-baseline gap-x-4 gap-y-2">
        {icon}
        <h3 className="text-2xl font-light leading-tight tracking-[-0.025em]">{title}</h3>
        {extra}
      </div>
      {children}
    </section>
  );
}

const ROW_BASE = "min-w-0 border-t border-border py-3 first:border-t-0 first:pt-0";
const LINK_FOCUS = "group focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground";

export const BRIEFING_EMPTY = "text-sm text-muted-foreground";
export const BRIEFING_ROW = `flex items-start gap-3 ${ROW_BASE}`;
export const BRIEFING_LINK_ROW = `${BRIEFING_ROW} ${LINK_FOCUS}`;
export const BRIEFING_LINK_BLOCK = `block ${ROW_BASE} ${LINK_FOCUS}`;
export const BRIEFING_TITLE = "text-base leading-snug line-clamp-1 underline decoration-transparent decoration-1 underline-offset-4 transition-colors group-hover:decoration-current";
export const BRIEFING_META = "flex flex-wrap items-center gap-x-4 gap-y-1 font-mono text-[11px] text-muted-foreground";
export const BRIEFING_TAG = "font-mono text-[11px] text-muted-foreground";
