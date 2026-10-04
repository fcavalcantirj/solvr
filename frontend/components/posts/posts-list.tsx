"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import type { APIPost, APISearchReplyMatch } from "@/lib/api-types";
import { PostCard } from "./post-card";
import styles from "./posts-mosaic.module.css";

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
      <div className="border-t border-border px-4 py-24 text-center">
        <p className="text-base text-muted-foreground">{error}</p>
        <button
          type="button"
          onClick={() => fetchPage(1, true)}
          className="mt-6 font-mono text-[11px] uppercase tracking-[0.18em] border border-border px-6 py-3 hover:bg-foreground hover:text-background transition-colors focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
        >
          RETRY
        </button>
      </div>
    );
  }

  if (!loading && posts.length === 0) {
    return (
      <div className="border-t border-border px-4 py-24 text-center">
        <p className="text-base text-muted-foreground">No posts found.</p>
      </div>
    );
  }

  return (
    <div>
      <div className={styles.mosaic}>
        {posts.map((post) => (
          <PostCard key={post.id} post={post} matchedReplies={post.matched_replies} />
        ))}
      </div>
      {hasMore && (
        <div className="px-4 pt-10 text-center sm:px-6 lg:px-12">
          <button
            type="button"
            onClick={() => fetchPage(page + 1, false)}
            disabled={loading}
            className="w-full font-mono text-[11px] uppercase tracking-[0.18em] border border-border px-8 py-5 hover:bg-foreground hover:text-background disabled:opacity-50 transition-colors focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
          >
            {loading ? "LOADING…" : "LOAD MORE"}
          </button>
        </div>
      )}
    </div>
  );
}
