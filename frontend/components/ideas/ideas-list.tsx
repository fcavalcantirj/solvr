"use client";

import { useState } from "react";
import Link from "next/link";
import { Sparkles, MessageSquare, GitBranch, Zap, ChevronDown, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useIdeas, IdeaListItem, IdeaStage, UseIdeasOptions } from "@/hooks/use-ideas";
import { useSearch } from "@/hooks/use-posts";
import { VoteButton } from "@/components/ui/vote-button";

const stageConfig: Record<IdeaStage | string, { label: string; color: string }> = {
  spark: { label: "SPARK", color: "bg-amber-500/10 text-amber-600 border-amber-500/20" },
  developing: { label: "DEVELOPING", color: "bg-blue-500/10 text-blue-600 border-blue-500/20" },
  mature: { label: "MATURE", color: "bg-purple-500/10 text-purple-600 border-purple-500/20" },
  realized: { label: "REALIZED", color: "bg-emerald-500/10 text-emerald-600 border-emerald-500/20" },
  archived: { label: "ARCHIVED", color: "bg-gray-500/10 text-gray-600 border-gray-500/20" },
};

const potentialConfig = {
  high: { label: "HIGH POTENTIAL", icon: Zap },
  rising: { label: "RISING", icon: Sparkles },
  needs_validation: { label: "NEEDS VALIDATION", icon: Sparkles },
};

interface IdeasListProps {
  options?: UseIdeasOptions & { searchQuery?: string };
}

export function IdeasList({ options }: IdeasListProps) {
  const [expandedIdea, setExpandedIdea] = useState<string | null>(null);

  // Use search when there's a query, otherwise use regular ideas fetch
  const isSearching = Boolean(options?.searchQuery?.trim());

  const ideasResult = useIdeas(options);
  const searchResult = useSearch(options?.searchQuery || '', 'idea');

  // Select appropriate result based on whether we're searching
  const { ideas, loading, error, total, hasMore, loadMore } = isSearching
    ? {
        ideas: searchResult.posts.map(post => ({
          id: post.id,
          title: post.title,
          snippet: post.snippet,
          stage: post.status as IdeaStage,
          tags: post.tags,
          supportScore: post.votes,
          engagementCount: post.responses,
          viewCount: post.views,
          author: post.author,
          timestamp: post.time,
        })),
        loading: searchResult.loading,
        error: searchResult.error,
        total: searchResult.posts.length,
        hasMore: false,
        loadMore: () => {},
      }
    : ideasResult;

  if (loading && ideas.length === 0) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
        <span className="ml-2 font-mono text-sm text-muted-foreground">Loading ideas...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-12">
        <p className="font-mono text-sm text-destructive">{error}</p>
        <Button variant="outline" onClick={() => window.location.reload()} className="mt-4">
          Try Again
        </Button>
      </div>
    );
  }

  if (ideas.length === 0) {
    return (
      <div className="text-center py-12">
        <p className="font-mono text-sm text-muted-foreground">No ideas found</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between mb-6">
        <span className="font-mono text-xs text-muted-foreground">
          Showing {ideas.length} of {total} ideas
        </span>
      </div>

      {ideas.map((idea) => (
        <IdeaCard
          key={idea.id}
          idea={idea}
          expanded={expandedIdea === idea.id}
          onToggleExpand={() => setExpandedIdea(expandedIdea === idea.id ? null : idea.id)}
        />
      ))}

      {/* Load More */}
      {hasMore && (
        <div className="flex justify-center pt-6">
          <Button
            variant="outline"
            className="font-mono text-xs tracking-wider bg-transparent"
            onClick={loadMore}
            disabled={loading}
          >
            {loading ? (
              <>
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                LOADING...
              </>
            ) : (
              "LOAD MORE IDEAS"
            )}
          </Button>
        </div>
      )}
    </div>
  );
}

interface IdeaCardProps {
  idea: IdeaListItem;
  expanded: boolean;
  onToggleExpand: () => void;
}

function IdeaCard({ idea, expanded, onToggleExpand }: IdeaCardProps) {
  const stage = stageConfig[idea.stage] || stageConfig.spark;
  const potential = potentialConfig[idea.potential];

  return (
    <div className="bg-card border border-border hover:border-foreground/20 transition-colors">
      <div className="p-4 sm:p-6">
        <div className="flex gap-3 sm:gap-4">
          {/* Vote Column - Desktop */}
          <div className="hidden sm:flex w-12 flex-shrink-0">
            <VoteButton
              postId={idea.id}
              initialScore={idea.support}
              direction="vertical"
              size="sm"
              showDownvote
            />
          </div>

          {/* Content */}
          <div className="flex-1 min-w-0">
        {/* Header */}
        <div className="flex items-start justify-between gap-4 mb-4">
          <div className="flex items-center gap-2 flex-wrap">
            <span className={cn("px-2 py-0.5 font-mono text-[10px] tracking-wider border", stage.color)}>
              {stage.label}
            </span>
            {potential && (
              <span className="flex items-center gap-1 px-2 py-0.5 bg-secondary font-mono text-[10px] tracking-wider text-foreground border border-border">
                {(() => {
                  const Icon = potential.icon;
                  return <Icon className="w-2.5 h-2.5" />;
                })()}
                {potential.label}
              </span>
            )}
            <span className="font-mono text-[10px] text-muted-foreground">
              {idea.id}
            </span>
          </div>
          <span className="font-mono text-[10px] text-muted-foreground flex-shrink-0">
            {idea.timestamp}
          </span>
        </div>

        {/* Title */}
        <Link href={`/ideas/${idea.id}`}>
          <h3 className="font-mono text-lg font-medium text-foreground hover:underline mb-3">
            {idea.title}
          </h3>
        </Link>

        {/* Spark (the core idea) */}
        <div className="bg-secondary/50 border-l-2 border-foreground/20 pl-4 py-2 mb-4">
          <p className="font-mono text-sm text-foreground/80 italic">
            {'"'}{idea.spark}{'"'}
          </p>
        </div>

        {/* Tags */}
        <div className="flex flex-wrap gap-2 mb-4">
          {idea.tags.map((tag) => (
            <span
              key={tag}
              className="px-2 py-0.5 bg-secondary text-foreground font-mono text-[10px] tracking-wider border border-border hover:border-foreground/30 cursor-pointer transition-colors"
            >
              {tag}
            </span>
          ))}
        </div>

        {/* Author and Supporters */}
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
          <div className="flex items-center gap-3 sm:gap-4 flex-wrap">
            {/* Author */}
            <div className="flex items-center gap-2">
              <div
                className={cn(
                  "w-6 h-6 flex items-center justify-center font-mono text-[9px] font-bold shrink-0",
                  idea.author.type === "ai"
                    ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
                    : "bg-foreground text-background"
                )}
              >
                {idea.author.type === "ai" ? "AI" : idea.author.avatar || idea.author.name.slice(0, 2).toUpperCase()}
              </div>
              <span className="font-mono text-xs text-muted-foreground truncate max-w-[120px] sm:max-w-none">
                {idea.author.name}
              </span>
            </div>

            {/* Supporters preview */}
            {idea.supporters && idea.supporters.length > 0 && (
              <div className="flex items-center">
                <div className="flex -space-x-1">
                  {idea.supporters.slice(0, 3).map((supporter, idx) => (
                    <div
                      key={idx}
                      className={cn(
                        "w-5 h-5 flex items-center justify-center font-mono text-[8px] font-bold border-2 border-card shrink-0",
                        supporter.type === "ai"
                          ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
                          : "bg-foreground text-background"
                      )}
                    >
                      {supporter.type === "ai" ? "AI" : supporter.name.slice(0, 1).toUpperCase()}
                    </div>
                  ))}
                </div>
                {idea.supporters.length > 3 && (
                  <span className="font-mono text-[10px] text-muted-foreground ml-2">
                    +{idea.supporters.length - 3}
                  </span>
                )}
              </div>
            )}
          </div>

          {/* Stats */}
          <div className="flex items-center gap-3 sm:gap-4">
            {/* Mobile Vote */}
            <div className="sm:hidden">
              <VoteButton
                postId={idea.id}
                initialScore={idea.support}
                direction="horizontal"
                size="sm"
                showDownvote
              />
            </div>
            <span className="flex items-center gap-1 sm:gap-1.5 font-mono text-xs text-muted-foreground">
              <MessageSquare className="w-3.5 h-3.5 shrink-0" />
              {idea.comments}
            </span>
            {idea.branches > 0 && (
              <span className="flex items-center gap-1 sm:gap-1.5 font-mono text-xs text-muted-foreground">
                <GitBranch className="w-3.5 h-3.5 shrink-0" />
                {idea.branches}
              </span>
            )}
          </div>
        </div>

        {/* Recent Comment Preview */}
        {idea.recentComment && (
          <button
            onClick={onToggleExpand}
            className="w-full mt-4 pt-4 border-t border-border text-left"
          >
            <div className="flex items-start gap-3">
              <div
                className={cn(
                  "w-5 h-5 flex items-center justify-center font-mono text-[8px] font-bold flex-shrink-0",
                  idea.recentComment.type === "ai"
                    ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
                    : "bg-foreground text-background"
                )}
              >
                {idea.recentComment.type === "ai" ? "AI" : idea.recentComment.author.slice(0, 1).toUpperCase()}
              </div>
              <div className="flex-1 min-w-0">
                <span className="font-mono text-[10px] text-muted-foreground">
                  {idea.recentComment.author}:
                </span>
                <p className="font-mono text-xs text-foreground/70 truncate">
                  {idea.recentComment.content}
                </p>
              </div>
              <ChevronDown
                className={cn(
                  "w-4 h-4 text-muted-foreground transition-transform flex-shrink-0",
                  expanded && "rotate-180"
                )}
              />
            </div>
          </button>
        )}
          </div>
        </div>
      </div>

      {/* Expanded Discussion */}
      {expanded && idea.recentComment && (
        <div className="px-4 sm:px-6 pb-4 sm:pb-6 pt-2 border-t border-border bg-secondary/30">
          <div className="space-y-3">
            <div className="flex items-start gap-3">
              <div
                className={cn(
                  "w-6 h-6 flex items-center justify-center font-mono text-[9px] font-bold flex-shrink-0",
                  idea.recentComment.type === "ai"
                    ? "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
                    : "bg-foreground text-background"
                )}
              >
                {idea.recentComment.type === "ai" ? "AI" : "U"}
              </div>
              <div className="flex-1">
                <div className="flex items-center gap-2 mb-1">
                  <span className="font-mono text-xs font-medium">{idea.recentComment.author}</span>
                  <span className="font-mono text-[10px] text-muted-foreground">just now</span>
                </div>
                <p className="text-sm text-foreground">{idea.recentComment.content}</p>
              </div>
            </div>
          </div>
          <div className="mt-4 flex items-center gap-2">
            <input
              type="text"
              placeholder="Add to the discussion..."
              className="flex-1 bg-card border border-border px-3 py-2 font-mono text-xs focus:outline-none focus:border-foreground placeholder:text-muted-foreground"
            />
            <Button size="sm" className="font-mono text-[10px] tracking-wider">
              REPLY
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
