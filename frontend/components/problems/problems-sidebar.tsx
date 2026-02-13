"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { Bot, User, Trophy, AlertCircle, CheckCircle2, GitBranch } from "lucide-react";
import { useProblemsStats } from "@/hooks/use-problems-stats";
import { useTrending } from "@/hooks/use-stats";
import { api, APIFeedItem } from "@/lib/api";

function formatNumber(n: number): string {
  if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k';
  return n.toLocaleString();
}

function daysSince(dateStr: string): number {
  const diff = Date.now() - new Date(dateStr).getTime();
  return Math.floor(diff / (1000 * 60 * 60 * 24));
}

function formatSolveTime(days: number): string {
  if (days < 1) return '<1d';
  return `${days}d`;
}

interface ProblemsSidebarProps {
  onTagClick?: (tag: string) => void;
}

export function ProblemsSidebar({ onTagClick }: ProblemsSidebarProps) {
  const { stats: problemsStats, loading: statsLoading } = useProblemsStats();
  const { trending, loading: trendingLoading } = useTrending();
  const [stuckProblems, setStuckProblems] = useState<APIFeedItem[]>([]);
  const [stuckLoading, setStuckLoading] = useState(true);

  useEffect(() => {
    api.getStuckProblems({ page: 1, per_page: 3 })
      .then(res => setStuckProblems(res.data))
      .catch(() => setStuckProblems([]))
      .finally(() => setStuckLoading(false));
  }, []);

  const statsItems = [
    { label: "TOTAL PROBLEMS", value: statsLoading ? "—" : formatNumber(problemsStats?.total_problems ?? 0) },
    { label: "SOLVED", value: statsLoading ? "—" : formatNumber(problemsStats?.solved_count ?? 0) },
    { label: "ACTIVE APPROACHES", value: statsLoading ? "—" : formatNumber(problemsStats?.active_approaches ?? 0) },
    { label: "AVG. SOLVE TIME", value: statsLoading ? "—" : `${problemsStats?.avg_solve_time_days ?? 0}d` },
  ];

  return (
    <aside className="space-y-6 lg:sticky lg:top-6 lg:self-start">
      {/* Stats Grid */}
      <div className="grid grid-cols-2 gap-3">
        {statsItems.map((stat) => (
          <div key={stat.label} className="border border-border bg-card p-4">
            <p className="font-mono text-[10px] tracking-wider text-muted-foreground mb-1">
              {stat.label}
            </p>
            <p className="text-2xl font-light tracking-tight">{stat.value}</p>
          </div>
        ))}
      </div>

      {/* Stuck Problems */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border flex items-center gap-2">
          <AlertCircle size={14} className="text-foreground" />
          <h3 className="font-mono text-xs tracking-wider">NEEDS FRESH EYES</h3>
        </div>
        <div className="divide-y divide-border">
          {stuckLoading ? (
            <div className="p-4 text-center">
              <span className="font-mono text-[10px] text-muted-foreground">Loading...</span>
            </div>
          ) : stuckProblems.length === 0 ? (
            <div className="p-4 text-center">
              <span className="font-mono text-[10px] text-muted-foreground">No stuck problems</span>
            </div>
          ) : (
            stuckProblems.map((problem) => (
              <Link key={problem.id} href={`/problems/${problem.id}`} className="block p-4 hover:bg-secondary/50 transition-colors">
                <p className="text-sm font-light leading-snug mb-2 line-clamp-2">
                  {problem.title}
                </p>
                <div className="flex items-center gap-3">
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground flex items-center gap-1">
                    <GitBranch size={10} />
                    {problem.approach_count || 0} attempts
                  </span>
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                    {daysSince(problem.created_at)}d stuck
                  </span>
                </div>
              </Link>
            ))
          )}
        </div>
        <div className="p-3 border-t border-border">
          <Link href="/problems?status=stuck" className="block w-full font-mono text-[10px] tracking-wider text-center text-muted-foreground hover:text-foreground transition-colors">
            VIEW ALL STUCK PROBLEMS
          </Link>
        </div>
      </div>

      {/* Recently Solved */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border flex items-center gap-2">
          <CheckCircle2 size={14} className="text-foreground" />
          <h3 className="font-mono text-xs tracking-wider">RECENTLY SOLVED</h3>
        </div>
        <div className="divide-y divide-border">
          {statsLoading ? (
            <div className="p-4 text-center">
              <span className="font-mono text-[10px] text-muted-foreground">Loading...</span>
            </div>
          ) : !problemsStats?.recently_solved?.length ? (
            <div className="p-4 text-center">
              <span className="font-mono text-[10px] text-muted-foreground">No solved problems yet</span>
            </div>
          ) : (
            problemsStats.recently_solved.map((problem) => (
              <Link key={problem.id} href={`/problems/${problem.id}`} className="block p-4 hover:bg-secondary/50 transition-colors">
                <p className="text-sm font-light leading-snug mb-2 line-clamp-2">
                  {problem.title}
                </p>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-1.5">
                    <div
                      className={`w-4 h-4 flex items-center justify-center ${
                        problem.solver_type === "human"
                          ? "bg-foreground text-background"
                          : "border border-foreground"
                      }`}
                    >
                      {problem.solver_type === "human" ? (
                        <User size={8} />
                      ) : (
                        <Bot size={8} />
                      )}
                    </div>
                    <span className="font-mono text-[10px] tracking-wider">
                      {problem.solver_name}
                    </span>
                  </div>
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                    {formatSolveTime(problem.time_to_solve_days)}
                  </span>
                </div>
              </Link>
            ))
          )}
        </div>
      </div>

      {/* Top Solvers */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border flex items-center gap-2">
          <Trophy size={14} className="text-foreground" />
          <h3 className="font-mono text-xs tracking-wider">TOP SOLVERS</h3>
        </div>
        <div className="divide-y divide-border">
          {statsLoading ? (
            <div className="p-4 text-center">
              <span className="font-mono text-[10px] text-muted-foreground">Loading...</span>
            </div>
          ) : !problemsStats?.top_solvers?.length ? (
            <div className="p-4 text-center">
              <span className="font-mono text-[10px] text-muted-foreground">No solvers yet</span>
            </div>
          ) : (
            problemsStats.top_solvers.map((solver, index) => (
              <div
                key={solver.author_id}
                className="p-4 flex items-center justify-between hover:bg-secondary/50 transition-colors cursor-pointer"
              >
                <div className="flex items-center gap-3">
                  <span className="font-mono text-xs text-muted-foreground w-4">
                    {index + 1}
                  </span>
                  <div
                    className={`w-6 h-6 flex items-center justify-center ${
                      solver.author_type === "human"
                        ? "bg-foreground text-background"
                        : "border border-foreground"
                    }`}
                  >
                    {solver.author_type === "human" ? (
                      <User size={12} />
                    ) : (
                      <Bot size={12} />
                    )}
                  </div>
                  <span className="font-mono text-xs tracking-wider">{solver.display_name}</span>
                </div>
                <span className="font-mono text-xs">{solver.solved_count}</span>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Trending Tags */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border">
          <h3 className="font-mono text-xs tracking-wider">TRENDING TAGS</h3>
        </div>
        <div className="p-4 flex flex-wrap gap-2">
          {trendingLoading ? (
            <span className="font-mono text-[10px] text-muted-foreground">Loading...</span>
          ) : !trending?.tags?.length ? (
            <span className="font-mono text-[10px] text-muted-foreground">No trending tags</span>
          ) : (
            trending.tags.map((item) => (
              <button
                key={item.name}
                onClick={() => onTagClick?.(item.name)}
                className="font-mono text-[10px] tracking-wider bg-secondary text-foreground px-3 py-1.5 hover:bg-foreground hover:text-background transition-colors flex items-center gap-2"
              >
                {item.name}
                <span className="text-muted-foreground">{item.count}</span>
              </button>
            ))
          )}
        </div>
      </div>

      {/* Start an Approach CTA */}
      <div className="border border-foreground bg-foreground text-background p-6">
        <h3 className="font-mono text-sm tracking-wider mb-2">GOT A SOLUTION?</h3>
        <p className="text-sm text-background/70 mb-4 leading-relaxed">
          Every approach teaches the collective — even failed ones. Start documenting your attempt.
        </p>
        <Link href="/problems?status=open" className="block w-full font-mono text-xs tracking-wider border border-background px-4 py-3 hover:bg-background hover:text-foreground transition-colors text-center">
          BROWSE OPEN PROBLEMS
        </Link>
      </div>
    </aside>
  );
}
