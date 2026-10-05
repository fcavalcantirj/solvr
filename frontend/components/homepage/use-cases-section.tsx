"use client";

import Link from 'next/link';
import { ArrowRight } from 'lucide-react';

import { Prompt } from '@/components/prompt/prompt';
import { roles } from '@/components/prompt/prompt-align';
import { api } from '@/lib/api';
import { guideForPreset } from '@/lib/docs/use-cases';
import type { APIConnectPreset } from '@/lib/api-types';
import { trackCta } from '@/lib/track-attrs';

// starter_prompt_copied: a card's sentence was copied. Reported ONLY after the clipboard
// write succeeded, for the card's use case and the agent its sentence is for. The
// sentence itself is never sent.
function reportCopied(example: APIConnectPreset) {
  void api.postFunnelEvent?.({
    event: 'starter_prompt_copied',
    entry_surface: 'homepage_use_cases',
    preset: example.value,
    role: roles(example.prompt).a?.toLowerCase(),
  });
}

// Right under the hero: three ways to put two agents to work, each the API's example
// sentence (GET /v1/connect/examples, read on the server) in the compact Prompt. The
// sentence is the same for all three; the marked words change. Each card copies its
// sentence, opens /connect with its use case chosen, and links the guide that was tested.
// Without the examples (the API could not be read) the cards keep their way onward.
export function UseCasesSection({ examples }: { examples?: APIConnectPreset[] | null }) {
  return (
    <section
      data-testid="use-cases-section"
      aria-labelledby="use-cases-heading"
      className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border"
    >
      <div className="max-w-7xl mx-auto">
        <h2 id="use-cases-heading" className="max-w-[30ch] text-3xl md:text-4xl font-light tracking-tight mb-8">
          One sentence, three ways to put two agents to work.
        </h2>
        {examples && examples.length > 0 ? (
          <div className="grid gap-4 lg:grid-cols-3 lg:gap-5">
            {examples.map((example) => {
              const guide = guideForPreset(example.value);
              return (
                <div key={example.value} data-testid="use-case-card">
                  <Prompt
                    variant="card"
                    title={example.label}
                    preset={example}
                    onCopied={() => reportCopied(example)}
                    copyTrack={trackCta('copy_prompt', 'page')}
                    footer={<CardLinks preset={example.value} guideSlug={guide?.slug} />}
                  />
                </div>
              );
            })}
          </div>
        ) : (
          <p className="text-muted-foreground">
            <Link href="/connect" {...trackCta('connect_agents', 'page')} className="text-foreground underline underline-offset-4">
              Copy the sentence at Connect
            </Link>
            .
          </p>
        )}
      </div>
    </section>
  );
}

function CardLinks({ preset, guideSlug }: { preset: string; guideSlug?: string }) {
  const link =
    'group inline-flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground transition-colors hover:text-foreground';
  return (
    <span className="flex flex-wrap items-center gap-x-5 gap-y-2">
      <Link href={`/connect?preset=${preset}`} {...trackCta('make_it_yours', 'page')} className={link}>
        Make it yours
        <ArrowRight aria-hidden="true" className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
      </Link>
      {guideSlug ? (
        <Link href={`/docs/guides/${guideSlug}`} {...trackCta('guide', 'page')} className={link}>
          Guide
        </Link>
      ) : null}
    </span>
  );
}
