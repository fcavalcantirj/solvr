"use client";

import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { track } from "@/lib/analytics";
import { cn } from "@/lib/utils";

// What a Copy button says about a copy (SPEC.md 27.7: code_copy): the page family it sits
// on, and a stable lowercase id for WHAT was copied. Never the text itself.
export interface CopyReport {
  surface: "skill" | "mcp" | "api_docs" | "how_it_works" | "amcp" | "ipfs";
  item: string;
}

// The one copy control of the marketing pages: a caption-sized word beside the code it
// copies, never an unlabeled icon. On an ink tile it is drawn in the paper colour.
// `report` is required, so a Copy button cannot be added that nobody hears about.
export function CopyButton({
  text,
  report,
  onInk = false,
  className,
}: {
  text: string;
  report: CopyReport;
  onInk?: boolean;
  className?: string;
}) {
  const [copied, setCopied] = useState(false);

  // "Copied" is said, and the copy reported, only once the clipboard took the text: a
  // browser can refuse the write, or have no clipboard at all, and neither may claim
  // otherwise.
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      return;
    }
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
    track("code_copy", { surface: report.surface, item: report.item });
  };

  return (
    <button
      type="button"
      onClick={copy}
      className={cn(
        "inline-flex min-h-10 shrink-0 cursor-pointer items-center gap-2 px-1 font-mono text-[11px] uppercase tracking-[0.18em] transition-colors",
        "focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2",
        onInk
          ? "text-background/70 hover:text-background focus-visible:outline-background"
          : "text-muted-foreground hover:text-foreground focus-visible:outline-foreground",
        className,
      )}
    >
      {copied ? <Check aria-hidden="true" size={14} /> : <Copy aria-hidden="true" size={14} />}
      <span aria-live="polite">{copied ? "Copied" : "Copy"}</span>
    </button>
  );
}
