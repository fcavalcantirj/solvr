"use client";

import type { ReactNode } from "react";
import { cn } from "@/lib/utils";
import { CopyPromptButton } from "./copy-prompt-button";
import { PromptSentence } from "./prompt-sentence";
import type { PromptPreset } from "./prompt-types";

type Variant = "connect" | "guide" | "card";

interface PromptProps {
  variant: Variant;
  preset: PromptPreset;
  // connect: the other use cases, held invisibly in the same place so switching
  // pairs swaps words without moving anything else on the page.
  others?: PromptPreset[];
  // card: its heading.
  title?: string;
  onIntentChange?: (intent: string) => void;
  onVisibilityToggle?: () => void;
  // Called once the clipboard took the sentence: the surface that shows this prompt
  // reports the copy from here (lib/funnel.ts), so a press that copied nothing is not one.
  onCopied?: () => void;
  intentLabel?: string;
  intentMaxChars?: number;
  // card: what sits beside the copy action (a link onward).
  footer?: ReactNode;
  // connect, guide: what sits under the copy action in the side column.
  aside?: ReactNode;
}

const SENTENCE: Record<Variant, string> = {
  connect:
    "text-[1.375rem] leading-[1.45] sm:text-[1.75rem] sm:leading-[1.4] lg:text-[2.5rem] lg:leading-[1.34]",
  guide:
    "text-[1.3125rem] leading-[1.45] sm:text-[1.625rem] sm:leading-[1.42] lg:text-[2.25rem] lg:leading-[1.36]",
  card: "text-[1.0625rem] leading-[1.7]",
};

// Prompt is the one prompt, set big. Every word is the API's; the page only gives
// the deciding words their look and copies the plain text.
export function Prompt({
  variant,
  preset,
  others = [],
  title,
  onIntentChange,
  onVisibilityToggle,
  onCopied,
  intentLabel,
  intentMaxChars,
  footer,
  aside,
}: PromptProps) {
  const editable = variant === "connect";

  const sentence = (item: PromptPreset, active: boolean) => (
    <p
      key={item.value}
      aria-hidden={active ? undefined : true}
      data-testid={active ? "prompt-sentence" : undefined}
      className={cn(
        "prompt-sentence font-light tracking-[-0.018em] text-foreground [grid-area:1/1] [text-wrap:pretty]",
        SENTENCE[variant],
        // Phones show only the sentence in use: holding the tallest one there
        // leaves a hole between the words and the Copy button.
        !active && "invisible max-lg:hidden",
      )}
    >
      <PromptSentence
        segments={item.prompt.segments}
        editable={editable && active}
        onIntentChange={onIntentChange}
        onVisibilityToggle={onVisibilityToggle}
        intentLabel={intentLabel}
        intentMaxChars={intentMaxChars}
      />
    </p>
  );

  if (variant === "card") {
    return (
      <article className="flex h-full flex-col border border-border bg-card p-6 sm:p-8">
        {title ? (
          <h3 className="font-mono text-xs uppercase tracking-[0.2em] text-foreground">{title}</h3>
        ) : null}
        <div className="mt-5 grid">{sentence(preset, true)}</div>
        <div className="mt-auto flex flex-wrap items-center justify-between gap-x-6 gap-y-3 pt-8">
          <CopyPromptButton text={preset.prompt.text} size="sm" onCopied={onCopied} />
          {footer}
        </div>
      </article>
    );
  }

  return (
    <div className="grid gap-10 lg:grid-cols-12 lg:gap-x-16">
      <div className="lg:col-span-8">
        <div className="grid">
          {sentence(preset, true)}
          {others.map((item) => sentence(item, false))}
        </div>
      </div>
      <div className="flex flex-col gap-6 lg:sticky lg:top-28 lg:col-span-4 lg:self-start lg:border-l lg:border-border lg:pl-10 lg:pt-1">
        {/* Every pair's line holds the same place, so the Copy button never moves. */}
        <div className="grid max-w-[34ch] text-base leading-relaxed text-muted-foreground lg:text-[1.0625rem]">
          <p className="[grid-area:1/1]">{preset.next}</p>
          {others.map((item) => (
            <p key={item.value} aria-hidden="true" className="invisible [grid-area:1/1] max-lg:hidden">
              {item.next}
            </p>
          ))}
        </div>
        <CopyPromptButton text={preset.prompt.text} size="lg" onCopied={onCopied} />
        <p className="font-mono text-[11px] uppercase tracking-[0.2em] text-muted-foreground">
          {preset.prompt.word_count} words, plain text
        </p>
        {aside}
      </div>
    </div>
  );
}
