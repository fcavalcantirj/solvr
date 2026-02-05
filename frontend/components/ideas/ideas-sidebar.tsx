"use client";

import Link from "next/link";
import { Sparkles, TrendingUp, Users, Zap, ArrowRight, GitBranch, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useIdeasStats } from "@/hooks/use-ideas-stats";
import { formatRelativeTime } from "@/lib/api";

export function IdeasSidebar() {
  const { stats, loading, error } = useIdeasStats();

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (error || !stats) {
    return (
      <div className="bg-card border border-border p-5">
        <p className="font-mono text-xs text-muted-foreground">
          {error || "Failed to load stats"}
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Fresh Sparks */}
      {stats.freshSparks.length > 0 && (
        <div className="bg-card border border-border p-5">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
              FRESH SPARKS
            </h3>
            <Sparkles className="w-3 h-3 text-amber-500" />
          </div>
          <div className="space-y-3">
            {stats.freshSparks.map((idea) => (
              <Link key={idea.id} href={`/ideas/${idea.id}`} className="block group">
                <div className="flex items-start justify-between gap-2">
                  <span className="font-mono text-xs text-foreground group-hover:underline leading-tight">
                    {idea.title}
                  </span>
                  <div className="flex items-center gap-2 flex-shrink-0">
                    <span className="font-mono text-[10px] text-amber-600">+{idea.support}</span>
                    <span className="font-mono text-[10px] text-muted-foreground">
                      {formatRelativeTime(idea.createdAt)}
                    </span>
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
      )}

      {/* Ready to Develop */}
      {stats.readyToDevelop.length > 0 && (
        <div className="bg-card border border-border p-5">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
              READY TO DEVELOP
            </h3>
            <Zap className="w-3 h-3 text-blue-500" />
          </div>
          <div className="space-y-3">
            {stats.readyToDevelop.map((idea) => (
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
                          style={{ width: `${idea.validationScore}%` }}
                        />
                      </div>
                      <span className="font-mono text-[10px] text-emerald-600">
                        {idea.validationScore}%
                      </span>
                    </div>
                  </div>
                </div>
              </Link>
            ))}
          </div>
        </div>
      )}

      {/* Recently Realized */}
      {stats.recentlyRealized.length > 0 && (
        <div className="bg-card border border-border p-5">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
              RECENTLY REALIZED
            </h3>
            <GitBranch className="w-3 h-3 text-emerald-500" />
          </div>
          <div className="space-y-3">
            {stats.recentlyRealized.map((idea) => (
              <div key={idea.id}>
                <Link href={`/ideas/${idea.id}`} className="block group">
                  <span className="font-mono text-xs text-foreground group-hover:underline leading-tight">
                    {idea.title}
                  </span>
                </Link>
                {idea.evolvedInto && (
                  <div className="flex items-center gap-2 mt-1">
                    <ArrowRight className="w-2.5 h-2.5 text-muted-foreground" />
                    <Link
                      href={`/problems/${idea.evolvedInto}`}
                      className="font-mono text-[10px] text-emerald-600 hover:underline"
                    >
                      {idea.evolvedInto}
                    </Link>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Top Sparklers */}
      {stats.topSparklers.length > 0 && (
        <div className="bg-card border border-border p-5">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
              TOP SPARKLERS
            </h3>
            <Users className="w-3 h-3 text-muted-foreground" />
          </div>
          <div className="space-y-3">
            {stats.topSparklers.map((sparkler, idx) => (
              <div key={sparkler.id} className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="font-mono text-[10px] text-muted-foreground w-4">
                    {idx + 1}.
                  </span>
                  <div
                    className={cn(
                      "w-6 h-6 flex items-center justify-center font-mono text-[9px] font-bold",
                      sparkler.type === "agent"
                        ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
                        : "bg-foreground text-background"
                    )}
                  >
                    {sparkler.type === "agent" ? "AI" : sparkler.name.slice(0, 2).toUpperCase()}
                  </div>
                  <span className="font-mono text-xs">{sparkler.name}</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="font-mono text-[10px] text-muted-foreground">
                    {sparkler.ideasCount} ideas
                  </span>
                  <span className="font-mono text-[10px] text-emerald-600">
                    {sparkler.realizedCount} realized
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Trending Tags */}
      {stats.trendingTags.length > 0 && (
        <div className="bg-card border border-border p-5">
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
              TRENDING TAGS
            </h3>
            <TrendingUp className="w-3 h-3 text-muted-foreground" />
          </div>
          <div className="space-y-2">
            {stats.trendingTags.map((item) => (
              <div key={item.name} className="flex items-center justify-between">
                <span className="font-mono text-xs text-foreground hover:underline cursor-pointer">
                  #{item.name}
                </span>
                <div className="flex items-center gap-2">
                  <span className="font-mono text-[10px] text-muted-foreground">
                    {item.count}
                  </span>
                  {item.growth > 0 && (
                    <span className="font-mono text-[10px] text-emerald-600">
                      +{item.growth}%
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Idea Pipeline Stats */}
      <div className="bg-foreground text-background p-5">
        <h3 className="font-mono text-xs tracking-wider text-background/60 mb-4">
          IDEA PIPELINE
        </h3>
        <div className="space-y-3">
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="font-mono text-[10px] text-background/60">SPARK → DEVELOPING</span>
              <span className="font-mono text-xs">{stats.pipelineStats.sparkToDeveloping}%</span>
            </div>
            <div className="h-1 bg-background/20 overflow-hidden">
              <div className="h-full bg-amber-500" style={{ width: `${stats.pipelineStats.sparkToDeveloping}%` }} />
            </div>
          </div>
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="font-mono text-[10px] text-background/60">DEVELOPING → MATURE</span>
              <span className="font-mono text-xs">{stats.pipelineStats.developingToMature}%</span>
            </div>
            <div className="h-1 bg-background/20 overflow-hidden">
              <div className="h-full bg-blue-500" style={{ width: `${stats.pipelineStats.developingToMature}%` }} />
            </div>
          </div>
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="font-mono text-[10px] text-background/60">MATURE → REALIZED</span>
              <span className="font-mono text-xs">{stats.pipelineStats.matureToRealized}%</span>
            </div>
            <div className="h-1 bg-background/20 overflow-hidden">
              <div className="h-full bg-emerald-500" style={{ width: `${stats.pipelineStats.matureToRealized}%` }} />
            </div>
          </div>
        </div>
        <p className="font-mono text-[10px] text-background/40 mt-4">
          AVG TIME TO REALIZATION: {stats.pipelineStats.avgDaysToRealization} DAYS
        </p>
      </div>
    </div>
  );
}
