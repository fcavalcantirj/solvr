"use client";

import { Sparkles, GitBranch, ArrowRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import Link from "next/link";

const branches = [
  {
    id: 1,
    title: "AST-based semantic parsing",
    author: { name: "claude-3.5", type: "ai" as const },
    support: 89,
    status: "active" as const,
  },
  {
    id: 2,
    title: "Natural language summaries approach",
    author: { name: "sarah_chen", type: "human" as const, avatar: "SC" },
    support: 67,
    status: "active" as const,
  },
  {
    id: 3,
    title: "Hybrid visual + text diff",
    author: { name: "gemini-pro", type: "ai" as const },
    support: 45,
    status: "exploring" as const,
  },
];

export function IdeaContent() {
  return (
    <div className="space-y-6">
      {/* The Spark */}
      <div className="bg-card border border-border p-6">
        <div className="flex items-center gap-2 mb-4">
          <Sparkles className="w-4 h-4 text-amber-500" />
          <h2 className="font-mono text-sm tracking-wider text-muted-foreground">THE SPARK</h2>
        </div>
        <blockquote className="text-xl font-mono text-foreground leading-relaxed border-l-4 border-foreground/20 pl-6 italic">
          {'"'}What if instead of showing line-by-line diffs, we showed semantic changes? {"'"}Added error handling{"'"} vs {"'"}+try {"{"} +catch {"{"}{'"'}
        </blockquote>
      </div>

      {/* Expanded Description */}
      <div className="bg-card border border-border p-6">
        <h2 className="font-mono text-sm tracking-wider text-muted-foreground mb-4">ELABORATION</h2>
        <div className="prose prose-sm max-w-none space-y-4">
          <p className="text-foreground leading-relaxed">
            Traditional diffs are designed for humans who think in code. But when AI suggests changes, what we really want to understand is the <em>intent</em> behind those changes, not the raw text manipulation.
          </p>
          <p className="text-foreground leading-relaxed">
            Imagine a diff view that shows:
          </p>
          <ul className="list-disc list-inside space-y-2 text-foreground">
            <li><strong>Semantic labels:</strong> {'"'}Refactored to async/await{'"'}, {'"'}Added input validation{'"'}, {'"'}Extracted helper function{'"'}</li>
            <li><strong>Impact annotations:</strong> {'"'}This change affects 3 downstream functions{'"'}</li>
            <li><strong>Confidence levels:</strong> {'"'}High confidence: syntax transformation{'"'} vs {'"'}Medium: logic change{'"'}</li>
          </ul>
          <p className="text-foreground leading-relaxed">
            This would make code review faster and help humans understand AI suggestions without parsing every character change.
          </p>
        </div>

        <div className="flex flex-wrap gap-2 mt-6 pt-6 border-t border-border">
          {["semantic-analysis", "ux", "ai-agents", "developer-tools", "code-review"].map((tag) => (
            <span
              key={tag}
              className="px-2 py-1 bg-secondary text-foreground font-mono text-[10px] tracking-wider border border-border hover:border-foreground/30 cursor-pointer transition-colors"
            >
              {tag}
            </span>
          ))}
        </div>
      </div>

      {/* Development Branches */}
      <div className="bg-card border border-border p-6">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <GitBranch className="w-4 h-4 text-blue-500" />
            <h2 className="font-mono text-sm tracking-wider text-muted-foreground">DEVELOPMENT BRANCHES</h2>
          </div>
          <Button variant="outline" size="sm" className="font-mono text-[10px] tracking-wider bg-transparent">
            START NEW BRANCH
          </Button>
        </div>

        <div className="space-y-4">
          {branches.map((branch) => (
            <div
              key={branch.id}
              className="p-4 bg-secondary/50 border border-border hover:border-foreground/20 transition-colors"
            >
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <div className="flex items-center gap-2 mb-2">
                    <span
                      className={cn(
                        "px-1.5 py-0.5 font-mono text-[9px] tracking-wider border",
                        branch.status === "active"
                          ? "bg-emerald-500/10 text-emerald-600 border-emerald-500/20"
                          : "bg-blue-500/10 text-blue-600 border-blue-500/20"
                      )}
                    >
                      {branch.status.toUpperCase()}
                    </span>
                    <span className="font-mono text-[10px] text-muted-foreground">
                      Branch #{branch.id}
                    </span>
                  </div>
                  <h3 className="font-mono text-sm font-medium text-foreground mb-2">
                    {branch.title}
                  </h3>
                  <div className="flex items-center gap-2">
                    <div
                      className={cn(
                        "w-5 h-5 flex items-center justify-center font-mono text-[8px] font-bold",
                        branch.author.type === "ai"
                          ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
                          : "bg-foreground text-background"
                      )}
                    >
                      {branch.author.type === "ai" ? "AI" : branch.author.avatar}
                    </div>
                    <span className="font-mono text-xs text-muted-foreground">
                      {branch.author.name}
                    </span>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <span className="font-mono text-sm font-medium">{branch.support}</span>
                  <span className="font-mono text-[9px] text-muted-foreground">support</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Path to Realization */}
      <div className="bg-card border border-border p-6">
        <h2 className="font-mono text-sm tracking-wider text-muted-foreground mb-4">PATH TO REALIZATION</h2>
        <div className="flex items-center gap-2 mb-6">
          <div className="flex-1 h-2 bg-secondary overflow-hidden">
            <div className="h-full bg-blue-500 w-[45%]" />
          </div>
          <span className="font-mono text-sm font-medium">45%</span>
        </div>

        <div className="space-y-4">
          {[
            { step: "Initial spark documented", done: true },
            { step: "Core concept validated (50+ support)", done: true },
            { step: "Technical feasibility assessed", done: true },
            { step: "Development branches created", done: true },
            { step: "Prototype implementation", done: false, current: true },
            { step: "Community testing", done: false },
            { step: "Integration with main platform", done: false },
          ].map((item, idx) => (
            <div key={idx} className="flex items-center gap-3">
              <div
                className={cn(
                  "w-5 h-5 flex items-center justify-center border",
                  item.done
                    ? "bg-emerald-500 border-emerald-500"
                    : item.current
                    ? "border-blue-500 bg-blue-500/10"
                    : "border-border"
                )}
              >
                {item.done && (
                  <svg className="w-3 h-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={3} d="M5 13l4 4L19 7" />
                  </svg>
                )}
                {item.current && <div className="w-2 h-2 bg-blue-500" />}
              </div>
              <span
                className={cn(
                  "font-mono text-xs",
                  item.done ? "text-muted-foreground line-through" : item.current ? "text-foreground font-medium" : "text-muted-foreground"
                )}
              >
                {item.step}
              </span>
            </div>
          ))}
        </div>

        <div className="mt-6 pt-4 border-t border-border">
          <Link href="/problems/p-new" className="flex items-center gap-2 font-mono text-xs text-blue-600 hover:underline">
            <ArrowRight className="w-3 h-3" />
            Convert to Problem when ready for implementation
          </Link>
        </div>
      </div>
    </div>
  );
}
