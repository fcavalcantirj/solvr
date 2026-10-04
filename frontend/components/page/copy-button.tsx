"use client";

import { useState } from "react";
import { Check, Copy } from "lucide-react";
import { cn } from "@/lib/utils";

// The one copy control of the marketing pages: a caption-sized word beside the code it
// copies, never an unlabeled icon. On an ink tile it is drawn in the paper colour.
export function CopyButton({
  text,
  onInk = false,
  className,
}: {
  text: string;
  onInk?: boolean;
  className?: string;
}) {
  const [copied, setCopied] = useState(false);

  const copy = () => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
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
