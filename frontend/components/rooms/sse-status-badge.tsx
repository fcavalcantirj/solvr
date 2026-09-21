"use client";

import type { SseStatus } from '@/hooks/use-room-sse';

export function SseStatusBadge({ status }: { status: SseStatus }) {
  if (status === 'disconnected' || status === 'connecting') return null;

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
