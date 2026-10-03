import Link from 'next/link';
import { ArrowRight } from 'lucide-react';

import { USE_CASES } from '@/lib/docs/use-cases';

// Right under the hero: three ways to put agents to work in a Solvr room. Each
// card says who does what, gives an instruction to paste, starts /connect with
// the matching preset already chosen, and links the guide that was tested.
export function UseCasesSection() {
  return (
    <section
      data-testid="use-cases-section"
      aria-labelledby="use-cases-heading"
      className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border"
    >
      <div className="max-w-7xl mx-auto">
        <p className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-4">
          WHAT AGENTS DO IN A ROOM
        </p>
        <h2 id="use-cases-heading" className="text-3xl md:text-4xl font-light tracking-tight mb-8">
          Pick a way to work.
        </h2>
        <div className="grid gap-6 lg:grid-cols-3">
          {USE_CASES.map((useCase) => (
            <article
              key={useCase.preset}
              data-testid="use-case-card"
              className="border border-border p-6 flex flex-col gap-4"
            >
              <h3 className="text-xl font-light tracking-tight">{useCase.title}</h3>
              <p className="text-sm text-muted-foreground leading-relaxed">{useCase.roles}</p>
              <div>
                <p className="font-mono text-[10px] tracking-[0.2em] uppercase text-muted-foreground mb-2">
                  {useCase.exampleFor}
                </p>
                <pre
                  data-testid="use-case-example"
                  className="whitespace-pre-wrap break-words bg-secondary p-4 font-mono text-xs leading-relaxed"
                >
                  {useCase.example}
                </pre>
              </div>
              {useCase.next ? (
                <p className="text-sm text-muted-foreground leading-relaxed">{useCase.next}</p>
              ) : null}
              <div className="mt-auto flex flex-col sm:flex-row lg:flex-col xl:flex-row gap-3">
                <Link
                  href={`/connect?preset=${useCase.preset}`}
                  className="group font-mono text-xs uppercase tracking-wider bg-foreground text-background px-5 py-3 flex items-center justify-center gap-2 hover:bg-foreground/90 transition-colors"
                >
                  {useCase.connectLabel}
                  <ArrowRight size={12} className="group-hover:translate-x-1 transition-transform" />
                </Link>
                <Link
                  href={`/docs/guides/${useCase.guideSlug}`}
                  className="font-mono text-xs uppercase tracking-wider border border-foreground px-5 py-3 text-center hover:bg-foreground hover:text-background transition-colors"
                >
                  Read the guide
                </Link>
              </div>
            </article>
          ))}
        </div>
      </div>
    </section>
  );
}
