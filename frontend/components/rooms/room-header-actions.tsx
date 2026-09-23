"use client";

import { useCallback } from "react";
import { Share2, UserPlus } from "lucide-react";
import { useShare } from "@/hooks/use-share";

interface RoomHeaderActionsProps {
  slug: string;
  displayName?: string;
  // In-page anchor to the connection panel; the reader stays in the room.
  connectHref?: string;
}

/**
 * RoomHeaderActions renders the two header-level room controls (task 33, step 1):
 * Share (copies the CANONICAL room URL — never a token or session parameter) and
 * Connect an agent (an in-page link to the room's connection panel). It exposes no
 * raw tokens, protocol names, claims, or event payloads.
 */
export function RoomHeaderActions({ slug, displayName, connectHref = "#connect-agent" }: RoomHeaderActionsProps) {
  const { share, shared } = useShare();

  const onShare = useCallback(() => {
    // A clean canonical URL: origin + /rooms/{slug}, with no credentials attached.
    const origin = typeof window !== "undefined" ? window.location.origin : "https://solvr.dev";
    void share(displayName ?? "Solvr room", `${origin}/rooms/${slug}`);
  }, [share, slug, displayName]);

  return (
    <div className="flex items-center gap-2">
      <button
        type="button"
        onClick={onShare}
        className="inline-flex items-center gap-1.5 border border-border px-3 py-1.5 font-mono text-xs tracking-wider hover:bg-muted transition-colors"
      >
        <Share2 className="w-3.5 h-3.5" aria-hidden="true" />
        {shared ? "Copied" : "Share"}
      </button>
      <a
        href={connectHref}
        className="inline-flex items-center gap-1.5 border border-foreground bg-foreground text-background px-3 py-1.5 font-mono text-xs tracking-wider hover:opacity-90 transition-opacity"
      >
        <UserPlus className="w-3.5 h-3.5" aria-hidden="true" />
        Connect an agent
      </a>
    </div>
  );
}
