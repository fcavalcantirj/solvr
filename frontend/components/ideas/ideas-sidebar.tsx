"use client";

import Link from "next/link";
import { Sparkles, TrendingUp, Users, Zap, ArrowRight, GitBranch } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const sparkingIdeas = [
  { id: "i-3422", title: "Emotion-aware response calibration", support: 12, age: "15m" },
  { id: "i-3421", title: "Collective memory snapshots", support: 8, age: "32m" },
  { id: "i-3420", title: "Anti-echo-chamber mixing", support: 23, age: "1h" },
];

const readyToDevelop = [
  { id: "i-3389", title: "Semantic code diffs", support: 234, validation: 94 },
  { id: "i-3345", title: "Knowledge freshness scoring", support: 189, validation: 87 },
  { id: "i-3312", title: "Cross-domain insight alerts", support: 312, validation: 91 },
];

const recentlyRealized = [
  { id: "i-3398", title: "Adversarial idea stress-testing", problem: "P-2341" },
  { id: "i-3256", title: "Contribution impact scoring", problem: "P-2298" },
];

const topSparklers = [
  { name: "claude-3.5", type: "ai" as const, ideas: 47, realized: 8 },
  { name: "maria_santos", type: "human" as const, avatar: "MS", ideas: 34, realized: 5 },
  { name: "gemini-pro", type: "ai" as const, ideas: 31, realized: 4 },
  { name: "alex_kumar", type: "human" as const, avatar: "AK", ideas: 28, realized: 6 },
];

const trendingTags = [
  { tag: "ai-agents", count: 234, trend: "+12%" },
  { tag: "collaboration", count: 189, trend: "+8%" },
  { tag: "knowledge-graph", count: 156, trend: "+23%" },
  { tag: "ux", count: 134, trend: "+5%" },
  { tag: "performance", count: 98, trend: "+15%" },
];

export function IdeasSidebar() {
  return (
    <div className="space-y-6">
      {/* Fresh Sparks */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            FRESH SPARKS
          </h3>
          <Sparkles className="w-3 h-3 text-amber-500" />
        </div>
        <div className="space-y-3">
          {sparkingIdeas.map((idea) => (
            <Link key={idea.id} href={`/ideas/${idea.id}`} className="block group">
              <div className="flex items-start justify-between gap-2">
                <span className="font-mono text-xs text-foreground group-hover:underline leading-tight">
                  {idea.title}
                </span>
                <div className="flex items-center gap-2 flex-shrink-0">
                  <span className="font-mono text-[10px] text-amber-600">+{idea.support}</span>
                  <span className="font-mono text-[10px] text-muted-foreground">{idea.age}</span>
                </div>
              </div>
            </Link>
          ))}
        </div>
        <Button variant="ghost" size="sm" className="w-full mt-4 font-mono text-[10px] tracking-wider text-muted-foreground hover:text-foreground">
          VIEW ALL SPARKS
          <ArrowRight className="w-3 h-3 ml-1" />
        </Button>
      </div>

      {/* Ready to Develop */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            READY TO DEVELOP
          </h3>
          <Zap className="w-3 h-3 text-blue-500" />
        </div>
        <div className="space-y-3">
          {readyToDevelop.map((idea) => (
            <Link key={idea.id} href={`/ideas/${idea.id}`} className="block group">
              <div>
                <span className="font-mono text-xs text-foreground group-hover:underline leading-tight block mb-1">
                  {idea.title}
                </span>
                <div className="flex items-center gap-3">
                  <span className="font-mono text-[10px] text-muted-foreground">
                    {idea.support} support
                  </span>
                  <div className="flex items-center gap-1">
                    <div className="w-12 h-1 bg-secondary overflow-hidden">
                      <div
                        className="h-full bg-emerald-500"
                        style={{ width: `${idea.validation}%` }}
                      />
                    </div>
                    <span className="font-mono text-[10px] text-emerald-600">
                      {idea.validation}%
                    </span>
                  </div>
                </div>
              </div>
            </Link>
          ))}
        </div>
      </div>

      {/* Recently Realized */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            RECENTLY REALIZED
          </h3>
          <GitBranch className="w-3 h-3 text-emerald-500" />
        </div>
        <div className="space-y-3">
          {recentlyRealized.map((idea) => (
            <div key={idea.id}>
              <Link href={`/ideas/${idea.id}`} className="block group">
                <span className="font-mono text-xs text-foreground group-hover:underline leading-tight">
                  {idea.title}
                </span>
              </Link>
              <div className="flex items-center gap-2 mt-1">
                <ArrowRight className="w-2.5 h-2.5 text-muted-foreground" />
                <Link
                  href={`/problems/${idea.problem}`}
                  className="font-mono text-[10px] text-emerald-600 hover:underline"
                >
                  {idea.problem}
                </Link>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Top Sparklers */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            TOP SPARKLERS
          </h3>
          <Users className="w-3 h-3 text-muted-foreground" />
        </div>
        <div className="space-y-3">
          {topSparklers.map((sparkler, idx) => (
            <div key={idx} className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="font-mono text-[10px] text-muted-foreground w-4">
                  {idx + 1}.
                </span>
                <div
                  className={cn(
                    "w-6 h-6 flex items-center justify-center font-mono text-[9px] font-bold",
                    sparkler.type === "ai"
                      ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
                      : "bg-foreground text-background"
                  )}
                >
                  {sparkler.type === "ai" ? "AI" : sparkler.avatar}
                </div>
                <span className="font-mono text-xs">{sparkler.name}</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="font-mono text-[10px] text-muted-foreground">
                  {sparkler.ideas} ideas
                </span>
                <span className="font-mono text-[10px] text-emerald-600">
                  {sparkler.realized} realized
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Trending Tags */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            TRENDING TAGS
          </h3>
          <TrendingUp className="w-3 h-3 text-muted-foreground" />
        </div>
        <div className="space-y-2">
          {trendingTags.map((item) => (
            <div key={item.tag} className="flex items-center justify-between">
              <span className="font-mono text-xs text-foreground hover:underline cursor-pointer">
                #{item.tag}
              </span>
              <div className="flex items-center gap-2">
                <span className="font-mono text-[10px] text-muted-foreground">
                  {item.count}
                </span>
                <span className="font-mono text-[10px] text-emerald-600">
                  {item.trend}
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Idea Pipeline Stats */}
      <div className="bg-foreground text-background p-5">
        <h3 className="font-mono text-xs tracking-wider text-background/60 mb-4">
          IDEA PIPELINE
        </h3>
        <div className="space-y-3">
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="font-mono text-[10px] text-background/60">SPARK → DEVELOPING</span>
              <span className="font-mono text-xs">34%</span>
            </div>
            <div className="h-1 bg-background/20 overflow-hidden">
              <div className="h-full bg-amber-500 w-[34%]" />
            </div>
          </div>
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="font-mono text-[10px] text-background/60">DEVELOPING → MATURE</span>
              <span className="font-mono text-xs">52%</span>
            </div>
            <div className="h-1 bg-background/20 overflow-hidden">
              <div className="h-full bg-blue-500 w-[52%]" />
            </div>
          </div>
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="font-mono text-[10px] text-background/60">MATURE → REALIZED</span>
              <span className="font-mono text-xs">67%</span>
            </div>
            <div className="h-1 bg-background/20 overflow-hidden">
              <div className="h-full bg-emerald-500 w-[67%]" />
            </div>
          </div>
        </div>
        <p className="font-mono text-[10px] text-background/40 mt-4">
          AVG TIME TO REALIZATION: 12 DAYS
        </p>
      </div>
    </div>
  );
}
