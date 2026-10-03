"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Bell, BellOff } from "lucide-react";
import { api } from "@/lib/api";
import type { APIRoomViewerNotifications } from "@/lib/api-types";

/**
 * RoomNotifyToggle (idx 92): the per-room opt-in for replies to you and requested reviews.
 * Off by default; shown only when the API says this viewer can opt in. The state shown is
 * always the API's last answer.
 */
export function RoomNotifyToggle({ slug, initial }: { slug: string; initial: APIRoomViewerNotifications }) {
  const [state, setState] = useState(initial);
  const [busy, setBusy] = useState(false);

  useEffect(() => setState(initial), [initial]);

  if (!state.available) return null;

  const toggle = async () => {
    setBusy(true);
    try {
      const res = await api.setRoomNotifications(slug, !state.subscribed);
      setState({ available: true, subscribed: res.data.subscribed, paused: res.data.paused });
    } catch {
      // The control keeps the state the API last confirmed.
    } finally {
      setBusy(false);
    }
  };

  return (
    <div data-testid="room-notify-toggle" className="flex flex-wrap items-center gap-2 font-mono text-[11px] text-muted-foreground">
      <button
        type="button"
        onClick={toggle}
        disabled={busy}
        className="inline-flex items-center gap-1.5 border border-border px-2.5 py-1 hover:bg-muted transition-colors disabled:opacity-50"
      >
        {state.subscribed ? <BellOff className="w-3 h-3" aria-hidden="true" /> : <Bell className="w-3 h-3" aria-hidden="true" />}
        {state.subscribed ? "Turn off room notifications" : "Notify me about replies"}
      </button>
      {state.subscribed && state.paused && (
        <span>
          Paused for every room — resume in{" "}
          <Link href="/notifications" className="underline underline-offset-2">
            Notifications
          </Link>
          .
        </span>
      )}
    </div>
  );
}
