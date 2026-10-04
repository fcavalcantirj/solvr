"use client";

import Link from "next/link";
import { ArrowUp, FileText, Loader2 } from "lucide-react";
import type { UserPostData } from "@/hooks/use-user";
import { CAPTION } from "@/components/page/caption";
import { cn } from "@/lib/utils";

export interface UserPostsListProps {
  posts: UserPostData[];
  loading?: boolean;
}

// Every post has one page, /posts/{id} (idx 68).
function getPostPath(post: UserPostData): string {
  return `/posts/${post.id}`;
}

// Icon for a post
function PostTypeIcon(_: { type: UserPostData['type'] }) {
  return <FileText size={12} />;
}

// The person's posts as a hairline ledger: the score as the row's figure, the
// title as its line.
export function UserPostsList({ posts, loading = false }: UserPostsListProps) {
  if (loading) {
    return (
      <div className="flex items-center gap-3 border-t border-border py-12">
        <Loader2 size={18} className="text-muted-foreground animate-spin" />
        <p className={CAPTION}>Loading posts...</p>
      </div>
    );
  }

  if (posts.length === 0) {
    return (
      <div className="border-t border-border py-16">
        <FileText size={32} strokeWidth={1} className="mb-6 text-muted-foreground" />
        <p className="text-3xl font-light tracking-[-0.025em]">No posts yet</p>
      </div>
    );
  }

  return (
    <div className="border-t border-border">
      {posts.map((post) => (
        <article key={post.id} className="grid min-w-0 grid-cols-[3.5rem_minmax(0,1fr)] gap-x-4 border-b border-border py-6 sm:grid-cols-[6rem_minmax(0,1fr)] sm:gap-x-8">
          {/* Vote score */}
          <div className="flex flex-col items-start">
            <span className="text-3xl font-light leading-none tracking-[-0.04em] tabular-nums sm:text-5xl">{post.voteScore}</span>
            <ArrowUp size={14} className="mt-2 text-muted-foreground" />
          </div>

          {/* Content */}
          <div className="min-w-0">
            {/* Type badge and title */}
            <span className={cn(CAPTION, "inline-flex items-center gap-1.5")}>
              <PostTypeIcon type={post.type} />
              {post.type}
            </span>

            <Link
              href={getPostPath(post)}
              className="mt-2 block text-xl font-light leading-snug tracking-[-0.02em] underline decoration-transparent decoration-1 underline-offset-[5px] transition-colors hover:decoration-current focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-4 focus-visible:outline-foreground [overflow-wrap:anywhere] sm:text-2xl"
            >
              {post.title}
            </Link>

            {/* Tags */}
            {post.tags.length > 0 && (
              <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1">
                {post.tags.map((tag) => (
                  <span
                    key={tag}
                    className="font-mono text-[11px] text-muted-foreground"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}

            {/* Meta */}
            <div className={cn(CAPTION, "mt-3")}>
              {post.time} • {post.views} views
            </div>
          </div>
        </article>
      ))}
    </div>
  );
}
