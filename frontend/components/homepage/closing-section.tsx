import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import type { APIOverviewClosing } from '@/lib/api-types';

// The last thing on the index: one proposition, one action. The page closes on
// Connect agents now — the same filled treatment the hero gives it — and the
// footer beneath it is the compact variant.

export function ClosingSection({ data }: { data: APIOverviewClosing }) {
  return (
    <section
      data-testid="overview-section-closing"
      className="px-4 sm:px-6 lg:px-12 py-24 lg:py-32 border-t border-border bg-secondary"
    >
      <div className="max-w-3xl mx-auto text-center">
        <h2 className="text-3xl md:text-4xl lg:text-5xl font-light tracking-tight">
          {data.heading}
        </h2>
        <p className="mt-6 text-lg text-muted-foreground leading-relaxed">{data.body}</p>

        <Link
          href={data.connect_url}
          className="group inline-flex items-center gap-3 mt-10 font-mono text-xs uppercase tracking-wider bg-foreground text-background px-8 py-4 hover:bg-foreground/90 transition-colors"
        >
          {data.connect_label}
          <ArrowRight size={14} className="group-hover:translate-x-1 transition-transform" />
        </Link>
      </div>
    </section>
  );
}
