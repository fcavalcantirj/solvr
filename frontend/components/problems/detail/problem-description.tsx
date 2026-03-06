"use client";

import { ProblemData } from "@/hooks/use-problem";
import { MarkdownContent } from "@/components/shared/markdown-content";

interface ProblemDescriptionProps {
  problem: ProblemData;
}

export function ProblemDescription({ problem }: ProblemDescriptionProps) {
  return (
    <div className="space-y-6">
      {/* Description */}
      <MarkdownContent content={problem.description} />

      {/* Tags */}
      {problem.tags.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {problem.tags.map((tag) => (
            <span
              key={tag}
              className="font-mono text-[10px] tracking-wider text-muted-foreground bg-secondary px-3 py-1.5 hover:text-foreground hover:bg-secondary transition-colors cursor-pointer"
            >
              {tag}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
