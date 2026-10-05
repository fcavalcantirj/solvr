"use client";

import { useCallback, useRef } from "react";
import { Share2, UserPlus, Repeat } from "lucide-react";
import { useShare } from "@/hooks/use-share";
import { api } from "@/lib/api";
import { ShareOutcome } from "./share-outcome";

interface RoomHeaderActionsProps {
  slug: string;
  displayName?: string;
  // In-page anchor to the connection panel; the reader stays in the room.
  connectHref?: string;
  // A private room's share link is a viewing link, never an invitation — the
  // control says so (task ~513, step 4). Undefined/false = public.
  isPrivate?: boolean;
  // The API's "Try this workflow" link (GET /v1/rooms/{slug} try_workflow_url): a fresh
  // room seeded from this public room's task. null/undefined = not offered (private).
  tryWorkflowUrl?: string | null;
}

// One read of a room's share link (GET /v1/rooms/{slug}/share).
interface ShareLinkRead {
  slug: string;
  // Settles with the API's share link, or null when it composed none. Never rejects.
  pending: Promise<string | null>;
  // Set once `pending` settled, so a click can use the answer without waiting.
  settled?: { link: string | null };
}

/**
 * RoomHeaderActions renders the two header-level room controls (task 33, step 1):
 * Share and Connect an agent (an in-page link to the room's connection panel). It
 * exposes no raw tokens, protocol names, claims, or event payloads.
 *
 * Share hands over the share link the API composed: the room page with ?via=share,
 * which that page counts once per tab as a share_visit and then removes (idx 88). After
 * the share sheet or the clipboard took it, share_link_copied is reported for the room.
 * When the API composes no share link (it refuses a private room) or cannot be read,
 * Share falls back to the clean canonical room URL and reports nothing. Either way the
 * link names only the public slug, never a token or session parameter.
 *
 * For a PRIVATE room the Share control adds a note explaining recipients still
 * need authorization: the copied link is a clean canonical URL, not a bearer
 * credential or a private-room invitation (task ~513, step 4).
 */
export function RoomHeaderActions({ slug, displayName, connectHref = "#connect-agent", isPrivate = false, tryWorkflowUrl }: RoomHeaderActionsProps) {
  const { share, shared } = useShare();
  const linkRead = useRef<ShareLinkRead | null>(null);

  // readShareLink asks the API for this room's share link, once per room shown. A
  // refusal or a failed read is remembered as "no share link" just the same.
  const readShareLink = useCallback((): ShareLinkRead => {
    if (linkRead.current?.slug === slug) return linkRead.current;
    const read: ShareLinkRead = {
      slug,
      pending: Promise.resolve()
        .then(() => api.getRoomShare(slug))
        .then((res) => res?.data?.share_url || null)
        .catch(() => null)
        .then((link) => {
          read.settled = { link };
          return link;
        }),
    };
    linkRead.current = read;
    return read;
  }, [slug]);

  // A browser opens its share sheet, or writes the clipboard, only inside the click
  // itself. So the link is read when the visitor reaches for the button (pointer or
  // keyboard focus), and the click can then share at once instead of waiting for the API.
  const warmShareLink = useCallback(() => {
    readShareLink();
  }, [readShareLink]);

  const onShare = useCallback(() => {
    const title = displayName ?? "Solvr room";
    // The fallback: origin + /rooms/{slug}, with nothing attached.
    const origin = typeof window !== "undefined" ? window.location.origin : "https://solvr.dev";
    const canonical = `${origin}/rooms/${slug}`;

    const shareNow = async (link: string | null) => {
      if (!link) {
        await share(title, canonical);
        return;
      }
      // share_link_copied: reported ONLY after the share or the copy succeeded.
      if (await share(title, link)) {
        void api.postFunnelEvent?.({
          event: "share_link_copied",
          entry_surface: "room_page",
          source: { kind: "room", ref: slug },
        });
      }
    };

    const read = readShareLink();
    if (read.settled) {
      void shareNow(read.settled.link);
    } else {
      void read.pending.then(shareNow);
    }
  }, [share, slug, displayName, readShareLink]);

  return (
    <div className="flex min-w-0 flex-col items-start gap-3 lg:items-end">
      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={onShare}
          onPointerEnter={warmShareLink}
          onFocus={warmShareLink}
          className="inline-flex items-center gap-2 border border-border px-4 py-2.5 font-mono text-[11px] uppercase tracking-[0.18em] hover:border-foreground transition-colors"
        >
          <Share2 className="w-3.5 h-3.5" aria-hidden="true" />
          {shared ? "Copied" : "Share"}
        </button>
        <a
          href={connectHref}
          className="inline-flex items-center gap-2 border border-foreground bg-foreground text-background px-4 py-2.5 font-mono text-[11px] uppercase tracking-[0.18em] hover:bg-background hover:text-foreground transition-colors"
        >
          <UserPlus className="w-3.5 h-3.5" aria-hidden="true" />
          Connect an agent
        </a>
      </div>
      {tryWorkflowUrl && (
        <div className="flex flex-wrap items-center gap-2">
          <a
            href={tryWorkflowUrl}
            className="inline-flex items-center gap-2 border border-border px-4 py-2.5 font-mono text-[11px] uppercase tracking-[0.18em] hover:border-foreground transition-colors"
          >
            <Repeat className="w-3.5 h-3.5" aria-hidden="true" />
            Try this workflow
          </a>
          <ShareOutcome slug={slug} />
        </div>
      )}
      {isPrivate && (
        <p
          data-testid="share-private-note"
          className="font-mono text-[11px] leading-relaxed text-muted-foreground max-w-xs lg:text-right"
        >
          Private room — sharing the link does not grant access. Recipients still need
          the owner&apos;s authorization to read this room.
        </p>
      )}
    </div>
  );
}
