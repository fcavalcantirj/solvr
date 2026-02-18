"use client";

import Link from "next/link";
import { Bot, User, GitBranch, MessageCircle, Eye, Clock, CheckCircle2, AlertCircle, Loader2 } from "lucide-react";
import { useProblems, ProblemListItem, UseProblemsOptions } from "@/hooks/use-problems";
import { useSearch } from "@/hooks/use-posts";
import { VoteButton } from "@/components/ui/vote-button";
import { CrystallizationBadge } from "@/components/problems/detail/crystallization-badge";

// Map API weight (1-5) to display labels
function mapWeight(weight?: number): "critical" | "high" | "medium" | "low" {
  if (!weight) return "medium";
  if (weight >= 5) return "critical";
  if (weight >= 4) return "high";
  if (weight >= 2) return "medium";
  return "low";
}

const weightStyles: Record<string, { label: string; className: string }> = {
  critical: { label: "CRITICAL", className: "bg-foreground text-background" },
  high: { label: "HIGH", className: "border border-foreground text-foreground" },
  medium: { label: "MED", className: "bg-secondary text-foreground" },
  low: { label: "LOW", className: "bg-secondary text-muted-foreground" },
};

const statusConfig: Record<string, { label: string; icon: typeof Clock; className: string }> = {
  open: { label: "OPEN", icon: Clock, className: "text-muted-foreground" },
  in_progress: { label: "IN PROGRESS", icon: Loader2, className: "text-foreground" },
  stuck: { label: "STUCK", icon: AlertCircle, className: "text-foreground" },
  solved: { label: "SOLVED", icon: CheckCircle2, className: "text-foreground font-medium" },
  closed: { label: "CLOSED", icon: CheckCircle2, className: "text-muted-foreground" },
  stale: { label: "STALE", icon: Clock, className: "text-muted-foreground" },
};

interface ProblemsListProps {
  status?: string;
  tags?: string[];
  sort?: 'newest' | 'votes' | 'approaches';
  searchQuery?: string;
}

export function ProblemsList({ status, tags, sort, searchQuery }: ProblemsListProps) {
  // Use search when there's a query, otherwise use regular problems fetch
  const isSearching = Boolean(searchQuery?.trim());

  const options: UseProblemsOptions = { status, tags, sort };
  const problemsResult = useProblems(options);
  const searchResult = useSearch(searchQuery || '', 'problem');

  // Select appropriate result based on whether we're searching
  const { problems, loading, error, hasMore, loadMore } = isSearching
    ? {
        problems: searchResult.posts.map(post => ({
          id: post.id,
          title: post.title,
          snippet: post.snippet,
          status: post.status,
          tags: post.tags,
          voteScore: post.votes,
          approachesCount: post.responses,
          commentsCount: post.comments,
          viewCount: post.views,
          author: post.author,
          timestamp: post.time,
        })),
        loading: searchResult.loading,
        error: searchResult.error,
        hasMore: false,
        loadMore: () => {},
      }
    : problemsResult;

  if (loading && problems.length === 0) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="animate-spin text-muted-foreground" size={24} />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-20">
        <p className="font-mono text-sm text-muted-foreground">
          Failed to load problems. Please try again.
        </p>
      </div>
    );
  }

  if (problems.length === 0) {
    return (
      <div className="flex items-center justify-center py-20">
        <p className="font-mono text-sm text-muted-foreground">
          No problems found.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {problems.map((problem) => (
        <ProblemCard key={problem.id} problem={problem} />
      ))}

      {/* Load More */}
      {hasMore && (
        <div className="flex justify-center pt-4">
          <button
            onClick={loadMore}
            disabled={loading}
            className="font-mono text-xs tracking-wider border border-border px-8 py-3 hover:bg-foreground hover:text-background hover:border-foreground transition-colors disabled:opacity-50"
          >
            {loading ? "LOADING..." : "LOAD MORE PROBLEMS"}
          </button>
        </div>
      )}
    </div>
  );
}

function ProblemCard({ problem }: { problem: ProblemListItem }) {
  const weight = mapWeight();
  const statusCfg = statusConfig[problem.status] || statusConfig.open;
  const StatusIcon = statusCfg.icon;

  return (
    <Link
      href={`/problems/${problem.id}`}
      className="block border border-border bg-card hover:border-foreground/30 transition-colors"
    >
      <div className="p-6">
        <div className="flex gap-3 sm:gap-4">
          {/* Vote Column - Desktop */}
          <div className="hidden sm:flex w-12 flex-shrink-0">
            <VoteButton
              postId={problem.id}
              initialScore={problem.voteScore}
              direction="vertical"
              size="sm"
              showDownvote
            />
          </div>

          {/* Content */}
          <div className="flex-1 min-w-0">
            {/* Header */}
            <div className="flex items-center gap-2 flex-wrap mb-4">
              <span
                className={`font-mono text-[10px] tracking-wider px-2 py-1 ${weightStyles[weight].className}`}
              >
                {weightStyles[weight].label}
              </span>
              <span
                className={`font-mono text-[10px] tracking-wider flex items-center gap-1.5 ${statusCfg.className}`}
              >
                <StatusIcon size={12} className={problem.status === "in_progress" ? "animate-spin" : ""} />
                {statusCfg.label}
              </span>
              {problem.crystallizationCid && (
                <CrystallizationBadge crystallizationCid={problem.crystallizationCid} variant="compact" />
              )}
            </div>

            {/* Title */}
            <h3 className="text-lg font-light tracking-tight mb-3 leading-snug text-balance">
              {problem.title}
            </h3>

            {/* Description */}
            <p className="text-sm text-muted-foreground leading-relaxed mb-4 line-clamp-2">
              {problem.snippet}
            </p>

            {/* Tags */}
            {problem.tags.length > 0 && (
              <div className="flex flex-wrap gap-1.5 mb-4">
                {problem.tags.slice(0, 4).map((tag) => (
                  <span
                    key={tag}
                    className="font-mono text-[10px] tracking-wider text-muted-foreground bg-secondary px-2 py-1"
                  >
                    {tag}
                  </span>
                ))}
                {problem.tags.length > 4 && (
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground px-2 py-1">
                    +{problem.tags.length - 4}
                  </span>
                )}
              </div>
            )}

            {/* Footer */}
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 pt-4 border-t border-border">
              {/* Author */}
              <div className="flex items-center gap-2">
                <div
                  className={`w-6 h-6 flex items-center justify-center ${
                    problem.author.type === "human"
                      ? "bg-foreground text-background"
                      : "border border-foreground"
                  }`}
                >
                  {problem.author.type === "human" ? (
                    <User size={12} />
                  ) : (
                    <Bot size={12} />
                  )}
                </div>
                <span className="font-mono text-xs tracking-wider">
                  {problem.author.name}
                </span>
                <span className="font-mono text-[10px] text-muted-foreground">
                  {problem.timestamp}
                </span>
              </div>

              {/* Stats */}
              <div className="flex items-center gap-4">
                {/* Mobile Vote */}
                <div className="sm:hidden">
                  <VoteButton
                    postId={problem.id}
                    initialScore={problem.voteScore}
                    direction="horizontal"
                    size="sm"
                    showDownvote
                  />
                </div>
                <div className="flex items-center gap-1.5 text-muted-foreground">
                  <GitBranch size={14} />
                  <span className="font-mono text-xs">
                    {problem.approachesCount}
                  </span>
                </div>
                <div className="flex items-center gap-1.5 text-muted-foreground">
                  <MessageCircle size={14} />
                  <span className="font-mono text-xs">
                    {problem.commentsCount}
                  </span>
                </div>
                <div className="flex items-center gap-1.5 text-muted-foreground">
                  <Eye size={14} />
                  <span className="font-mono text-xs">
                    {problem.viewCount}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Link>
  );
}
