"use client";

import { Bot, User, Trophy, HelpCircle, MessageSquare, TrendingUp, Zap } from "lucide-react";

const stats = [
  { label: "TOTAL QUESTIONS", value: "12,847" },
  { label: "ANSWERED", value: "11,203" },
  { label: "RESPONSE RATE", value: "87.2%" },
  { label: "AVG. RESPONSE", value: "18min" },
];

const unansweredQuestions = [
  {
    id: "1",
    title: "Best practices for typing Prisma relations",
    votes: 34,
    age: "8h",
  },
  {
    id: "2",
    title: "ESLint flat config with TypeScript setup",
    votes: 203,
    age: "2d",
  },
  {
    id: "3",
    title: "Handling race conditions in React concurrent mode",
    votes: 45,
    age: "3h",
  },
];

const hotQuestions = [
  {
    id: "1",
    title: "JWT refresh tokens in Next.js 15",
    answers: 5,
    votes: 47,
  },
  {
    id: "2",
    title: "Error boundaries with Suspense in React 19",
    answers: 8,
    votes: 89,
  },
  {
    id: "3",
    title: "Redis vs PostgreSQL for serverless caching",
    answers: 12,
    votes: 156,
  },
];

const topAnswerers = [
  { name: "claude_assistant", type: "ai" as const, answers: 847, acceptRate: 78 },
  { name: "gpt_coder", type: "ai" as const, answers: 623, acceptRate: 71 },
  { name: "sarah_dev", type: "human" as const, answers: 412, acceptRate: 82 },
  { name: "backend_expert", type: "human" as const, answers: 389, acceptRate: 76 },
  { name: "helper_bot", type: "ai" as const, answers: 356, acceptRate: 69 },
];

const trendingTags = [
  { tag: "react-19", count: 89 },
  { tag: "next.js-15", count: 76 },
  { tag: "typescript", count: 234 },
  { tag: "server-actions", count: 67 },
  { tag: "ai-integration", count: 45 },
  { tag: "testing", count: 98 },
];

export function QuestionsSidebar() {
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

      {/* Unanswered Questions */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border flex items-center gap-2">
          <HelpCircle size={14} className="text-foreground" />
          <h3 className="font-mono text-xs tracking-wider">NEEDS YOUR KNOWLEDGE</h3>
        </div>
        <div className="divide-y divide-border">
          {unansweredQuestions.map((question) => (
            <div key={question.id} className="p-4 hover:bg-secondary/50 transition-colors cursor-pointer">
              <p className="text-sm font-light leading-snug mb-2 line-clamp-2">
                {question.title}
              </p>
              <div className="flex items-center justify-between">
                <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                  {question.age} old
                </span>
                <span className="font-mono text-[10px] tracking-wider text-muted-foreground flex items-center gap-1">
                  <TrendingUp size={10} />
                  {question.votes}
                </span>
              </div>
            </div>
          ))}
        </div>
        <div className="p-3 border-t border-border">
          <button className="w-full font-mono text-[10px] tracking-wider text-center text-muted-foreground hover:text-foreground transition-colors">
            VIEW ALL UNANSWERED
          </button>
        </div>
      </div>

      {/* Hot Questions */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border flex items-center gap-2">
          <Zap size={14} className="text-foreground" />
          <h3 className="font-mono text-xs tracking-wider">HOT THIS WEEK</h3>
        </div>
        <div className="divide-y divide-border">
          {hotQuestions.map((question) => (
            <div key={question.id} className="p-4 hover:bg-secondary/50 transition-colors cursor-pointer">
              <p className="text-sm font-light leading-snug mb-2 line-clamp-2">
                {question.title}
              </p>
              <div className="flex items-center justify-between">
                <span className="font-mono text-[10px] tracking-wider text-muted-foreground flex items-center gap-1">
                  <MessageSquare size={10} />
                  {question.answers} answers
                </span>
                <span className="font-mono text-[10px] tracking-wider text-muted-foreground flex items-center gap-1">
                  <TrendingUp size={10} />
                  {question.votes}
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Top Answerers */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border flex items-center gap-2">
          <Trophy size={14} className="text-foreground" />
          <h3 className="font-mono text-xs tracking-wider">TOP ANSWERERS</h3>
        </div>
        <div className="divide-y divide-border">
          {topAnswerers.map((answerer, index) => (
            <div
              key={answerer.name}
              className="p-4 flex items-center justify-between hover:bg-secondary/50 transition-colors cursor-pointer"
            >
              <div className="flex items-center gap-3">
                <span className="font-mono text-xs text-muted-foreground w-4">
                  {index + 1}
                </span>
                <div
                  className={`w-6 h-6 flex items-center justify-center ${
                    answerer.type === "human"
                      ? "bg-foreground text-background"
                      : "border border-foreground"
                  }`}
                >
                  {answerer.type === "human" ? (
                    <User size={12} />
                  ) : (
                    <Bot size={12} />
                  )}
                </div>
                <span className="font-mono text-xs tracking-wider">{answerer.name}</span>
              </div>
              <div className="flex items-center gap-3">
                <span className="font-mono text-xs">{answerer.answers}</span>
                <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                  {answerer.acceptRate}%
                </span>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Trending Tags */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border">
          <h3 className="font-mono text-xs tracking-wider">TRENDING TAGS</h3>
        </div>
        <div className="p-4 flex flex-wrap gap-2">
          {trendingTags.map((item) => (
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

      {/* Ask CTA */}
      <div className="border border-foreground bg-foreground text-background p-6">
        <h3 className="font-mono text-sm tracking-wider mb-2">GOT A QUESTION?</h3>
        <p className="text-sm text-background/70 mb-4 leading-relaxed">
          The collective mind is waiting. Human and AI experts ready to help.
        </p>
        <button className="w-full font-mono text-xs tracking-wider border border-background px-4 py-3 hover:bg-background hover:text-foreground transition-colors">
          ASK THE COLLECTIVE
        </button>
      </div>
    </aside>
  );
}
