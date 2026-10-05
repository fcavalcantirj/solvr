"use client";

import { useEffect, useState } from "react";
import { Check, Copy } from "lucide-react";
import { cn } from "@/lib/utils";

interface CopyPromptButtonProps {
  // Exactly what the agent receives: the API's prompt.text.
  text: string;
  size?: "lg" | "sm";
  // Called once the clipboard took the text, never for a press that copied nothing.
  onCopied?: () => void;
  className?: string;
}

// CopyPromptButton copies the plain prompt text. A blocked clipboard says how to
// copy by hand instead of pretending it worked. It carries no click mark: the copy is
// reported by onCopied (prompt_copy, SPEC.md 27.7), one event for one action.
export function CopyPromptButton({ text, size = "lg", onCopied, className }: CopyPromptButtonProps) {
  const [state, setState] = useState<"idle" | "copied" | "failed">("idle");

  // A new prompt is not the one that was copied.
  useEffect(() => setState("idle"), [text]);

  useEffect(() => {
    if (state !== "copied") return;
    const timer = setTimeout(() => setState("idle"), 2400);
    return () => clearTimeout(timer);
  }, [state]);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setState("copied");
      onCopied?.();
    } catch {
      setState("failed");
    }
  };

  const large = size === "lg";
  return (
    <div className={cn("flex flex-col gap-2", className)}>
      <button
        type="button"
        onClick={copy}
        className={cn(
          "group inline-flex cursor-pointer items-center justify-center gap-3 font-mono uppercase transition-colors focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-foreground",
          large
            ? "border border-foreground w-full bg-foreground px-8 py-5 text-sm tracking-[0.2em] text-background hover:bg-background hover:text-foreground transition-colors"
            : "border border-foreground px-4 py-2.5 text-[11px] tracking-[0.18em] text-foreground hover:bg-foreground hover:text-background",
        )}
      >
        {state === "copied" ? (
          <Check aria-hidden="true" className={large ? "h-4 w-4" : "h-3.5 w-3.5"} />
        ) : (
          <Copy aria-hidden="true" className={large ? "h-4 w-4" : "h-3.5 w-3.5"} />
        )}
        <span>{state === "copied" ? "Copied" : "Copy prompt"}</span>
      </button>
      <span role="status" className="sr-only">
        {state === "copied" ? "Prompt copied" : ""}
      </span>
      {state === "failed" ? (
        <p role="alert" className="text-sm leading-relaxed text-muted-foreground">
          Your browser blocked the clipboard. Select the sentence and copy it by hand.
        </p>
      ) : null}
    </div>
  );
}
