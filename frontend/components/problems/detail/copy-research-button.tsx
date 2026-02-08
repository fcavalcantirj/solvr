"use client";

import { useState } from "react";
import { Copy, Check, Loader2, AlertCircle } from "lucide-react";
import { useToast } from "@/hooks/use-toast";
import { api } from "@/lib/api";

interface CopyResearchButtonProps {
  problemId: string;
  isClosed: boolean;
}

type ButtonState = "idle" | "loading" | "success" | "error";

export function CopyResearchButton({ problemId, isClosed }: CopyResearchButtonProps) {
  const [state, setState] = useState<ButtonState>("idle");
  const [hintPhase, setHintPhase] = useState<"none" | "first" | "second">("none");
  const { toast } = useToast();

  const handleCopy = async () => {
    if (isClosed || state === "loading") return;

    setState("loading");

    try {
      // Fresh fetch from API
      const response = await api.exportProblem(problemId);

      // Copy to clipboard
      await navigator.clipboard.writeText(response.markdown);

      setState("success");
      setHintPhase("first");
      toast({
        title: "Copied to clipboard",
        description: `~${response.token_estimate.toLocaleString()} tokens ready for research`,
      });

      // Reset button after 2 seconds
      setTimeout(() => setState("idle"), 2000);
      // Show second hint after 2.5 seconds
      setTimeout(() => setHintPhase("second"), 2500);
      // Hide all hints after 5 seconds
      setTimeout(() => setHintPhase("none"), 5000);
    } catch (error) {
      setState("error");
      toast({
        title: "Failed to copy",
        description: error instanceof Error ? error.message : "Unknown error",
        variant: "destructive",
      });
      // Reset after 3 seconds
      setTimeout(() => setState("idle"), 3000);
    }
  };

  const getIcon = () => {
    switch (state) {
      case "loading":
        return <Loader2 size={14} className="animate-spin" />;
      case "success":
        return <Check size={14} />;
      case "error":
        return <AlertCircle size={14} />;
      default:
        return <Copy size={14} />;
    }
  };

  const getLabel = () => {
    switch (state) {
      case "loading":
        return "COPYING...";
      case "success":
        return "COPIED!";
      case "error":
        return "FAILED";
      default:
        return "COPY FOR RESEARCH";
    }
  };

  return (
    <div className="flex items-center gap-3">
      <button
        onClick={handleCopy}
        disabled={isClosed || state === "loading"}
        className={`
          font-mono text-xs tracking-wider px-5 py-2.5
          flex items-center gap-2 transition-all
          ${isClosed
            ? "bg-muted text-muted-foreground cursor-not-allowed opacity-50"
            : state === "success"
              ? "bg-foreground text-background"
              : state === "error"
                ? "bg-red-600 text-white"
                : "bg-foreground text-background hover:opacity-80 hover:shadow-none border-2 border-foreground shadow-[2px_2px_0_0_hsl(var(--foreground))]"
          }
        `}
        title={isClosed ? "Cannot copy closed problems" : "Copy problem details for LLM research"}
      >
        {getIcon()}
        {getLabel()}
      </button>
      {hintPhase === "first" && !isClosed && (
        <span className="font-mono text-[11px] text-foreground/70 bg-secondary px-2 py-1 border border-border">
          ← tip: enable research mode on your LLM
        </span>
      )}
      {hintPhase === "second" && !isClosed && (
        <span className="font-mono text-[11px] text-green-900 bg-green-100 px-2 py-1 border border-green-300">
          ← ask LLM: "summarize without losing detail" and post here
        </span>
      )}
    </div>
  );
}
