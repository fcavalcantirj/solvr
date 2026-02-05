"use client";

import { Bot, User, Clock, GitBranch } from "lucide-react";
import { ProblemData } from "@/hooks/use-problem";

interface ProblemSidePanelProps {
  problem: ProblemData;
  approachesCount: number;
}

export function ProblemSidePanel({ problem, approachesCount }: ProblemSidePanelProps) {
  return (
    <aside className="space-y-6 lg:sticky lg:top-6 lg:self-start">
      {/* Quick Stats */}
      <div className="grid grid-cols-2 gap-3">
        <div className="border border-border bg-card p-4">
          <div className="flex items-center gap-2 text-muted-foreground mb-1">
            <GitBranch size={12} />
            <span className="font-mono text-[10px] tracking-wider">APPROACHES</span>
          </div>
          <p className="text-2xl font-light">{approachesCount}</p>
        </div>
        <div className="border border-border bg-card p-4">
          <div className="flex items-center gap-2 text-muted-foreground mb-1">
            <Clock size={12} />
            <span className="font-mono text-[10px] tracking-wider">POSTED</span>
          </div>
          <p className="text-lg font-light">{problem.time}</p>
        </div>
      </div>

      {/* Author */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border">
          <h3 className="font-mono text-xs tracking-wider">POSTED BY</h3>
        </div>
        <div className="p-4">
          <div className="flex items-center gap-3">
            <div
              className={`w-8 h-8 flex items-center justify-center ${
                problem.author.type === "human"
                  ? "bg-foreground text-background"
                  : "border border-foreground"
              }`}
            >
              {problem.author.type === "human" ? (
                <User size={14} />
              ) : (
                <Bot size={14} />
              )}
            </div>
            <div>
              <p className="font-mono text-xs tracking-wider">{problem.author.displayName}</p>
              <p className="font-mono text-[10px] text-muted-foreground">
                {problem.author.type === "human" ? "Human" : "AI Agent"}
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Tags */}
      {problem.tags.length > 0 && (
        <div className="border border-border bg-card">
          <div className="p-4 border-b border-border">
            <h3 className="font-mono text-xs tracking-wider">TAGS</h3>
          </div>
          <div className="p-4">
            <div className="flex flex-wrap gap-2">
              {problem.tags.map((tag) => (
                <span
                  key={tag}
                  className="font-mono text-[10px] tracking-wider text-muted-foreground bg-secondary px-2 py-1"
                >
                  {tag}
                </span>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Help CTA */}
      <div className="border border-foreground bg-foreground text-background p-5">
        <h3 className="font-mono text-xs tracking-wider mb-2">HAVE AN IDEA?</h3>
        <p className="text-sm text-background/70 mb-4 leading-relaxed">
          Even partial solutions help. Start an approach and document your thinking.
        </p>
        <button className="w-full font-mono text-[10px] tracking-wider border border-background px-4 py-2.5 hover:bg-background hover:text-foreground transition-colors">
          START AN APPROACH
        </button>
      </div>
    </aside>
  );
}
