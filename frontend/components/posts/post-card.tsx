"use client";

import Link from "next/link";
import { MessageSquare, ArrowUp, ArrowUpRight } from "lucide-react";
import { formatRelativeTime, truncateText } from "@/lib/api";
import type { APIPost, APISearchReplyMatch } from "@/lib/api-types";
import { AuthorLink } from '@/lib/profile-href';
import styles from "./posts-mosaic.module.css";

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
          <mark key={i} className="bg-prompt-accent text-prompt-accent-foreground px-0.5">
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
    <article className={styles.tile}>
      <h2 className={styles.title}>
        <Link href={`/posts/${post.id}`} aria-label={post.title} className={styles.titleLink}>
          {post.title}
          <ArrowUpRight aria-hidden="true" className={styles.arrow} strokeWidth={1} />
        </Link>
      </h2>
      <p className={styles.excerpt}>
        {truncateText(post.description, 180)}
      </p>

      {matchedReplies && matchedReplies.length > 0 && (
        <ul aria-label="Matching replies" className="mt-6 space-y-4 border-l border-current pl-4">
          {matchedReplies.map((reply) => (
            <li key={reply.id}>
              <Link
                href={reply.url}
                className="block text-sm leading-relaxed hover:underline focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-4 focus-visible:outline-current"
              >
                <Excerpt text={reply.snippet} />
              </Link>
              <p className="font-mono text-[11px] uppercase tracking-[0.18em] opacity-70 mt-2">
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

      <div className={styles.footer}>
        <div className={styles.byline}>
          <AuthorLink
            author={post.author}
            className="min-w-0 hover:underline"
          >
            {post.author.display_name}
          </AuthorLink>
          <time
            dateTime={post.created_at}
            className="opacity-70"
          >
            {formatRelativeTime(post.created_at)}
          </time>
        </div>
        <div className={styles.counts}>
          <span className="inline-flex items-center gap-1.5">
            <ArrowUp aria-hidden="true" size={13} />
            {post.vote_score}
          </span>
          <span
            className="inline-flex items-center gap-1.5"
            aria-label={replyLabel}
          >
            <MessageSquare aria-hidden="true" size={13} />
            {replyLabel}
          </span>
        </div>

        {post.tags && post.tags.length > 0 && (
          <div className={styles.tags}>
            {post.tags.map((tag) => (
              <Link
                key={tag}
                href={`/posts?q=${encodeURIComponent(tag)}`}
                className="py-1 underline decoration-transparent underline-offset-4 transition-colors hover:decoration-current"
              >
                #{tag}
              </Link>
            ))}
          </div>
        )}
      </div>
    </article>
  );
}
