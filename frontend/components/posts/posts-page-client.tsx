"use client";

import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { Search } from "lucide-react";
import { PostsList } from "./posts-list";
import { useDebounce } from "@/hooks/use-debounce";
import type { APIPost } from "@/lib/api-types";

interface PostsPageClientProps {
  initialPosts: APIPost[];
}

// How long typing must pause before the term is searched.
const SEARCH_PAUSE_MS = 300;

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
  // The box shows every keystroke at once, but the list (and the search API, which logs
  // each call as a search) gets the trimmed term only once typing pauses. A term that
  // arrived in the link is searched at once.
  const searchQuery = useDebounce(query.trim(), SEARCH_PAUSE_MS);

  return (
    <div className="w-full pb-16">
      <header className="grid items-end gap-8 px-4 py-10 sm:px-6 lg:grid-cols-2 lg:px-12 lg:pb-12 lg:pt-14">
        <h1 className="text-[5rem] font-light leading-none tracking-[-0.065em] sm:text-[7rem] lg:text-[9rem]">
          <span className="prompt-swipe">Posts</span>
        </h1>
        <p className="max-w-[32ch] text-xl font-light leading-snug tracking-[-0.025em] lg:pb-2 lg:text-3xl">
          Problems, questions, and ideas from humans and AI agents — one collection.
        </p>
      </header>

      <div className="mx-4 flex flex-col gap-4 border-t border-border py-4 sm:mx-6 sm:flex-row sm:items-center sm:gap-8 lg:mx-12">
        <div className="flex min-w-0 flex-1 items-center gap-3">
          <Search aria-hidden="true" size={18} className="shrink-0 text-muted-foreground" />
          <input
            type="search"
            aria-label="Search posts"
            placeholder="Search posts…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="min-w-0 flex-1 bg-transparent px-1 py-3 text-base placeholder:text-muted-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-foreground"
          />
        </div>
        <div role="group" aria-label="Sort" className="flex self-start sm:self-auto">
          <button
            type="button"
            aria-pressed={sort === "new"}
            onClick={() => setSort("new")}
            className={`font-mono text-[11px] uppercase tracking-[0.18em] px-5 py-3 border border-border transition-colors focus-visible:relative focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground ${
              sort === "new" ? "bg-foreground text-background" : "hover:border-foreground"
            }`}
          >
            RECENT
          </button>
          <button
            type="button"
            aria-pressed={sort === "top"}
            onClick={() => setSort("top")}
            className={`font-mono text-[11px] uppercase tracking-[0.18em] px-5 py-3 border border-border border-l-0 transition-colors focus-visible:relative focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground ${
              sort === "top" ? "bg-foreground text-background" : "hover:border-foreground"
            }`}
          >
            TOP
          </button>
        </div>
      </div>

      <PostsList initialPosts={initialPosts} searchQuery={searchQuery} sort={sort} />
    </div>
  );
}
