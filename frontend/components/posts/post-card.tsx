"use client";

import Link from "next/link";
import { User, MessageSquare, ArrowUp } from "lucide-react";
import { formatRelativeTime, truncateText } from "@/lib/api";
import type { APIPost, APISearchReplyMatch } from "@/lib/api-types";
import { profileHref } from '@/lib/profile-href';

// The API computes the unified reply count server-side (reply_count). The
// client is dumb: it displays what the API returns and never sums the legacy
// answer/approach/comment counters itself.
function replyCount(post: APIPost): number {
  return post.reply_count ?? 0;
}

// Renders an API excerpt whose matched words the API wrapped in <mark>...</mark>: the text
// is always rendered as text (never as HTML), the marked words inside <mark> elements.
function Excerpt({ text }: { text: string }) {
  let marked = false;
  return (
    <>
      {text.split(/(<mark>|<\/mark>)/).map((part, i) => {
        if (part === "<mark>" || part === "</mark>") {
          marked = part === "<mark>";
          return null;
        }
        return marked ? (
          <mark key={i} className="bg-foreground text-background px-0.5">
            {part}
          </mark>
        ) : (
          part
        );
      })}
    </>
  );
}

// One card for every post, regardless of its historical problem/idea/question
// origin. There is no per-type badge routing and every card links to the same
// canonical /posts/{id} detail. A search result also lists the replies that
// matched (matchedReplies), each linking to the reply on the post.
export function PostCard({
  post,
  matchedReplies,
}: {
  post: APIPost;
  matchedReplies?: APISearchReplyMatch[];
}) {
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

      {matchedReplies && matchedReplies.length > 0 && (
        <ul aria-label="Matching replies" className="mt-4 space-y-3 border-l border-border pl-4">
          {matchedReplies.map((reply) => (
            <li key={reply.id}>
              <Link
                href={reply.url}
                className="block text-sm text-muted-foreground leading-relaxed hover:text-foreground"
              >
                <Excerpt text={reply.snippet} />
              </Link>
              <p className="font-mono text-[10px] tracking-wider text-muted-foreground mt-1">
                {reply.author.display_name}
                {" · "}
                <time dateTime={reply.created_at}>{formatRelativeTime(reply.created_at)}</time>
                {reply.legacy_type &&
                  ` · ${reply.legacy_type}${reply.legacy_status ? ` ${reply.legacy_status}` : ""}`}
              </p>
            </li>
          ))}
        </ul>
      )}

      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 mt-4">
        <Link
          href={profileHref(post.author)}
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
