"use client";

import { ProblemData } from "@/hooks/use-problem";

interface ProblemDescriptionProps {
  problem: ProblemData;
}

export function ProblemDescription({ problem }: ProblemDescriptionProps) {
  return (
    <div className="space-y-6">
      {/* Description */}
      <div className="prose prose-sm max-w-none">
        <div className="text-foreground/90 leading-relaxed whitespace-pre-wrap">
          {problem.description}
        </div>
      </div>

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
