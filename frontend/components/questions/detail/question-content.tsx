"use client";

import { ThumbsUp, ThumbsDown, Flag } from "lucide-react";
import { Button } from "@/components/ui/button";

export function QuestionContent() {
  return (
    <div className="bg-card border border-border p-8">
      <div className="flex gap-6">
        <div className="flex flex-col items-center gap-2">
          <Button variant="ghost" size="icon" className="h-8 w-8 hover:bg-emerald-500/10 hover:text-emerald-600">
            <ThumbsUp className="w-4 h-4" />
          </Button>
          <span className="font-mono text-sm font-medium">47</span>
          <Button variant="ghost" size="icon" className="h-8 w-8 hover:bg-red-500/10 hover:text-red-600">
            <ThumbsDown className="w-4 h-4" />
          </Button>
        </div>

        <div className="flex-1 space-y-6">
          <div className="prose prose-sm max-w-none">
            <p className="text-foreground leading-relaxed">
              I{"'"}m working on integrating multiple AI agents into a real-time collaborative document editing system. The challenge is understanding how transformer context windows behave when:
            </p>

            <ol className="list-decimal list-inside space-y-2 text-foreground mt-4">
              <li>Multiple users are editing simultaneously</li>
              <li>The document exceeds typical context limits (128K+ tokens)</li>
              <li>AI needs to maintain coherent understanding across edit sessions</li>
            </ol>

            <p className="text-foreground leading-relaxed mt-4">
              Specifically, I{"'"}m trying to understand:
            </p>

            <div className="bg-secondary/50 border border-border p-4 mt-4 font-mono text-sm">
              <p className="text-foreground">1. How do sliding window approaches compare to sparse attention for this use case?</p>
              <p className="text-foreground mt-2">2. What{"'"}s the latency/quality tradeoff when chunking documents for parallel processing?</p>
              <p className="text-foreground mt-2">3. Are there emerging architectures specifically designed for collaborative scenarios?</p>
            </div>

            <p className="text-foreground leading-relaxed mt-4">
              I{"'"}ve reviewed the literature on Longformer and BigBird but would appreciate practical insights from anyone who has implemented similar systems.
            </p>
          </div>

          <div className="flex flex-wrap gap-2 pt-4 border-t border-border">
            {["transformers", "context-windows", "real-time", "collaborative-editing", "architecture"].map((tag) => (
              <span
                key={tag}
                className="px-2 py-1 bg-secondary text-foreground font-mono text-[10px] tracking-wider border border-border hover:border-foreground/30 cursor-pointer transition-colors"
              >
                {tag}
              </span>
            ))}
          </div>

          <div className="flex items-center justify-between pt-4">
            <div className="flex items-center gap-4">
              <Button variant="ghost" size="sm" className="font-mono text-xs text-muted-foreground hover:text-foreground">
                <Flag className="w-3 h-3 mr-2" />
                FLAG
              </Button>
            </div>

            <div className="flex items-center gap-2 text-xs text-muted-foreground font-mono">
              <span>edited 4h ago</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
