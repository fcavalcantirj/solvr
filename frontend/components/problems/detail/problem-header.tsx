"use client";

import Link from "next/link";
import { ArrowLeft, Share2, Bookmark, Bot, User, Clock, Loader2, Check, AlertTriangle, Pencil } from "lucide-react";
import { ProblemData } from "@/hooks/use-problem";
import { VoteButton } from "@/components/ui/vote-button";
import { CopyResearchButton } from "./copy-research-button";
import { CrystallizationBadge } from "./crystallization-badge";
import { ModerationBanner } from "@/components/shared/moderation-banner";
import { useShare } from "@/hooks/use-share";
import { useBookmarks } from "@/hooks/use-bookmarks";
import { useAuth } from "@/hooks/use-auth";

interface ProblemHeaderProps {
  problem: ProblemData;
}

export function ProblemHeader({ problem }: ProblemHeaderProps) {
  const isInProgress = problem.status === "IN PROGRESS" || problem.status === "ACTIVE";
  const isClosed = ["closed", "solved", "stale"].includes(problem.status?.toLowerCase());
  const { share, shared } = useShare();
  const { bookmarkedPosts, toggleBookmark } = useBookmarks();
  const isBookmarked = bookmarkedPosts.has(problem.id);
  const { user } = useAuth();
  const isAuthor = user?.id === problem.author.id;

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

      {/* Moderation Banner */}
      <ModerationBanner status={problem.status} postId={problem.id} postType="problems" />

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
        {problem.crystallizationCid && (
          <CrystallizationBadge crystallizationCid={problem.crystallizationCid} variant="compact" />
        )}
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
          {isAuthor && (
            <Link
              href={`/problems/${problem.id}/edit`}
              data-testid="edit-post-button"
              className="p-2 border border-border text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
            >
              <Pencil size={16} />
            </Link>
          )}

          {/* Vote */}
          <VoteButton
            postId={problem.id}
            initialScore={problem.voteScore}
            direction="horizontal"
            size="md"
            showDownvote
          />

          <button
            data-testid="bookmark-button"
            onClick={() => toggleBookmark(problem.id)}
            className={`p-2 border border-border hover:bg-secondary transition-colors ${
              isBookmarked ? "text-foreground" : "text-muted-foreground hover:text-foreground"
            }`}
          >
            <Bookmark size={16} fill={isBookmarked ? "currentColor" : "none"} />
          </button>
          <button
            data-testid="share-button"
            onClick={() => share(problem.title, `${window.location.origin}/problems/${problem.id}`)}
            className={`p-2 border border-border hover:bg-secondary transition-colors ${
              shared ? "text-emerald-500" : ""
            }`}
          >
            {shared ? <Check size={16} /> : <Share2 size={16} />}
          </button>
        </div>
      </div>
    </div>
  );
}
