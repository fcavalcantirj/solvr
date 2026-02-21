"use client";

import { useState, useCallback, useRef, useEffect } from "react";
import Link from "next/link";
import {
  Bot,
  User,
  ArrowUp,
  MessageSquare,
  MessageCircle,
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
  Check,
  Bell,
  Flag,
} from "lucide-react";
import { ReportModal } from "@/components/ui/report-modal";
import { usePosts, useSearch, FeedPost, PostType } from "@/hooks/use-posts";
import { SearchMethodBadge } from "@/components/search/search-method-badge";
import { VoteButton } from "@/components/ui/vote-button";
import { mapStatusFilter, mapSortFilter, mapTimeframeFilter } from "@/lib/filter-utils";
import { useShare } from "@/hooks/use-share";
import { usePolling } from "@/hooks/use-polling";
import { useBookmarks } from "@/hooks/use-bookmarks";
import { api } from "@/lib/api";

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
  "UNDER REVIEW": { className: "text-yellow-600", dot: "bg-yellow-500" },
  REJECTED: { className: "text-red-600 font-medium", dot: "bg-red-500" },
};

interface FeedListProps {
  type?: PostType | 'all';
  searchQuery?: string;
  status?: string;
  sort?: string;
  timeframe?: string;
}

const POLLING_INTERVAL = 30000; // 30 seconds

export function FeedList({ type, searchQuery, status, sort, timeframe }: FeedListProps) {
  const [hoveredPost, setHoveredPost] = useState<string | null>(null);
  const [sharedPostId, setSharedPostId] = useState<string | null>(null);
  const [newPostsAvailable, setNewPostsAvailable] = useState(false);
  const [newPostsCount, setNewPostsCount] = useState(0);
  const [openMenuPostId, setOpenMenuPostId] = useState<string | null>(null);
  const [reportPostId, setReportPostId] = useState<string | null>(null);
  const latestPostIdRef = useRef<string | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);
  const { share } = useShare();
  const { bookmarkedPosts, toggleBookmark } = useBookmarks();

  // Close dropdown on outside click
  useEffect(() => {
    if (!openMenuPostId) return;
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setOpenMenuPostId(null);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [openMenuPostId]);

  const handleShare = async (post: FeedPost) => {
    const postUrl = `${window.location.origin}${typeConfig[post.type].link}/${post.id}`;
    await share(post.title, postUrl);
    setSharedPostId(post.id);
    // Reset after 2 seconds
    setTimeout(() => setSharedPostId(null), 2000);
  };

  // Use search when there's a query, otherwise use posts
  const isSearching = Boolean(searchQuery?.trim());

  const postsResult = usePosts({
    type: type === 'all' ? undefined : type,
    per_page: 20,
    status: status ? mapStatusFilter(status) : undefined,
    sort: sort ? mapSortFilter(sort) : undefined,
    timeframe: timeframe ? mapTimeframeFilter(timeframe) as 'today' | 'week' | 'month' | undefined : undefined,
  });
  
  const searchResult = useSearch(searchQuery || '', type);
  
  // Select the appropriate result based on whether we're searching
  const { posts, loading, error, total, hasMore, page, refetch, loadMore } = isSearching
    ? {
        posts: searchResult.posts,
        loading: searchResult.loading,
        error: searchResult.error,
        total: searchResult.posts.length,
        hasMore: false,
        page: 1,
        refetch: () => {},
        loadMore: () => {},
      }
    : postsResult;

  const searchMethod = isSearching ? searchResult.searchMethod : undefined;

  // Check for new posts via polling
  const checkForNewPosts = useCallback(async () => {
    if (isSearching || loading) return;

    try {
      const response = await api.getPosts({
        type: type === 'all' ? undefined : type,
        per_page: 1,
        sort: 'new',
      });

      if (response.data.length > 0) {
        const latestId = response.data[0].id;

        // If we have a reference point and it's different, we have new posts
        if (latestPostIdRef.current && latestId !== latestPostIdRef.current) {
          // Count how many new posts (rough estimate based on total)
          const currentTotal = total;
          const newTotal = response.meta.total;
          const diff = newTotal - currentTotal;

          if (diff > 0) {
            setNewPostsAvailable(true);
            setNewPostsCount(diff);
          }
        }

        // Update reference on first load
        if (!latestPostIdRef.current && posts.length > 0) {
          latestPostIdRef.current = posts[0].id;
        }
      }
    } catch {
      // Silently ignore polling errors
    }
  }, [isSearching, loading, type, total, posts]);

  // Enable polling only when not searching
  usePolling(checkForNewPosts, POLLING_INTERVAL, { enabled: !isSearching && posts.length > 0 });

  // Handle showing new posts
  const handleShowNewPosts = () => {
    setNewPostsAvailable(false);
    setNewPostsCount(0);
    latestPostIdRef.current = null;
    refetch();
  };

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
      {/* New Posts Banner */}
      {newPostsAvailable && (
        <button
          onClick={handleShowNewPosts}
          className="w-full mb-4 py-3 px-4 bg-foreground/5 border border-foreground/20 hover:bg-foreground/10 transition-colors flex items-center justify-center gap-2 group"
        >
          <Bell size={14} className="text-foreground" />
          <span className="font-mono text-xs tracking-wider">
            {newPostsCount > 1 ? `${newPostsCount} NEW POSTS AVAILABLE` : 'NEW POST AVAILABLE'}
          </span>
          <span className="font-mono text-xs text-muted-foreground group-hover:text-foreground transition-colors">
            — CLICK TO REFRESH
          </span>
        </button>
      )}

      {/* Results Count */}
      <div className="flex items-center justify-between mb-4">
        <p className="font-mono text-xs text-muted-foreground">
          Showing <span className="text-foreground">{posts.length}</span>{" "}
          of <span className="text-foreground">{total}</span> results
        </p>
        <div className="flex items-center gap-3">
          {isSearching && <SearchMethodBadge method={searchMethod} />}
          <button
            onClick={refetch}
            disabled={loading}
            className="font-mono text-xs text-muted-foreground hover:text-foreground transition-colors flex items-center gap-1.5"
          >
            <RefreshCw size={10} className={loading ? 'animate-spin' : ''} />
            {loading ? 'Updating...' : 'Refresh'}
          </button>
        </div>
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
                  <div className="hidden sm:flex w-12 flex-shrink-0">
                    <VoteButton
                      postId={post.id}
                      initialScore={post.votes}
                      initialUserVote={post.userVote}
                      direction="vertical"
                      size="md"
                      showDownvote
                    />
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
                        <div className="sm:hidden">
                          <VoteButton
                            postId={post.id}
                            initialScore={post.votes}
                            initialUserVote={post.userVote}
                            direction="horizontal"
                            size="sm"
                            showDownvote
                          />
                        </div>
                        <div className="flex items-center gap-1.5 text-muted-foreground">
                          <GitBranch size={14} />
                          <span className="font-mono text-xs">
                            {post.responses}
                          </span>
                        </div>
                        <div className="flex items-center gap-1.5 text-muted-foreground">
                          <MessageCircle size={14} />
                          <span className="font-mono text-xs">{post.comments}</span>
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
                    onClick={(e) => {
                      e.stopPropagation();
                      e.preventDefault();
                      toggleBookmark(post.id);
                    }}
                    className={`w-8 h-8 flex items-center justify-center transition-colors ${
                      bookmarkedPosts.has(post.id)
                        ? "text-foreground"
                        : "text-muted-foreground hover:text-foreground hover:bg-secondary"
                    }`}
                  >
                    <Bookmark size={14} fill={bookmarkedPosts.has(post.id) ? "currentColor" : "none"} />
                  </button>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      handleShare(post);
                    }}
                    className={`w-8 h-8 flex items-center justify-center transition-colors ${
                      sharedPostId === post.id
                        ? "text-green-500"
                        : "text-muted-foreground hover:text-foreground hover:bg-secondary"
                    }`}
                  >
                    {sharedPostId === post.id ? <Check size={14} /> : <Share2 size={14} />}
                  </button>
                  <div className="relative" ref={openMenuPostId === post.id ? menuRef : undefined}>
                    <button
                      data-testid="feed-more-button"
                      onClick={(e) => {
                        e.stopPropagation();
                        e.preventDefault();
                        setOpenMenuPostId(openMenuPostId === post.id ? null : post.id);
                      }}
                      className="w-8 h-8 flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-secondary transition-colors"
                    >
                      <MoreHorizontal size={14} />
                    </button>
                    {openMenuPostId === post.id && (
                      <div
                        data-testid="feed-more-dropdown"
                        className="absolute right-0 top-full mt-1 bg-card border border-border shadow-md z-10 min-w-[140px]"
                      >
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            e.preventDefault();
                            setOpenMenuPostId(null);
                            setReportPostId(post.id);
                          }}
                          className="w-full px-3 py-2 text-left font-mono text-xs tracking-wider hover:bg-secondary transition-colors flex items-center gap-2"
                        >
                          <Flag className="w-3 h-3" />
                          REPORT
                        </button>
                      </div>
                    )}
                  </div>
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

      {/* Report Modal */}
      <ReportModal
        isOpen={reportPostId !== null}
        onClose={() => setReportPostId(null)}
        targetType="post"
        targetId={reportPostId || ''}
        targetLabel="post"
      />
    </div>
  );
}
