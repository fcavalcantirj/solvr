"use client";

import Link from "next/link";
import { User, MessageSquare, ArrowUp } from "lucide-react";
import { formatRelativeTime, truncateText } from "@/lib/api";
import type { APIPost } from "@/lib/api-types";

// The API computes the unified reply count server-side (reply_count). The
// client is dumb: it displays what the API returns and never sums the legacy
// answer/approach/comment counters itself.
function replyCount(post: APIPost): number {
  return post.reply_count ?? 0;
}

// One card for every post, regardless of its historical problem/idea/question
// origin. There is no per-type badge routing and every card links to the same
// canonical /posts/{id} detail.
export function PostCard({ post }: { post: APIPost }) {
  const replies = replyCount(post);
  const replyLabel = `${replies} ${replies === 1 ? "reply" : "replies"}`;

  return (
    <article className="border border-border p-5 hover:border-foreground transition-colors">
      <Link href={`/posts/${post.id}`} className="block group">
        <h2 className="text-lg font-light tracking-tight group-hover:underline">
          {post.title}
        </h2>
        <p className="text-sm text-muted-foreground mt-2 leading-relaxed">
          {truncateText(post.description, 180)}
        </p>
      </Link>

      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 mt-4">
        <Link
          href={`/users/${post.author.id}`}
          className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground hover:text-foreground"
        >
          <User size={12} />
          {post.author.display_name}
        </Link>
        <time
          dateTime={post.created_at}
          className="font-mono text-xs text-muted-foreground"
        >
          {formatRelativeTime(post.created_at)}
        </time>
        <span className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground">
          <ArrowUp size={12} />
          {post.vote_score}
        </span>
        <span
          className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground"
          aria-label={replyLabel}
        >
          <MessageSquare size={12} />
          {replyLabel}
        </span>
      </div>

      {post.tags && post.tags.length > 0 && (
        <div className="flex flex-wrap gap-2 mt-3">
          {post.tags.map((tag) => (
            <Link
              key={tag}
              href={`/posts?q=${encodeURIComponent(tag)}`}
              className="font-mono text-[10px] tracking-wider px-2 py-1 border border-border text-muted-foreground hover:text-foreground hover:border-foreground transition-colors"
            >
              #{tag}
            </Link>
          ))}
        </div>
      )}
    </article>
  );
}
