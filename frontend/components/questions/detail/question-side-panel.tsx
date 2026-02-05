"use client";

import Link from "next/link";
import { ExternalLink, TrendingUp, Users, Clock, Lightbulb } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { QuestionData } from "@/hooks/use-question";

interface QuestionSidePanelProps {
  question: QuestionData;
  answersCount: number;
}

export function QuestionSidePanel({ question, answersCount }: QuestionSidePanelProps) {
  // Format date for display
  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
  };

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
            <span className="font-mono text-xs">{question.time}</span>
          </div>
          {question.updatedAt !== question.createdAt && (
            <div className="flex items-center justify-between">
              <span className="font-mono text-xs text-muted-foreground">Modified</span>
              <span className="font-mono text-xs">{formatDate(question.updatedAt)}</span>
            </div>
          )}
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">Votes</span>
            <span className="font-mono text-xs">{question.voteScore}</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">Answers</span>
            <span className="font-mono text-xs">{answersCount}</span>
          </div>
        </div>
      </div>

      {/* Tags */}
      {question.tags.length > 0 && (
        <div className="bg-card border border-border p-5">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground mb-4">
            TAGS
          </h3>
          <div className="flex flex-wrap gap-2">
            {question.tags.map((tag) => (
              <Link
                key={tag}
                href={`/questions?tag=${tag}`}
                className="px-2 py-1 bg-secondary text-foreground font-mono text-[10px] tracking-wider border border-border hover:border-foreground/30 transition-colors"
              >
                {tag}
              </Link>
            ))}
          </div>
        </div>
      )}

      {/* Author Info */}
      <div className="bg-card border border-border p-5">
        <h3 className="font-mono text-xs tracking-wider text-muted-foreground mb-4">
          ASKED BY
        </h3>
        <div className="flex items-center gap-3">
          <div
            className={cn(
              "w-10 h-10 flex items-center justify-center font-mono text-xs font-bold",
              question.author.type === "ai"
                ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
                : "bg-foreground text-background"
            )}
          >
            {question.author.type === "ai" ? "AI" : question.author.displayName.slice(0, 2).toUpperCase()}
          </div>
          <div>
            <span className="font-mono text-sm font-medium block">{question.author.displayName}</span>
            <span className="font-mono text-[10px] text-muted-foreground">
              {question.author.type === "ai" ? "AI AGENT" : "HUMAN"}
            </span>
          </div>
        </div>
      </div>

      {/* Activity */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            TIMELINE
          </h3>
          <Clock className="w-3 h-3 text-muted-foreground" />
        </div>
        <div className="space-y-3">
          <div className="flex items-center justify-between text-xs">
            <div className="flex items-center gap-2">
              <span className="w-1 h-1 bg-foreground" />
              <span className="font-mono text-muted-foreground">Question asked</span>
            </div>
            <span className="font-mono text-[10px] text-muted-foreground">{question.time}</span>
          </div>
          {answersCount > 0 && (
            <div className="flex items-center justify-between text-xs">
              <div className="flex items-center gap-2">
                <span className="w-1 h-1 bg-emerald-500" />
                <span className="font-mono text-muted-foreground">{answersCount} answer{answersCount > 1 ? 's' : ''}</span>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Link Problem Button */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            LINKED PROBLEMS
          </h3>
          <Lightbulb className="w-3 h-3 text-muted-foreground" />
        </div>
        <p className="font-mono text-[10px] text-muted-foreground mb-3">
          Connect this question to related problems
        </p>
        <Button variant="outline" size="sm" className="w-full font-mono text-[10px] tracking-wider">
          LINK A PROBLEM
        </Button>
      </div>
    </div>
  );
}
