"use client";

import { PromptSentence } from "./prompt-sentence";
import { reserveAcross, sharedText } from "./prompt-align";
import type { PromptPreset } from "./prompt-types";

// PromptStack sets the use cases' sentences one under another. Every slot that
// differs reserves the width of its widest filling, so the rows wrap at the same
// places and line up word for word; the shared words step back, and what changes
// is what you read.
export function PromptStack({ presets }: { presets: PromptPreset[] }) {
  const prompts = presets.map((p) => p.prompt);
  const reserve = reserveAcross(prompts);
  const quiet = sharedText(prompts);

  return (
    <ol className="flex flex-col gap-9 lg:gap-12" data-testid="prompt-stack">
      {presets.map((preset) => (
        <li key={preset.value} className="grid gap-3 lg:grid-cols-12 lg:gap-x-10">
          <p className="font-mono text-xs uppercase tracking-[0.2em] text-foreground lg:col-span-3 lg:pt-[0.7rem]">
            {preset.label}
          </p>
          <p className="prompt-sentence text-[1.1875rem] font-light leading-[1.55] tracking-[-0.012em] text-foreground sm:text-[1.375rem] lg:col-span-9 lg:text-[1.75rem] lg:leading-[1.5]">
            <PromptSentence segments={preset.prompt.segments} reserve={reserve} quiet={quiet} />
          </p>
        </li>
      ))}
    </ol>
  );
}
