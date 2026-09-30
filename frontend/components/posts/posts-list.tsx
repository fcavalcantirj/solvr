"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { APIPost, APISearchReplyMatch } from "@/lib/api-types";
import { PostCard } from "./post-card";

const PER_PAGE = 20;

interface PostsListProps {
  initialPosts?: APIPost[];
  // A trimmed search term, or "" to browse the whole collection.
  searchQuery: string;
  // Recent ("new") or Top ("top") browse ordering. Ignored while searching,
  // where the API orders by relevance.
  sort: "new" | "top";
}

// Renders the canonical Posts collection. All ordering, filtering and search is
// the API's job; this component only fetches, paginates and displays.
export function PostsList({ initialPosts = [], searchQuery, sort }: PostsListProps) {
  // A search result also carries the replies that matched; a browsed post has none.
  const [posts, setPosts] = useState<Array<APIPost & { matched_replies?: APISearchReplyMatch[] }>>(initialPosts);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchPage = useCallback(
    async (targetPage: number, replace: boolean) => {
      setLoading(true);
      setError(null);
      try {
        const q = searchQuery.trim();
        const res = q
          ? await api.search({ q, page: targetPage, per_page: PER_PAGE })
          : await api.getPosts({ sort, page: targetPage, per_page: PER_PAGE });
        setPosts((prev) => (replace ? res.data : [...prev, ...res.data]));
        setHasMore(res.meta.has_more);
        setPage(targetPage);
      } catch {
        setError("Could not load posts.");
      } finally {
        setLoading(false);
      }
    },
    [searchQuery, sort],
  );

  // Refetch from page 1 whenever the query or sort changes.
  useEffect(() => {
    fetchPage(1, true);
  }, [fetchPage]);

  if (error) {
    return (
      <div className="py-12 text-center">
        <p className="font-mono text-sm text-muted-foreground">{error}</p>
        <button
          type="button"
          onClick={() => fetchPage(1, true)}
          className="mt-4 font-mono text-xs tracking-wider border border-border px-4 py-2 hover:bg-foreground hover:text-background transition-colors"
        >
          RETRY
        </button>
      </div>
    );
  }

  if (!loading && posts.length === 0) {
    return (
      <div className="py-12 text-center">
        <p className="font-mono text-sm text-muted-foreground">No posts found.</p>
      </div>
    );
  }

  return (
    <div>
      <div className="flex flex-col gap-4">
        {posts.map((post) => (
          <PostCard key={post.id} post={post} matchedReplies={post.matched_replies} />
        ))}
      </div>
      {hasMore && (
        <div className="mt-6 text-center">
          <button
            type="button"
            onClick={() => fetchPage(page + 1, false)}
            disabled={loading}
            className="font-mono text-xs tracking-wider border border-border px-6 py-3 hover:bg-foreground hover:text-background disabled:opacity-50 transition-colors"
          >
            {loading ? "LOADING…" : "LOAD MORE"}
          </button>
        </div>
      )}
    </div>
  );
}
