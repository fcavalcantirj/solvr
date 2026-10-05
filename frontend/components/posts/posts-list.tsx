"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { track } from "@/lib/analytics";
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
//
// It also says what the visitor did with the list (SPEC.md 27.7), each time only after the
// list that was asked for arrived:
//   search       a term of at least two characters the visitor settled on, with the number
//                of posts that matched. Never the term the page opened with (a link brought
//                it), never the same term twice in a row, never an answer that came late.
//   sort_change  the list is now in another order than the one it last showed.
//   load_more    another page was added.
export function PostsList({ initialPosts = [], searchQuery, sort }: PostsListProps) {
  // A search result also carries the replies that matched; a browsed post has none.
  const [posts, setPosts] = useState<Array<APIPost & { matched_replies?: APISearchReplyMatch[] }>>(initialPosts);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // The newest read of a first page. An answer to an older one reports nothing.
  const latestRead = useRef(0);
  // The term and the order of the list last shown; null until the first list arrived.
  const shown = useRef<{ query: string; sort: string } | null>(null);
  // The term the page opened with came from a link, not from the visitor; it counts as a
  // search only once the visitor has searched for something else.
  const openedWith = useRef(searchQuery.trim());
  const visitorSearched = useRef(false);

  const fetchPage = useCallback(
    async (targetPage: number, replace: boolean) => {
      setLoading(true);
      setError(null);
      const q = searchQuery.trim();
      const read = replace ? ++latestRead.current : latestRead.current;
      if (q !== openedWith.current) visitorSearched.current = true;
      try {
        const res = q
          ? await api.search({ q, page: targetPage, per_page: PER_PAGE })
          : await api.getPosts({ sort, page: targetPage, per_page: PER_PAGE });
        setPosts((prev) => (replace ? res.data : [...prev, ...res.data]));
        setHasMore(res.meta.has_more);
        setPage(targetPage);

        if (read !== latestRead.current) return;
        if (!replace) {
          track("load_more", { list: "posts", page: targetPage });
          return;
        }
        const before = shown.current;
        shown.current = { query: q, sort };
        if (q.length >= 2 && visitorSearched.current && before?.query !== q) {
          track("search", { search_term: q, results: res.meta.total, list: "posts" });
        }
        if (before && before.sort !== sort) track("sort_change", { list: "posts", sort });
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
