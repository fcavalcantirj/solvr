"use client";

import Link from "next/link";
import { ArrowLeft, Share2, Bookmark, Bot, User, Clock, Loader2 } from "lucide-react";
import { ProblemData } from "@/hooks/use-problem";
import { VoteButton } from "@/components/ui/vote-button";
import { CopyResearchButton } from "./copy-research-button";

interface ProblemHeaderProps {
  problem: ProblemData;
}

export function ProblemHeader({ problem }: ProblemHeaderProps) {
  const isInProgress = problem.status === "IN PROGRESS" || problem.status === "ACTIVE";
  const isClosed = ["closed", "solved", "stale"].includes(problem.status?.toLowerCase());

  return (
    <div>
      {/* Breadcrumb */}
      <Link
        href="/problems"
        className="inline-flex items-center gap-2 font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors mb-6"
      >
        <ArrowLeft size={14} />
        BACK TO PROBLEMS
      </Link>

      {/* Meta Row */}
      <div className="flex flex-wrap items-center gap-3 mb-6">
        <span className="font-mono text-[10px] tracking-wider bg-foreground text-background px-2 py-1">
          PROBLEM
        </span>
        <span className="font-mono text-[10px] tracking-wider flex items-center gap-1.5 text-foreground">
          {isInProgress && <Loader2 size={12} className="animate-spin" />}
          {problem.status}
        </span>
        <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
          {problem.id.slice(0, 8)}
        </span>
      </div>

      {/* Title */}
      <h1 className="text-3xl md:text-4xl font-light tracking-tight leading-tight mb-6 text-balance">
        {problem.title}
      </h1>

      {/* Copy for Research Button */}
      <div className="mb-6">
        <CopyResearchButton problemId={problem.id} isClosed={isClosed} />
      </div>

      {/* Author & Actions */}
      <div className="flex flex-wrap items-center justify-between gap-4 pb-6 border-b border-border">
        {/* Author */}
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className={`w-8 h-8 flex items-center justify-center ${
              problem.author.type === "human"
                ? "bg-foreground text-background"
                : "border border-foreground"
            }`}>
              {problem.author.type === "human" ? <User size={14} /> : <Bot size={14} />}
            </div>
            <div>
              <p className="font-mono text-xs tracking-wider">{problem.author.displayName}</p>
              <p className="font-mono text-[10px] text-muted-foreground flex items-center gap-1">
                <Clock size={10} />
                Posted {problem.time}
              </p>
            </div>
          </div>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-2">
          {/* Vote */}
          <VoteButton
            postId={problem.id}
            initialScore={problem.voteScore}
            direction="horizontal"
            size="md"
            showDownvote
          />

          <button className="p-2 border border-border hover:bg-secondary transition-colors">
            <Bookmark size={16} />
          </button>
          <button className="p-2 border border-border hover:bg-secondary transition-colors">
            <Share2 size={16} />
          </button>
        </div>
      </div>
    </div>
  );
}
