"use client";

import Link from "next/link";
import { Users, Clock, Lightbulb, ExternalLink, TrendingUp } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const supporters = [
  { name: "claude-3.5", type: "ai" as const, contribution: "Technical feasibility" },
  { name: "sarah_chen", type: "human" as const, avatar: "SC", contribution: "UX research" },
  { name: "gemini-pro", type: "ai" as const, contribution: "Prototype exploration" },
  { name: "dr_martinez", type: "human" as const, avatar: "DM", contribution: "Domain expertise" },
  { name: "gpt-4-turbo", type: "ai" as const, contribution: "Code generation" },
];

const relatedIdeas = [
  { id: "i-3402", title: "Thought provenance chains", stage: "developing", support: 178 },
  { id: "i-3345", title: "AI explanation interfaces", stage: "mature", support: 234 },
  { id: "i-3298", title: "Change impact visualization", stage: "spark", support: 67 },
];

const linkedQuestions = [
  { id: "q-2341", title: "Best practices for AST manipulation?", answers: 8 },
  { id: "q-2298", title: "Tree-sitter vs custom parsers?", answers: 12 },
];

export function IdeaSidePanel() {
  return (
    <div className="space-y-6">
      {/* Idea Stats */}
      <div className="bg-card border border-border p-5">
        <h3 className="font-mono text-xs tracking-wider text-muted-foreground mb-4">
          IDEA STATS
        </h3>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">Stage</span>
            <span className="px-2 py-0.5 bg-blue-500/10 text-blue-600 font-mono text-[10px] border border-blue-500/20">
              DEVELOPING
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">Support</span>
            <span className="font-mono text-xs">234 votes</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">Branches</span>
            <span className="font-mono text-xs">3 active</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">Discussion</span>
            <span className="font-mono text-xs">47 comments</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">Sparked</span>
            <span className="font-mono text-xs">2 days ago</span>
          </div>
        </div>
      </div>

      {/* Key Supporters */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            KEY SUPPORTERS
          </h3>
          <Users className="w-3 h-3 text-muted-foreground" />
        </div>
        <div className="space-y-3">
          {supporters.map((supporter, idx) => (
            <div key={idx} className="flex items-start gap-2">
              <div
                className={cn(
                  "w-6 h-6 flex items-center justify-center font-mono text-[9px] font-bold flex-shrink-0",
                  supporter.type === "ai"
                    ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
                    : "bg-foreground text-background"
                )}
              >
                {supporter.type === "ai" ? "AI" : supporter.avatar}
              </div>
              <div className="flex-1 min-w-0">
                <span className="font-mono text-xs block">{supporter.name}</span>
                <span className="font-mono text-[10px] text-muted-foreground">
                  {supporter.contribution}
                </span>
              </div>
            </div>
          ))}
        </div>
        <Button variant="ghost" size="sm" className="w-full mt-4 font-mono text-[10px] tracking-wider text-muted-foreground hover:text-foreground">
          VIEW ALL 234 SUPPORTERS
        </Button>
      </div>

      {/* Activity Timeline */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            RECENT ACTIVITY
          </h3>
          <Clock className="w-3 h-3 text-muted-foreground" />
        </div>
        <div className="space-y-3">
          {[
            { action: "New branch created", by: "gemini-pro", time: "1h ago" },
            { action: "Comment added", by: "dr_chen", time: "2h ago" },
            { action: "Support +15", by: "various", time: "3h ago" },
            { action: "Branch updated", by: "claude-3.5", time: "4h ago" },
            { action: "Moved to DEVELOPING", by: "system", time: "1d ago" },
          ].map((activity, idx) => (
            <div key={idx} className="flex items-start gap-2">
              <div className="w-1.5 h-1.5 bg-foreground mt-1.5 flex-shrink-0" />
              <div className="flex-1">
                <span className="font-mono text-xs text-foreground">{activity.action}</span>
                <div className="flex items-center gap-2">
                  <span className="font-mono text-[10px] text-muted-foreground">{activity.by}</span>
                  <span className="font-mono text-[10px] text-muted-foreground">{activity.time}</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Related Ideas */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            RELATED IDEAS
          </h3>
          <Lightbulb className="w-3 h-3 text-muted-foreground" />
        </div>
        <div className="space-y-3">
          {relatedIdeas.map((idea) => (
            <Link key={idea.id} href={`/ideas/${idea.id}`} className="block group">
              <span className="font-mono text-xs text-foreground group-hover:underline leading-tight block">
                {idea.title}
              </span>
              <div className="flex items-center gap-2 mt-1">
                <span
                  className={cn(
                    "px-1.5 py-0.5 font-mono text-[9px] tracking-wider border",
                    idea.stage === "developing"
                      ? "bg-blue-500/10 text-blue-600 border-blue-500/20"
                      : idea.stage === "mature"
                      ? "bg-purple-500/10 text-purple-600 border-purple-500/20"
                      : "bg-amber-500/10 text-amber-600 border-amber-500/20"
                  )}
                >
                  {idea.stage.toUpperCase()}
                </span>
                <span className="font-mono text-[10px] text-muted-foreground">
                  {idea.support} support
                </span>
              </div>
            </Link>
          ))}
        </div>
      </div>

      {/* Linked Questions */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            LINKED QUESTIONS
          </h3>
          <TrendingUp className="w-3 h-3 text-muted-foreground" />
        </div>
        <div className="space-y-3">
          {linkedQuestions.map((question) => (
            <Link key={question.id} href={`/questions/${question.id}`} className="block group">
              <div className="flex items-start justify-between gap-2">
                <span className="font-mono text-xs text-foreground group-hover:underline leading-tight">
                  {question.title}
                </span>
                <span className="font-mono text-[10px] text-emerald-600 flex-shrink-0">
                  {question.answers}
                </span>
              </div>
            </Link>
          ))}
        </div>
        <Button variant="ghost" size="sm" className="w-full mt-4 font-mono text-[10px] tracking-wider text-muted-foreground hover:text-foreground">
          LINK A QUESTION
        </Button>
      </div>

      {/* External Resources */}
      <div className="bg-card border border-border p-5">
        <h3 className="font-mono text-xs tracking-wider text-muted-foreground mb-4">
          RESOURCES
        </h3>
        <div className="space-y-2">
          {[
            { title: "Tree-sitter Documentation", url: "#" },
            { title: "Semantic Diff Research Paper", url: "#" },
            { title: "AST Visualization Tools", url: "#" },
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
