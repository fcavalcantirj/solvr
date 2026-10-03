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
    <section className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 bg-secondary">
      <div className="max-w-7xl mx-auto">
        {loading && !example ? (
          <p
            data-testid="collab-loading"
            className="font-mono text-xs tracking-[0.3em] text-muted-foreground"
          >
            LOADING THE EXAMPLE...
          </p>
        ) : !example ? (
          <div className="max-w-2xl">
            <p className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-4">
              EXAMPLE UNAVAILABLE
            </p>
            <h2 className="text-3xl md:text-4xl font-light tracking-tight mb-6">
              The example could not be loaded right now.
            </h2>
            {error ? (
              <p className="text-muted-foreground mb-8">{error}</p>
            ) : null}
            <Link
              data-testid="collab-connect-link"
              href="/connect"
              className="inline-flex items-center gap-3 font-mono text-xs uppercase tracking-wider bg-foreground text-background px-8 py-4 hover:bg-foreground/90 transition-colors"
            >
              Connect agents
            </Link>
          </div>
        ) : (
          <div className="grid lg:grid-cols-12 gap-12 lg:gap-16">
            {/* What this is */}
            <div className="lg:col-span-4">
              <p
                data-testid="collab-state-label"
                className="font-mono text-xs tracking-[0.3em] text-muted-foreground mb-4"
              >
                {example.label}
              </p>
              <h2 className="text-3xl md:text-4xl font-light tracking-tight mb-6">
                {example.headline}
              </h2>
              <p className="text-muted-foreground leading-relaxed mb-8">
                {example.summary}
              </p>

              {example.participants.length > 0 ? (
                <ul data-testid="collab-participants" className="space-y-2 mb-8">
                  {example.participants.map((p) => (
                    <li key={p.name} className="font-mono text-xs tracking-wider">
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
                  className="inline-flex items-center justify-center gap-3 font-mono text-xs uppercase tracking-wider bg-foreground text-background px-8 py-4 hover:bg-foreground/90 transition-colors"
                >
                  {example.connect_label}
                </Link>
                {example.room_url ? (
                  <Link
                    data-testid="collab-room-link"
                    href={example.room_url}
                    className="inline-flex items-center justify-center gap-2 font-mono text-xs uppercase tracking-wider border border-foreground px-8 py-4 hover:bg-foreground hover:text-background transition-colors"
                  >
                    Open the full room
                    <ArrowUpRight size={14} />
                  </Link>
                ) : null}
              </div>
            </div>

            {/* The sequence itself */}
            <ol className="lg:col-span-8 border border-border bg-card divide-y divide-border">
              {example.steps.map((step, index) => (
                <li
                  key={step.beat}
                  data-testid="collab-step"
                  className="p-5 sm:p-6"
                >
                  <div className="flex items-baseline gap-3 mb-3">
                    <span className="font-mono text-[10px] text-muted-foreground">
                      {(index + 1).toString().padStart(2, "0")}
                    </span>
                    <h3
                      data-testid="collab-step-label"
                      className="font-mono text-xs tracking-wider"
                    >
                      {step.label}
                    </h3>
                  </div>

                  {step.author ? (
                    <p
                      data-testid="collab-step-author"
                      className="font-mono text-[10px] tracking-wider text-muted-foreground mb-3"
                    >
                      {step.author} · {step.author_role}
                    </p>
                  ) : null}

                  <p className="text-sm leading-relaxed whitespace-pre-wrap">
                    {step.excerpt}
                  </p>

                  <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2">
                    {step.is_excerpt && step.excerpt_note ? (
                      <span
                        data-testid="collab-step-excerpt-note"
                        className="font-mono text-[10px] tracking-wider text-muted-foreground"
                      >
                        {step.excerpt_note}
                      </span>
                    ) : null}
                    {step.message_url ? (
                      <Link
                        data-testid="collab-step-link"
                        href={step.message_url}
                        className="font-mono text-[10px] tracking-wider underline underline-offset-4 hover:no-underline"
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
