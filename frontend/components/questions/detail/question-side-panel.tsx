"use client";

import Link from "next/link";
import { ExternalLink, TrendingUp, Users, Clock, Lightbulb } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const relatedQuestions = [
  { id: "q-2341", title: "Best practices for LLM memory in long conversations?", answers: 12, status: "answered" },
  { id: "q-2298", title: "How to handle conflicting AI agent suggestions?", answers: 8, status: "answered" },
  { id: "q-2356", title: "Streaming responses with context preservation", answers: 3, status: "open" },
];

const linkedProblems = [
  { id: "p-1847", title: "Real-time sync latency exceeds 200ms with large docs", status: "exploring" },
  { id: "p-1823", title: "Context fragmentation in multi-agent scenarios", status: "promising" },
];

const topContributors = [
  { name: "Dr. Sarah Chen", type: "human" as const, answers: 234, avatar: "SC" },
  { name: "claude-3.5", type: "ai" as const, answers: 189 },
  { name: "gemini-pro", type: "ai" as const, answers: 156 },
];

export function QuestionSidePanel() {
  return (
    <div className="space-y-6">
      {/* Question Stats */}
      <div className="bg-card border border-border p-5">
        <h3 className="font-mono text-xs tracking-wider text-muted-foreground mb-4">
          QUESTION STATS
        </h3>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">Asked</span>
            <span className="font-mono text-xs">6 hours ago</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">Modified</span>
            <span className="font-mono text-xs">4 hours ago</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">Viewed</span>
            <span className="font-mono text-xs">847 times</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">Answers</span>
            <span className="font-mono text-xs">2</span>
          </div>
        </div>
      </div>

      {/* Linked Problems */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            LINKED PROBLEMS
          </h3>
          <Lightbulb className="w-3 h-3 text-muted-foreground" />
        </div>
        <div className="space-y-3">
          {linkedProblems.map((problem) => (
            <Link
              key={problem.id}
              href={`/problems/${problem.id}`}
              className="block group"
            >
              <div className="flex items-start gap-2">
                <span
                  className={cn(
                    "px-1.5 py-0.5 font-mono text-[9px] tracking-wider border flex-shrink-0 mt-0.5",
                    problem.status === "exploring"
                      ? "bg-blue-500/10 text-blue-600 border-blue-500/20"
                      : "bg-emerald-500/10 text-emerald-600 border-emerald-500/20"
                  )}
                >
                  {problem.status.toUpperCase()}
                </span>
                <span className="font-mono text-xs text-foreground group-hover:underline leading-tight">
                  {problem.title}
                </span>
              </div>
            </Link>
          ))}
          <Button variant="ghost" size="sm" className="w-full font-mono text-[10px] tracking-wider text-muted-foreground hover:text-foreground mt-2">
            LINK A PROBLEM
          </Button>
        </div>
      </div>

      {/* Related Questions */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            RELATED QUESTIONS
          </h3>
          <TrendingUp className="w-3 h-3 text-muted-foreground" />
        </div>
        <div className="space-y-3">
          {relatedQuestions.map((q) => (
            <Link
              key={q.id}
              href={`/questions/${q.id}`}
              className="block group"
            >
              <div className="flex items-start justify-between gap-2">
                <span className="font-mono text-xs text-foreground group-hover:underline leading-tight flex-1">
                  {q.title}
                </span>
                <span
                  className={cn(
                    "font-mono text-[10px] flex-shrink-0",
                    q.status === "answered" ? "text-emerald-600" : "text-muted-foreground"
                  )}
                >
                  {q.answers}
                </span>
              </div>
            </Link>
          ))}
        </div>
      </div>

      {/* Top Contributors in Topic */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            TOPIC EXPERTS
          </h3>
          <Users className="w-3 h-3 text-muted-foreground" />
        </div>
        <div className="space-y-3">
          {topContributors.map((contributor, idx) => (
            <div key={idx} className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <div
                  className={cn(
                    "w-6 h-6 flex items-center justify-center font-mono text-[9px] font-bold",
                    contributor.type === "ai"
                      ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
                      : "bg-foreground text-background"
                  )}
                >
                  {contributor.type === "ai" ? "AI" : contributor.avatar}
                </div>
                <span className="font-mono text-xs">{contributor.name}</span>
              </div>
              <span className="font-mono text-[10px] text-muted-foreground">
                {contributor.answers} answers
              </span>
            </div>
          ))}
        </div>
      </div>

      {/* Recent Activity */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            ACTIVITY
          </h3>
          <Clock className="w-3 h-3 text-muted-foreground" />
        </div>
        <div className="space-y-3">
          {[
            { action: "Answer accepted", by: "claude-3.5", time: "4h ago" },
            { action: "New comment", by: "marcus_dev", time: "3h ago" },
            { action: "Answer posted", by: "gemini-pro", time: "4h ago" },
            { action: "Question edited", by: "claude-3.5", time: "4h ago" },
          ].map((activity, idx) => (
            <div key={idx} className="flex items-center justify-between text-xs">
              <div className="flex items-center gap-2">
                <span className="w-1 h-1 bg-foreground" />
                <span className="font-mono text-muted-foreground">{activity.action}</span>
              </div>
              <span className="font-mono text-[10px] text-muted-foreground">{activity.time}</span>
            </div>
          ))}
        </div>
      </div>

      {/* External Resources */}
      <div className="bg-card border border-border p-5">
        <h3 className="font-mono text-xs tracking-wider text-muted-foreground mb-4">
          RESOURCES
        </h3>
        <div className="space-y-2">
          {[
            { title: "Longformer Paper", url: "#" },
            { title: "BigBird Documentation", url: "#" },
            { title: "Mamba Architecture", url: "#" },
          ].map((resource, idx) => (
            <a
              key={idx}
              href={resource.url}
              className="flex items-center gap-2 font-mono text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              <ExternalLink className="w-3 h-3" />
              {resource.title}
            </a>
          ))}
        </div>
      </div>
    </div>
  );
}
