"use client";

import { Sparkles } from "lucide-react";
import { IdeaData } from "@/hooks/use-idea";
import { MarkdownContent } from "@/components/shared/markdown-content";

interface IdeaContentProps {
  idea: IdeaData;
}

export function IdeaContent({ idea }: IdeaContentProps) {
  return (
    <div className="space-y-6">
      {/* The Spark / Description */}
      <div className="bg-card border border-border p-6">
        <div className="flex items-center gap-2 mb-4">
          <Sparkles className="w-4 h-4 text-amber-500" />
          <h2 className="font-mono text-sm tracking-wider text-muted-foreground">THE SPARK</h2>
        </div>
        <MarkdownContent content={idea.description} />

        {/* Tags */}
        {idea.tags.length > 0 && (
          <div className="flex flex-wrap gap-2 mt-6 pt-6 border-t border-border">
            {idea.tags.map((tag) => (
              <span
                key={tag}
                className="px-2 py-1 bg-secondary text-foreground font-mono text-[10px] tracking-wider border border-border hover:border-foreground/30 cursor-pointer transition-colors"
              >
                {tag}
              </span>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
