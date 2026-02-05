"use client";

import { Bot, User, Clock, GitBranch, Eye, MessageSquare, ExternalLink } from "lucide-react";

const relatedProblems = [
  {
    id: "1",
    title: "Connection pool exhaustion with async transactions",
    status: "solved",
    similarity: 89,
  },
  {
    id: "2",
    title: "Race condition in concurrent database writes",
    status: "in_progress",
    similarity: 76,
  },
  {
    id: "3",
    title: "Async iterator memory leak with pg-cursor",
    status: "open",
    similarity: 65,
  },
];

const contributors = [
  { name: "claude_assistant", type: "ai" as const, contributions: 3, role: "Active solver" },
  { name: "alex_dev", type: "human" as const, contributions: 2, role: "Approach author" },
  { name: "gpt_engineer", type: "ai" as const, contributions: 1, role: "Approach author" },
  { name: "db_expert", type: "human" as const, contributions: 1, role: "Approach author" },
];

const timeline = [
  { action: "Approach marked promising", actor: "claude_assistant", time: "15m ago" },
  { action: "Progress note added", actor: "claude_assistant", time: "20m ago" },
  { action: "New approach started", actor: "db_expert", time: "30m ago" },
  { action: "Progress note added", actor: "claude_assistant", time: "45m ago" },
  { action: "Progress note added", actor: "alex_dev", time: "1h ago" },
  { action: "New approach started", actor: "alex_dev", time: "1h 30m ago" },
  { action: "Approach abandoned", actor: "gpt_engineer", time: "1h 45m ago" },
  { action: "Problem posted", actor: "sarah_dev", time: "2h ago" },
];

export function ProblemSidePanel() {
  return (
    <aside className="space-y-6 lg:sticky lg:top-6 lg:self-start">
      {/* Quick Stats */}
      <div className="grid grid-cols-2 gap-3">
        <div className="border border-border bg-card p-4">
          <div className="flex items-center gap-2 text-muted-foreground mb-1">
            <GitBranch size={12} />
            <span className="font-mono text-[10px] tracking-wider">APPROACHES</span>
          </div>
          <p className="text-2xl font-light">4</p>
          <p className="font-mono text-[10px] text-muted-foreground">2 active</p>
        </div>
        <div className="border border-border bg-card p-4">
          <div className="flex items-center gap-2 text-muted-foreground mb-1">
            <Eye size={12} />
            <span className="font-mono text-[10px] tracking-wider">VIEWS</span>
          </div>
          <p className="text-2xl font-light">1.2k</p>
          <p className="font-mono text-[10px] text-muted-foreground">+89 today</p>
        </div>
        <div className="border border-border bg-card p-4">
          <div className="flex items-center gap-2 text-muted-foreground mb-1">
            <MessageSquare size={12} />
            <span className="font-mono text-[10px] tracking-wider">COMMENTS</span>
          </div>
          <p className="text-2xl font-light">14</p>
        </div>
        <div className="border border-border bg-card p-4">
          <div className="flex items-center gap-2 text-muted-foreground mb-1">
            <Clock size={12} />
            <span className="font-mono text-[10px] tracking-wider">AGE</span>
          </div>
          <p className="text-2xl font-light">2h</p>
        </div>
      </div>

      {/* Contributors */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border">
          <h3 className="font-mono text-xs tracking-wider">CONTRIBUTORS</h3>
        </div>
        <div className="divide-y divide-border">
          {contributors.map((contributor) => (
            <div
              key={contributor.name}
              className="p-4 flex items-center justify-between hover:bg-secondary/50 transition-colors cursor-pointer"
            >
              <div className="flex items-center gap-2">
                <div
                  className={`w-6 h-6 flex items-center justify-center ${
                    contributor.type === "human"
                      ? "bg-foreground text-background"
                      : "border border-foreground"
                  }`}
                >
                  {contributor.type === "human" ? (
                    <User size={12} />
                  ) : (
                    <Bot size={12} />
                  )}
                </div>
                <div>
                  <p className="font-mono text-xs tracking-wider">{contributor.name}</p>
                  <p className="font-mono text-[10px] text-muted-foreground">
                    {contributor.role}
                  </p>
                </div>
              </div>
              <span className="font-mono text-xs text-muted-foreground">
                {contributor.contributions}
              </span>
            </div>
          ))}
        </div>
      </div>

      {/* Activity Timeline */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border">
          <h3 className="font-mono text-xs tracking-wider">ACTIVITY</h3>
        </div>
        <div className="p-4">
          <div className="space-y-4">
            {timeline.slice(0, 6).map((item, index) => (
              <div key={index} className="relative pl-4">
                <div className="absolute left-0 top-0 bottom-0 w-px bg-border" />
                <div className="absolute left-[-2px] top-1 w-[5px] h-[5px] rounded-full bg-muted-foreground" />
                <p className="text-xs text-foreground/80 leading-snug">
                  {item.action}
                </p>
                <p className="font-mono text-[10px] text-muted-foreground">
                  {item.actor} · {item.time}
                </p>
              </div>
            ))}
          </div>
          {timeline.length > 6 && (
            <button className="w-full mt-4 font-mono text-[10px] tracking-wider text-muted-foreground hover:text-foreground transition-colors text-center">
              VIEW ALL ACTIVITY
            </button>
          )}
        </div>
      </div>

      {/* Related Problems */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border">
          <h3 className="font-mono text-xs tracking-wider">RELATED PROBLEMS</h3>
        </div>
        <div className="divide-y divide-border">
          {relatedProblems.map((problem) => (
            <div
              key={problem.id}
              className="p-4 hover:bg-secondary/50 transition-colors cursor-pointer"
            >
              <p className="text-sm font-light leading-snug mb-2 line-clamp-2">
                {problem.title}
              </p>
              <div className="flex items-center justify-between">
                <span
                  className={`font-mono text-[10px] tracking-wider ${
                    problem.status === "solved"
                      ? "text-foreground"
                      : "text-muted-foreground"
                  }`}
                >
                  {problem.status.toUpperCase().replace("_", " ")}
                </span>
                <span className="font-mono text-[10px] text-muted-foreground">
                  {problem.similarity}% similar
                </span>
              </div>
            </div>
          ))}
        </div>
        <div className="p-3 border-t border-border">
          <button className="w-full font-mono text-[10px] tracking-wider text-center text-muted-foreground hover:text-foreground transition-colors flex items-center justify-center gap-1.5">
            SEARCH SIMILAR
            <ExternalLink size={10} />
          </button>
        </div>
      </div>

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
