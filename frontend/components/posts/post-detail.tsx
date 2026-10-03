"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { User, ArrowUp, MessageSquare, Users, Archive, Pencil } from "lucide-react";
import { api, formatRelativeTime } from "@/lib/api";
import type { APIPost, APIReply, APIRoom } from "@/lib/api-types";
import { MarkdownContent } from "@/components/shared/markdown-content";
import { resolveLegacyAnchor } from "@/lib/legacy-anchor";

// One detail layout for every post, whatever its historical origin. Everything
// shown is server-owned data; the client only renders it.
export function PostDetail({ postId }: { postId: string }) {
  const [post, setPost] = useState<APIPost | null>(null);
  const [replies, setReplies] = useState<APIReply[]>([]);
  const [relatedRooms, setRelatedRooms] = useState<APIRoom[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(false);
    try {
      const res = await api.getPost(postId);
      setPost(res.data);
      const [rep, rooms] = await Promise.allSettled([
        api.getPostReplies(postId),
        api.getRelatedRooms(postId),
      ]);
      if (rep.status === "fulfilled") setReplies(rep.value.data ?? []);
      if (rooms.status === "fulfilled") setRelatedRooms(rooms.value.data ?? []);
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [postId]);

  useEffect(() => {
    load();
  }, [load]);

  // Legacy contribution deep links (#approach-<id>, #answer-<id>, …) arrive here
  // after a legacy detail URL is redirected to this canonical post. Resolve the
  // anchor through the migration mapping the API carries on each reply and, when
  // a matching reply is loaded, scroll to it. Bare canonical anchors fall back
  // to a direct element lookup. Best-effort — a missing anchor is a no-op.
  useEffect(() => {
    if (replies.length === 0 || typeof window === "undefined") return;
    const hash = window.location.hash;
    if (!hash) return;
    const canonicalId = resolveLegacyAnchor(hash, replies) ?? hash.slice(1);
    document.getElementById(canonicalId)?.scrollIntoView?.();
  }, [replies]);

  if (loading) {
    return <div className="py-20 text-center font-mono text-sm text-muted-foreground">Loading…</div>;
  }

  if (error || !post) {
    return (
      <div className="py-20 text-center space-y-4">
        <p className="font-mono text-sm text-muted-foreground">Could not load this post.</p>
        <button
          type="button"
          onClick={load}
          className="px-4 py-2 border border-border font-mono text-xs hover:bg-foreground/5 transition-colors"
        >
          RETRY
        </button>
      </div>
    );
  }

  const notPublic =
    (post.moderation_state && post.moderation_state !== "approved") ||
    (post.publication_state && post.publication_state !== "published");

  return (
    <article className="max-w-3xl mx-auto space-y-8">
      {notPublic && (
        <div className="p-4 border border-yellow-500/30 bg-yellow-500/10 font-mono text-xs text-yellow-600 dark:text-yellow-400">
          Awaiting moderation — this post is not yet publicly discoverable. Public discovery waits for approved moderation.
        </div>
      )}

      <header className="space-y-4">
        <h1 className="text-2xl sm:text-3xl font-light tracking-tight">{post.title}</h1>
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
          <Link
            href={`/users/${post.author.id}`}
            className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground hover:text-foreground"
          >
            <User size={12} />
            {post.author.display_name}
          </Link>
          <time dateTime={post.created_at} className="font-mono text-xs text-muted-foreground">
            {formatRelativeTime(post.created_at)}
          </time>
          <span className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground">
            <ArrowUp size={12} />
            {post.vote_score}
          </span>
          <Link
            href={`/posts/${post.id}/edit`}
            className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground hover:text-foreground"
          >
            <Pencil size={12} />
            Edit
          </Link>
        </div>
      </header>

      <MarkdownContent content={post.description} />

      {post.tags && post.tags.length > 0 && (
        <div className="flex flex-wrap gap-2">
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

      {/* idx 88: start a fresh room seeded from this post. The API decides whether the
          post may seed one (only a public post does); the link only names it. */}
      <section className="border-t border-border pt-6">
        <Link
          href={`/connect?post=${encodeURIComponent(post.id)}`}
          className="inline-flex items-center gap-2 font-mono text-xs tracking-wider border border-border px-3 py-1.5 hover:bg-muted transition-colors"
        >
          <Users size={12} /> Try this workflow
        </Link>
      </section>

      {relatedRooms.length > 0 && (
        <section className="border-t border-border pt-6 space-y-3">
          <h2 className="font-mono text-xs tracking-wider text-muted-foreground inline-flex items-center gap-2">
            <Users size={12} /> RELATED ROOMS
          </h2>
          <ul className="space-y-2">
            {relatedRooms.map((room) => (
              <li key={room.id}>
                <Link href={`/rooms/${room.slug}`} className="font-mono text-sm hover:underline">
                  {room.display_name}
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}

      {post.crystallization_cid && (
        <section className="border-t border-border pt-6 space-y-2">
          <h2 className="font-mono text-xs tracking-wider text-muted-foreground inline-flex items-center gap-2">
            <Archive size={12} /> SAVED SNAPSHOT
          </h2>
          <p className="font-mono text-xs text-muted-foreground">
            An immutable saved copy exists{post.crystallized_at ? ` (${formatRelativeTime(post.crystallized_at)})` : ""}. The live post may differ from it.
          </p>
          <a
            href={`https://ipfs.io/ipfs/${post.crystallization_cid}`}
            target="_blank"
            rel="noopener noreferrer"
            className="font-mono text-xs underline underline-offset-4 break-all"
          >
            {post.crystallization_cid}
          </a>
        </section>
      )}

      <section className="border-t border-border pt-6 space-y-4">
        <h2 className="font-mono text-xs tracking-wider text-muted-foreground inline-flex items-center gap-2">
          <MessageSquare size={12} /> REPLIES ({post.reply_count ?? replies.length})
        </h2>
        {replies.length === 0 ? (
          <p className="font-mono text-sm text-muted-foreground">No replies yet.</p>
        ) : (
          <ul className="space-y-4">
            {replies.map((r) => (
              <li id={r.id} key={r.id} className="border border-border p-4 space-y-2 scroll-mt-24">
                <div className="flex items-center gap-3">
                  <Link
                    href={`/users/${r.author.id}`}
                    className="font-mono text-xs text-muted-foreground hover:text-foreground"
                  >
                    {r.author.display_name}
                  </Link>
                  <time dateTime={r.created_at} className="font-mono text-xs text-muted-foreground">
                    {formatRelativeTime(r.created_at)}
                  </time>
                </div>
                <MarkdownContent content={r.body} variant="compact" />
              </li>
            ))}
          </ul>
        )}
      </section>
    </article>
  );
}
