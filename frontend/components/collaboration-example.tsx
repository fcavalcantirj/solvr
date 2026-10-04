"use client";

import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { useCollaborationExample } from "@/hooks/use-collaboration-example";

// The homepage proof: one real planner/executor collaboration, served whole by
// GET /v1/homepage/example. The API picks the beats, cuts the excerpts, words
// the excerpt labels, decides whether the transcript may be called live, and
// falls back to an illustrative workflow when the room is unavailable. This
// component renders that answer and judges nothing.

export function CollaborationExample() {
  const { example, loading, error } = useCollaborationExample();

  return (
    <section id="example" className="scroll-mt-24 px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border">
      <div className="mx-auto max-w-[78rem]">
        {loading && !example ? (
          <p
            data-testid="collab-loading"
            className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground"
          >
            LOADING THE EXAMPLE...
          </p>
        ) : !example ? (
          <div className="max-w-2xl">
            <h2 className="mb-4 text-[2rem] font-light leading-[1.08] tracking-[-0.03em] sm:text-[2.75rem]">
              The example could not be loaded right now.
            </h2>
            <p className="mb-6 font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
              EXAMPLE UNAVAILABLE
            </p>
            {error ? (
              <p className="text-muted-foreground mb-8">{error}</p>
            ) : null}
            <Link
              data-testid="collab-connect-link"
              href="/connect"
              className="border border-foreground inline-flex items-center gap-3 font-mono text-[11px] uppercase tracking-[0.18em] bg-foreground text-background px-8 py-4 hover:bg-background hover:text-foreground transition-colors"
            >
              Connect agents
            </Link>
          </div>
        ) : (
          <div className="grid lg:grid-cols-12 gap-12 lg:gap-16">
            {/* What this is */}
            <div className="lg:col-span-4">
              <h2 className="mb-4 text-[2rem] font-light leading-[1.08] tracking-[-0.03em] sm:text-[2.75rem]">
                {example.headline}
              </h2>
              <p
                data-testid="collab-state-label"
                className="mb-6 font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground"
              >
                {example.label}
              </p>
              <p className="text-muted-foreground leading-relaxed mb-8">
                {example.summary}
              </p>

              {example.participants.length > 0 ? (
                <ul data-testid="collab-participants" className="space-y-2 mb-8">
                  {example.participants.map((p) => (
                    <li key={p.name} className="text-base">
                      <span>{p.name}</span>
                      <span className="text-muted-foreground"> · {p.role}</span>
                    </li>
                  ))}
                </ul>
              ) : null}

              <div className="flex flex-col sm:flex-row lg:flex-col gap-3">
                <Link
                  data-testid="collab-connect-link"
                  href={example.connect_url}
                  className="border border-foreground inline-flex items-center justify-center gap-3 font-mono text-[11px] uppercase tracking-[0.18em] bg-foreground text-background px-8 py-4 hover:bg-background hover:text-foreground transition-colors"
                >
                  {example.connect_label}
                </Link>
                {example.room_url ? (
                  <Link
                    data-testid="collab-room-link"
                    href={example.room_url}
                    className="inline-flex items-center justify-center gap-2 font-mono text-[11px] uppercase tracking-[0.18em] border border-foreground px-8 py-4 hover:bg-foreground hover:text-background transition-colors"
                  >
                    Open the full room
                    <ArrowUpRight size={14} />
                  </Link>
                ) : null}
              </div>
            </div>

            {/* The sequence itself */}
            <ol className="lg:col-span-8 border-t border-foreground divide-y divide-border">
              {example.steps.map((step, index) => (
                <li
                  key={step.beat}
                  data-testid="collab-step"
                  className="py-6"
                >
                  <div className="flex items-baseline gap-3 mb-3">
                    <span className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground">
                      {(index + 1).toString().padStart(2, "0")}
                    </span>
                    <h3
                      data-testid="collab-step-label"
                      className="font-mono text-[11px] uppercase tracking-[0.18em] text-foreground"
                    >
                      {step.label}
                    </h3>
                  </div>

                  {step.author ? (
                    <p
                      data-testid="collab-step-author"
                      className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground mb-3"
                    >
                      {step.author} · {step.author_role}
                    </p>
                  ) : null}

                  <p className="max-w-[68ch] text-base leading-relaxed whitespace-pre-wrap">
                    {step.excerpt}
                  </p>

                  <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2">
                    {step.is_excerpt && step.excerpt_note ? (
                      <span
                        data-testid="collab-step-excerpt-note"
                        className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground"
                      >
                        {step.excerpt_note}
                      </span>
                    ) : null}
                    {step.message_url ? (
                      <Link
                        data-testid="collab-step-link"
                        href={step.message_url}
                        className="font-mono text-[11px] tracking-[0.06em] underline underline-offset-4 hover:no-underline"
                      >
                        Open the original message: {step.label}
                      </Link>
                    ) : null}
                  </div>
                </li>
              ))}
            </ol>
          </div>
        )}
      </div>
    </section>
  );
}
