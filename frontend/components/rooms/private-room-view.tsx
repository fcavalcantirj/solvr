"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import type { APIRoomDetailResponse } from "@/lib/api-types";
import { useAuth } from "@/hooks/use-auth";
import { RoomDetailClient } from "./room-detail-client";

// PrivateRoomView is the CLIENT-side gate for a private room (BART-156). The server page
// can't fetch a private room (SSR carries no JWT → backend 403), so when it sees a 403 it
// renders this instead of a 404. Here we re-fetch with the logged-in human's JWT (attached
// client-side by the api singleton) and hand off to the same RoomDetailClient the public
// path uses. Nothing about a private room is ever server-rendered, so it stays unindexed.
type State =
  | { kind: "loading" }
  | { kind: "needs-login" }
  | { kind: "forbidden" }
  | { kind: "not-found" }
  | { kind: "error" }
  | { kind: "ready"; data: APIRoomDetailResponse["data"] };

export function PrivateRoomView({ slug }: { slug: string }) {
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const [state, setState] = useState<State>({ kind: "loading" });

  useEffect(() => {
    if (authLoading) return;
    if (!isAuthenticated) {
      setState({ kind: "needs-login" });
      return;
    }
    let cancelled = false;
    setState({ kind: "loading" });
    api
      .fetchRoom(slug)
      .then((res) => {
        if (!cancelled) setState({ kind: "ready", data: res.data });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        const status = (err as { statusCode?: number })?.statusCode;
        if (status === 403) setState({ kind: "forbidden" });
        else if (status === 404) setState({ kind: "not-found" });
        else setState({ kind: "error" });
      });
    return () => {
      cancelled = true;
    };
  }, [slug, isAuthenticated, authLoading]);

  if (state.kind === "ready") {
    const { room, agents, recent_messages, owner_display_name } = state.data;
    return (
      <RoomDetailClient
        room={room}
        initialMessages={recent_messages || []}
        initialAgents={agents || []}
        ownerDisplayName={owner_display_name}
      />
    );
  }

  return (
    <div className="flex flex-1 items-start pb-16 pt-10 lg:pt-16">
      <div className="min-w-0 max-w-[60rem]">
        {(state.kind === "loading" || authLoading) && (
          <p className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">Loading room…</p>
        )}
        {state.kind === "needs-login" && (
          <>
            <h1 className="max-w-[16ch] text-[clamp(2.75rem,6vw,6.5rem)] font-light leading-[1.0] tracking-[-0.055em]">This room is private</h1>
            <p className="mb-8 mt-10 max-w-[48ch] border-t border-border pt-5 text-lg font-light leading-snug text-muted-foreground">
              Log in to view this room and post as yourself.
            </p>
            <Link
              href="/login"
              className="inline-flex items-center bg-foreground px-6 py-3 font-mono text-[11px] uppercase tracking-[0.18em] text-background hover:opacity-90"
            >
              Log in
            </Link>
          </>
        )}
        {state.kind === "forbidden" && (
          <>
            <h1 className="max-w-[16ch] text-[clamp(2.75rem,6vw,6.5rem)] font-light leading-[1.0] tracking-[-0.055em]">This room is private</h1>
            <p className="mt-10 max-w-[48ch] border-t border-border pt-5 text-lg font-light leading-snug text-muted-foreground">
              You&apos;re not a member of this room.
            </p>
          </>
        )}
        {state.kind === "not-found" && (
          <>
            <h1 className="max-w-[16ch] text-[clamp(2.75rem,6vw,6.5rem)] font-light leading-[1.0] tracking-[-0.055em]">Room not found</h1>
            <p className="mt-10 max-w-[48ch] border-t border-border pt-5 text-lg font-light leading-snug text-muted-foreground">
              This room doesn&apos;t exist or has been deleted.
            </p>
          </>
        )}
        {state.kind === "error" && (
          <>
            <h1 className="max-w-[16ch] text-[clamp(2.75rem,6vw,6.5rem)] font-light leading-[1.0] tracking-[-0.055em]">Something went wrong</h1>
            <p className="mt-10 max-w-[48ch] border-t border-border pt-5 text-lg font-light leading-snug text-muted-foreground">
              Couldn&apos;t load this room. Try again in a moment.
            </p>
          </>
        )}
      </div>
    </div>
  );
}
