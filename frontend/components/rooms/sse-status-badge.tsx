"use client";

import type { SseStatus } from '@/hooks/use-room-sse';

export function SseStatusBadge({ status }: { status: SseStatus }) {
  // The initial connecting phase is not an error state — show nothing yet.
  if (status === 'connecting') return null;

  // Retries exhausted: live updates are genuinely gone (API/stream failure).
  // This is deliberately distinct from the transient RECONNECTING state below so
  // a reader can tell a give-up from a temporary blip and knows to reload.
  if (status === 'disconnected') {
    return (
      <div className="flex items-center gap-1.5" role="status">
        <div className="w-2 h-2 bg-red-700 dark:bg-red-500 rounded-full" />
        <span className="font-mono text-[10px] tracking-wider text-red-700 dark:text-red-400">
          LIVE UPDATES OFFLINE — RELOAD TO RECONNECT
        </span>
      </div>
    );
  }

  if (status === 'connected') {
    return (
      <div className="flex items-center gap-1.5">
        <div className="w-2 h-2 bg-green-700 dark:bg-green-500 rounded-full animate-pulse" />
        <span className="font-mono text-[10px] tracking-wider text-green-700 dark:text-green-400">
          LIVE
        </span>
      </div>
    );
  }

  // reconnecting
  return (
    <div className="flex items-center gap-1.5">
      <div className="w-2 h-2 bg-amber-700 dark:bg-amber-500 rounded-full animate-pulse" />
      <span className="font-mono text-[10px] tracking-wider text-amber-700 dark:text-amber-400">
        RECONNECTING...
      </span>
    </div>
  );
}
