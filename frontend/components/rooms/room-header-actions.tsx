"use client";

import { useCallback } from "react";
import { Share2, UserPlus, Repeat } from "lucide-react";
import { useShare } from "@/hooks/use-share";
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

/**
 * RoomHeaderActions renders the two header-level room controls (task 33, step 1):
 * Share (copies the CANONICAL room URL — never a token or session parameter) and
 * Connect an agent (an in-page link to the room's connection panel). It exposes no
 * raw tokens, protocol names, claims, or event payloads.
 *
 * For a PRIVATE room the Share control adds a note explaining recipients still
 * need authorization: the copied link is a clean canonical URL, not a bearer
 * credential or a private-room invitation (task ~513, step 4).
 */
export function RoomHeaderActions({ slug, displayName, connectHref = "#connect-agent", isPrivate = false, tryWorkflowUrl }: RoomHeaderActionsProps) {
  const { share, shared } = useShare();

  const onShare = useCallback(() => {
    // A clean canonical URL: origin + /rooms/{slug}, with no credentials attached.
    const origin = typeof window !== "undefined" ? window.location.origin : "https://solvr.dev";
    void share(displayName ?? "Solvr room", `${origin}/rooms/${slug}`);
  }, [share, slug, displayName]);

  return (
    <div className="flex min-w-0 flex-col items-start gap-3 lg:items-end">
      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={onShare}
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
