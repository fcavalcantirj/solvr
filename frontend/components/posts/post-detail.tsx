"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { User, ArrowUp, MessageSquare, Users, Archive, Pencil } from "lucide-react";
import { api, formatRelativeTime } from "@/lib/api";
import type { APIPost, APIPostSourceRoom, APIReply, APIRoom } from "@/lib/api-types";
import { MarkdownContent } from "@/components/shared/markdown-content";
import { resolveLegacyAnchor } from "@/lib/legacy-anchor";
import { profileHref } from "@/lib/profile-href";

// What the server already read for the page (task idx 81): with it the post, its
// first replies page and its rooms are in the server HTML, and nothing is refetched.
export interface PostDetailInitial {
  post: APIPost;
  replies: APIReply[];
  rooms: APIRoom[];
  // How many reply pages the API counts; page 2 onward live at /posts/{id}/replies/{n}.
  replyPages: number;
  // The public room the post was saved from (task idx 82), or null.
  sourceRoom?: APIPostSourceRoom | null;
}


// One detail layout for every post, whatever its historical origin. Everything
// shown is server-owned data; the client only renders it.
export function PostDetail({ postId, initial }: { postId: string; initial?: PostDetailInitial }) {
  const [post, setPost] = useState<APIPost | null>(initial?.post ?? null);
  const [replies, setReplies] = useState<APIReply[]>(initial?.replies ?? []);
  const [relatedRooms, setRelatedRooms] = useState<APIRoom[]>(initial?.rooms ?? []);
  const [sourceRoom, setSourceRoom] = useState<APIPostSourceRoom | null>(initial?.sourceRoom ?? null);
  const [replyPages] = useState(initial?.replyPages ?? 1);
  const [loading, setLoading] = useState(!initial);
  const [error, setError] = useState(false);
  const [hasInitial] = useState(initial !== undefined);

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
      if (rooms.status === "fulfilled") {
        setRelatedRooms(rooms.value.data ?? []);
        setSourceRoom(rooms.value.source_room ?? null);
      }
    } catch {
      setError(true);
    } finally {
      setLoading(false);
    }
  }, [postId]);

  useEffect(() => {
    if (!hasInitial) load();
  }, [load, hasInitial]);

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
    <article className="mx-auto max-w-[76rem] space-y-10">
      {notPublic && (
        <div role="status" className="max-w-[44rem] border-l border-amber-700 pl-3 text-sm text-amber-700 dark:border-amber-400 dark:text-amber-400">
          Awaiting moderation — this post is not yet publicly discoverable. Public discovery waits for approved moderation.
        </div>
      )}

      <header className="space-y-8">
        <h1 className="max-w-[22ch] text-[2.5rem] font-light leading-[1.05] tracking-[-0.04em] [overflow-wrap:anywhere] sm:text-[3.5rem] lg:text-[4.5rem]">{post.title}</h1>
        <div className="flex flex-wrap items-center gap-x-5 gap-y-2 border-y border-border py-4">
          <Link
            href={profileHref(post.author)}
            className="inline-flex items-center gap-1.5 font-mono text-[11px] tracking-[0.06em] text-muted-foreground hover:text-foreground"
          >
            <User size={12} />
            {post.author.display_name}
          </Link>
          <time dateTime={post.created_at} className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground" suppressHydrationWarning>
            {formatRelativeTime(post.created_at)}
          </time>
          <span className="inline-flex items-center gap-1.5 font-mono text-[11px] tracking-[0.06em] text-muted-foreground">
            <ArrowUp size={12} />
            {post.vote_score}
          </span>
          <Link
            href={`/posts/${post.id}/edit`}
            className="inline-flex items-center gap-1.5 font-mono text-[11px] tracking-[0.06em] text-muted-foreground hover:text-foreground"
          >
            <Pencil size={12} />
            Edit
          </Link>
        </div>
      </header>

      <div className="min-w-0 max-w-[44rem] space-y-10 [overflow-wrap:anywhere]">
      <MarkdownContent content={post.description} className="text-[1.0625rem] leading-relaxed" />

      {post.tags && post.tags.length > 0 && (
        <div className="flex flex-wrap gap-x-5 gap-y-2">
          {post.tags.map((tag) => (
            <Link
              key={tag}
              href={`/posts?q=${encodeURIComponent(tag)}`}
              className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground underline-offset-4 transition-colors hover:text-foreground hover:underline"
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
          className="inline-flex items-center gap-2 border border-foreground px-4 py-2.5 font-mono text-[11px] uppercase tracking-[0.18em] transition-colors hover:bg-foreground hover:text-background"
        >
          <Users size={12} /> Try this workflow
        </Link>
      </section>

      {sourceRoom && (
        <p className="border-t border-border pt-6 font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground inline-flex items-center gap-2">
          <Users size={12} /> SAVED FROM{" "}
          <Link href={`/rooms/${sourceRoom.slug}`} className="text-foreground hover:underline">
            {sourceRoom.display_name}
          </Link>
        </p>
      )}

      {relatedRooms.length > 0 && (
        <section className="border-t border-border pt-6 space-y-3">
          <h2 className="font-mono text-[11px] uppercase tracking-[0.18em] text-foreground inline-flex items-center gap-2">
            <Users size={12} /> RELATED ROOMS
          </h2>
          <ul className="space-y-2">
            {relatedRooms.map((room) => (
              <li key={room.id}>
                <Link href={`/rooms/${room.slug}`} className="text-lg font-light tracking-[-0.01em] underline-offset-4 hover:underline">
                  {room.display_name}
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}

      {post.crystallization_cid && (
        <section className="border-t border-border pt-6 space-y-2">
          <h2 className="font-mono text-[11px] uppercase tracking-[0.18em] text-foreground inline-flex items-center gap-2">
            <Archive size={12} /> SAVED SNAPSHOT
          </h2>
          <p className="text-sm text-muted-foreground">
            An immutable saved copy exists{post.crystallized_at ? ` (${formatRelativeTime(post.crystallized_at)})` : ""}. The live post may differ from it.
          </p>
          <a
            href={`https://ipfs.io/ipfs/${post.crystallization_cid}`}
            target="_blank"
            rel="noopener noreferrer"
            className="font-mono text-[11px] tracking-[0.06em] underline underline-offset-4 break-all"
          >
            {post.crystallization_cid}
          </a>
        </section>
      )}

      <section className="border-t border-border pt-6 space-y-4">
        <h2 className="font-mono text-[11px] uppercase tracking-[0.18em] text-foreground inline-flex items-center gap-2">
          <MessageSquare size={12} /> REPLIES ({post.reply_count ?? replies.length})
        </h2>
        {replies.length === 0 ? (
          <p className="text-base text-muted-foreground">No replies yet.</p>
        ) : (
          <ul className="border-b border-border">
            {replies.map((r) => (
              <li id={r.id} key={r.id} className="border-t border-border py-6 space-y-3 scroll-mt-24">
                <div className="flex items-center gap-3">
                  <Link
                    href={profileHref(r.author)}
                    className="text-sm text-foreground underline-offset-4 hover:underline"
                  >
                    {r.author.display_name}
                  </Link>
                  <time dateTime={r.created_at} className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground" suppressHydrationWarning>
                    {formatRelativeTime(r.created_at)}
                  </time>
                </div>
                <MarkdownContent content={r.body} variant="compact" />
              </li>
            ))}
          </ul>
        )}
        {replyPages > 1 && (
          <Link
            href={`/posts/${post.id}/replies/2`}
            rel="next"
            className="inline-block font-mono text-[11px] uppercase tracking-[0.18em] underline underline-offset-4 hover:text-muted-foreground"
          >
            {`Later replies (page 2 of ${replyPages})`}
          </Link>
        )}
      </section>
      </div>
    </article>
  );
}
