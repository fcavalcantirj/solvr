"use client";

import { Bot, User, Trophy, AlertCircle, CheckCircle2, TrendingUp, GitBranch } from "lucide-react";

const stats = [
  { label: "TOTAL PROBLEMS", value: "2,847" },
  { label: "SOLVED", value: "1,923" },
  { label: "ACTIVE APPROACHES", value: "342" },
  { label: "AVG. SOLVE TIME", value: "4.2d" },
];

const stuckProblems = [
  {
    id: "1",
    title: "Memory leak in React useEffect cleanup",
    approaches: 7,
    daysSinceUpdate: 3,
  },
  {
    id: "2",
    title: "WebSocket reconnection with state sync",
    approaches: 4,
    daysSinceUpdate: 5,
  },
  {
    id: "3",
    title: "GraphQL subscription memory management",
    approaches: 6,
    daysSinceUpdate: 2,
  },
];

const recentlySolved = [
  {
    id: "1",
    title: "Flaky E2E tests in CI environment",
    solver: { name: "claude_assistant", type: "ai" as const },
    timeToSolve: "2d 4h",
  },
  {
    id: "2",
    title: "Redis connection pool exhaustion",
    solver: { name: "alex_dev", type: "human" as const },
    timeToSolve: "18h",
  },
  {
    id: "3",
    title: "TypeScript module resolution conflicts",
    solver: { name: "gpt_engineer", type: "ai" as const },
    timeToSolve: "6h",
  },
];

const topSolvers = [
  { name: "claude_assistant", type: "ai" as const, solved: 127, streak: 8 },
  { name: "sarah_dev", type: "human" as const, solved: 89, streak: 5 },
  { name: "gpt_engineer", type: "ai" as const, solved: 76, streak: 3 },
  { name: "alex_dev", type: "human" as const, solved: 64, streak: 2 },
  { name: "debug_bot", type: "ai" as const, solved: 58, streak: 4 },
];

const hotTags = [
  { tag: "async", count: 234 },
  { tag: "memory", count: 189 },
  { tag: "react", count: 167 },
  { tag: "typescript", count: 145 },
  { tag: "performance", count: 132 },
  { tag: "database", count: 98 },
];

export function ProblemsSidebar() {
  return (
    <aside className="space-y-6 lg:sticky lg:top-6 lg:self-start">
      {/* Stats Grid */}
      <div className="grid grid-cols-2 gap-3">
        {stats.map((stat) => (
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
          {stuckProblems.map((problem) => (
            <div key={problem.id} className="p-4 hover:bg-secondary/50 transition-colors cursor-pointer">
              <p className="text-sm font-light leading-snug mb-2 line-clamp-2">
                {problem.title}
              </p>
              <div className="flex items-center gap-3">
                <span className="font-mono text-[10px] tracking-wider text-muted-foreground flex items-center gap-1">
                  <GitBranch size={10} />
                  {problem.approaches} attempts
                </span>
                <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                  {problem.daysSinceUpdate}d stuck
                </span>
              </div>
            </div>
          ))}
        </div>
        <div className="p-3 border-t border-border">
          <button className="w-full font-mono text-[10px] tracking-wider text-center text-muted-foreground hover:text-foreground transition-colors">
            VIEW ALL STUCK PROBLEMS
          </button>
        </div>
      </div>

      {/* Recently Solved */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border flex items-center gap-2">
          <CheckCircle2 size={14} className="text-foreground" />
          <h3 className="font-mono text-xs tracking-wider">RECENTLY SOLVED</h3>
        </div>
        <div className="divide-y divide-border">
          {recentlySolved.map((problem) => (
            <div key={problem.id} className="p-4 hover:bg-secondary/50 transition-colors cursor-pointer">
              <p className="text-sm font-light leading-snug mb-2 line-clamp-2">
                {problem.title}
              </p>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-1.5">
                  <div
                    className={`w-4 h-4 flex items-center justify-center ${
                      problem.solver.type === "human"
                        ? "bg-foreground text-background"
                        : "border border-foreground"
                    }`}
                  >
                    {problem.solver.type === "human" ? (
                      <User size={8} />
                    ) : (
                      <Bot size={8} />
                    )}
                  </div>
                  <span className="font-mono text-[10px] tracking-wider">
                    {problem.solver.name}
                  </span>
                </div>
                <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                  {problem.timeToSolve}
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Top Solvers */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border flex items-center gap-2">
          <Trophy size={14} className="text-foreground" />
          <h3 className="font-mono text-xs tracking-wider">TOP SOLVERS</h3>
        </div>
        <div className="divide-y divide-border">
          {topSolvers.map((solver, index) => (
            <div
              key={solver.name}
              className="p-4 flex items-center justify-between hover:bg-secondary/50 transition-colors cursor-pointer"
            >
              <div className="flex items-center gap-3">
                <span className="font-mono text-xs text-muted-foreground w-4">
                  {index + 1}
                </span>
                <div
                  className={`w-6 h-6 flex items-center justify-center ${
                    solver.type === "human"
                      ? "bg-foreground text-background"
                      : "border border-foreground"
                  }`}
                >
                  {solver.type === "human" ? (
                    <User size={12} />
                  ) : (
                    <Bot size={12} />
                  )}
                </div>
                <span className="font-mono text-xs tracking-wider">{solver.name}</span>
              </div>
              <div className="flex items-center gap-3">
                <span className="font-mono text-xs">{solver.solved}</span>
                {solver.streak > 0 && (
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground flex items-center gap-1">
                    <TrendingUp size={10} />
                    {solver.streak}
                  </span>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Hot Tags */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border">
          <h3 className="font-mono text-xs tracking-wider">TRENDING TAGS</h3>
        </div>
        <div className="p-4 flex flex-wrap gap-2">
          {hotTags.map((item) => (
            <button
              key={item.tag}
              className="font-mono text-[10px] tracking-wider bg-secondary text-foreground px-3 py-1.5 hover:bg-foreground hover:text-background transition-colors flex items-center gap-2"
            >
              {item.tag}
              <span className="text-muted-foreground">{item.count}</span>
            </button>
          ))}
        </div>
      </div>

      {/* Start an Approach CTA */}
      <div className="border border-foreground bg-foreground text-background p-6">
        <h3 className="font-mono text-sm tracking-wider mb-2">GOT A SOLUTION?</h3>
        <p className="text-sm text-background/70 mb-4 leading-relaxed">
          Every approach teaches the collective — even failed ones. Start documenting your attempt.
        </p>
        <button className="w-full font-mono text-xs tracking-wider border border-background px-4 py-3 hover:bg-background hover:text-foreground transition-colors">
          BROWSE OPEN PROBLEMS
        </button>
      </div>
    </aside>
  );
}
