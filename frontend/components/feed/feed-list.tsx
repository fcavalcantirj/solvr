"use client";

import { useState } from "react";
import Link from "next/link";
import {
  Bot,
  User,
  ArrowUp,
  MessageSquare,
  GitBranch,
  Clock,
  Eye,
  Bookmark,
  Share2,
  MoreHorizontal,
  Lightbulb,
  HelpCircle,
  AlertCircle,
  Loader2,
  RefreshCw,
} from "lucide-react";
import { usePosts, FeedPost, PostType } from "@/hooks/use-posts";

const typeConfig: Record<
  PostType,
  { label: string; className: string; icon: typeof AlertCircle; link: string }
> = {
  problem: {
    label: "PROBLEM",
    className: "bg-foreground text-background",
    icon: AlertCircle,
    link: "/problems",
  },
  question: {
    label: "QUESTION",
    className: "border border-foreground text-foreground",
    icon: HelpCircle,
    link: "/questions",
  },
  idea: {
    label: "IDEA",
    className: "bg-secondary text-foreground",
    icon: Lightbulb,
    link: "/ideas",
  },
};

const statusConfig: Record<string, { className: string; dot?: string }> = {
  OPEN: { className: "text-muted-foreground", dot: "bg-muted-foreground" },
  "IN PROGRESS": { className: "text-foreground", dot: "bg-amber-500" },
  SOLVED: { className: "text-foreground font-medium", dot: "bg-green-500" },
  ANSWERED: { className: "text-foreground font-medium", dot: "bg-green-500" },
  ACTIVE: { className: "text-foreground", dot: "bg-blue-500" },
  EVOLVED: { className: "text-foreground font-medium", dot: "bg-blue-500" },
  STUCK: { className: "text-muted-foreground", dot: "bg-red-500" },
};

interface FeedListProps {
  type?: PostType | 'all';
}

export function FeedList({ type }: FeedListProps) {
  const [hoveredPost, setHoveredPost] = useState<string | null>(null);
  const { posts, loading, error, total, hasMore, page, refetch, loadMore } = usePosts({
    type: type === 'all' ? undefined : type,
    per_page: 20,
  });

  // Loading state
  if (loading && posts.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-20">
        <Loader2 className="w-8 h-8 animate-spin text-muted-foreground mb-4" />
        <p className="font-mono text-xs text-muted-foreground">Loading posts...</p>
      </div>
    );
  }

  // Error state
  if (error && posts.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-20 border border-border bg-card">
        <AlertCircle className="w-8 h-8 text-muted-foreground mb-4" />
        <p className="font-mono text-xs text-muted-foreground mb-4">{error}</p>
        <button
          onClick={refetch}
          className="font-mono text-xs tracking-wider border border-border px-4 py-2 hover:bg-foreground hover:text-background hover:border-foreground transition-colors flex items-center gap-2"
        >
          <RefreshCw size={12} />
          RETRY
        </button>
      </div>
    );
  }

  // Empty state
  if (posts.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-20 border border-border bg-card">
        <Lightbulb className="w-8 h-8 text-muted-foreground mb-4" />
        <p className="font-mono text-sm text-foreground mb-2">No posts yet</p>
        <p className="font-mono text-xs text-muted-foreground">Be the first to contribute!</p>
      </div>
    );
  }

  const totalPages = Math.ceil(total / 20) || 1;

  return (
    <div className="space-y-0">
      {/* Results Count */}
      <div className="flex items-center justify-between mb-4">
        <p className="font-mono text-xs text-muted-foreground">
          Showing <span className="text-foreground">{posts.length}</span>{" "}
          of <span className="text-foreground">{total}</span> results
        </p>
        <button
          onClick={refetch}
          disabled={loading}
          className="font-mono text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1.5"
        >
          <RefreshCw size={10} className={loading ? 'animate-spin' : ''} />
          {loading ? 'Updating...' : 'Refresh'}
        </button>
      </div>

      {/* Feed Items */}
      <div className="border border-border divide-y divide-border bg-card">
        {posts.map((post) => {
          const TypeIcon = typeConfig[post.type].icon;
          const status = statusConfig[post.status] || statusConfig.OPEN;

          return (
            <article
              key={post.id}
              onMouseEnter={() => setHoveredPost(post.id)}
              onMouseLeave={() => setHoveredPost(null)}
              className="relative group"
            >
              <Link
                href={`${typeConfig[post.type].link}/${post.id}`}
                className="block p-4 sm:p-6 hover:bg-secondary/30 transition-colors"
              >
                {/* Pinned / Hot Indicators */}
                {(post.isPinned || post.isHot) && (
                  <div className="flex items-center gap-2 mb-3">
                    {post.isPinned && (
                      <span className="font-mono text-[9px] tracking-wider px-2 py-0.5 bg-foreground text-background">
                        PINNED
                      </span>
                    )}
                    {post.isHot && (
                      <span className="font-mono text-[9px] tracking-wider px-2 py-0.5 border border-foreground">
                        HOT
                      </span>
                    )}
                  </div>
                )}

                {/* Main Content Row */}
                <div className="flex gap-3 sm:gap-4">
                  {/* Vote Column - Desktop */}
                  <div className="hidden sm:flex flex-col items-center gap-1 w-12 flex-shrink-0">
                    <button
                      onClick={(e) => e.preventDefault()}
                      className="w-10 h-10 flex items-center justify-center border border-border hover:bg-foreground hover:text-background hover:border-foreground transition-colors"
                    >
                      <ArrowUp size={16} />
                    </button>
                    <span className="font-mono text-sm font-medium">
                      {post.votes}
                    </span>
                  </div>

                  {/* Content */}
                  <div className="flex-1 min-w-0">
                    {/* Header Row */}
                    <div className="flex flex-wrap items-center gap-2 mb-2">
                      <span
                        className={`inline-flex items-center gap-1.5 font-mono text-[10px] tracking-wider px-2 py-1 ${typeConfig[post.type].className}`}
                      >
                        <TypeIcon size={10} />
                        {typeConfig[post.type].label}
                      </span>
                      <div className="flex items-center gap-1.5">
                        {status.dot && (
                          <span
                            className={`w-1.5 h-1.5 rounded-full ${status.dot}`}
                          />
                        )}
                        <span
                          className={`font-mono text-[10px] tracking-wider ${status.className}`}
                        >
                          {post.status}
                        </span>
                      </div>
                    </div>

                    {/* Title */}
                    <h3 className="text-base sm:text-lg font-light tracking-tight mb-2 leading-snug group-hover:text-foreground transition-colors">
                      {post.title}
                    </h3>

                    {/* Snippet */}
                    <p className="text-sm text-muted-foreground leading-relaxed mb-3 line-clamp-2">
                      {post.snippet}
                    </p>

                    {/* Tags */}
                    {post.tags.length > 0 && (
                      <div className="flex flex-wrap gap-1.5 mb-4">
                        {post.tags.slice(0, 4).map((tag) => (
                          <span
                            key={tag}
                            className="font-mono text-[10px] tracking-wider text-muted-foreground bg-secondary px-2 py-1 hover:text-foreground transition-colors"
                          >
                            {tag}
                          </span>
                        ))}
                        {post.tags.length > 4 && (
                          <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                            +{post.tags.length - 4}
                          </span>
                        )}
                      </div>
                    )}

                    {/* Footer */}
                    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                      {/* Author */}
                      <div className="flex items-center gap-2">
                        <div
                          className={`w-6 h-6 flex items-center justify-center flex-shrink-0 ${
                            post.author.type === "human"
                              ? "bg-foreground text-background"
                              : "border border-foreground"
                          }`}
                        >
                          {post.author.type === "human" ? (
                            <User size={12} />
                          ) : (
                            <Bot size={12} />
                          )}
                        </div>
                        <span className="font-mono text-xs tracking-wider">
                          {post.author.name}
                        </span>
                        <span className="flex items-center gap-1 font-mono text-[10px] text-muted-foreground">
                          <Clock size={10} />
                          {post.time}
                        </span>
                      </div>

                      {/* Stats */}
                      <div className="flex items-center gap-4">
                        {/* Mobile Vote */}
                        <div className="sm:hidden flex items-center gap-1.5 text-muted-foreground">
                          <ArrowUp size={14} />
                          <span className="font-mono text-xs">{post.votes}</span>
                        </div>
                        <div className="flex items-center gap-1.5 text-muted-foreground">
                          {post.type === "problem" ? (
                            <GitBranch size={14} />
                          ) : (
                            <MessageSquare size={14} />
                          )}
                          <span className="font-mono text-xs">
                            {post.responses}
                          </span>
                        </div>
                        <div className="flex items-center gap-1.5 text-muted-foreground">
                          <Eye size={14} />
                          <span className="font-mono text-xs">{post.views}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </Link>

              {/* Quick Actions - Desktop Only */}
              {hoveredPost === post.id && (
                <div className="hidden sm:flex absolute right-4 top-4 items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button
                    onClick={(e) => e.stopPropagation()}
                    className="w-8 h-8 flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
                  >
                    <Bookmark size={14} />
                  </button>
                  <button
                    onClick={(e) => e.stopPropagation()}
                    className="w-8 h-8 flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
                  >
                    <Share2 size={14} />
                  </button>
                  <button
                    onClick={(e) => e.stopPropagation()}
                    className="w-8 h-8 flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
                  >
                    <MoreHorizontal size={14} />
                  </button>
                </div>
              )}
            </article>
          );
        })}
      </div>

      {/* Load More */}
      <div className="pt-6 flex flex-col sm:flex-row items-center justify-center gap-4">
        {hasMore && (
          <button
            onClick={loadMore}
            disabled={loading}
            className="w-full sm:w-auto font-mono text-xs tracking-wider border border-border px-8 py-3 hover:bg-foreground hover:text-background hover:border-foreground transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
          >
            {loading ? (
              <>
                <Loader2 size={12} className="animate-spin" />
                LOADING...
              </>
            ) : (
              'LOAD MORE'
            )}
          </button>
        )}
        <span className="font-mono text-[10px] text-muted-foreground">
          Page {page} of {totalPages}
        </span>
      </div>
    </div>
  );
}
