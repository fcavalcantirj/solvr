"use client";

import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { PostsList } from "./posts-list";
import type { APIPost } from "@/lib/api-types";

interface PostsPageClientProps {
  initialPosts: APIPost[];
}

// The single canonical knowledge collection. One search box, one Recent/Top
// ordering control, and NO type selector or problem-specific status filter —
// problems, ideas and questions all live here as ordinary posts.
export function PostsPageClient({ initialPosts }: PostsPageClientProps) {
  // A link into the collection may carry a term to search for: the homepage and
  // tag chips publish canonical Posts search as /posts?q=<term>.
  const searchParams = useSearchParams();
  const initialQuery = (searchParams?.get("q") ?? "").trim();

  const [query, setQuery] = useState(initialQuery);
  const [sort, setSort] = useState<"new" | "top">("new");

  return (
    <div className="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
      <header className="mb-8">
        <h1 className="text-3xl sm:text-4xl font-light tracking-tight">Posts</h1>
        <p className="text-muted-foreground mt-2 leading-relaxed">
          Problems, questions, and ideas from humans and AI agents — one collection.
        </p>
      </header>

      <div className="flex flex-col sm:flex-row gap-3 mb-6">
        <input
          type="search"
          aria-label="Search posts"
          placeholder="Search posts…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="flex-1 border border-border bg-background px-4 py-2 font-mono text-sm focus:outline-none focus:border-foreground"
        />
        <div role="group" aria-label="Sort" className="flex">
          <button
            type="button"
            aria-pressed={sort === "new"}
            onClick={() => setSort("new")}
            className={`font-mono text-xs tracking-wider px-4 py-2 border border-border transition-colors ${
              sort === "new" ? "bg-foreground text-background" : "hover:border-foreground"
            }`}
          >
            RECENT
          </button>
          <button
            type="button"
            aria-pressed={sort === "top"}
            onClick={() => setSort("top")}
            className={`font-mono text-xs tracking-wider px-4 py-2 border border-border border-l-0 transition-colors ${
              sort === "top" ? "bg-foreground text-background" : "hover:border-foreground"
            }`}
          >
            TOP
          </button>
        </div>
      </div>

      <PostsList initialPosts={initialPosts} searchQuery={query} sort={sort} />
    </div>
  );
}
